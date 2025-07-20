// Package events provides a game-agnostic event bus for loose coupling between
// toolkit components and game systems without requiring direct dependencies.
//
// Purpose:
// This package enables components to communicate without direct dependencies,
// supporting observable and extensible game systems through event-driven
// architecture. It allows the toolkit to remain decoupled while still
// coordinating complex interactions.
//
// Scope:
//   - Event bus implementation with pub/sub pattern
//   - Event interface and base types
//   - Typed event support with generics
//   - Event filtering and routing capabilities
//   - Synchronous event delivery (same goroutine)
//   - Event metadata and correlation
//   - No game-specific event types
//
// Non-Goals:
//   - Game event definitions: Define these in your game implementation
//   - Event persistence: Use external storage if needed
//   - Network transport: This is for in-process events only
//   - Async delivery: Events are delivered synchronously
//   - Event ordering guarantees: No order guarantees between subscribers
//   - Event replay: No built-in event sourcing
//   - Dead letter handling: Failed handlers are logged, not retried
//
// Integration:
// This package is used throughout the toolkit for:
//   - spatial: Movement and room transition events
//   - behavior: AI decision and state change events
//   - spawn: Entity placement events
//   - selectables: Selection made events
//
// Games subscribe to toolkit events and publish their own domain events.
// This creates a clean boundary between infrastructure and game logic.
//
// Example:
//
//	bus := events.NewBus()
//
//	// Subscribe to toolkit events
//	bus.Subscribe("entity.moved", func(e events.Event) {
//	    data := e.Data.(MovedEventData)
//	    fmt.Printf("Entity %s moved from %v to %v\n",
//	        data.EntityID, data.From, data.To)
//	})
//
//	// Subscribe with typed events
//	bus.SubscribeTyped(func(e events.TypedEvent[MovedEventData]) {
//	    fmt.Printf("Typed: Entity %s moved\n", e.Data.EntityID)
//	})
//
//	// Publish from toolkit components
//	bus.Publish(events.New("entity.moved", MovedEventData{
//	    EntityID: "goblin-1",
//	    From:     Position{10, 10},
//	    To:       Position{15, 10},
//	}))
package events
