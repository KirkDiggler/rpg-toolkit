// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// mockEntity implements core.Entity for testing
type mockEntity struct {
	id         string
	entityType string
}

func (m *mockEntity) GetID() string   { return m.id }
func (m *mockEntity) GetType() string { return m.entityType }

func TestBaseCondition(t *testing.T) {
	source := &mockEntity{id: "caster-1", entityType: "character"}
	duration := PermanentDuration{}

	condition := NewCondition("cond-1", "poisoned", "poison_dart", source, duration)

	// Test properties
	if condition.ID() != "cond-1" {
		t.Errorf("Expected ID 'cond-1', got %s", condition.ID())
	}

	if condition.Type() != "poisoned" {
		t.Errorf("Expected type 'poisoned', got %s", condition.Type())
	}

	if condition.Source() != "poison_dart" {
		t.Errorf("Expected source 'poison_dart', got %s", condition.Source())
	}

	if condition.SourceEntity() != source {
		t.Error("Source entity mismatch")
	}

	// Applied time should be recent
	if time.Since(condition.AppliedAt()) > time.Second {
		t.Error("Applied time too old")
	}

	// Should not be expired (permanent duration)
	event := events.NewGameEvent(events.EventTurnEnd, nil, nil)
	if condition.IsExpired(event) {
		t.Error("Permanent condition should not expire")
	}
}

func TestPermanentDuration(t *testing.T) {
	duration := PermanentDuration{}

	// Should never expire
	testEvents := []string{
		events.EventTurnEnd,
		events.EventRoundEnd,
		events.EventAfterDamage,
		"custom_event",
	}

	for _, eventType := range testEvents {
		event := events.NewGameEvent(eventType, nil, nil)
		if duration.IsExpired(event) {
			t.Errorf("Permanent duration expired on %s", eventType)
		}
	}

	if duration.Description() != "Permanent" {
		t.Errorf("Expected description 'Permanent', got %s", duration.Description())
	}
}

func TestRoundsDuration(t *testing.T) {
	duration := NewRoundsDuration(3)

	// Should not expire on non-round events
	turnEvent := events.NewGameEvent(events.EventTurnEnd, nil, nil)
	if duration.IsExpired(turnEvent) {
		t.Error("Rounds duration should not expire on turn end")
	}

	// First round - should start tracking
	round1 := events.NewGameEvent(events.EventRoundEnd, nil, nil)
	round1.Context().Set("round", 1)
	if duration.IsExpired(round1) {
		t.Error("Should not expire on first round")
	}

	// Second round - still active
	round2 := events.NewGameEvent(events.EventRoundEnd, nil, nil)
	round2.Context().Set("round", 2)
	if duration.IsExpired(round2) {
		t.Error("Should not expire on second round")
	}

	// Third round - still active (started on round 1, so expires after round 3)
	round3 := events.NewGameEvent(events.EventRoundEnd, nil, nil)
	round3.Context().Set("round", 3)
	if duration.IsExpired(round3) {
		t.Error("Should not expire on third round")
	}

	// Fourth round - should expire
	round4 := events.NewGameEvent(events.EventRoundEnd, nil, nil)
	round4.Context().Set("round", 4)
	if !duration.IsExpired(round4) {
		t.Error("Should expire after 3 rounds")
	}
}

func TestTurnsDuration(t *testing.T) {
	entity := &mockEntity{id: "target-1", entityType: "character"}
	duration := NewTurnsDuration(2, entity.GetID())

	// Wrong entity's turn - should not count
	otherEntity := &mockEntity{id: "other-1", entityType: "character"}
	otherTurn := events.NewGameEvent(events.EventTurnEnd, otherEntity, nil)
	if duration.IsExpired(otherTurn) {
		t.Error("Should not expire on other entity's turn")
	}

	// First turn of correct entity
	turn1 := events.NewGameEvent(events.EventTurnEnd, entity, nil)
	if duration.IsExpired(turn1) {
		t.Error("Should not expire on first turn")
	}

	// Second turn - should expire
	turn2 := events.NewGameEvent(events.EventTurnEnd, entity, nil)
	if !duration.IsExpired(turn2) {
		t.Error("Should expire after 2 turns")
	}
}

func TestUntilDamagedDuration(t *testing.T) {
	target := &mockEntity{id: "target-1", entityType: "character"}
	duration := NewUntilDamagedDuration(target.GetID())

	// Non-damage event - should not expire
	turnEvent := events.NewGameEvent(events.EventTurnEnd, nil, nil)
	if duration.IsExpired(turnEvent) {
		t.Error("Should not expire on non-damage event")
	}

	// Damage to different entity - should not expire
	otherEntity := &mockEntity{id: "other-1", entityType: "character"}
	otherDamage := events.NewGameEvent(events.EventAfterDamage, nil, otherEntity)
	if duration.IsExpired(otherDamage) {
		t.Error("Should not expire when different entity is damaged")
	}

	// Damage to correct entity - should expire
	targetDamage := events.NewGameEvent(events.EventAfterDamage, nil, target)
	if !duration.IsExpired(targetDamage) {
		t.Error("Should expire when target is damaged")
	}
}

func TestEventDuration(t *testing.T) {
	// Simple event duration
	duration := NewEventDuration("combat_end", nil)

	// Wrong event type
	wrongEvent := events.NewGameEvent("turn_end", nil, nil)
	if duration.IsExpired(wrongEvent) {
		t.Error("Should not expire on wrong event type")
	}

	// Correct event type
	rightEvent := events.NewGameEvent("combat_end", nil, nil)
	if !duration.IsExpired(rightEvent) {
		t.Error("Should expire on correct event type")
	}

	// With condition function
	target := &mockEntity{id: "target-1", entityType: "character"}
	conditionalDuration := NewEventDuration("save_made", func(e events.Event) bool {
		// Only expire if it's the right target and they succeeded
		if e.Target() != nil && e.Target().GetID() == target.GetID() {
			if success, ok := e.Context().Get("success"); ok {
				return success.(bool)
			}
		}
		return false
	})

	// Save made by different entity
	otherSave := events.NewGameEvent("save_made", nil, &mockEntity{id: "other", entityType: "character"})
	otherSave.Context().Set("success", true)
	if conditionalDuration.IsExpired(otherSave) {
		t.Error("Should not expire on other entity's save")
	}

	// Failed save by correct entity
	failedSave := events.NewGameEvent("save_made", nil, target)
	failedSave.Context().Set("success", false)
	if conditionalDuration.IsExpired(failedSave) {
		t.Error("Should not expire on failed save")
	}

	// Successful save by correct entity
	successSave := events.NewGameEvent("save_made", nil, target)
	successSave.Context().Set("success", true)
	if !conditionalDuration.IsExpired(successSave) {
		t.Error("Should expire on successful save")
	}
}