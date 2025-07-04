// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

func TestCombatFlow_Hit(t *testing.T) {
	// Set up deterministic dice rolls
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := dice.DefaultRoller
	defer dice.SetDefaultRoller(original)

	mockRoller := mock_dice.NewMockRoller(ctrl)
	dice.SetDefaultRoller(mockRoller)

	// Expect: d20 roll = 15, blessed d4 = 3, damage 2d6 = 6,4
	mockRoller.EXPECT().RollN(1, 20).Return([]int{15}, nil)
	mockRoller.EXPECT().RollN(1, 4).Return([]int{3}, nil)
	mockRoller.EXPECT().RollN(2, 6).Return([]int{6, 4}, nil)

	// Create event bus and register handlers
	bus := events.NewBus()
	registerCombatHandlers(bus)
	registerRageHandler(bus)
	registerBlessedHandler(bus)

	// Create entities
	attacker := &SimpleEntity{id: "test1", name: "Test Attacker"}
	target := &SimpleEntity{id: "test2", name: "Test Target"}

	// Track if damage was applied
	damageApplied := false
	var finalDamage int

	// Override damage applied handler to capture result
	bus.SubscribeFunc(events.EventAfterTakeDamage, 200, func(_ context.Context, e events.Event) error {
		damageApplied = true
		damage, _ := e.Context().Get("damage")
		finalDamage = damage.(int)
		return nil
	})

	// Execute attack
	attackEvent := events.NewGameEvent(events.EventBeforeAttackRoll, attacker, target)
	attackEvent.Context().Set("weapon", "greatsword")
	attackEvent.Context().Set("is_raging", true)
	attackEvent.Context().Set("is_blessed", true)

	err := bus.Publish(context.Background(), attackEvent)
	if err != nil {
		t.Fatalf("Failed to publish attack event: %v", err)
	}

	// Verify results
	if !damageApplied {
		t.Error("Expected damage to be applied after hit")
	}

	// 15 + 3 = 18 vs AC 15 = hit
	// Damage: 6 + 4 (base) + 2 (rage) = 12
	expectedDamage := 12
	if finalDamage != expectedDamage {
		t.Errorf("Expected %d damage, got %d", expectedDamage, finalDamage)
	}
}

func TestCombatFlow_Miss(t *testing.T) {
	// Set up deterministic dice rolls
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := dice.DefaultRoller
	defer dice.SetDefaultRoller(original)

	mockRoller := mock_dice.NewMockRoller(ctrl)
	dice.SetDefaultRoller(mockRoller)

	// Expect: d20 roll = 5, blessed d4 = 2 (total 7, miss)
	mockRoller.EXPECT().RollN(1, 20).Return([]int{5}, nil)
	mockRoller.EXPECT().RollN(1, 4).Return([]int{2}, nil)
	// No damage rolls expected on miss

	// Create event bus and register handlers
	bus := events.NewBus()
	registerCombatHandlers(bus)
	registerRageHandler(bus)
	registerBlessedHandler(bus)

	// Create entities
	attacker := &SimpleEntity{id: "test1", name: "Test Attacker"}
	target := &SimpleEntity{id: "test2", name: "Test Target"}

	// Track if damage was applied
	damageApplied := false
	bus.SubscribeFunc(events.EventAfterTakeDamage, 200, func(_ context.Context, _ events.Event) error {
		damageApplied = true
		return nil
	})

	// Execute attack
	attackEvent := events.NewGameEvent(events.EventBeforeAttackRoll, attacker, target)
	attackEvent.Context().Set("weapon", "greatsword")
	attackEvent.Context().Set("is_raging", true)
	attackEvent.Context().Set("is_blessed", true)

	err := bus.Publish(context.Background(), attackEvent)
	if err != nil {
		t.Fatalf("Failed to publish attack event: %v", err)
	}

	// Verify no damage was applied
	if damageApplied {
		t.Error("No damage should be applied on a miss")
	}
}

func TestRageModifier(t *testing.T) {
	bus := events.NewBus()
	registerRageHandler(bus)

	// Create a damage event
	attacker := &SimpleEntity{id: "barb", name: "Barbarian"}
	target := &SimpleEntity{id: "gob", name: "Goblin"}

	damageEvent := events.NewGameEvent(events.EventOnDamageRoll, attacker, target)
	damageEvent.Context().Set("is_raging", true)

	// Capture modifiers
	var capturedModifiers []events.Modifier
	bus.SubscribeFunc(events.EventOnDamageRoll, 100, func(_ context.Context, e events.Event) error {
		capturedModifiers = e.Context().Modifiers()
		return nil
	})

	err := bus.Publish(context.Background(), damageEvent)
	if err != nil {
		t.Fatalf("Failed to publish damage event: %v", err)
	}

	// Verify rage modifier was added
	if len(capturedModifiers) != 1 {
		t.Fatalf("Expected 1 modifier, got %d", len(capturedModifiers))
	}

	mod := capturedModifiers[0]
	if mod.Source() != "rage" {
		t.Errorf("Expected modifier source 'rage', got %s", mod.Source())
	}
	if mod.Type() != events.ModifierDamageBonus {
		t.Errorf("Expected modifier type %s, got %s", events.ModifierDamageBonus, mod.Type())
	}
	if mod.ModifierValue().GetValue() != 2 {
		t.Errorf("Expected rage bonus of 2, got %d", mod.ModifierValue().GetValue())
	}
}

func TestBlessedModifier(t *testing.T) {
	// Set up mock dice for blessed d4
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := dice.DefaultRoller
	defer dice.SetDefaultRoller(original)

	mockRoller := mock_dice.NewMockRoller(ctrl)
	dice.SetDefaultRoller(mockRoller)
	mockRoller.EXPECT().RollN(1, 4).Return([]int{3}, nil)

	bus := events.NewBus()
	registerBlessedHandler(bus)

	// Create an attack event
	attacker := &SimpleEntity{id: "pal", name: "Paladin"}
	target := &SimpleEntity{id: "dem", name: "Demon"}

	attackEvent := events.NewGameEvent(events.EventBeforeAttackRoll, attacker, target)
	attackEvent.Context().Set("is_blessed", true)

	// Capture modifiers
	var capturedModifiers []events.Modifier
	bus.SubscribeFunc(events.EventBeforeAttackRoll, 100, func(_ context.Context, e events.Event) error {
		capturedModifiers = e.Context().Modifiers()
		return nil
	})

	err := bus.Publish(context.Background(), attackEvent)
	if err != nil {
		t.Fatalf("Failed to publish attack event: %v", err)
	}

	// Verify blessed modifier was added
	if len(capturedModifiers) != 1 {
		t.Fatalf("Expected 1 modifier, got %d", len(capturedModifiers))
	}

	mod := capturedModifiers[0]
	if mod.Source() != "blessed" {
		t.Errorf("Expected modifier source 'blessed', got %s", mod.Source())
	}
	if mod.Type() != events.ModifierAttackBonus {
		t.Errorf("Expected modifier type %s, got %s", events.ModifierAttackBonus, mod.Type())
	}
	if mod.ModifierValue().GetValue() != 3 {
		t.Errorf("Expected blessed bonus of 3 (mocked), got %d", mod.ModifierValue().GetValue())
	}
}
