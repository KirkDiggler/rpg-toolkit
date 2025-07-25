package dnd5e_test

import (
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

func Example_gameplay() {
	// During a game session...

	// Load character from saved data
	charData := loadCharacterData("ragnar")         // You implement this
	raceData := loadRaceData("human")               // You implement this
	classData := loadClassData("fighter")           // You implement this
	backgroundData := loadBackgroundData("soldier") // You implement this

	character, _ := dnd5e.LoadCharacterFromData(charData, &raceData, &classData, &backgroundData)

	// Player fails a save vs poison
	character.AddCondition(conditions.Condition{
		Type:   conditions.Poisoned,
		Source: "giant_spider_bite",
	})

	// Cleric casts Bless on the party
	character.AddEffect(effects.NewBlessEffect("cleric_spell_123"))

	// Check if character is poisoned
	if character.HasCondition(conditions.Poisoned) {
		fmt.Println("Character has disadvantage on attack rolls")
	}

	// Calculate AC with Shield spell
	character.AddEffect(effects.NewShieldEffect("wizard_reaction"))
	fmt.Printf("AC with Shield: %d\n", character.AC())

	// Save character state
	updatedData := character.ToData()
	saveCharacterData(updatedData) // You implement this

	// The saved data includes:
	// - Current conditions (poisoned)
	// - Active effects (bless, shield)
	// - All other character state
}

// Helper stubs for the example
func loadCharacterData(_ string) dnd5e.CharacterData { return dnd5e.CharacterData{} }
func loadRaceData(_ string) dnd5e.RaceData           { return dnd5e.RaceData{} }
func loadClassData(_ string) dnd5e.ClassData         { return dnd5e.ClassData{} }
func loadBackgroundData(_ string) dnd5e.Background   { return dnd5e.Background{} }
func saveCharacterData(_ dnd5e.CharacterData)        {}

func TestEffectStacking(_ *testing.T) {
	// Example of how effects work
	character := &dnd5e.Character{}

	// Multiple effects can stack
	character.AddEffect(effects.Effect{
		Type:    effects.EffectMageArmor,
		Source:  "wizard_spell",
		ACBonus: 3, // Base AC becomes 13 + Dex
	})

	character.AddEffect(effects.Effect{
		Type:    effects.EffectShield,
		Source:  "wizard_reaction",
		ACBonus: 5, // +5 AC
	})

	// Both effects apply
	// Base AC 10 + Mage Armor 3 + Shield 5 = 18 (plus Dex)
}
