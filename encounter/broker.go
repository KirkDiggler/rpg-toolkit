package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/types"
)

// Broker is the process-scoped event router. Encounters publish through it;
// gRPC stream handlers subscribe through it. Internally uses a Transport
// to distribute event bytes (locally and, in future, across processes).
type Broker struct {
	transport Transport

	mu          sync.Mutex
	subscribers map[subscriberKey][]*Subscription
	listeners   map[types.EncounterID]TransportSubscription
	closed      bool
}

type subscriberKey struct {
	EncID    types.EncounterID
	PlayerID types.PlayerID
}

// NewBroker constructs a Broker over the given Transport.
func NewBroker(t Transport) *Broker {
	return &Broker{
		transport:   t,
		subscribers: make(map[subscriberKey][]*Subscription),
		listeners:   make(map[types.EncounterID]TransportSubscription),
	}
}

// Publish encodes the event and writes it to the encounter's transport channel.
// Encounters call this from inside verb methods.
func (b *Broker) Publish(evt events.EncounterEvent) error {
	payload, err := encodeEvent(evt)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	return b.transport.Publish(channelFor(evt.EncounterID()), payload)
}

// Subscribe registers a per-player subscription. The returned Subscription
// delivers only events whose Audience contains playerID. Subscriptions
// outlive any single Encounter object — the transport channel is the spine.
func (b *Broker) Subscribe(encID types.EncounterID, playerID types.PlayerID) (*Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errors.New("broker closed")
	}

	sub := &Subscription{
		broker:   b,
		encID:    encID,
		playerID: playerID,
		events:   make(chan events.EncounterEvent, 64),
	}
	key := subscriberKey{EncID: encID, PlayerID: playerID}
	b.subscribers[key] = append(b.subscribers[key], sub)

	// First subscriber for this encounter starts the listener goroutine.
	if _, ok := b.listeners[encID]; !ok {
		ts, err := b.transport.Subscribe(channelFor(encID))
		if err != nil {
			return nil, fmt.Errorf("transport subscribe: %w", err)
		}
		b.listeners[encID] = ts
		go b.listen(encID, ts)
	}
	return sub, nil
}

// Close stops all listeners and closes all subscriptions. Idempotent.
func (b *Broker) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	listeners := b.listeners
	subs := b.subscribers
	b.listeners = nil
	b.subscribers = nil
	b.mu.Unlock()

	// Close transport listeners first — listener goroutines exit when their
	// input channels close.
	for _, ts := range listeners {
		_ = ts.Close()
	}
	// Close subscriptions after listeners. Subscription.Close is idempotent
	// (sync.Once guards the channel close).
	for _, list := range subs {
		for _, sub := range list {
			_ = sub.Close()
		}
	}
	return nil
}

// listen runs one goroutine per encounter the broker is aware of. Decodes
// events and fans out to per-player subscribers in the audience.
//
// Subscribers are snapshotted under lock; channel sends happen OUTSIDE the
// lock so a slow subscriber can't stall the listener.
func (b *Broker) listen(encID types.EncounterID, ts TransportSubscription) {
	for payload := range ts.Payloads() {
		evt, err := decodeEvent(payload)
		if err != nil {
			// Malformed payload — skip. Future: surface a metric.
			continue
		}

		b.mu.Lock()
		var targets []*Subscription
		for _, playerID := range evt.Audience() {
			targets = append(targets, b.subscribers[subscriberKey{EncID: encID, PlayerID: playerID}]...)
		}
		b.mu.Unlock()

		for _, sub := range targets {
			select {
			case sub.events <- evt:
			default:
				// Subscriber buffer full — drop. Tests size buffers high enough.
			}
		}
	}
}

// channelFor returns the transport channel key for an encounter.
func channelFor(encID types.EncounterID) string {
	return "enc:" + string(encID)
}

// Subscription is a per-player view onto an encounter's event stream.
type Subscription struct {
	broker   *Broker
	encID    types.EncounterID
	playerID types.PlayerID
	events   chan events.EncounterEvent
	once     sync.Once
}

// Events returns the channel of events delivered to this player.
func (s *Subscription) Events() <-chan events.EncounterEvent { return s.events }

// Close removes this subscription from the broker registry and closes
// the events channel. Idempotent — only the Subscription owns its close,
// guarded by sync.Once. Broker.Close() calls this; nobody else closes
// s.events directly.
func (s *Subscription) Close() error {
	s.once.Do(func() {
		s.broker.mu.Lock()
		if s.broker.subscribers != nil {
			key := subscriberKey{EncID: s.encID, PlayerID: s.playerID}
			subs := s.broker.subscribers[key]
			for i, sub := range subs {
				if sub == s {
					s.broker.subscribers[key] = append(subs[:i], subs[i+1:]...)
					break
				}
			}
		}
		s.broker.mu.Unlock()
		close(s.events)
	})
	return nil
}

// --- Codec (private to broker) ---
//
// Wire format is JSON with a top-level "_type" discriminator. Concrete event
// types do not see it. Future Transport implementations can substitute a
// different codec without changing event types.

type wireEnvelope struct {
	Type    string          `json:"_type"`
	Payload json.RawMessage `json:"payload"`
}

func encodeEvent(evt events.EncounterEvent) ([]byte, error) {
	var typeName string
	switch evt.(type) {
	case *events.MoveEvent:
		typeName = "MoveEvent"
	case *events.HexRevealedEvent:
		typeName = "HexRevealedEvent"
	case *events.DoorOpenedEvent:
		typeName = "DoorOpenedEvent"
	default:
		return nil, fmt.Errorf("unknown event type %T", evt)
	}
	payload, err := json.Marshal(evt) // each concrete provides MarshalJSON
	if err != nil {
		return nil, err
	}
	return json.Marshal(wireEnvelope{Type: typeName, Payload: payload})
}

func decodeEvent(b []byte) (events.EncounterEvent, error) {
	var env wireEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	switch env.Type {
	case "MoveEvent":
		var e events.MoveEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "HexRevealedEvent":
		var e events.HexRevealedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "DoorOpenedEvent":
		var e events.DoorOpenedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	default:
		return nil, fmt.Errorf("unknown event type %q", env.Type)
	}
}
