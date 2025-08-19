// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// Example_theMagicPattern demonstrates THE pattern that makes this package special.
// Look at how explicit and beautiful the connection is!
func Example_theMagicPattern() {
	// Create the bus - your event highway
	bus := events.NewEventBus()
	ctx := context.Background()

	// Define a topic at compile-time
	var CriticalHitTopic = events.DefineTypedTopic[CriticalHitEvent]("combat.critical")

	// THE MAGIC MOMENT - Connect topic to bus
	// This is what makes our events special!
	crits := CriticalHitTopic.On(bus)

	// Now everything is type-safe and explicit
	_, _ = crits.Subscribe(ctx, func(_ context.Context, e CriticalHitEvent) error {
		fmt.Printf("CRITICAL HIT! %s deals %d damage to %s!\n",
			e.AttackerID, e.Damage, e.TargetID)
		return nil
	})

	// Publish with complete type safety
	_ = crits.Publish(ctx, CriticalHitEvent{
		AttackerID: "rogue",
		TargetID:   "ogre",
		Damage:     42,
	})

	// Output:
	// CRITICAL HIT! rogue deals 42 damage to ogre!
}

// Example_whyOnBusMatters shows why the '.On(bus)' pattern is revolutionary.
func Example_whyOnBusMatters() {
	// You can have multiple buses for different purposes
	combatBus := events.NewEventBus()
	uiBus := events.NewEventBus()
	ctx := context.Background()

	// Same topic definition
	var DamageTopic = events.DefineTypedTopic[SimpleDamageEvent]("damage")

	// EXPLICITLY connect to different buses
	combatDamage := DamageTopic.On(combatBus) // Combat system
	uiDamage := DamageTopic.On(uiBus)         // UI system

	// Each connection is independent
	_, _ = combatDamage.Subscribe(ctx, func(_ context.Context, e SimpleDamageEvent) error {
		fmt.Printf("[COMBAT] Processing %d damage\n", e.Amount)
		return nil
	})

	_, _ = uiDamage.Subscribe(ctx, func(_ context.Context, e SimpleDamageEvent) error {
		fmt.Printf("[UI] Showing damage number: %d\n", e.Amount)
		return nil
	})

	// Publish to combat bus only
	_ = combatDamage.Publish(ctx, SimpleDamageEvent{Amount: 25})

	// Output:
	// [COMBAT] Processing 25 damage
}

// Example_discoverableAPI shows how the pattern makes APIs self-documenting.
func Example_discoverableAPI() {
	bus := events.NewEventBus()

	// Your IDE shows you EXACTLY what's happening at each step:

	// Step 1: You see the topic definition
	var SpellCastTopic = events.DefineTypedTopic[SpellCastEvent]("magic.cast")

	// Step 2: You see the connection (THE MAGIC!)
	spells := SpellCastTopic.On(bus)

	// Step 3: Your IDE shows all available methods
	// spells.Subscribe(...)
	// spells.Publish(...)
	// spells.Unsubscribe(...)

	// Use it to avoid compiler warning
	_ = spells

	// No guessing, no string matching, no runtime errors
	// Just beautiful, explicit, type-safe connections

	fmt.Println("API is self-documenting through IDE autocomplete!")
	// Output:
	// API is self-documenting through IDE autocomplete!
}

// Supporting types for the examples
type CriticalHitEvent struct {
	AttackerID string
	TargetID   string
	Damage     int
}

type SimpleDamageEvent struct {
	Amount int
}

type SpellCastEvent struct {
	CasterID string
	SpellID  string
	TargetID string
}
