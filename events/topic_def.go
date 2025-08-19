// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

// TypedTopicDef defines a typed topic that can be connected to a bus.
// This is created once at package level and used to get typed topics.
//
// THE MAGIC: Topics are defined at compile-time but connected at runtime via '.On(bus)'.
// This separation enables dynamic feature application with complete type safety.
type TypedTopicDef[T any] struct {
	topic Topic
}

// On connects this topic definition to a bus, returning a typed topic for pub/sub.
//
// THIS IS THE MAGIC PATTERN that makes events beautiful:
//
//	attacks := combat.AttackTopic.On(bus)  // SEE the connection
//	attacks.Subscribe(ctx, handleAttack)   // Type-safe from here
//
// The explicit connection makes it crystal clear where events flow.
func (d *TypedTopicDef[T]) On(bus EventBus) TypedTopic[T] {
	return &typedTopic[T]{
		bus:   bus,
		topic: d.topic,
	}
}

// ChainedTopicDef defines a typed topic that supports chain processing.
// This is created once at package level and used to get chained topics.
//
// THE JOURNEY: Events accumulate modifiers as they flow through features.
// Each feature can add its contribution to the chain.
type ChainedTopicDef[T any] struct {
	topic Topic
}

// On connects this topic definition to a bus, returning a chained topic for pub/sub with chains.
//
// THE ACCUMULATION PATTERN in action:
//
//	attacks := combat.AttackChain.On(bus)           // Connect to journey
//	chain, _ := attacks.PublishWithChain(ctx, e, c) // Gather modifiers
//	result, _ := chain.Execute(ctx, e)              // Apply all at once
//
// This enables events to journey through systems, accumulating changes.
func (d *ChainedTopicDef[T]) On(bus EventBus) ChainedTopic[T] {
	return &chainedTopic[T]{
		bus:   bus,
		topic: d.topic,
	}
}

// DefineTypedTopic creates a new typed topic definition.
// The rulebook provides the topic ID to ensure uniqueness.
//
// Example:
//
//	var AttackTopic = events.DefineTypedTopic[AttackEvent]("combat.attack")
func DefineTypedTopic[T any](topic Topic) *TypedTopicDef[T] {
	return &TypedTopicDef[T]{
		topic: topic,
	}
}

// DefineChainedTopic creates a new chained topic definition.
// The rulebook provides the topic ID to ensure uniqueness.
//
// Example:
//
//	var AttackChain = events.DefineChainedTopic[AttackEvent]("combat.attack")
func DefineChainedTopic[T any](topic Topic) *ChainedTopicDef[T] {
	return &ChainedTopicDef[T]{
		topic: topic,
	}
}
