package encounter

import (
	"errors"
	"sync"
)

// InMemoryTransport is a Transport that fans out within a single process,
// suitable for tests and dev. Backed by per-channel goroutines and Go channels.
//
// Not safe to use after Close. Subscribers receive only events published
// after they subscribe (no replay).
type InMemoryTransport struct {
	mu          sync.Mutex
	closed      bool
	subscribers map[string][]chan []byte
}

// NewInMemoryTransport returns an empty transport ready for use.
func NewInMemoryTransport() *InMemoryTransport {
	return &InMemoryTransport{
		subscribers: make(map[string][]chan []byte),
	}
}

// Publish delivers payload to all current subscribers of channel.
//
// Sends are performed under t.mu so that subscription/transport Close calls
// (which also take t.mu before closing the underlying channel) cannot
// interleave with a send and cause a send-on-closed-channel panic. Sends
// are non-blocking (select + default), so holding the lock is bounded.
func (t *InMemoryTransport) Publish(channel string, payload []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return errors.New("transport closed")
	}
	for _, ch := range t.subscribers[channel] {
		select {
		case ch <- payload:
		default:
			// Buffered channel full — drop. Test buffers (size 64) are
			// sized to avoid this in normal use.
		}
	}
	return nil
}

// Subscribe registers a new subscriber for the channel.
func (t *InMemoryTransport) Subscribe(channel string) (TransportSubscription, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil, errors.New("transport closed")
	}
	ch := make(chan []byte, 64)
	t.subscribers[channel] = append(t.subscribers[channel], ch)
	return &inMemSubscription{
		transport: t,
		channel:   channel,
		ch:        ch,
	}, nil
}

// Close terminates all in-flight subscriptions.
func (t *InMemoryTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	for _, subs := range t.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}
	t.subscribers = nil
	return nil
}

type inMemSubscription struct {
	transport *InMemoryTransport
	channel   string
	ch        chan []byte
	once      sync.Once
}

func (s *inMemSubscription) Payloads() <-chan []byte { return s.ch }

func (s *inMemSubscription) Close() error {
	s.once.Do(func() {
		s.transport.mu.Lock()
		defer s.transport.mu.Unlock()
		if s.transport.subscribers == nil {
			return // transport already fully closed
		}
		subs := s.transport.subscribers[s.channel]
		for i, ch := range subs {
			if ch == s.ch {
				s.transport.subscribers[s.channel] = append(subs[:i], subs[i+1:]...)
				close(ch)
				return
			}
		}
	})
	return nil
}
