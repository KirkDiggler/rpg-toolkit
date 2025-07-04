// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package proficiency_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

// MockEntity for testing
type MockEntity struct {
	id  string
	typ string
}

func (e *MockEntity) GetID() string   { return e.id }
func (e *MockEntity) GetType() string { return e.typ }

func TestSimpleProficiency(t *testing.T) {
	// Create event bus
	bus := events.NewBus()

	// Create a character entity
	character := &MockEntity{id: "char-1", typ: "character"}

	// Track if proficiency bonus was applied
	bonusApplied := false

	// Create a weapon proficiency that adds bonus to attack rolls
	weaponProf := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		ID:      "prof-longsword",
		Type:    "proficiency.weapon",
		Owner:   character,
		Subject: "longsword",
		Source:  "fighter-class",
		ApplyFunc: func(p *proficiency.SimpleProficiency, bus events.EventBus) error {
			// Subscribe to attack roll events
			p.Subscribe(bus, events.EventBeforeAttack, 100, func(_ context.Context, e events.Event) error {
				// Check if this is our owner attacking with our subject
				if e.Source() != nil && e.Source().GetID() == p.Owner().GetID() {
					// In a real implementation, check if weapon matches
					// For test, just mark that we applied bonus
					bonusApplied = true
				}
				return nil
			})
			return nil
		},
	})

	// Apply the proficiency
	if err := weaponProf.Apply(bus); err != nil {
		t.Fatalf("Failed to apply proficiency: %v", err)
	}

	// Simulate an attack event
	attackEvent := events.NewGameEvent(events.EventBeforeAttack, character, nil)
	if err := bus.Publish(context.Background(), attackEvent); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Check if proficiency bonus was applied
	if !bonusApplied {
		t.Error("Expected proficiency bonus to be applied")
	}

	// Remove proficiency
	if err := weaponProf.Remove(bus); err != nil {
		t.Fatalf("Failed to remove proficiency: %v", err)
	}

	// Reset flag
	bonusApplied = false

	// Attack again - should not apply bonus
	attackEvent2 := events.NewGameEvent(events.EventBeforeAttack, character, nil)
	if err := bus.Publish(context.Background(), attackEvent2); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	if bonusApplied {
		t.Error("Expected no proficiency bonus after removal")
	}
}

func TestProficiencyMetadata(t *testing.T) {
	character := &MockEntity{id: "char-1", typ: "character"}

	prof := proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		ID:      "prof-athletics",
		Type:    "proficiency.skill",
		Owner:   character,
		Subject: "athletics",
		Source:  "barbarian-class",
	})

	// Test metadata
	if prof.GetID() != "prof-athletics" {
		t.Errorf("Expected ID 'prof-athletics', got %s", prof.GetID())
	}

	if prof.GetType() != "proficiency.skill" {
		t.Errorf("Expected type 'proficiency.skill', got %s", prof.GetType())
	}

	if prof.Owner().GetID() != character.GetID() {
		t.Errorf("Expected owner ID %s, got %s", character.GetID(), prof.Owner().GetID())
	}

	if prof.Subject() != "athletics" {
		t.Errorf("Expected subject 'athletics', got %s", prof.Subject())
	}

	if prof.Source() != "barbarian-class" {
		t.Errorf("Expected source 'barbarian-class', got %s", prof.Source())
	}
}
