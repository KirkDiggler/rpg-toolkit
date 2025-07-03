// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package main demonstrates the conditions system with concentration mechanics
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

// Character represents a simple game entity
type Character struct {
	id   string
	name string
	hp   int
}

// GetID implements Entity interface
func (c *Character) GetID() string { return c.id }

// GetType implements Entity interface
func (c *Character) GetType() string { return "character" }

func main() {
	// Create event bus and relationship manager
	bus := events.NewBus()
	relationshipMgr := conditions.NewRelationshipManager(bus)

	// Create our characters
	cleric := &Character{id: "cleric_1", name: "Alara the Healer", hp: 30}
	fighter := &Character{id: "fighter_1", name: "Thorin Ironforge", hp: 45}
	rogue := &Character{id: "rogue_1", name: "Shadow", hp: 28}
	wizard := &Character{id: "wizard_1", name: "Merlin", hp: 20}
	goblin := &Character{id: "goblin_1", name: "Sneaky Goblin", hp: 15}

	fmt.Println("=== RPG Toolkit: Conditions & Concentration Demo ===")

	// Demo 1: Basic Concentration
	fmt.Println("--- Scenario 1: Cleric casts Bless (Concentration) ---")
	fmt.Printf("%s casts Bless on %s and %s\n", cleric.name, fighter.name, rogue.name)

	// Create blessed conditions
	blessFighter := createBlessCondition(fighter, cleric)
	blessRogue := createBlessCondition(rogue, cleric)

	// Apply the conditions
	if err := blessFighter.Apply(bus); err != nil {
		log.Fatal(err)
	}
	if err := blessRogue.Apply(bus); err != nil {
		log.Fatal(err)
	}

	// Establish concentration relationship
	err := relationshipMgr.CreateRelationship(
		conditions.RelationshipConcentration,
		cleric,
		[]conditions.Condition{blessFighter, blessRogue},
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Simulate attacks with bless
	fmt.Println("\nFighters attack with Bless bonus:")
	simulateAttack(bus, fighter, goblin)
	simulateAttack(bus, rogue, goblin)

	// Demo 2: Breaking Concentration
	fmt.Println("\n--- Scenario 2: Cleric loses concentration ---")
	fmt.Printf("%s takes 15 damage and fails concentration save!\n", cleric.name)
	cleric.hp -= 15

	// Break concentration
	if err := relationshipMgr.BreakAllRelationships(cleric); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFighters attack without Bless:")
	simulateAttack(bus, fighter, goblin)
	simulateAttack(bus, rogue, goblin)

	// Demo 3: Switching Concentration
	fmt.Println("\n--- Scenario 3: Wizard switches concentration spells ---")
	fmt.Printf("%s casts Hold Person on %s (Concentration)\n", wizard.name, goblin.name)

	holdPerson := createHoldCondition(goblin, wizard)
	if err := holdPerson.Apply(bus); err != nil {
		log.Fatal(err)
	}

	err = relationshipMgr.CreateRelationship(
		conditions.RelationshipConcentration,
		wizard,
		[]conditions.Condition{holdPerson},
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s is paralyzed!\n", goblin.name)

	// Switch to a different concentration spell
	fmt.Printf("\n%s decides to cast Haste on %s instead (also Concentration)\n", wizard.name, fighter.name)
	fmt.Println("This automatically breaks concentration on Hold Person!")

	haste := createHasteCondition(fighter, wizard)
	if err := haste.Apply(bus); err != nil {
		log.Fatal(err)
	}

	// This will automatically break the Hold Person
	err = relationshipMgr.CreateRelationship(
		conditions.RelationshipConcentration,
		wizard,
		[]conditions.Condition{haste},
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\n%s is no longer paralyzed!\n", goblin.name)
	fmt.Printf("%s is now hasted!\n", fighter.name)

	// Demo 4: Aura Effects
	fmt.Println("\n--- Scenario 4: Paladin's Aura ---")
	paladin := &Character{id: "paladin_1", name: "Sir Galahad", hp: 50}

	fmt.Printf("%s's Aura of Protection affects allies within 10 feet\n", paladin.name)

	// Create aura conditions for nearby allies
	auraCleric := createAuraCondition(cleric, paladin)
	auraFighter := createAuraCondition(fighter, paladin)

	if err := auraCleric.Apply(bus); err != nil {
		log.Fatal(err)
	}
	if err := auraFighter.Apply(bus); err != nil {
		log.Fatal(err)
	}

	err = relationshipMgr.CreateRelationship(
		conditions.RelationshipAura,
		paladin,
		[]conditions.Condition{auraCleric, auraFighter},
		map[string]any{"range": 10},
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s and %s gain +2 to all saving throws\n", cleric.name, fighter.name)

	// In a real game, UpdateAuras() would be called when entities move
	fmt.Printf("\nIf %s moves more than 10 feet away, they would lose the aura bonus\n", fighter.name)

	fmt.Println("\n=== Demo Complete ===")
}

// Helper function to simulate an attack
func simulateAttack(bus events.EventBus, attacker, target core.Entity) {
	ctx := context.Background()
	attack := events.NewGameEvent(events.EventBeforeAttack, attacker, target)

	// Base attack roll
	baseRoll := dice.D20(1)
	fmt.Printf("%s attacks %s: ", attacker.(*Character).name, target.(*Character).name)

	// Publish the event to gather modifiers
	if err := bus.Publish(ctx, attack); err != nil {
		log.Fatal(err)
	}

	// Get total with modifiers
	total := baseRoll.GetValue()
	for _, mod := range attack.Context().Modifiers() {
		if mod.Type() == events.ModifierAttackBonus {
			fmt.Printf("base %s + %s ", baseRoll.GetDescription(), mod.ModifierValue().GetDescription())
			total += mod.ModifierValue().GetValue()
		}
	}

	if len(attack.Context().Modifiers()) == 0 {
		fmt.Printf("%s", baseRoll.GetDescription())
	}

	fmt.Printf("= %d total\n", total)
}

// Condition creators

func createBlessCondition(target, caster core.Entity) *conditions.SimpleCondition {
	return conditions.NewSimpleCondition(conditions.SimpleConditionConfig{
		ID:     fmt.Sprintf("bless_%s_%s", caster.GetID(), target.GetID()),
		Type:   "blessed",
		Target: target,
		Source: caster.GetID(),
		ApplyFunc: func(c *conditions.SimpleCondition, bus events.EventBus) error {
			c.Subscribe(bus, events.EventBeforeAttack, 50, func(_ context.Context, e events.Event) error {
				if e.Source().GetID() != target.GetID() {
					return nil
				}

				bonus := dice.D4(1)
				e.Context().AddModifier(events.NewModifier(
					"blessed",
					events.ModifierAttackBonus,
					bonus,
					100,
				))

				return nil
			})

			// Also add to saves in a real implementation
			return nil
		},
	})
}

func createHoldCondition(target, caster core.Entity) *conditions.SimpleCondition {
	return conditions.NewSimpleCondition(conditions.SimpleConditionConfig{
		ID:     fmt.Sprintf("hold_%s_%s", caster.GetID(), target.GetID()),
		Type:   "paralyzed",
		Target: target,
		Source: caster.GetID(),
		ApplyFunc: func(_ *conditions.SimpleCondition, _ events.EventBus) error {
			// In a real implementation, would prevent movement and actions
			return nil
		},
		RemoveFunc: func(_ *conditions.SimpleCondition, _ events.EventBus) error {
			// Cleanup when removed
			return nil
		},
	})
}

func createHasteCondition(target, caster core.Entity) *conditions.SimpleCondition {
	return conditions.NewSimpleCondition(conditions.SimpleConditionConfig{
		ID:     fmt.Sprintf("haste_%s_%s", caster.GetID(), target.GetID()),
		Type:   "hasted",
		Target: target,
		Source: caster.GetID(),
		ApplyFunc: func(_ *conditions.SimpleCondition, _ events.EventBus) error {
			// In a real implementation: double speed, +2 AC, advantage on Dex saves, extra action
			return nil
		},
	})
}

func createAuraCondition(target, source core.Entity) *conditions.SimpleCondition {
	return conditions.NewSimpleCondition(conditions.SimpleConditionConfig{
		ID:     fmt.Sprintf("aura_protection_%s_%s", source.GetID(), target.GetID()),
		Type:   "aura_protection",
		Target: target,
		Source: source.GetID(),
		ApplyFunc: func(_ *conditions.SimpleCondition, _ events.EventBus) error {
			// Would add save bonuses
			return nil
		},
	})
}
