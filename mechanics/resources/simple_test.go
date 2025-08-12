// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// MockEntity for testing
type MockEntity struct {
	id  string
	typ string
}

func (e *MockEntity) GetID() string   { return e.id }
func (e *MockEntity) GetType() string { return e.typ }

func TestSimpleResource(t *testing.T) {
	owner := &MockEntity{id: "char-1", typ: "character"}

	// Create a spell slot resource
	spellSlot := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:              "spell-slot-3",
		Type:            resources.ResourceTypeSpellSlot,
		Owner:           owner,
		Key:             core.MustNewRef(core.RefInput{
			Module: "core",
			Type:   "spell_slot",
			Value:  "level_3",
		}),
		Current:         2,
		Maximum:         3,
		RestoreType:     resources.RestoreLongRest,
		LongRestRestore: -1, // Full restore
	})

	// Test initial state
	if spellSlot.GetID() != "spell-slot-3" {
		t.Errorf("Expected ID 'spell-slot-3', got %s", spellSlot.GetID())
	}
	if spellSlot.Current() != 2 {
		t.Errorf("Expected current 2, got %d", spellSlot.Current())
	}
	if spellSlot.Maximum() != 3 {
		t.Errorf("Expected maximum 3, got %d", spellSlot.Maximum())
	}
	if !spellSlot.IsAvailable() {
		t.Error("Expected resource to be available")
	}

	// Test consumption
	err := spellSlot.Consume(1)
	if err != nil {
		t.Fatalf("Failed to consume resource: %v", err)
	}
	if spellSlot.Current() != 1 {
		t.Errorf("Expected current 1 after consumption, got %d", spellSlot.Current())
	}

	// Test over-consumption
	err = spellSlot.Consume(2)
	if err == nil {
		t.Error("Expected error when consuming more than available")
	}

	// Test restoration
	spellSlot.Restore(1)
	if spellSlot.Current() != 2 {
		t.Errorf("Expected current 2 after restoration, got %d", spellSlot.Current())
	}

	// Test over-restoration
	spellSlot.Restore(5)
	if spellSlot.Current() != 3 {
		t.Errorf("Expected current to cap at maximum 3, got %d", spellSlot.Current())
	}

	// Test rest restoration
	spellSlot.SetCurrent(1)
	restoreAmount := spellSlot.RestoreOnLongRest()
	if restoreAmount != 2 {
		t.Errorf("Expected long rest to restore 2 (to full), got %d", restoreAmount)
	}
}

func TestResourceBoundaries(t *testing.T) {
	owner := &MockEntity{id: "char-1", typ: "character"}

	resource := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:      "test-resource",
		Type:    resources.ResourceTypeCustom,
		Owner:   owner,
		Key:     core.MustNewRef(core.RefInput{
			Module: "test",
			Type:   "resource",
			Value:  "test",
		}),
		Current: 5,
		Maximum: 10,
	})

	// Test negative consumption
	err := resource.Consume(-1)
	if err == nil {
		t.Error("Expected error when consuming negative amount")
	}

	// Test SetCurrent boundaries
	resource.SetCurrent(-5)
	if resource.Current() != 0 {
		t.Errorf("Expected current to be 0 when set negative, got %d", resource.Current())
	}

	resource.SetCurrent(20)
	if resource.Current() != 10 {
		t.Errorf("Expected current to cap at maximum 10, got %d", resource.Current())
	}

	// Test SetMaximum adjusting current
	resource.SetCurrent(8)
	resource.SetMaximum(5)
	if resource.Maximum() != 5 {
		t.Errorf("Expected maximum 5, got %d", resource.Maximum())
	}
	if resource.Current() != 5 {
		t.Errorf("Expected current to adjust to new maximum 5, got %d", resource.Current())
	}
}

func TestAbilityUseResource(t *testing.T) {
	owner := &MockEntity{id: "char-1", typ: "character"}

	// Create a short rest ability
	secondWind := resources.CreateAbilityUse(owner, "second_wind", 1, resources.RestoreShortRest)

	if secondWind.Current() != 1 {
		t.Errorf("Expected 1 use available, got %d", secondWind.Current())
	}

	// Use the ability
	err := secondWind.Consume(1)
	if err != nil {
		t.Fatalf("Failed to use ability: %v", err)
	}

	if secondWind.IsAvailable() {
		t.Error("Expected ability to be unavailable after use")
	}

	// Check short rest restoration
	restoreAmount := secondWind.RestoreOnShortRest()
	if restoreAmount != 1 {
		t.Errorf("Expected short rest to restore 1 use, got %d", restoreAmount)
	}

	// Check long rest also restores it
	restoreAmount = secondWind.RestoreOnLongRest()
	if restoreAmount != 1 {
		t.Errorf("Expected long rest to restore 1 use, got %d", restoreAmount)
	}
}

func TestActionEconomy(t *testing.T) {
	owner := &MockEntity{id: "char-1", typ: "character"}

	actions := resources.CreateActionEconomy(owner)
	if len(actions) != 3 {
		t.Fatalf("Expected 3 action economy resources, got %d", len(actions))
	}

	// Find the standard action
	var action resources.Resource
	actionKey := core.MustNewRef(core.RefInput{
		Module: "core",
		Type:   "action",
		Value:  "standard",
	})
	for _, a := range actions {
		if a.Key().Equals(actionKey) {
			action = a
			break
		}
	}

	if action == nil {
		t.Fatal("Could not find standard action resource")
	}

	// Test action consumption
	if !action.IsAvailable() {
		t.Error("Expected action to be available")
	}

	err := action.Consume(1)
	if err != nil {
		t.Fatalf("Failed to consume action: %v", err)
	}

	if action.IsAvailable() {
		t.Error("Expected action to be unavailable after use")
	}

	// Actions don't restore on rest
	if action.RestoreOnShortRest() != 0 {
		t.Error("Expected actions not to restore on short rest")
	}
	if action.RestoreOnLongRest() != 0 {
		t.Error("Expected actions not to restore on long rest")
	}
}
