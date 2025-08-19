// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

// TypedTopicDef defines a typed topic that can be connected to a bus.
// This is created once at package level and used to get typed topics.
type TypedTopicDef[T any] struct {
	topic Topic
}

// On connects this topic definition to a bus, returning a typed topic for pub/sub.
// This is the key pattern: `attacks := combat.AttackTopic.On(bus)`
func (d *TypedTopicDef[T]) On(bus EventBus) TypedTopic[T] {
	return &typedTopic[T]{
		bus:   bus,
		topic: d.topic,
	}
}

// ChainedTopicDef defines a typed topic that supports chain processing.
// This is created once at package level and used to get chained topics.
type ChainedTopicDef[T any] struct {
	topic Topic
}

// On connects this topic definition to a bus, returning a chained topic for pub/sub with chains.
// This enables: `attacks := combat.AttackChain.On(bus)`
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
