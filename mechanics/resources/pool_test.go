// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

func TestSimplePool(t *testing.T) {
	owner := &MockEntity{id: "wizard-1", typ: "character"}
	pool := resources.NewSimplePool(owner)

	// Create spell slots for a 5th level wizard
	spellSlots := resources.CreateSpellSlots(owner, map[int]int{
		1: 4,
		2: 3,
		3: 2,
	})

	// Add resources to pool
	for _, slot := range spellSlots {
		err := pool.Add(slot)
		if err != nil {
			t.Fatalf("Failed to add resource: %v", err)
		}
	}

	// Test retrieval
	slot1, exists := pool.Get("dnd5e:resource:spell_slots_1")
	if !exists {
		t.Fatal("Expected to find dnd5e:resource:spell_slots_1")
	}
	if slot1.Maximum() != 4 {
		t.Errorf("Expected 4 level 1 spell slots, got %d", slot1.Maximum())
	}

	// Test consumption with event bus
	bus := events.NewBus()
	err := pool.Consume("dnd5e:resource:spell_slots_1", 1, bus)
	if err != nil {
		t.Fatalf("Failed to consume spell slot: %v", err)
	}

	if slot1.Current() != 3 {
		t.Errorf("Expected 3 spell slots remaining, got %d", slot1.Current())
	}

	// Test spell slot consumption with upcasting
	err = pool.ConsumeSpellSlot(2, bus)
	if err != nil {
		t.Fatalf("Failed to consume level 2 spell slot: %v", err)
	}

	slot2, _ := pool.Get("dnd5e:resource:spell_slots_2")
	if slot2.Current() != 2 {
		t.Errorf("Expected 2 level 2 slots remaining, got %d", slot2.Current())
	}

	// Test upcasting when lower level not available
	// Consume all level 1 slots
	slot1.SetCurrent(0)

	// Try to cast level 1 spell - should use level 2 slot
	err = pool.ConsumeSpellSlot(1, bus)
	if err != nil {
		t.Fatalf("Failed to upcast spell: %v", err)
	}
	if slot2.Current() != 1 {
		t.Errorf("Expected 1 level 2 slot remaining after upcast, got %d", slot2.Current())
	}
}

func TestPoolRests(t *testing.T) {
	owner := &MockEntity{id: "fighter-1", typ: "character"}
	pool := resources.NewSimplePool(owner)
	bus := events.NewBus()

	// Add Second Wind (short rest) and Action Surge (long rest)
	secondWind := resources.CreateAbilityUse(owner, "second_wind", 1, resources.RestoreShortRest)
	actionSurge := resources.CreateAbilityUse(owner, "action_surge", 1, resources.RestoreLongRest)

	err := pool.Add(secondWind)
	if err != nil {
		t.Fatalf("Failed to add secondWind: %v", err)
	}
	err = pool.Add(actionSurge)
	if err != nil {
		t.Fatalf("Failed to add actionSurge: %v", err)
	}

	// Use both abilities
	err = pool.Consume("dnd5e:resource:second_wind_uses", 1, bus)
	if err != nil {
		t.Fatalf("Failed to consume second_wind: %v", err)
	}
	err = pool.Consume("dnd5e:resource:action_surge_uses", 1, bus)
	if err != nil {
		t.Fatalf("Failed to consume action_surge: %v", err)
	}

	if secondWind.IsAvailable() || actionSurge.IsAvailable() {
		t.Error("Expected both abilities to be used")
	}

	// Short rest - should restore Second Wind only
	pool.ProcessShortRest(bus)

	if !secondWind.IsAvailable() {
		t.Error("Expected Second Wind to be restored on short rest")
	}
	if actionSurge.IsAvailable() {
		t.Error("Expected Action Surge NOT to be restored on short rest")
	}

	// Long rest - should restore both
	err = secondWind.Consume(1) // Use it again
	if err != nil {
		t.Fatalf("Failed to consume second wind again: %v", err)
	}
	pool.ProcessLongRest(bus)

	if !secondWind.IsAvailable() || !actionSurge.IsAvailable() {
		t.Error("Expected both abilities to be restored on long rest")
	}
}

func TestHitDiceRestoration(t *testing.T) {
	owner := &MockEntity{id: "fighter-10", typ: "character"}
	pool := resources.NewSimplePool(owner)

	// Create hit dice for level 10 fighter
	hitDice := resources.CreateHitDice(owner, "d10", 10)
	err := pool.Add(hitDice)
	if err != nil {
		t.Fatalf("Failed to add hit dice: %v", err)
	}

	// Use 6 hit dice
	err = hitDice.Consume(6)
	if err != nil {
		t.Fatalf("Failed to consume hit dice: %v", err)
	}
	if hitDice.Current() != 4 {
		t.Errorf("Expected 4 hit dice remaining, got %d", hitDice.Current())
	}

	// Long rest restores half level (5)
	restoreAmount := hitDice.RestoreOnLongRest()
	if restoreAmount != 5 {
		t.Errorf("Expected to restore 5 hit dice on long rest, got %d", restoreAmount)
	}

	hitDice.Restore(restoreAmount)
	if hitDice.Current() != 9 {
		t.Errorf("Expected 9 hit dice after long rest, got %d", hitDice.Current())
	}
}

func TestResourceByType(t *testing.T) {
	owner := &MockEntity{id: "char-1", typ: "character"}
	pool := resources.NewSimplePool(owner)

	// Add various resource types
	err := pool.Add(resources.CreateAbilityUse(owner, "rage", 3, resources.RestoreLongRest))
	if err != nil {
		t.Fatalf("Failed to add rage resource: %v", err)
	}
	err = pool.Add(resources.CreateAbilityUse(owner, "second_wind", 1, resources.RestoreShortRest))
	if err != nil {
		t.Fatalf("Failed to add second_wind resource: %v", err)
	}
	err = pool.Add(resources.CreateHitDice(owner, "d12", 5))
	if err != nil {
		t.Fatalf("Failed to add hit dice resource: %v", err)
	}

	// Get all ability uses
	abilityUses := pool.GetByType(resources.ResourceTypeAbilityUse)
	if len(abilityUses) != 2 {
		t.Errorf("Expected 2 ability use resources, got %d", len(abilityUses))
	}

	// Get hit dice
	hitDice := pool.GetByType(resources.ResourceTypeHitDice)
	if len(hitDice) != 1 {
		t.Errorf("Expected 1 hit dice resource, got %d", len(hitDice))
	}
}
