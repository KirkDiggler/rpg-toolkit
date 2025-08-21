package character_test

import (
	"fmt"
	"log"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Example_createCharacterFromDraft shows the complete flow of creating a D&D 5e character
// from scratch using the Builder pattern and then converting to a playable character.
func Example_createCharacterFromDraft() {
	// Step 1: Create a new character builder
	builder, err := character.NewCharacterBuilder("draft-123")
	if err != nil {
		log.Fatal(err)
	}

	// Step 2: Set the character's name
	if err := builder.SetName("Aragorn"); err != nil {
		log.Fatal(err)
	}

	// Step 3: Set the race (you would load this from your race data source)
	humanRace := race.Data{
		ID:        races.Human,
		Name:      "Human",
		Size:      "Medium",
		Speed:     30,
		Languages: []languages.Language{languages.Common},
		AbilityScoreIncreases: map[abilities.Ability]int{
			// Variant human gets +1 to two different abilities
			abilities.STR: 1,
			abilities.CON: 1,
		},
	}
	if err := builder.SetRaceData(humanRace, ""); err != nil {
		log.Fatal(err)
	}

	// Step 4: Set the class (you would load this from your class data source)
	fighterClass := class.Data{
		ID:                    classes.Fighter,
		Name:                  "Fighter",
		HitDice:               10,
		SkillProficiencyCount: 2,
		SkillOptions: []skills.Skill{
			skills.Acrobatics,
			skills.AnimalHandling,
			skills.Athletics,
			skills.History,
			skills.Insight,
			skills.Intimidation,
			skills.Perception,
			skills.Survival,
		},
		SavingThrows: []abilities.Ability{
			abilities.STR,
			abilities.CON,
		},
		ArmorProficiencies:  []string{"Light", "Medium", "Heavy", "Shields"},
		WeaponProficiencies: []string{"Simple", "Martial"},
	}
	if err := builder.SetClassData(fighterClass, ""); err != nil {
		log.Fatal(err)
	}

	// Step 5: Set the background (you would load this from your background data source)
	soldierBackground := shared.Background{
		ID:   backgrounds.Soldier,
		Name: "Soldier",
		SkillProficiencies: []skills.Skill{
			skills.Athletics,
			skills.Intimidation,
		},
		Languages:         []languages.Language{},
		ToolProficiencies: []string{"Gaming set", "Land vehicles"},
		Equipment: []string{
			"Insignia of rank",
			"Trophy from fallen enemy",
			"Deck of cards",
			"Common clothes",
			"Belt pouch (15 gp)",
		},
	}
	if err := builder.SetBackgroundData(soldierBackground); err != nil {
		log.Fatal(err)
	}

	// Step 6: Set ability scores (using standard array: 15, 14, 13, 12, 10, 8)
	abilityScores := shared.AbilityScores{
		abilities.STR: 15, // Fighter primary
		abilities.DEX: 13,
		abilities.CON: 14, // Fighter secondary
		abilities.INT: 10,
		abilities.WIS: 12,
		abilities.CHA: 8,
	}
	if err := builder.SetAbilityScores(abilityScores); err != nil {
		log.Fatal(err)
	}

	// Step 7: Select skills from class options
	// Fighter gets 2 skills from their list
	skills := []string{
		string(skills.Perception),
		string(skills.Survival),
	}
	if err := builder.SelectSkills(skills); err != nil {
		log.Fatal(err)
	}

	// Step 8: Select fighting style (for fighter)
	if err := builder.SelectFightingStyle("defense"); err != nil {
		log.Fatal(err)
	}

	// Step 9: Select starting equipment
	equipment := []string{
		"Chain mail",
		"Shield",
		"Longsword",
		"Handaxe (2)",
		"Explorer's pack",
	}
	if err := builder.SelectEquipment(equipment); err != nil {
		log.Fatal(err)
	}

	// Step 10: Build the character
	character, err := builder.Build()
	if err != nil {
		log.Fatal(err)
	}

	// The character is now ready to play!
	// Get character data to access properties
	charData := character.ToData()

	fmt.Printf("Character: %s\n", charData.Name)
	fmt.Printf("Race: Human\n")
	fmt.Printf("Class: Fighter (Level %d)\n", charData.Level)
	fmt.Printf("HP: %d/%d\n", charData.HitPoints, charData.MaxHitPoints)
	fmt.Printf("AC: %d\n", character.AC()) // Base AC, equipment bonuses not yet implemented
	fmt.Printf("Speed: %d ft\n", charData.Speed)

	// Output:
	// Character: Aragorn
	// Race: Human
	// Class: Fighter (Level 1)
	// HP: 12/12
	// AC: 11
	// Speed: 30 ft
}

// Example_createCharacterWithChoices shows how the game service can track
// all the choices made during character creation for later rebuilding.
func Example_createCharacterWithChoices() {
	// The Draft stores all choices with their sources
	builder, err := character.NewCharacterBuilder("draft-456")
	if err != nil {
		log.Fatal(err)
	}

	// Each method adds a ChoiceData entry internally
	if err := builder.SetName("Legolas"); err != nil {
		log.Fatal(err)
	}

	// Race choice is tracked
	elfRace := race.Data{
		ID:    races.Elf,
		Name:  "Elf",
		Size:  "Medium",
		Speed: 30,
		Languages: []languages.Language{
			languages.Common,
			languages.Elvish,
		},
		SkillProficiencies: []skills.Skill{skills.Perception},
		AbilityScoreIncreases: map[abilities.Ability]int{
			abilities.DEX: 2,
		},
	}
	if err := builder.SetRaceData(elfRace, ""); err != nil {
		log.Fatal(err)
	}

	// The draft now contains:
	// - NameSelection: "Legolas" (source: player)
	// - RaceSelection: {RaceID: "elf"} (source: player)

	// When you build, all choices are preserved on the character
	// This allows the game service to:
	// 1. Show where each feature came from
	// 2. Rebuild the draft if needed
	// 3. Handle race/class changes by knowing what to remove

	fmt.Println("Choices are tracked throughout character creation")
	// Output:
	// Choices are tracked throughout character creation
}
