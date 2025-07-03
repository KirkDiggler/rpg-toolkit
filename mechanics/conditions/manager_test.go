// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

func TestEventManager(t *testing.T) {
	bus := events.NewBus()
	manager := NewEventManager(bus)

	// Register entities
	player := &mockEntity{id: "player-1", entityType: "character"}
	monster := &mockEntity{id: "monster-1", entityType: "monster"}
	manager.RegisterEntity(player)
	manager.RegisterEntity(monster)

	// Test adding a condition
	condition := NewCondition("cond-1", "poisoned", "poison_dart", monster, PermanentDuration{})
	err := manager.Add(player.GetID(), condition)
	if err != nil {
		t.Fatalf("Failed to add condition: %v", err)
	}

	// Test HasCondition
	if !manager.HasCondition(player.GetID(), "poisoned") {
		t.Error("Expected player to have poisoned condition")
	}

	if manager.HasCondition(player.GetID(), "blessed") {
		t.Error("Player should not have blessed condition")
	}

	// Test Get
	retrieved, exists := manager.Get(player.GetID(), "cond-1")
	if !exists {
		t.Error("Expected to find condition")
	}
	if retrieved.ID() != "cond-1" {
		t.Error("Retrieved wrong condition")
	}

	// Test GetByType
	poisoned := manager.GetByType(player.GetID(), "poisoned")
	if len(poisoned) != 1 {
		t.Errorf("Expected 1 poisoned condition, got %d", len(poisoned))
	}

	// Test GetAll
	all := manager.GetAll(player.GetID())
	if len(all) != 1 {
		t.Errorf("Expected 1 condition total, got %d", len(all))
	}

	// Test Remove
	err = manager.Remove(player.GetID(), "cond-1")
	if err != nil {
		t.Errorf("Failed to remove condition: %v", err)
	}

	if manager.HasCondition(player.GetID(), "poisoned") {
		t.Error("Condition should have been removed")
	}
}

func TestEventManagerMultipleConditions(t *testing.T) {
	bus := events.NewBus()
	manager := NewEventManager(bus)

	player := &mockEntity{id: "player-1", entityType: "character"}
	manager.RegisterEntity(player)

	// Add multiple conditions
	cond1 := NewCondition("cond-1", "poisoned", "dart", nil, PermanentDuration{})
	cond2 := NewCondition("cond-2", "blessed", "spell", nil, PermanentDuration{})
	cond3 := NewCondition("cond-3", "poisoned", "gas", nil, PermanentDuration{})

	_ = manager.Add(player.GetID(), cond1)
	_ = manager.Add(player.GetID(), cond2)
	_ = manager.Add(player.GetID(), cond3)

	// Should have 2 poisoned conditions
	poisoned := manager.GetByType(player.GetID(), "poisoned")
	if len(poisoned) != 2 {
		t.Errorf("Expected 2 poisoned conditions, got %d", len(poisoned))
	}

	// Should have 3 total conditions
	all := manager.GetAll(player.GetID())
	if len(all) != 3 {
		t.Errorf("Expected 3 total conditions, got %d", len(all))
	}

	// Remove all poisoned
	err := manager.RemoveType(player.GetID(), "poisoned")
	if err != nil {
		t.Errorf("Failed to remove type: %v", err)
	}

	// Should only have blessed left
	if !manager.HasCondition(player.GetID(), "blessed") {
		t.Error("Should still have blessed condition")
	}

	if manager.HasCondition(player.GetID(), "poisoned") {
		t.Error("Should not have poisoned conditions")
	}

	// Clear all
	manager.Clear(player.GetID())
	all = manager.GetAll(player.GetID())
	if len(all) != 0 {
		t.Error("Should have no conditions after clear")
	}
}

func TestEventManagerDurationExpiry(t *testing.T) {
	bus := events.NewBus()
	manager := NewEventManager(bus)

	player := &mockEntity{id: "player-1", entityType: "character"}
	manager.RegisterEntity(player)

	// Add condition that expires after 2 rounds
	duration := NewRoundsDuration(2)
	condition := NewCondition("cond-1", "blessed", "spell", nil, duration)
	_ = manager.Add(player.GetID(), condition)

	// Round 1 - should still have condition
	round1 := events.NewGameEvent(events.EventRoundEnd, nil, nil)
	round1.Context().Set("round", 1)
	_ = bus.Publish(context.Background(), round1)

	if !manager.HasCondition(player.GetID(), "blessed") {
		t.Error("Condition expired too early")
	}

	// Round 3 - should expire
	round3 := events.NewGameEvent(events.EventRoundEnd, nil, nil)
	round3.Context().Set("round", 3)
	_ = bus.Publish(context.Background(), round3)

	if manager.HasCondition(player.GetID(), "blessed") {
		t.Error("Condition should have expired")
	}
}

func TestEventManagerUntilDamaged(t *testing.T) {
	bus := events.NewBus()
	manager := NewEventManager(bus)

	player := &mockEntity{id: "player-1", entityType: "character"}
	monster := &mockEntity{id: "monster-1", entityType: "monster"}
	manager.RegisterEntity(player)
	manager.RegisterEntity(monster)

	// Add condition that expires when damaged
	duration := NewUntilDamagedDuration(player.GetID())
	condition := NewCondition("cond-1", "sanctuary", "spell", nil, duration)
	_ = manager.Add(player.GetID(), condition)

	// Damage to different entity - should not expire
	damageOther := events.NewGameEvent(events.EventAfterDamage, nil, monster)
	_ = bus.Publish(context.Background(), damageOther)

	if !manager.HasCondition(player.GetID(), "sanctuary") {
		t.Error("Condition expired when different entity was damaged")
	}

	// Damage to player - should expire
	damagePlayer := events.NewGameEvent(events.EventAfterDamage, nil, player)
	_ = bus.Publish(context.Background(), damagePlayer)

	if manager.HasCondition(player.GetID(), "sanctuary") {
		t.Error("Condition should have expired when player was damaged")
	}
}

func TestEventManagerEvents(t *testing.T) {
	bus := events.NewBus()
	manager := NewEventManager(bus)

	player := &mockEntity{id: "player-1", entityType: "character"}
	manager.RegisterEntity(player)

	// Track events
	var appliedEvent, removedEvent events.Event

	bus.SubscribeFunc(EventConditionApplied, 100, func(ctx context.Context, e events.Event) error {
		appliedEvent = e
		return nil
	})

	bus.SubscribeFunc(EventConditionRemoved, 100, func(ctx context.Context, e events.Event) error {
		removedEvent = e
		return nil
	})

	// Add condition
	condition := NewCondition("cond-1", "blessed", "spell", nil, PermanentDuration{})
	_ = manager.Add(player.GetID(), condition)

	// Check applied event
	if appliedEvent == nil {
		t.Fatal("No condition applied event")
	}
	if appliedEvent.Source() != player {
		t.Error("Wrong source in applied event")
	}
	if id, _ := appliedEvent.Context().Get("condition_id"); id != "cond-1" {
		t.Error("Wrong condition ID in event")
	}
	if condType, _ := appliedEvent.Context().Get("condition_type"); condType != "blessed" {
		t.Error("Wrong condition type in event")
	}

	// Remove condition
	_ = manager.Remove(player.GetID(), "cond-1")

	// Check removed event
	if removedEvent == nil {
		t.Fatal("No condition removed event")
	}
	if removedEvent.Source() != player {
		t.Error("Wrong source in removed event")
	}
}

func TestPoisonedConditionExample(t *testing.T) {
	bus := events.NewBus()
	manager := NewEventManager(bus)

	player := &mockEntity{id: "player-1", entityType: "character"}
	monster := &mockEntity{id: "monster-1", entityType: "monster"}
	manager.RegisterEntity(player)
	manager.RegisterEntity(monster)

	// Track if disadvantage was applied
	var hasDisadvantage bool

	// Subscribe to check for disadvantage
	bus.SubscribeFunc(events.EventAttackRoll, 200, func(ctx context.Context, e events.Event) error {
		mods := e.Context().Modifiers()
		for _, mod := range mods {
			if mod.Type() == events.ModifierDisadvantage && mod.Source() == "poisoned" {
				hasDisadvantage = true
			}
		}
		return nil
	})

	// Apply poisoned condition
	poisoned := NewPoisonedCondition("poison-1", "poison_dart", monster, PermanentDuration{})
	_ = manager.Add(player.GetID(), poisoned)

	// Player attacks - should have disadvantage
	attackEvent := events.NewGameEvent(events.EventAttackRoll, player, monster)
	_ = bus.Publish(context.Background(), attackEvent)

	if !hasDisadvantage {
		t.Error("Poisoned player should have disadvantage on attack")
	}

	// Remove condition
	_ = manager.Remove(player.GetID(), "poison-1")

	// Reset and try again
	hasDisadvantage = false
	attackEvent2 := events.NewGameEvent(events.EventAttackRoll, player, monster)
	_ = bus.Publish(context.Background(), attackEvent2)

	if hasDisadvantage {
		t.Error("Player should not have disadvantage after poison removed")
	}
}

func TestClearAll(t *testing.T) {
	bus := events.NewBus()
	manager := NewEventManager(bus)

	// Create multiple entities with conditions
	entities := []*mockEntity{
		{id: "player-1", entityType: "character"},
		{id: "player-2", entityType: "character"},
		{id: "monster-1", entityType: "monster"},
	}

	for _, entity := range entities {
		manager.RegisterEntity(entity)
		condition := NewCondition(entity.GetID()+"-cond", "test", "test", nil, PermanentDuration{})
		_ = manager.Add(entity.GetID(), condition)
	}

	// Verify all have conditions
	for _, entity := range entities {
		if !manager.HasCondition(entity.GetID(), "test") {
			t.Errorf("Entity %s should have condition", entity.GetID())
		}
	}

	// Clear all
	manager.ClearAll()

	// Verify none have conditions
	for _, entity := range entities {
		if manager.HasCondition(entity.GetID(), "test") {
			t.Errorf("Entity %s should not have condition after clear all", entity.GetID())
		}
	}
}
