package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

func main() {
	// Check for CLI mode
	if len(os.Args) > 1 && os.Args[1] == "cli" {
		cli := NewCLI()
		cli.Run()
		return
	}

	// Original demo mode
	fmt.Println("=== D&D 5e Rulebook Example ===")
	fmt.Println()

	// 1. Load game data (in real app, from API/DB/files)
	raceData := getHumanRaceData()
	classData := getFighterClassData()
	backgroundData := getSoldierBackgroundData()

	// 2. Create a character using direct creation
	fmt.Println("Creating character: Ragnar the Fighter")
	char := createCharacter(raceData, classData, backgroundData)

	// 3. Display initial character state
	displayCharacter(char)

	// 4. Simulate combat scenario
	fmt.Println("\n=== Combat Simulation ===")
	simulateCombat(char)

	// 5. Save character state
	fmt.Println("\n=== Saving Character ===")
	saveCharacter(char)

	// 6. Load character state
	fmt.Println("\n=== Loading Character ===")
	loadedChar := loadCharacter(raceData, classData, backgroundData)
	displayCharacter(loadedChar)

	// 7. Demonstrate builder pattern
	fmt.Println("\n=== Builder Pattern Demo ===")
	demonstrateBuilder(raceData, classData, backgroundData)

	fmt.Println("\n\nTo run interactive CLI mode: go run . cli")
}

func createCharacter(raceData *race.Data, classData *class.Data,
	backgroundData *shared.BackgroundData) *character.Character {
	char, err := character.NewFromCreationData(character.CreationData{
		ID:             "char-ragnar-001",
		Name:           "Ragnar",
		RaceData:       raceData,
		ClassData:      classData,
		BackgroundData: backgroundData,
		AbilityScores: shared.AbilityScores{
			constants.STR: 15,
			constants.DEX: 14,
			constants.CON: 13,
			constants.INT: 12,
			constants.WIS: 10,
			constants.CHA: 8,
		},
		Choices: map[string]any{
			"skills":   []string{"athletics", "intimidation"},
			"language": "orcish",
		},
	})

	if err != nil {
		log.Fatal("Failed to create character:", err)
	}

	return char
}

func displayCharacter(char *character.Character) {
	data := char.ToData()
	fmt.Printf("Character: %s (Level %d %s %s)\n", data.Name, data.Level, data.RaceID, data.ClassID)
	fmt.Printf("HP: %d/%d\n", data.HitPoints, data.MaxHitPoints)
	fmt.Printf("AC: %d\n", char.AC())
	fmt.Printf("Ability Scores: STR %d, DEX %d, CON %d, INT %d, WIS %d, CHA %d\n",
		data.AbilityScores[constants.STR],
		data.AbilityScores[constants.DEX],
		data.AbilityScores[constants.CON],
		data.AbilityScores[constants.INT],
		data.AbilityScores[constants.WIS],
		data.AbilityScores[constants.CHA])

	if len(data.Conditions) > 0 {
		fmt.Print("Conditions: ")
		for _, c := range data.Conditions {
			fmt.Printf("%s ", c.Type)
		}
		fmt.Println()
	}

	if len(data.Effects) > 0 {
		fmt.Print("Effects: ")
		for _, e := range data.Effects {
			fmt.Printf("%s ", e.Type)
		}
		fmt.Println()
	}

	if len(data.Skills) > 0 {
		fmt.Print("Skills: ")
		for skill, prof := range data.Skills {
			if prof > 0 {
				fmt.Printf("%s ", skill)
			}
		}
		fmt.Println()
	}
}

func simulateCombat(char *character.Character) {
	fmt.Println("Round 1: Giant spider attacks!")
	fmt.Println("- Spider bite hits! Ragnar must make a CON save...")
	fmt.Println("- Failed! Ragnar is poisoned.")

	char.AddCondition(conditions.Condition{
		Type:   conditions.Poisoned,
		Source: "giant_spider_bite",
	})

	fmt.Println("\nRound 2: Cleric casts Bless on Ragnar")
	char.AddEffect(effects.NewBlessEffect("cleric_spell_123"))

	fmt.Println("- Ragnar attacks with Bless: +1d4 to attack rolls")
	fmt.Printf("- Current AC: %d\n", char.AC())

	fmt.Println("\nRound 3: Wizard casts Shield on Ragnar")
	char.AddEffect(effects.NewShieldEffect("wizard_reaction"))
	fmt.Printf("- AC with Shield: %d (+5 bonus)\n", char.AC())

	// Check conditions
	if char.HasCondition(conditions.Poisoned) {
		fmt.Println("- Ragnar has disadvantage on attack rolls (poisoned)")
	}
}

func saveCharacter(char *character.Character) {
	data := char.ToData()

	// In real app, save to database
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal character:", err)
	}

	err = os.WriteFile("ragnar.json", jsonData, 0644) // #nosec G306 - Example code, not sensitive data
	if err != nil {
		log.Fatal("Failed to save character:", err)
	}

	fmt.Println("Character saved to ragnar.json")
	fmt.Printf("- Conditions saved: %d\n", len(data.Conditions))
	fmt.Printf("- Effects saved: %d\n", len(data.Effects))
}

func loadCharacter(raceData *race.Data, classData *class.Data, backgroundData *shared.BackgroundData) *character.Character {
	// In real app, load from database
	jsonData, err := os.ReadFile("ragnar.json")
	if err != nil {
		log.Fatal("Failed to read character file:", err)
	}

	var data character.Data
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		log.Fatal("Failed to unmarshal character:", err)
	}

	char, err := character.LoadCharacterFromData(data, raceData, classData, backgroundData)
	if err != nil {
		log.Fatal("Failed to load character:", err)
	}

	fmt.Println("Character loaded from ragnar.json")
	return char
}

func loadCharacterFromFile(filename string, raceData *race.Data, classData *class.Data,
	backgroundData *shared.BackgroundData) *character.Character {
	// #nosec G304 - This is an example CLI that accepts user-provided filenames
	jsonData, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Failed to read character file: %v\n", err)
		return nil
	}

	var data character.Data
	err = json.Unmarshal(jsonData, &data)
	if err != nil {
		fmt.Printf("Failed to unmarshal character: %v\n", err)
		return nil
	}

	char, err := character.LoadCharacterFromData(data, raceData, classData, backgroundData)
	if err != nil {
		fmt.Printf("Failed to load character: %v\n", err)
		return nil
	}

	return char
}

func saveCharacterToFile(char *character.Character, filename string) {
	data := char.ToData()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal character: %v\n", err)
		return
	}

	err = os.WriteFile(filename, jsonData, 0644) // #nosec G306 - Example code, not sensitive data
	if err != nil {
		fmt.Printf("Failed to save character: %v\n", err)
		return
	}

	fmt.Printf("Character saved to %s\n", filename)
	fmt.Printf("- Conditions saved: %d\n", len(data.Conditions))
	fmt.Printf("- Effects saved: %d\n", len(data.Effects))
}

func demonstrateBuilder(raceData *race.Data, classData *class.Data, backgroundData *shared.BackgroundData) {
	fmt.Println("Creating character step-by-step with builder...")

	builder, err := dnd5e.NewCharacterBuilder("demo-builder-thorin")
	if err != nil {
		log.Fatal("Failed to create builder:", err)
	}

	// Step 1: Name
	if err := builder.SetName("Thorin"); err != nil {
		log.Fatal("Failed to set name:", err)
	}
	fmt.Println("✓ Name set")

	// Step 2: Race
	if err := builder.SetRaceData(*raceData, ""); err != nil {
		log.Fatal("Failed to set race:", err)
	}
	fmt.Println("✓ Race set")

	// Step 3: Class
	if err := builder.SetClassData(*classData); err != nil {
		log.Fatal("Failed to set class:", err)
	}
	fmt.Println("✓ Class set")

	// Step 4: BackgroundData
	if err := builder.SetBackgroundData(*backgroundData); err != nil {
		log.Fatal("Failed to set background:", err)
	}
	fmt.Println("✓ BackgroundData set")

	// Step 5: Ability Scores
	if err := builder.SetAbilityScores(shared.AbilityScores{
		constants.STR: 16, constants.DEX: 13, constants.CON: 14,
		constants.INT: 10, constants.WIS: 12, constants.CHA: 11,
	}); err != nil {
		log.Fatal("Failed to set ability scores:", err)
	}
	fmt.Println("✓ Ability scores set")

	// Step 6: Skills
	if err := builder.SelectSkills([]string{"athletics", "perception"}); err != nil {
		log.Fatal("Failed to select skills:", err)
	}
	fmt.Println("✓ Skills selected")

	// Check progress
	progress := builder.Progress()
	fmt.Printf("\nProgress: %.0f%% complete\n", progress.PercentComplete)
	fmt.Printf("Can build: %v\n", progress.CanBuild)

	// Save draft (useful for multi-step UIs)
	draftData := builder.ToData()
	fmt.Printf("\nDraft saved with ID: %s\n", draftData.ID)

	// Build character
	if progress.CanBuild {
		char, err := builder.Build()
		if err != nil {
			log.Fatal("Failed to build character:", err)
		}
		fmt.Println("\n✓ Character built successfully!")
		displayCharacter(char)
	}
}

// Sample game data (in real app, load from API/DB/files)

func getHumanRaceData() *race.Data {
	return &race.Data{
		ID:          "human",
		Name:        "Human",
		Description: "Humans are the most adaptable people",
		Size:        "medium",
		Speed:       30,
		AbilityScoreIncreases: map[string]int{
			string(constants.STR): 1,
			string(constants.DEX): 1,
			string(constants.CON): 1,
			string(constants.INT): 1,
			string(constants.WIS): 1,
			string(constants.CHA): 1,
		},
		Languages: []string{"common"},
		LanguageChoice: &race.ChoiceData{
			ID:          "human_language",
			Type:        "language",
			Choose:      1,
			From:        []string{"dwarvish", "elvish", "giant", "gnomish", "goblin", "halfling", "orcish"},
			Description: "Choose one additional language",
		},
	}
}

func getFighterClassData() *class.Data {
	return &class.Data{
		ID:                    "fighter",
		Name:                  "Fighter",
		Description:           "Masters of martial combat",
		HitDice:               10,
		HitPointsPerLevel:     6,
		SkillProficiencyCount: 2,
		SkillOptions: []string{
			"acrobatics", "animal_handling", "athletics", "history",
			"insight", "intimidation", "perception", "survival",
		},
		SavingThrows:        []string{string(constants.STR), string(constants.CON)},
		ArmorProficiencies:  []string{"light", "medium", "heavy", "shields"},
		WeaponProficiencies: []string{"simple", "martial"},
		Features: map[int][]class.FeatureData{
			1: {
				{
					ID:          "fighting_style",
					Name:        "Fighting Style",
					Level:       1,
					Description: "Adopt a particular style of fighting",
					Choice: &class.ChoiceData{
						ID:     "fighting_style_choice",
						Type:   "fighting_style",
						Choose: 1,
						From:   []string{"archery", "defense", "dueling", "great_weapon_fighting", "protection", "two_weapon_fighting"},
					},
				},
				{
					ID:          "second_wind",
					Name:        "Second Wind",
					Level:       1,
					Description: "Regain hit points as a bonus action",
				},
			},
		},
	}
}

func getSoldierBackgroundData() *shared.BackgroundData {
	return &shared.BackgroundData{
		ID:                 "soldier",
		Name:               "Soldier",
		Description:        "You have served in an army",
		SkillProficiencies: []string{"athletics", "intimidation"},
		ToolProficiencies:  []string{"gaming_set"},
		Equipment:          []string{"insignia", "trophy", "deck_of_cards", "common_clothes", "pouch"},
		Feature: shared.FeatureData{
			ID:          "military_rank",
			Name:        "Military Rank",
			Description: "You have a military rank from your career as a soldier",
		},
	}
}
