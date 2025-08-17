// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/damage"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// ExampleRageIntegration demonstrates the full rage workflow with conditions
func ExampleRageIntegration() {
	ctx := context.Background()
	bus := events.NewBus()
	
	// Create a condition manager
	conditionManager := conditions.NewMemoryManager(bus)
	
	// Create a level 5 barbarian with 3 rage uses
	barbarian := &mockEntity{
		id:         "barbarian_001", 
		entityType: dnd5e.EntityTypeCharacter,
	}
	
	// Create the rage feature
	rage := features.NewRageV2("rage_001", 3, 5, bus)
	
	// Activate rage
	input := features.RageV2Input{
		ConditionManager: conditionManager,
	}
	
	if err := rage.Activate(ctx, barbarian, input); err != nil {
		fmt.Printf("Failed to activate rage: %v\n", err)
		return
	}
	
	fmt.Println("Rage activated!")
	
	// Check if raging
	if hasRage, _ := conditionManager.HasCondition(ctx, barbarian, conditions.Raging); hasRage {
		fmt.Println("Barbarian is raging!")
	}
	
	// Simulate combat - barbarian attacks
	attack := dnd5e.NewAttackEvent(
		barbarian, 
		&mockEntity{id: "goblin_001", entityType: dnd5e.EntityTypeCharacter},
		true,  // melee
		dnd5e.AbilityStrength,
		8, // base damage
	)
	
	// The rage condition handler will automatically add damage bonus
	_ = bus.Publish(attack)
	
	// Get the modified damage (would be 8 + 2 = 10 for level 5)
	finalDamage := attack.Context().ApplyModifiers(float64(attack.Damage))
	fmt.Printf("Attack damage with rage: %.0f\n", finalDamage)
	
	// Simulate taking damage - resistance should apply
	damageEvent := dnd5e.NewDamageReceivedEvent(
		barbarian,
		&mockEntity{id: "orc_001", entityType: dnd5e.EntityTypeCharacter},
		20, // incoming damage
		damage.TypeSlashing,
	)
	
	// The rage condition handler will apply resistance
	_ = bus.Publish(damageEvent)
	
	// Get the modified damage (would be 20 * 0.5 = 10 with resistance)
	finalIncoming := damageEvent.Context().ApplyModifiers(float64(damageEvent.Amount))
	fmt.Printf("Damage taken with resistance: %.0f\n", finalIncoming)
	
	// Simulate time passing - tick rounds
	for round := 1; round <= 10; round++ {
		output, _ := conditionManager.TickDuration(ctx, &conditions.TickDurationInput{
			Target:       barbarian,
			DurationType: conditions.DurationRounds,
			Amount:       1,
		})
		
		if len(output.ExpiredConditions) > 0 {
			fmt.Printf("Rage ended after round %d\n", round)
			break
		}
	}
	
	// Output:
	// Rage activated!
	// Barbarian is raging!
	// Attack damage with rage: 10
	// Damage taken with resistance: 10
	// Rage ended after round 10
}

// TestCombatWithRage demonstrates rage in a combat scenario
func TestCombatWithRage(t *testing.T) {
	ctx := context.Background()
	bus := events.NewBus()
	conditionManager := conditions.NewMemoryManager(bus)
	
	// Create combatants
	barbarian := &mockEntity{id: "barbarian_001", entityType: dnd5e.EntityTypeCharacter}
	orc := &mockEntity{id: "orc_001", entityType: dnd5e.EntityTypeCharacter}
	
	// Create rage feature for level 9 barbarian (3 damage bonus)
	rage := features.NewRageV2("rage_001", 4, 9, bus)
	
	// Track events
	var attackDamage float64
	var receivedDamage float64
	
	// Subscribe to events to capture results
	bus.Subscribe(dnd5e.EventRefAttack, func(e interface{}) error {
		if attack, ok := e.(*dnd5e.AttackEvent); ok {
			attackDamage = attack.Context().ApplyModifiers(float64(attack.Damage))
		}
		return nil
	})
	
	bus.Subscribe(dnd5e.EventRefDamageReceived, func(e interface{}) error {
		if damage, ok := e.(*dnd5e.DamageReceivedEvent); ok {
			receivedDamage = damage.Context().ApplyModifiers(float64(damage.Amount))
		}
		return nil
	})
	
	// Activate rage
	input := features.RageV2Input{
		ConditionManager: conditionManager,
	}
	
	err := rage.Activate(ctx, barbarian, input)
	if err != nil {
		t.Fatalf("Failed to activate rage: %v", err)
	}
	
	// Barbarian attacks with a greataxe (1d12, using 7 as base)
	attack := dnd5e.NewAttackEvent(barbarian, orc, true, dnd5e.AbilityStrength, 7)
	bus.Publish(attack)
	
	// Should be 7 + 3 (level 9 rage bonus) = 10
	if attackDamage != 10 {
		t.Errorf("Expected attack damage to be 10, got %.0f", attackDamage)
	}
	
	// Orc retaliates with 15 slashing damage
	damageEvent := dnd5e.NewDamageReceivedEvent(barbarian, orc, 15, damage.TypeSlashing)
	bus.Publish(damageEvent)
	
	// Should be 15 * 0.5 (resistance) = 7.5
	if receivedDamage != 7.5 {
		t.Errorf("Expected received damage to be 7.5, got %.0f", receivedDamage)
	}
	
	// Test that magic damage isn't resisted
	magicDamage := dnd5e.NewDamageReceivedEvent(barbarian, orc, 10, damage.TypeFire)
	bus.Publish(magicDamage)
	
	// Should be 10 (no resistance to fire)
	if receivedDamage != 10 {
		t.Errorf("Expected fire damage to be 10 (no resistance), got %.0f", receivedDamage)
	}
}

// TestPersistenceAndReload demonstrates loading a character with an active rage condition
func TestPersistenceAndReload(t *testing.T) {
	ctx := context.Background()
	
	// Simulate saving a character's conditions
	savedConditions := []conditions.Condition{
		{
			Type:         conditions.Raging,
			Source:       "rage_feature",
			SourceEntity: "rage_001",
			DurationType: conditions.DurationRounds,
			Remaining:    7, // 7 rounds left
			Metadata: map[string]interface{}{
				"barbarian_level": 5,
				"feature_id":      "rage_001",
			},
		},
	}
	
	// Simulate loading the character
	bus := events.NewBus()
	conditionManager := conditions.NewMemoryManager(bus)
	barbarian := &mockEntity{id: "barbarian_001", entityType: dnd5e.EntityTypeCharacter}
	
	// Reapply saved conditions
	for _, condition := range savedConditions {
		_, err := conditionManager.ApplyCondition(ctx, &conditions.ApplyConditionInput{
			Target:    barbarian,
			Condition: condition,
			EventBus:  bus,
		})
		if err != nil {
			t.Fatalf("Failed to reapply condition: %v", err)
		}
	}
	
	// Verify rage is active
	hasRage, err := conditionManager.HasCondition(ctx, barbarian, conditions.Raging)
	if err != nil {
		t.Fatalf("Failed to check condition: %v", err)
	}
	if !hasRage {
		t.Error("Expected barbarian to be raging after reload")
	}
	
	// Verify the condition still provides benefits
	var attackDamage float64
	bus.Subscribe(dnd5e.EventRefAttack, func(e interface{}) error {
		if attack, ok := e.(*dnd5e.AttackEvent); ok {
			attackDamage = attack.Context().ApplyModifiers(float64(attack.Damage))
		}
		return nil
	})
	
	// Test that rage bonus still applies
	attack := dnd5e.NewAttackEvent(
		barbarian, 
		&mockEntity{id: "goblin_001", entityType: dnd5e.EntityTypeCharacter},
		true, 
		dnd5e.AbilityStrength, 
		5,
	)
	bus.Publish(attack)
	
	// Should be 5 + 2 (level 5 rage bonus) = 7
	if attackDamage != 7 {
		t.Errorf("Expected attack damage to be 7 after reload, got %.0f", attackDamage)
	}
	
	// Verify duration tracking continues
	output, err := conditionManager.TickDuration(ctx, &conditions.TickDurationInput{
		Target:       barbarian,
		DurationType: conditions.DurationRounds,
		Amount:       7, // Use up remaining rounds
	})
	
	if err != nil {
		t.Fatalf("Failed to tick duration: %v", err)
	}
	
	if len(output.ExpiredConditions) != 1 {
		t.Error("Expected rage to expire after 7 rounds")
	}
	
	// Verify rage is no longer active
	hasRage, _ = conditionManager.HasCondition(ctx, barbarian, conditions.Raging)
	if hasRage {
		t.Error("Expected rage to be expired")
	}
}