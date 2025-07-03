// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package main demonstrates a simple combat flow using the event-driven architecture.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// SimpleEntity is a minimal entity implementation for our example
type SimpleEntity struct {
	id   string
	name string
}

func (e *SimpleEntity) GetID() string   { return e.id }
func (e *SimpleEntity) GetType() string { return "creature" }
func (e *SimpleEntity) Name() string    { return e.name }

func main() {
	// Create event bus
	bus := events.NewBus()

	// Create our combatants
	barbarian := &SimpleEntity{id: "barb1", name: "Ragnar the Bold"}
	goblin := &SimpleEntity{id: "gob1", name: "Sneaky Goblin"}

	// Register our combat event handlers
	registerCombatHandlers(bus)

	// Register condition handlers
	registerRageHandler(bus)      // Rage adds damage
	registerBlessedHandler(bus)    // Blessed adds to attack rolls

	fmt.Println("=== Simple Combat Example ===")
	fmt.Println()
	fmt.Printf("%s attacks %s with a Greatsword!\n\n", barbarian.Name(), goblin.Name())

	// Create attack event
	attackEvent := events.NewGameEvent(events.EventBeforeAttack, barbarian, goblin)
	attackEvent.Context().Set("weapon", "greatsword")
	attackEvent.Context().Set("is_raging", true)
	attackEvent.Context().Set("is_blessed", true)

	// Fire the attack event chain
	if err := bus.Publish(context.Background(), attackEvent); err != nil {
		log.Fatalf("Failed to publish attack event: %v", err)
	}

	fmt.Println("\n=== Combat Complete ===")
}

func registerCombatHandlers(bus *events.Bus) {
	// Handle the attack flow
	bus.SubscribeFunc(events.EventBeforeAttack, 100, func(ctx context.Context, e events.Event) error {
		fmt.Println("1. Attack Roll Phase")
		fmt.Printf("   Attacker: %s\n", e.Source().(*SimpleEntity).Name())
		fmt.Printf("   Target: %s\n", e.Target().(*SimpleEntity).Name())

		// Roll base attack
		attackRoll := dice.D20(1)
		attackTotal := attackRoll.GetValue()
		fmt.Printf("   Base Roll: %s\n", attackRoll.GetDescription())

		// Apply modifiers
		for _, mod := range e.Context().Modifiers() {
			if mod.Type() == events.ModifierAttackBonus {
				fmt.Printf("   + %s: %s\n", mod.Source(), mod.ModifierValue().GetDescription())
				attackTotal += mod.ModifierValue().GetValue()
			}
		}

		fmt.Printf("   Total: %d vs AC 15\n", attackTotal)

		if attackTotal >= 15 {
			fmt.Println("   Result: HIT!")
			fmt.Println()
			
			// Trigger damage calculation
			damageEvent := events.NewGameEvent(events.EventCalculateDamage, e.Source(), e.Target())
			weapon, _ := e.Context().Get("weapon")
			damageEvent.Context().Set("weapon", weapon)
			
			// Copy over condition flags
			if isRaging, ok := e.Context().Get("is_raging"); ok {
				damageEvent.Context().Set("is_raging", isRaging)
			}
			
			return bus.Publish(ctx, damageEvent)
		}

		fmt.Println("   Result: MISS!")
		return nil
	})

	// Handle damage calculation
	bus.SubscribeFunc(events.EventCalculateDamage, 100, func(ctx context.Context, e events.Event) error {
		fmt.Println("2. Damage Calculation Phase")
		
		// Base weapon damage (2d6 for greatsword)
		baseDamage := dice.D6(2)
		totalDamage := baseDamage.GetValue()
		fmt.Printf("   Base Damage: %s\n", baseDamage.GetDescription())

		// Apply damage modifiers
		for _, mod := range e.Context().Modifiers() {
			if mod.Type() == events.ModifierDamageBonus {
				fmt.Printf("   + %s: %s\n", mod.Source(), mod.ModifierValue().GetDescription())
				totalDamage += mod.ModifierValue().GetValue()
			}
		}

		fmt.Printf("   Total Damage: %d slashing\n\n", totalDamage)

		// Apply the damage
		damageApplied := events.NewGameEvent(events.EventAfterDamage, e.Source(), e.Target())
		damageApplied.Context().Set("damage", totalDamage)
		damageApplied.Context().Set("damage_type", "slashing")
		
		return bus.Publish(ctx, damageApplied)
	})

	// Handle damage application
	bus.SubscribeFunc(events.EventAfterDamage, 100, func(ctx context.Context, e events.Event) error {
		fmt.Println("3. Damage Applied Phase")
		
		damage, _ := e.Context().Get("damage")
		damageType, _ := e.Context().Get("damage_type")
		
		fmt.Printf("   %s takes %d %s damage!\n", 
			e.Target().(*SimpleEntity).Name(), damage.(int), damageType.(string))
		
		return nil
	})
}

func registerRageHandler(bus *events.Bus) {
	// Rage adds +2 damage during damage calculation
	bus.SubscribeFunc(events.EventCalculateDamage, 50, func(ctx context.Context, e events.Event) error {
		// Check if the attacker is raging
		if isRaging, ok := e.Context().Get("is_raging"); ok && isRaging.(bool) {
			e.Context().AddModifier(events.NewModifier(
				"rage",
				events.ModifierDamageBonus,
				events.NewRawValue(2, "rage bonus"),
				100,
			))
		}
		return nil
	})
}

func registerBlessedHandler(bus *events.Bus) {
	// Blessed adds d4 to attack rolls
	bus.SubscribeFunc(events.EventBeforeAttack, 50, func(ctx context.Context, e events.Event) error {
		// Check if the attacker is blessed
		if isBlessed, ok := e.Context().Get("is_blessed"); ok && isBlessed.(bool) {
			e.Context().AddModifier(events.NewModifier(
				"blessed",
				events.ModifierAttackBonus,
				dice.D4(1),
				100,
			))
		}
		return nil
	})
}