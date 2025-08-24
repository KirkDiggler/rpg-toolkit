package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Choice represents a choice made during character creation
// Each concrete type implements this interface and knows how to validate itself
type Choice interface {
	GetSource() shared.ChoiceSource
	GetChoiceID() string
	GetCategory() shared.ChoiceCategory
}

// NameChoice represents choosing a character name
type NameChoice struct {
	Source   shared.ChoiceSource
	ChoiceID string
	Name     string
}

func (c NameChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c NameChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c NameChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceName }

// SkillChoice represents choosing skill proficiencies
type SkillChoice struct {
	Source   shared.ChoiceSource
	ChoiceID string
	Skills   []skills.Skill
}

func (c SkillChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c SkillChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c SkillChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceSkills }

// LanguageChoice represents choosing languages
type LanguageChoice struct {
	Source    shared.ChoiceSource
	ChoiceID  string
	Languages []languages.Language
}

func (c LanguageChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c LanguageChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c LanguageChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceLanguages }

// AbilityScoreChoice represents choosing ability score improvements
type AbilityScoreChoice struct {
	Source        shared.ChoiceSource
	ChoiceID      string
	AbilityScores shared.AbilityScores
}

func (c AbilityScoreChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c AbilityScoreChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c AbilityScoreChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceAbilityScores }

// FightingStyleChoice represents choosing a fighting style
type FightingStyleChoice struct {
	Source        shared.ChoiceSource
	ChoiceID      string
	FightingStyle string
}

func (c FightingStyleChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c FightingStyleChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c FightingStyleChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceFightingStyle }

// EquipmentChoice represents choosing starting equipment
type EquipmentChoice struct {
	Source    shared.ChoiceSource
	ChoiceID  string
	Equipment []string
}

func (c EquipmentChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c EquipmentChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c EquipmentChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceEquipment }

// RaceSelectionChoice represents choosing a character race and optional subrace
type RaceSelectionChoice struct {
	Source   shared.ChoiceSource
	ChoiceID string
	Race     races.Race
	Subrace  races.Subrace
}

func (c RaceSelectionChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c RaceSelectionChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c RaceSelectionChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceRace }

// ClassSelectionChoice represents choosing a character class
type ClassSelectionChoice struct {
	Source   shared.ChoiceSource
	ChoiceID string
	Class    classes.Class
}

func (c ClassSelectionChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c ClassSelectionChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c ClassSelectionChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceClass }

// BackgroundChoice represents choosing a character background
type BackgroundChoice struct {
	Source     shared.ChoiceSource
	ChoiceID   string
	Background backgrounds.Background
}

func (c BackgroundChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c BackgroundChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c BackgroundChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceBackground }

// SpellChoice represents choosing spells known or prepared
type SpellChoice struct {
	Source   shared.ChoiceSource
	ChoiceID string
	Spells   []string
}

func (c SpellChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c SpellChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c SpellChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceSpells }

// CantripChoice represents choosing cantrips
type CantripChoice struct {
	Source   shared.ChoiceSource
	ChoiceID string
	Cantrips []string
}

func (c CantripChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c CantripChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c CantripChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceCantrips }

// ExpertiseChoice represents choosing expertise (double proficiency bonus)
type ExpertiseChoice struct {
	Source    shared.ChoiceSource
	ChoiceID  string
	Expertise []string
}

func (c ExpertiseChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c ExpertiseChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c ExpertiseChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceExpertise }

// TraitChoice represents choosing traits (like draconic ancestry)
type TraitChoice struct {
	Source   shared.ChoiceSource
	ChoiceID string
	Traits   []string
}

func (c TraitChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c TraitChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c TraitChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceTraits }

// ToolProficiencyChoice represents choosing tool proficiencies
type ToolProficiencyChoice struct {
	Source   shared.ChoiceSource
	ChoiceID string
	Tools    []string
}

func (c ToolProficiencyChoice) GetSource() shared.ChoiceSource         { return c.Source }
func (c ToolProficiencyChoice) GetChoiceID() string                    { return c.ChoiceID }
func (c ToolProficiencyChoice) GetCategory() shared.ChoiceCategory     { return shared.ChoiceToolProficiency }

// InstrumentProficiencyChoice represents choosing musical instrument proficiencies
// This is separate from ToolProficiencyChoice as D&D 5e treats them distinctly
type InstrumentProficiencyChoice struct {
	Source      shared.ChoiceSource
	ChoiceID    string
	Instruments []string
}

func (c InstrumentProficiencyChoice) GetSource() shared.ChoiceSource     { return c.Source }
func (c InstrumentProficiencyChoice) GetChoiceID() string                { return c.ChoiceID }
func (c InstrumentProficiencyChoice) GetCategory() shared.ChoiceCategory { return shared.ChoiceToolProficiency } // For now, until we add a separate category