package proficiency_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/dnd5e/proficiency"
)

// MockCharacter implements a simple character for testing
type MockCharacter struct {
	id    string
	level int
}

func (m *MockCharacter) GetID() string   { return m.id }
func (m *MockCharacter) GetType() string { return "character" }

func TestWeaponProficiency(t *testing.T) {
	// Create event bus and manager
	eventBus := events.NewBus()
	manager := proficiency.NewManager(eventBus)

	// Create a level 5 fighter
	fighter := &MockCharacter{id: "fighter-1", level: 5}

	// Add fighter proficiencies
	err := manager.AddClassProficiencies(fighter, "fighter", 5)
	if err != nil {
		t.Fatalf("Failed to add fighter proficiencies: %v", err)
	}

	// Track modifiers applied
	var appliedModifiers []events.Modifier

	// Subscribe to capture modifiers
	eventBus.SubscribeFunc(events.EventOnAttackRoll, 0, func(_ context.Context, e events.Event) error {
		appliedModifiers = e.Context().Modifiers()
		return nil
	})

	// Test cases for different weapons
	testCases := []struct {
		weapon      string
		shouldApply bool
		description string
	}{
		{"longsword", true, "Fighter is proficient with martial weapons"},
		{"dagger", true, "Fighter is proficient with simple weapons"},
		{"exotic-weapon", false, "Fighter is not proficient with exotic weapons"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Reset modifiers
			appliedModifiers = nil

			// Create attack event
			attackEvent := events.NewGameEvent(events.EventOnAttackRoll, fighter, nil)
			attackEvent.Context().Set("weapon", tc.weapon)

			// Publish event
			err := eventBus.Publish(context.Background(), attackEvent)
			if err != nil {
				t.Fatalf("Failed to publish event: %v", err)
			}

			// Check if proficiency was applied
			proficiencyApplied := false
			for _, mod := range appliedModifiers {
				if mod.Source() == "weapon-proficiency" {
					proficiencyApplied = true
					// At level 5, proficiency bonus should be +3
					expectedBonus := 3
					if mod.ModifierValue().GetValue() != expectedBonus {
						t.Errorf("Expected proficiency bonus +%d, got +%d",
							expectedBonus, mod.ModifierValue().GetValue())
					}
				}
			}

			if proficiencyApplied != tc.shouldApply {
				t.Errorf("Weapon %s: expected proficiency applied=%v, got %v",
					tc.weapon, tc.shouldApply, proficiencyApplied)
			}
		})
	}
}

func TestSkillProficiencyWithExpertise(t *testing.T) {
	// Create event bus and manager
	eventBus := events.NewBus()
	manager := proficiency.NewManager(eventBus)

	// Create a level 3 rogue
	rogue := &MockCharacter{id: "rogue-1", level: 3}

	// Add rogue proficiencies (includes expertise)
	err := manager.AddClassProficiencies(rogue, "rogue", 3)
	if err != nil {
		t.Fatalf("Failed to add rogue proficiencies: %v", err)
	}

	// Test stealth check (has expertise)
	var stealthModifiers []events.Modifier
	eventBus.SubscribeFunc(events.EventOnAbilityCheck, 0, func(_ context.Context, e events.Event) error {
		skill, _ := e.Context().GetString("skill")
		if skill == "stealth" {
			stealthModifiers = e.Context().Modifiers()
		}
		return nil
	})

	// Create ability check event
	checkEvent := events.NewGameEvent(events.EventOnAbilityCheck, rogue, nil)
	checkEvent.Context().Set("skill", "stealth")

	// Publish event
	err = eventBus.Publish(context.Background(), checkEvent)
	if err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Check modifiers
	expertiseFound := false
	for _, mod := range stealthModifiers {
		if mod.Source() == "skill-expertise" {
			expertiseFound = true
			// At level 3, proficiency is +2, expertise doubles it to +4
			expectedBonus := 4
			if mod.ModifierValue().GetValue() != expectedBonus {
				t.Errorf("Expected expertise bonus +%d, got +%d",
					expectedBonus, mod.ModifierValue().GetValue())
			}
		}
	}

	if !expertiseFound {
		t.Error("Expected expertise modifier for stealth, but not found")
	}
}

func TestSavingThrowProficiency(t *testing.T) {
	// Create event bus and manager
	eventBus := events.NewBus()
	manager := proficiency.NewManager(eventBus)

	// Create a level 8 wizard
	wizard := &MockCharacter{id: "wizard-1", level: 8}

	// Add wizard proficiencies
	err := manager.AddClassProficiencies(wizard, "wizard", 8)
	if err != nil {
		t.Fatalf("Failed to add wizard proficiencies: %v", err)
	}

	// Test intelligence save (proficient) vs strength save (not proficient)
	testCases := []struct {
		ability    string
		proficient bool
	}{
		{"intelligence", true},
		{"wisdom", true},
		{"strength", false},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_save", tc.ability), func(t *testing.T) {
			var saveModifiers []events.Modifier

			// Subscribe to capture save modifiers
			eventBus.SubscribeFunc(events.EventOnSavingThrow, 0, func(_ context.Context, e events.Event) error {
				ability, _ := e.Context().GetString("ability")
				if ability == tc.ability {
					saveModifiers = e.Context().Modifiers()
				}
				return nil
			})

			// Create saving throw event
			saveEvent := events.NewGameEvent(events.EventOnSavingThrow, wizard, nil)
			saveEvent.Context().Set("ability", tc.ability)

			// Publish event
			err := eventBus.Publish(context.Background(), saveEvent)
			if err != nil {
				t.Fatalf("Failed to publish event: %v", err)
			}

			// Check if proficiency was applied
			proficiencyFound := false
			for _, mod := range saveModifiers {
				if mod.Source() == "save-proficiency" {
					proficiencyFound = true
					// At level 8, proficiency bonus should be +3
					expectedBonus := 3
					if mod.ModifierValue().GetValue() != expectedBonus {
						t.Errorf("Expected proficiency bonus +%d, got +%d",
							expectedBonus, mod.ModifierValue().GetValue())
					}
				}
			}

			if proficiencyFound != tc.proficient {
				t.Errorf("%s save: expected proficiency=%v, got %v",
					tc.ability, tc.proficient, proficiencyFound)
			}
		})
	}
}

// ExampleIntegration demonstrates the full integration
func ExampleIntegration() {
	// Create the event bus and proficiency manager
	eventBus := events.NewBus()
	manager := proficiency.NewManager(eventBus)

	// Create a character
	character := &MockCharacter{id: "hero-1", level: 5}

	// Add class proficiencies
	manager.AddClassProficiencies(character, "fighter", 5)

	// Subscribe to see attack roll calculations
	eventBus.SubscribeFunc(events.EventOnAttackRoll, 0, func(_ context.Context, e events.Event) error {
		fmt.Println("=== Attack Roll ===")
		fmt.Printf("Attacker: %s\n", e.Source().GetID())

		weapon, _ := e.Context().GetString("weapon")
		fmt.Printf("Weapon: %s\n", weapon)

		fmt.Println("Modifiers:")
		for _, mod := range e.Context().Modifiers() {
			fmt.Printf("  %s: +%d (%s)\n",
				mod.Source(),
				mod.ModifierValue().GetValue(),
				mod.ModifierValue().GetSource())
		}
		return nil
	})

	// Simulate an attack with a longsword
	attackEvent := events.NewGameEvent(events.EventOnAttackRoll, character, nil)
	attackEvent.Context().Set("weapon", "longsword")

	// The proficiency system automatically adds the bonus
	eventBus.Publish(context.Background(), attackEvent)

	// Output:
	// === Attack Roll ===
	// Attacker: hero-1
	// Weapon: longsword
	// Modifiers:
	//   weapon-proficiency: +3 (proficiency with longsword)
}
