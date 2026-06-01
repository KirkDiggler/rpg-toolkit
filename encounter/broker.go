package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
)

// Broker is the process-scoped event router. Encounters publish through it;
// gRPC stream handlers subscribe through it. Internally uses a Transport
// to distribute event bytes (locally and, in future, across processes).
type Broker struct {
	transport Transport
	clock     core.Clock

	mu          sync.Mutex
	subscribers map[subscriberKey][]*Subscription
	listeners   map[core.EncounterID]TransportSubscription
	listenerWG  sync.WaitGroup // tracks listener goroutines for clean shutdown
	closed      bool
}

type subscriberKey struct {
	EncID    core.EncounterID
	PlayerID core.PlayerID
}

// NewBroker constructs a Broker over the given Transport, stamping events with
// the system wall clock at publish. Use NewBrokerWithClock to inject a
// deterministic clock in tests.
func NewBroker(t Transport) *Broker {
	return NewBrokerWithClock(t, core.SystemClock{})
}

// NewBrokerWithClock constructs a Broker that stamps each published event's
// game-event time (Invariant 5) from the supplied clock. A nil clock falls
// back to the system wall clock.
func NewBrokerWithClock(t Transport, clock core.Clock) *Broker {
	if clock == nil {
		clock = core.SystemClock{}
	}
	return &Broker{
		transport:   t,
		clock:       clock,
		subscribers: make(map[subscriberKey][]*Subscription),
		listeners:   make(map[core.EncounterID]TransportSubscription),
	}
}

// Publish stamps the event with game-event time at the literal publish moment
// (Invariant 5), preserving any correlation id the encounter set on it
// (Invariant 8), then encodes and writes it to the encounter's transport
// channel. The broker is the single publish authority, so stamping here makes
// "game-event time at publish" literal for every event with no per-call-site
// boilerplate. Encounters call this from inside verb methods.
func (b *Broker) Publish(evt events.EncounterEvent) error {
	evt.Stamp(b.clock.Now(), evt.CorrelationID())
	payload, err := encodeEvent(evt)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	return b.transport.Publish(channelFor(evt.EncounterID()), payload)
}

// Subscribe registers a per-player subscription. The returned Subscription
// delivers only events whose Audience contains playerID. Subscriptions
// outlive any single Encounter object — the transport channel is the spine.
//
// If this is the first subscriber for the encounter, the transport
// subscription is acquired BEFORE the broker registry is mutated — so a
// transport.Subscribe failure leaves the broker state untouched (no leaked
// orphan subscription).
func (b *Broker) Subscribe(encID core.EncounterID, playerID core.PlayerID) (*Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errors.New("broker closed")
	}

	// Acquire transport subscription first if this encounter doesn't have a
	// listener yet. If this errors, no broker state has been mutated.
	var (
		newTS      TransportSubscription
		startedNew bool
	)
	if _, ok := b.listeners[encID]; !ok {
		ts, err := b.transport.Subscribe(channelFor(encID))
		if err != nil {
			return nil, fmt.Errorf("transport subscribe: %w", err)
		}
		newTS = ts
		startedNew = true
	}

	sub := &Subscription{
		broker:   b,
		encID:    encID,
		playerID: playerID,
		events:   make(chan events.EncounterEvent, 64),
	}
	key := subscriberKey{EncID: encID, PlayerID: playerID}
	b.subscribers[key] = append(b.subscribers[key], sub)

	if startedNew {
		b.listeners[encID] = newTS
		b.listenerWG.Add(1)
		go b.listen(encID, newTS)
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
	// Wait for listener goroutines to fully exit before closing subscription
	// channels. Without this, a listener mid-send to sub.events races with
	// Subscription.Close closing it.
	b.listenerWG.Wait()
	// Close subscriptions. Subscription.Close is idempotent (sync.Once guards
	// the channel close).
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
// Sends are performed under b.mu so that Subscription.Close (which also
// takes b.mu before closing sub.events) cannot interleave with a send and
// cause a send-on-closed-channel panic. Sends are non-blocking (select +
// default), so holding the lock during fanout is bounded.
func (b *Broker) listen(encID core.EncounterID, ts TransportSubscription) {
	defer b.listenerWG.Done()
	for payload := range ts.Payloads() {
		evt, err := decodeEvent(payload)
		if err != nil {
			// Malformed payload — skip. Future: surface a metric.
			continue
		}

		b.mu.Lock()
		for _, playerID := range evt.Audience() {
			for _, sub := range b.subscribers[subscriberKey{EncID: encID, PlayerID: playerID}] {
				select {
				case sub.events <- evt:
				default:
					// Subscriber buffer full — drop. Tests size buffers
					// high enough that this doesn't trip in normal use.
				}
			}
		}
		b.mu.Unlock()
	}
}

// channelFor returns the transport channel key for an encounter.
func channelFor(encID core.EncounterID) string {
	return "enc:" + string(encID)
}

// Subscription is a per-player view onto an encounter's event stream.
type Subscription struct {
	broker   *Broker
	encID    core.EncounterID
	playerID core.PlayerID
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
// different codec without changing event core.

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
	case *events.EntityAppearedEvent:
		typeName = "EntityAppearedEvent"
	case *events.EntityDisappearedEvent:
		typeName = "EntityDisappearedEvent"
	case *events.ActionResolvedEvent:
		typeName = "ActionResolvedEvent"
	case *events.AttackResolvedEvent:
		typeName = "AttackResolvedEvent"
	case *events.DamageDealtEvent:
		typeName = "DamageDealtEvent"
	case *events.ConditionAppliedEvent:
		typeName = "ConditionAppliedEvent"
	case *events.ModeChangedEvent:
		typeName = "ModeChangedEvent"
	case *events.TurnStartedEvent:
		typeName = "TurnStartedEvent"
	case *events.TurnEndedEvent:
		typeName = "TurnEndedEvent"
	case *events.EntityDiedEvent:
		typeName = "EntityDiedEvent"
	case *events.EntityRemovedEvent:
		typeName = "EntityRemovedEvent"
	case *events.EncounterEndedEvent:
		typeName = "EncounterEndedEvent"
	case *events.InputRequiredDeliveredEvent:
		typeName = "InputRequiredDeliveredEvent"
	case *events.ResourceChangedEvent:
		typeName = "ResourceChangedEvent"
	default:
		return nil, fmt.Errorf("unknown event type %T", evt)
	}
	payload, err := json.Marshal(evt) // each concrete provides MarshalJSON
	if err != nil {
		return nil, err
	}
	return json.Marshal(wireEnvelope{Type: typeName, Payload: payload})
}

// decodeEvent is a flat dispatcher over event types — complexity grows
// linearly with the event taxonomy. Adding a new event variant adds one
// case + one corresponding case in encodeEvent. Splitting by category
// would not reduce overall complexity, just relocate it.
//
//nolint:gocyclo // flat type dispatcher; complexity = event-taxonomy size
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
	case "EntityAppearedEvent":
		var e events.EntityAppearedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "EntityDisappearedEvent":
		var e events.EntityDisappearedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "ActionResolvedEvent":
		var e events.ActionResolvedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "AttackResolvedEvent":
		var e events.AttackResolvedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "DamageDealtEvent":
		var e events.DamageDealtEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "ConditionAppliedEvent":
		var e events.ConditionAppliedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "ModeChangedEvent":
		var e events.ModeChangedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "TurnStartedEvent":
		var e events.TurnStartedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "TurnEndedEvent":
		var e events.TurnEndedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "EntityDiedEvent":
		var e events.EntityDiedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "EntityRemovedEvent":
		var e events.EntityRemovedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "EncounterEndedEvent":
		var e events.EncounterEndedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "InputRequiredDeliveredEvent":
		var e events.InputRequiredDeliveredEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	case "ResourceChangedEvent":
		var e events.ResourceChangedEvent
		if err := json.Unmarshal(env.Payload, &e); err != nil {
			return nil, err
		}
		return &e, nil
	default:
		return nil, fmt.Errorf("unknown event type %q", env.Type)
	}
}
