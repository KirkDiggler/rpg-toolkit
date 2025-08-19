// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
)

// ChainedTopic provides pub/sub for events that accumulate modifiers on their journey.
//
// THE ACCUMULATION JOURNEY: Events travel through features, each adding its contribution.
// The chain collects all modifiers, then applies them in staged order.
//
// Connect with: attacks := AttackChain.On(bus)
//
// The journey pattern:
// 1. Event starts its journey (base values)
// 2. Features add modifiers as it passes through
// 3. Chain accumulates all contributions
// 4. Execute applies them in proper order
//
// Example:
//
//	attack := AttackEvent{AttackerID: "hero", Damage: 10}
//	chain := NewStagedChain[AttackEvent](stages)
//
//	// Publish to collect modifiers
//	modifiedChain, _ := topic.PublishWithChain(ctx, attack, chain)
//
//	// Execute to get result
//	result, _ := modifiedChain.Execute(ctx, attack)
//	// result.Damage might be 16 (10 + 2 rage + 4 bless)
type ChainedTopic[T any] interface {
	// SubscribeWithChain registers a handler that can add modifiers to the chain.
	//
	// The handler receives:
	// - ctx: Context for the operation
	// - event: The event data (immutable - don't modify directly)
	// - chain: The chain to add modifiers to
	//
	// The handler returns:
	// - chain: The same chain (possibly with modifiers added)
	// - error: Any error that occurred
	//
	// Example handler:
	//
	//	func(ctx context.Context, event AttackEvent, chain chain.Chain[AttackEvent]) (chain.Chain[AttackEvent], error) {
	//	    if event.AttackerID == rage.ownerID {
	//	        chain.Add(StageConditions, "rage", rageModifier)
	//	    }
	//	    return chain, nil
	//	}
	SubscribeWithChain(ctx context.Context,
		handler func(context.Context, T, chain.Chain[T]) (chain.Chain[T], error)) (string, error)

	// PublishWithChain sends the event to all subscribers who may add modifiers to the chain.
	//
	// Parameters:
	// - ctx: Context for the operation
	// - event: The event data (will not be modified)
	// - chain: The chain to collect modifiers into
	//
	// Returns:
	// - chain: The same chain with modifiers added by subscribers
	// - error: Any error from subscribers
	//
	// Important: This returns the CHAIN, not a modified event.
	// The caller already has the event - they need the built-up chain.
	//
	// Usage:
	//
	//	chain := NewStagedChain[AttackEvent](stages)
	//	modifiedChain, _ := topic.PublishWithChain(ctx, attack, chain)
	//	result, _ := modifiedChain.Execute(ctx, attack)
	PublishWithChain(ctx context.Context, event T, chain chain.Chain[T]) (chain.Chain[T], error)

	// Unsubscribe removes a subscription by ID.
	Unsubscribe(ctx context.Context, id string) error
}

// chainedTopic implements ChainedTopic[T]
type chainedTopic[T any] struct {
	bus   EventBus
	topic Topic
}

// chainedEvent wraps an event with its chain for passing through the bus
type chainedEvent[T any] struct {
	ctx   context.Context
	event T
	chain chain.Chain[T]
}

// SubscribeWithChain implements ChainedTopic[T]
func (t *chainedTopic[T]) SubscribeWithChain(ctx context.Context,
	handler func(context.Context, T, chain.Chain[T]) (chain.Chain[T], error)) (string, error) {
	// Wrap the handler to work with our chainedEvent wrapper
	wrappedHandler := func(payload any) error {
		if ce, ok := payload.(*chainedEvent[T]); ok {
			// Call the actual handler with the chain
			newChain, err := handler(ce.ctx, ce.event, ce.chain)
			if err != nil {
				return err
			}
			// Update the chain in place so PublishWithChain sees the changes
			ce.chain = newChain
		}
		return nil
	}

	return t.bus.Subscribe(ctx, t.topic, wrappedHandler)
}

// PublishWithChain implements ChainedTopic[T]
func (t *chainedTopic[T]) PublishWithChain(ctx context.Context, event T, chain chain.Chain[T]) (chain.Chain[T], error) {
	// Create wrapper that carries both event and chain
	ce := &chainedEvent[T]{
		ctx:   ctx,
		event: event,
		chain: chain,
	}

	// Publish through bus - handlers will modify ce.chain
	err := t.bus.Publish(ctx, t.topic, ce)
	if err != nil {
		return chain, err
	}

	// Return the modified chain
	return ce.chain, nil
}

// Unsubscribe implements ChainedTopic[T]
func (t *chainedTopic[T]) Unsubscribe(ctx context.Context, id string) error {
	return t.bus.Unsubscribe(ctx, id)
}
