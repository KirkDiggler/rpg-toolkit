// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Example events - just data, no interfaces required!
type AttackEvent struct {
	AttackerID string
	TargetID   string
	Damage     int
	Critical   bool
}

type DamageEvent struct {
	SourceID   string
	TargetID   string
	Amount     int
	DamageType string
}

type LevelUpEvent struct {
	PlayerID string
	NewLevel int
	HP       int
}

// Topic constants - defined by the rulebook for explicit reuse
const (
	TopicLevelUp events.Topic = "player.levelup"
	TopicAttack  events.Topic = "combat.attack"
	TopicDamage  events.Topic = "combat.damage"
)

// Stage constants - defined by the rulebook for processing order
const (
	StageBase       chain.Stage = "base"
	StageFeatures   chain.Stage = "features"
	StageConditions chain.Stage = "conditions"
	StageEquipment  chain.Stage = "equipment"
	StageFinal      chain.Stage = "final"
)

// Define topics at package level using the constants
var (
	// Pure notification topics
	LevelUpTopic = events.DefineTypedTopic[LevelUpEvent](TopicLevelUp)

	// Chained topics for modifier collection
	AttackChain = events.DefineChainedTopic[AttackEvent](TopicAttack)
	DamageChain = events.DefineChainedTopic[DamageEvent](TopicDamage)
)

// Example feature that subscribes to events
type RageFeature struct {
	ownerID string
	bonus   int
}

func (r *RageFeature) Apply(bus events.EventBus) error {
	// The beautiful .On(bus) pattern!
	attacks := AttackChain.On(bus)

	// Subscribe with type safety
	_, err := attacks.SubscribeWithChain(
		context.Background(),
		func(ctx context.Context, e AttackEvent, c chain.Chain[AttackEvent]) (chain.Chain[AttackEvent], error) {
			// Only modify our attacks
			if e.AttackerID == r.ownerID {
				c.Add(StageConditions, "rage-"+r.ownerID, func(ctx context.Context, event AttackEvent) (AttackEvent, error) {
					event.Damage += r.bonus
					return event, nil
				})
			}
			return c, nil
		},
	)

	return err
}

func ExampleTypedTopic() {
	bus := events.NewEventBus()
	ctx := context.Background()

	// Connect topic to bus - crystal clear!
	levelups := LevelUpTopic.On(bus)

	// Subscribe with type safety
	levelups.Subscribe(ctx, func(ctx context.Context, e LevelUpEvent) error {
		fmt.Printf("Player %s reached level %d (HP: %d)\n", e.PlayerID, e.NewLevel, e.HP)
		return nil
	})

	// Publish - compile-time type safe
	levelups.Publish(ctx, LevelUpEvent{
		PlayerID: "hero-123",
		NewLevel: 5,
		HP:       45,
	})

	// Output: Player hero-123 reached level 5 (HP: 45)
}

func ExampleChainedTopic() {
	bus := events.NewEventBus()
	ctx := context.Background()

	// Apply rage feature
	rage := &RageFeature{
		ownerID: "barbarian",
		bonus:   2,
	}
	rage.Apply(bus)

	// Define stages for processing
	stages := []chain.Stage{
		StageBase,
		StageFeatures,
		StageConditions,
		StageEquipment,
		StageFinal,
	}

	// Create attack event and chain
	attack := AttackEvent{
		AttackerID: "barbarian",
		TargetID:   "goblin",
		Damage:     10,
	}

	attackChain := events.NewStagedChain[AttackEvent](stages)

	// Connect and publish
	attacks := AttackChain.On(bus)
	modifiedChain, _ := attacks.PublishWithChain(ctx, attack, attackChain)

	// Execute chain to get result
	result, _ := modifiedChain.Execute(ctx, attack)

	fmt.Printf("Base damage: %d, Final damage: %d\n", attack.Damage, result.Damage)
	// Output: Base damage: 10, Final damage: 12
}

func TestBeautifulAPI(t *testing.T) {
	// This test shows how clean the API is
	bus := events.NewEventBus()

	// Look how clean this is!
	attacks := AttackChain.On(bus)
	levelups := LevelUpTopic.On(bus)
	damage := DamageChain.On(bus)

	// Each typed, each safe, each beautiful
	_ = attacks
	_ = levelups
	_ = damage

	// No strings in sight (except in topic definitions)
	// No interfaces to implement
	// Just data and type safety
}
