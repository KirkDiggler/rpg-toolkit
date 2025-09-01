// Package character provides D&D 5e character creation and management functionality
package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// FightingStyle is re-exported from choices package for convenience
type FightingStyle = choices.FightingStyle

// SetClassInput contains all data needed to set a character's class
type SetClassInput struct {
	ClassID    classes.Class
	SubclassID classes.Subclass // Optional, validated based on class
	Choices    ClassChoices
}

// ClassChoices represents the player's selections for class requirements
type ClassChoices struct {
	Skills        []skills.Skill       // Skills chosen from class options
	FightingStyle *FightingStyle       // Fighting style selection
	Equipment     []EquipmentSelection // Equipment selections
	Cantrips      []spells.Spell       // Cantrip selections
	Spells        []spells.Spell       // Spell selections
	Expertise     []skills.Skill       // Expertise choices (for Rogue, etc.)
}

// EquipmentSelection represents a single equipment selection for the new API
// This is different from the existing EquipmentChoice used in validation
type EquipmentSelection struct {
	RequirementIndex int              // Which equipment requirement this satisfies (0, 1, 2, etc.)
	SelectedOption   shared.Equipment // The chosen equipment item
	// For choices like "any martial weapon", the specific item is in SelectedOption
}

// SetRaceInput contains all data needed to set a character's race
type SetRaceInput struct {
	RaceID    races.Race
	SubraceID races.Subrace // Optional, validated based on race
	Choices   RaceChoices
}

// DraconicAncestry represents the dragon type chosen by a dragonborn
// TODO: Move to a more appropriate package when dragon types are formalized
type DraconicAncestry string

const (
	// AncestryBlack = "black"   // Acid damage
	AncestryBlack DraconicAncestry = "black" // Acid damage
	// AncestryBlue   DraconicAncestry = "blue"    // Lightning damage
	AncestryBlue DraconicAncestry = "blue" // Lightning damage
	// AncestryBrass  DraconicAncestry = "brass"   // Fire damage
	AncestryBrass DraconicAncestry = "brass" // Fire damage
	// AncestryBronze DraconicAncestry = "bronze"  // Lightning damage
	AncestryBronze DraconicAncestry = "bronze" // Lightning damage
	// AncestryCopper DraconicAncestry = "copper"  // Acid damage
	AncestryCopper DraconicAncestry = "copper" // Acid damage
	// AncestryGold   DraconicAncestry = "gold"    // Fire damage
	AncestryGold DraconicAncestry = "gold" // Fire damage
	// AncestryGreen  DraconicAncestry = "green"   // Poison damage
	AncestryGreen DraconicAncestry = "green" // Poison damage
	// AncestryRed    DraconicAncestry = "red"     // Fire damage
	AncestryRed DraconicAncestry = "red" // Fire damage
	// AncestrySilver DraconicAncestry = "silver"  // Cold damage
	AncestrySilver DraconicAncestry = "silver" // Cold damage
	// AncestryWhite  DraconicAncestry = "white"   // Cold damage
	AncestryWhite DraconicAncestry = "white" // Cold damage
)

// RaceChoices represents the player's selections for race requirements
type RaceChoices struct {
	Languages        []languages.Language      // For variant human, half-elf
	SkillProficiency *skills.Skill             // For half-elf skill choice
	AbilityIncrease  map[abilities.Ability]int // For variant human
	DraconicAncestry *DraconicAncestry         // For dragonborn
}

// SetBackgroundInput contains all data needed to set a character's background
type SetBackgroundInput struct {
	BackgroundID backgrounds.Background
	Choices      BackgroundChoices
}

// BackgroundChoices represents the player's selections for background requirements
type BackgroundChoices struct {
	Languages []languages.Language // If background offers language choice
	Tools     []proficiencies.Tool // If background offers tool choice
}

// SetAbilityScoresInput contains the ability scores to set
type SetAbilityScoresInput struct {
	Scores AbilityScores
	Method string // "standard", "point-buy", "rolled" - for validation
}

// AbilityScores wraps the shared type for cleaner API
type AbilityScores map[abilities.Ability]int
