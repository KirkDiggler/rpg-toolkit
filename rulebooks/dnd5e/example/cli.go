// Package main provides an interactive CLI demo for the D&D 5e rulebook
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// CLI provides an interactive command-line interface for D&D character management
type CLI struct {
	scanner *bufio.Scanner
	char    *character.Character
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	return &CLI{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Run starts the interactive CLI session
func (c *CLI) Run() {
	fmt.Println("=== D&D 5e Character Manager ===")
	fmt.Println()

	for {
		c.showMenu()
		choice := c.getInput("Enter choice: ")

		switch choice {
		case "1":
			c.createCharacterWizard()
		case "2":
			c.loadCharacterFromFile()
		case "3":
			c.showCharacter()
		case "4":
			c.applyCombatEffects()
		case "5":
			c.saveCharacter()
		case "6":
			c.demonstrateBuilder()
		case "q", "Q":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Println("Invalid choice, please try again.")
		}

		fmt.Println()
	}
}

func (c *CLI) showMenu() {
	fmt.Println("Main Menu:")
	fmt.Println("1. Create new character")
	fmt.Println("2. Load character from file")
	fmt.Println("3. Show current character")
	fmt.Println("4. Apply combat effects")
	fmt.Println("5. Save character")
	fmt.Println("6. Demonstrate builder pattern")
	fmt.Println("Q. Quit")
	fmt.Println()
}

func (c *CLI) createCharacterWizard() {
	fmt.Println("\n=== Character Creation Wizard ===")

	// Get name
	name := c.getInput("Enter character name: ")

	// Select race
	fmt.Println("\nAvailable races:")
	fmt.Println("1. Human (+1 to all abilities)")
	fmt.Println("2. Elf (+2 DEX, +1 INT)")
	fmt.Println("3. Dwarf (+2 CON)")
	raceChoice := c.getInput("Select race (1-3): ")

	var raceData *race.Data
	switch raceChoice {
	case "1":
		raceData = getHumanRaceData()
	case "2":
		raceData = getElfRaceData()
	case "3":
		raceData = getDwarfRaceData()
	default:
		raceData = getHumanRaceData()
	}

	// Select class
	fmt.Println("\nAvailable classes:")
	fmt.Println("1. Fighter (d10 HD, martial weapons)")
	fmt.Println("2. Wizard (d6 HD, spellcasting)")
	fmt.Println("3. Rogue (d8 HD, sneak attack)")
	classChoice := c.getInput("Select class (1-3): ")

	var classData *class.Data
	switch classChoice {
	case "1":
		classData = getFighterClassData()
	case "2":
		classData = getWizardClassData()
	case "3":
		classData = getRogueClassData()
	default:
		classData = getFighterClassData()
	}

	// Get ability scores
	fmt.Println("\nEnter ability scores (3-18):")
	str := c.getAbilityScoreInput("Strength: ")
	dex := c.getAbilityScoreInput("Dexterity: ")
	con := c.getAbilityScoreInput("Constitution: ")
	intl := c.getAbilityScoreInput("Intelligence: ")
	wis := c.getAbilityScoreInput("Wisdom: ")
	cha := c.getAbilityScoreInput("Charisma: ")

	// Background
	backgroundData := getSoldierBackgroundData()

	// Select skills
	fmt.Println("\nAvailable skills:")
	for i, skill := range classData.SkillOptions {
		fmt.Printf("%d. %s\n", i+1, skill)
	}
	fmt.Printf("Select %d skills (comma-separated numbers): ", classData.SkillProficiencyCount)
	skillInput := c.getInput("")

	selectedSkills := []string{}
	if skillInput != "" {
		for _, numStr := range strings.Split(skillInput, ",") {
			if num, err := strconv.Atoi(strings.TrimSpace(numStr)); err == nil && num > 0 && num <= len(classData.SkillOptions) {
				selectedSkills = append(selectedSkills, classData.SkillOptions[num-1])
			}
		}
	}

	// Build choices map
	choices := map[string]any{
		"skills": selectedSkills,
	}

	// Add language choice for races that have it (like human)
	if raceData.LanguageChoice != nil {
		fmt.Println("\nSelect an additional language:")
		for i, lang := range raceData.LanguageChoice.From {
			fmt.Printf("%d. %s\n", i+1, lang)
		}
		langChoice := c.getInput("Select language (1-" + strconv.Itoa(len(raceData.LanguageChoice.From)) + "): ")
		if num, err := strconv.Atoi(langChoice); err == nil && num > 0 && num <= len(raceData.LanguageChoice.From) {
			choices["language"] = raceData.LanguageChoice.From[num-1]
		}
	}

	// Create character
	char, err := character.NewFromCreationData(character.CreationData{
		ID:             fmt.Sprintf("char-%s-%d", name, time.Now().Unix()),
		Name:           name,
		RaceData:       raceData,
		ClassData:      classData,
		BackgroundData: backgroundData,
		AbilityScores: shared.AbilityScores{
			Strength:     str,
			Dexterity:    dex,
			Constitution: con,
			Intelligence: intl,
			Wisdom:       wis,
			Charisma:     cha,
		},
		Choices: choices,
	})

	if err != nil {
		fmt.Printf("Failed to create character: %v\n", err)
		return
	}

	c.char = char
	fmt.Println("\nCharacter created successfully!")
	c.displayCharacterSummary(char)
}

func (c *CLI) loadCharacterFromFile() {
	filename := c.getInput("Enter filename to load (e.g., ragnar.json): ")

	// Load game data (in real app, from proper source)
	raceData := getHumanRaceData()
	classData := getFighterClassData()
	backgroundData := getSoldierBackgroundData()

	char := loadCharacterFromFile(filename, raceData, classData, backgroundData)
	if char != nil {
		c.char = char
		fmt.Println("Character loaded successfully!")
		c.displayCharacterSummary(char)
	}
}

func (c *CLI) showCharacter() {
	if c.char == nil {
		fmt.Println("No character loaded. Please create or load a character first.")
		return
	}

	c.displayCharacterSummary(c.char)
}

func (c *CLI) applyCombatEffects() {
	if c.char == nil {
		fmt.Println("No character loaded. Please create or load a character first.")
		return
	}

	fmt.Println("\n=== Combat Effects ===")
	fmt.Println("1. Apply Poisoned condition")
	fmt.Println("2. Apply Bless effect")
	fmt.Println("3. Apply Shield spell")
	fmt.Println("4. Apply Rage")
	fmt.Println("5. Remove all conditions")
	fmt.Println("6. Remove all effects")
	fmt.Println("B. Back to main menu")

	choice := c.getInput("Select effect: ")

	switch choice {
	case "1":
		c.char.AddCondition(conditions.Condition{
			Type:   conditions.Poisoned,
			Source: "combat_effect",
		})
		fmt.Println("Applied Poisoned condition")
	case "2":
		c.char.AddEffect(effects.NewBlessEffect("cleric_spell"))
		fmt.Println("Applied Bless effect")
	case "3":
		c.char.AddEffect(effects.NewShieldEffect("wizard_reaction"))
		fmt.Println("Applied Shield spell (+5 AC)")
	case "4":
		c.char.AddEffect(effects.Effect{
			Type:   effects.EffectRage,
			Source: "barbarian_feature",
		})
		fmt.Println("Applied Rage")
	case "5":
		// Remove each condition type manually since there's no RemoveAllConditions
		data := c.char.ToData()
		if len(data.Conditions) > 0 {
			for _, cond := range data.Conditions {
				c.char.RemoveCondition(cond.Type)
			}
			fmt.Println("Removed all conditions")
		} else {
			fmt.Println("No conditions to remove")
		}
	case "6":
		// Note: Character doesn't have RemoveAllEffects, so we'd need to track them
		fmt.Println("Effect removal not implemented in this demo")
	case "b", "B":
		return
	}

	// Show updated state
	c.displayCharacterSummary(c.char)
}

func (c *CLI) saveCharacter() {
	if c.char == nil {
		fmt.Println("No character loaded. Please create or load a character first.")
		return
	}

	filename := c.getInput("Enter filename to save (e.g., mychar.json): ")
	saveCharacterToFile(c.char, filename)
}

func (c *CLI) demonstrateBuilder() {
	fmt.Println("\n=== Builder Pattern Demo ===")
	fmt.Println("Creating character step-by-step...")

	builder, err := dnd5e.NewCharacterBuilder("demo-draft-" + time.Now().Format("20060102150405"))
	if err != nil {
		fmt.Printf("Failed to create builder: %v\n", err)
		return
	}

	// Step through each stage
	fmt.Println("\nStep 1: Setting name...")
	if err := builder.SetName("Thorin"); err != nil {
		fmt.Printf("Failed to set name: %v\n", err)
		return
	}
	progress := builder.Progress()
	fmt.Printf("Progress: %.0f%%\n", progress.PercentComplete)

	fmt.Println("\nStep 2: Setting race...")
	if err := builder.SetRaceData(*getHumanRaceData(), ""); err != nil {
		fmt.Printf("Failed to set race: %v\n", err)
		return
	}
	progress = builder.Progress()
	fmt.Printf("Progress: %.0f%%\n", progress.PercentComplete)

	fmt.Println("\nStep 3: Setting class...")
	if err := builder.SetClassData(*getFighterClassData()); err != nil {
		fmt.Printf("Failed to set class: %v\n", err)
		return
	}
	progress = builder.Progress()
	fmt.Printf("Progress: %.0f%%\n", progress.PercentComplete)

	fmt.Println("\nStep 4: Setting background...")
	if err := builder.SetBackgroundData(*getSoldierBackgroundData()); err != nil {
		fmt.Printf("Failed to set background: %v\n", err)
		return
	}
	progress = builder.Progress()
	fmt.Printf("Progress: %.0f%%\n", progress.PercentComplete)

	fmt.Println("\nStep 5: Setting ability scores...")
	if err := builder.SetAbilityScores(shared.AbilityScores{
		Strength: 16, Dexterity: 13, Constitution: 14,
		Intelligence: 10, Wisdom: 12, Charisma: 11,
	}); err != nil {
		fmt.Printf("Failed to set ability scores: %v\n", err)
		return
	}
	progress = builder.Progress()
	fmt.Printf("Progress: %.0f%%\n", progress.PercentComplete)

	fmt.Println("\nStep 6: Selecting skills...")
	if err := builder.SelectSkills([]string{"athletics", "perception"}); err != nil {
		fmt.Printf("Failed to select skills: %v\n", err)
		return
	}
	progress = builder.Progress()
	fmt.Printf("Progress: %.0f%% - Can build: %v\n", progress.PercentComplete, progress.CanBuild)

	if progress.CanBuild {
		char, err := builder.Build()
		if err != nil {
			fmt.Printf("Failed to build: %v\n", err)
			return
		}

		fmt.Println("\nCharacter built successfully!")
		c.displayCharacterSummary(char)

		// Ask if user wants to keep this character
		if keep := c.getInput("Keep this character? (y/n): "); strings.ToLower(keep) == "y" {
			c.char = char
		}
	}
}

func (c *CLI) displayCharacterSummary(char *character.Character) {
	fmt.Println("\n--- Character Summary ---")
	displayCharacter(char)
	fmt.Println("------------------------")
}

func (c *CLI) getInput(prompt string) string {
	fmt.Print(prompt)
	c.scanner.Scan()
	return strings.TrimSpace(c.scanner.Text())
}

func (c *CLI) getAbilityScoreInput(prompt string) int {
	minVal := 3 // D&D abilities range from 3-18
	maxVal := 18
	for {
		input := c.getInput(prompt)
		if num, err := strconv.Atoi(input); err == nil && num >= minVal && num <= maxVal {
			return num
		}
		fmt.Printf("Please enter a number between %d and %d.\n", minVal, maxVal)
	}
}

// Additional race data for variety
func getElfRaceData() *race.Data {
	return &race.Data{
		ID:          "elf",
		Name:        "Elf",
		Description: "Elves are a magical people of otherworldly grace",
		Size:        "medium",
		Speed:       30,
		AbilityScoreIncreases: map[string]int{
			shared.AbilityDexterity:    2,
			shared.AbilityIntelligence: 1,
		},
		Languages: []string{"common", "elvish"},
		Traits: []race.TraitData{
			{
				ID:          "darkvision",
				Name:        "Darkvision",
				Description: "You can see in dim light within 60 feet",
			},
			{
				ID:          "keen_senses",
				Name:        "Keen Senses",
				Description: "You have proficiency in the Perception skill",
			},
		},
	}
}

func getDwarfRaceData() *race.Data {
	return &race.Data{
		ID:          "dwarf",
		Name:        "Dwarf",
		Description: "Bold and hardy, dwarves are known as skilled warriors",
		Size:        "medium",
		Speed:       25,
		AbilityScoreIncreases: map[string]int{
			shared.AbilityConstitution: 2,
		},
		Languages: []string{"common", "dwarvish"},
		Traits: []race.TraitData{
			{
				ID:          "darkvision",
				Name:        "Darkvision",
				Description: "You can see in dim light within 60 feet",
			},
			{
				ID:          "dwarven_resilience",
				Name:        "Dwarven Resilience",
				Description: "You have advantage on saving throws against poison",
			},
		},
	}
}

// Additional class data
func getWizardClassData() *class.Data {
	return &class.Data{
		ID:                    "wizard",
		Name:                  "Wizard",
		Description:           "Scholars of the arcane",
		HitDice:               6,
		HitPointsAt1st:        6,
		HitPointsPerLevel:     4,
		SkillProficiencyCount: 2,
		SkillOptions: []string{
			"arcana", "history", "insight", "investigation",
			"medicine", "religion",
		},
		SavingThrows:        []string{shared.AbilityIntelligence, shared.AbilityWisdom},
		ArmorProficiencies:  []string{},
		WeaponProficiencies: []string{"daggers", "darts", "slings", "quarterstaffs", "light_crossbows"},
		Spellcasting: &class.SpellcastingData{
			Ability: shared.AbilityIntelligence,
			CantripsKnown: map[int]int{
				1: 3, 2: 3, 3: 3, 4: 4, 5: 4, 6: 4, 7: 4, 8: 4, 9: 4, 10: 5,
				11: 5, 12: 5, 13: 5, 14: 5, 15: 5, 16: 5, 17: 5, 18: 5, 19: 5, 20: 5,
			},
			PreparedFormula: "intelligence_modifier + wizard_level",
			RitualCasting:   true,
		},
	}
}

func getRogueClassData() *class.Data {
	return &class.Data{
		ID:                    "rogue",
		Name:                  "Rogue",
		Description:           "Skilled in stealth and subterfuge",
		HitDice:               8,
		HitPointsAt1st:        8,
		HitPointsPerLevel:     5,
		SkillProficiencyCount: 4,
		SkillOptions: []string{
			"acrobatics", "athletics", "deception", "insight", "intimidation",
			"investigation", "perception", "performance", "persuasion",
			"sleight_of_hand", "stealth",
		},
		SavingThrows:        []string{shared.AbilityDexterity, shared.AbilityIntelligence},
		ArmorProficiencies:  []string{"light"},
		WeaponProficiencies: []string{"simple", "hand_crossbows", "longswords", "rapiers", "shortswords"},
		Features: map[int][]class.FeatureData{
			1: {
				{
					ID:          "sneak_attack",
					Name:        "Sneak Attack",
					Level:       1,
					Description: "Deal extra damage to one creature you hit",
				},
				{
					ID:          "thieves_cant",
					Name:        "Thieves' Cant",
					Level:       1,
					Description: "Secret mix of dialect, jargon, and code",
				},
			},
		},
	}
}
