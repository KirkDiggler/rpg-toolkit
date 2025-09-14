// Package dnd5e implements D&D 5e rules using bounded contexts
package dnd5e

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Race types from the race package
type (
	// RaceData contains D&D 5e race information
	RaceData = race.Data
	// SubraceData contains subrace variant information
	SubraceData = race.SubraceData
)

// Class types from the class package
type (
	// ClassData contains D&D 5e class information
	ClassData = class.Data
	// FeatureData represents a class feature
	FeatureData = class.FeatureData
	// SpellcastingData contains spellcasting information
	SpellcastingData = class.SpellcastingData
)

// Shared types used across the rulebook
type (
	// AbilityScores represents the six ability scores
	AbilityScores = shared.AbilityScores
	// ChoiceCategory represents types of character creation choices
	ChoiceCategory = shared.ChoiceCategory
)

// Character types from the character package
type (
	// Character represents a D&D 5e character
	Character = character.Character
	// CharacterData is the persistent character data structure
	CharacterData = character.Data
	// CharacterDraft represents an in-progress character
	CharacterDraft = character.Draft
)

// Choice category constants for character creation
const (
	// ChoiceName is the character name choice
	ChoiceName = shared.ChoiceName
	// ChoiceRace is the race selection choice
	ChoiceRace = shared.ChoiceRace
	// ChoiceClass is the class selection choice
	ChoiceClass = shared.ChoiceClass
	// ChoiceBackground is the background selection choice
	ChoiceBackground = shared.ChoiceBackground
	// ChoiceAbilityScores is the ability score assignment choice
	ChoiceAbilityScores = shared.ChoiceAbilityScores
	// ChoiceSkills is the skill proficiency selection choice
	ChoiceSkills = shared.ChoiceSkills
)
