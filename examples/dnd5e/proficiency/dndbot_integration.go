package proficiency

import (
	"context"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// DNDBotCharacter represents the interface we need from DND bot's character
type DNDBotCharacter interface {
	GetID() string
	GetName() string
	GetLevel() int
	GetClass() string
	GetProficiencies() map[string][]DNDBotProficiency
}

// DNDBotProficiency represents a proficiency from the DND bot
type DNDBotProficiency interface {
	GetKey() string
	GetName() string
	GetType() string
}

// CharacterWrapper wraps a DND bot character to implement core.Entity
type CharacterWrapper struct {
	character DNDBotCharacter
}

// NewCharacterWrapper creates a wrapper for DND bot characters
func NewCharacterWrapper(char DNDBotCharacter) *CharacterWrapper {
	return &CharacterWrapper{character: char}
}

// GetID implements core.Entity
func (w *CharacterWrapper) GetID() string {
	return w.character.GetID()
}

// GetType implements core.Entity
func (w *CharacterWrapper) GetType() string {
	return "character"
}

// MigrateDNDBotCharacter migrates a DND bot character's proficiencies to the toolkit system
func MigrateDNDBotCharacter(char DNDBotCharacter, manager *Manager) error {
	// Wrap the character as an entity
	entity := NewCharacterWrapper(char)
	level := char.GetLevel()

	// Get all proficiencies from the character
	proficiencies := char.GetProficiencies()

	// Process each proficiency type
	for profType, profs := range proficiencies {
		for _, prof := range profs {
			if err := migrateProficiency(entity, prof, profType, level, manager); err != nil {
				return fmt.Errorf("failed to migrate proficiency %s: %w", prof.GetKey(), err)
			}
		}
	}

	return nil
}

// migrateProficiency converts a single DND bot proficiency to the toolkit system
func migrateProficiency(entity core.Entity, prof DNDBotProficiency, profType string, level int, manager *Manager) error {
	key := prof.GetKey()
	source := "migrated"

	switch strings.ToLower(profType) {
	case "weapon":
		return manager.AddWeaponProficiency(entity, key, source, level)

	case "skill":
		// Convert skill key to our Skill type
		skill := Skill(key)
		return manager.AddSkillProficiency(entity, skill, source, level)

	case "saving-throw":
		// Extract the ability from the key (e.g., "saving-throw-strength" -> "strength")
		parts := strings.Split(key, "-")
		if len(parts) >= 3 {
			save := SavingThrow(parts[2])
			return manager.AddSavingThrowProficiency(entity, save, source, level)
		}
		return fmt.Errorf("invalid saving throw key: %s", key)

	case "armor":
		// For now, we'll store armor proficiencies as weapon proficiencies
		// In a full implementation, we'd have ArmorProficiency type
		return manager.AddWeaponProficiency(entity, key, source, level)

	case "tool", "language":
		// These would need their own proficiency types in a full implementation
		// For now, we skip them
		return nil

	default:
		return fmt.Errorf("unknown proficiency type: %s", profType)
	}
}

// Integration example showing how to use with DND bot
type IntegrationExample struct {
	manager  *Manager
	eventBus *events.Bus
}

// NewIntegrationExample creates a new integration example
func NewIntegrationExample() *IntegrationExample {
	eventBus := events.NewBus()
	return &IntegrationExample{
		manager:  NewManager(eventBus),
		eventBus: eventBus,
	}
}

// Example shows how to integrate proficiencies with combat
func (ie *IntegrationExample) Example() {
	// Subscribe to attack rolls to see proficiency bonuses in action
	ie.eventBus.SubscribeFunc(events.EventOnAttackRoll, 0, func(ctx context.Context, e events.Event) error {
		fmt.Printf("Attack roll modifiers:\n")
		for _, mod := range e.Context().Modifiers() {
			fmt.Printf("  - %s: +%d (%s)\n",
				mod.Source(),
				mod.ModifierValue().GetValue(),
				mod.ModifierValue().GetSource())
		}
		return nil
	})

	// Create a test character entity
	testChar := &testCharacter{
		id:    "fighter-123",
		name:  "Bruenor",
		level: 5,
		class: "fighter",
	}
	entity := NewCharacterWrapper(testChar)

	// Add fighter proficiencies
	ie.manager.AddClassProficiencies(entity, "fighter", 5)

	// Simulate an attack roll with a longsword
	attackEvent := events.NewGameEvent(events.EventOnAttackRoll, entity, nil)
	attackEvent.Context().Set("weapon", "longsword")

	// Publish the event - proficiency bonus will be added automatically
	ie.eventBus.Publish(context.Background(), attackEvent)
}

// Test implementation of DNDBotCharacter
type testCharacter struct {
	id    string
	name  string
	level int
	class string
}

func (tc *testCharacter) GetID() string    { return tc.id }
func (tc *testCharacter) GetName() string  { return tc.name }
func (tc *testCharacter) GetLevel() int    { return tc.level }
func (tc *testCharacter) GetClass() string { return tc.class }
func (tc *testCharacter) GetProficiencies() map[string][]DNDBotProficiency {
	return nil // Not needed for this example
}
