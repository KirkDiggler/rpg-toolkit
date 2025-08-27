package character

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ConvertFromChoiceData converts old ChoiceData to new Choice interface types
// This is a temporary helper for migration
func ConvertFromChoiceData(old ChoiceData) (Choice, error) {
	switch old.Category {
	case shared.ChoiceName:
		if old.NameSelection == nil {
			return nil, fmt.Errorf("name choice missing name selection")
		}
		return NameChoice{
			Source:   old.Source,
			ChoiceID: old.ChoiceID,
			Name:     *old.NameSelection,
		}, nil

	case shared.ChoiceSkills:
		return SkillChoice{
			Source:   old.Source,
			ChoiceID: old.ChoiceID,
			Skills:   old.SkillSelection,
		}, nil

	case shared.ChoiceLanguages:
		return LanguageChoice{
			Source:    old.Source,
			ChoiceID:  old.ChoiceID,
			Languages: old.LanguageSelection,
		}, nil

	case shared.ChoiceAbilityScores:
		if old.AbilityScoreSelection == nil {
			return nil, fmt.Errorf("ability score choice missing selection")
		}
		return AbilityScoreChoice{
			Source:        old.Source,
			ChoiceID:      old.ChoiceID,
			AbilityScores: *old.AbilityScoreSelection,
		}, nil

	case shared.ChoiceFightingStyle:
		if old.FightingStyleSelection == nil {
			return nil, fmt.Errorf("fighting style choice missing selection")
		}
		return FightingStyleChoice{
			Source:        old.Source,
			ChoiceID:      old.ChoiceID,
			FightingStyle: *old.FightingStyleSelection,
		}, nil

	case shared.ChoiceEquipment:
		return EquipmentChoice{
			Source:    old.Source,
			ChoiceID:  old.ChoiceID,
			Equipment: old.EquipmentSelection,
		}, nil

	case shared.ChoiceRace:
		if old.RaceSelection == nil {
			return nil, fmt.Errorf("race choice missing selection")
		}
		return RaceSelectionChoice{
			Source:   old.Source,
			ChoiceID: old.ChoiceID,
			Race:     old.RaceSelection.RaceID,
			Subrace:  old.RaceSelection.SubraceID,
		}, nil

	case shared.ChoiceClass:
		if old.ClassSelection == nil {
			return nil, fmt.Errorf("class choice missing selection")
		}
		return ClassSelectionChoice{
			Source:   old.Source,
			ChoiceID: old.ChoiceID,
			Class:    old.ClassSelection.ClassID,
		}, nil

	case shared.ChoiceBackground:
		if old.BackgroundSelection == nil {
			return nil, fmt.Errorf("background choice missing selection")
		}
		return BackgroundChoice{
			Source:     old.Source,
			ChoiceID:   old.ChoiceID,
			Background: *old.BackgroundSelection,
		}, nil

	case shared.ChoiceSpells:
		return SpellChoice{
			Source:   old.Source,
			ChoiceID: old.ChoiceID,
			Spells:   old.SpellSelection,
		}, nil

	case shared.ChoiceCantrips:
		return CantripChoice{
			Source:   old.Source,
			ChoiceID: old.ChoiceID,
			Cantrips: old.CantripSelection,
		}, nil

	case shared.ChoiceExpertise:
		return ExpertiseChoice{
			Source:    old.Source,
			ChoiceID:  old.ChoiceID,
			Expertise: old.ExpertiseSelection,
		}, nil

	case shared.ChoiceTraits:
		return TraitChoice{
			Source:   old.Source,
			ChoiceID: old.ChoiceID,
			Traits:   old.TraitSelection,
		}, nil

	case shared.ChoiceToolProficiency:
		// For now, treat all tool proficiencies the same
		// Later we can distinguish between tools and instruments
		return ToolProficiencyChoice{
			Source:   old.Source,
			ChoiceID: old.ChoiceID,
			Tools:    old.ToolProficiencySelection,
		}, nil

	default:
		return nil, fmt.Errorf("unknown choice category: %s", old.Category)
	}
}

// convertChoiceSource converts shared.ChoiceSource to choices.Source
func convertChoiceSource(source shared.ChoiceSource) choices.Source {
	switch source {
	case shared.SourceClass:
		return choices.SourceClass
	case shared.SourceRace:
		return choices.SourceRace
	case shared.SourceSubrace:
		return choices.SourceSubrace
	case shared.SourceBackground:
		return choices.SourceBackground
	case shared.SourceSubclass:
		return choices.SourceMulticlass // Map subclass to multiclass for now
	default:
		return choices.SourceManual
	}
}

// convertChoiceCategory converts shared.ChoiceCategory to choices.Field
func convertChoiceCategory(category shared.ChoiceCategory) choices.Field {
	switch category {
	case shared.ChoiceSkills:
		return choices.FieldSkills
	case shared.ChoiceLanguages:
		return choices.FieldLanguages
	case shared.ChoiceFightingStyle:
		return choices.FieldFightingStyle
	case shared.ChoiceExpertise:
		return choices.FieldExpertise
	case shared.ChoiceCantrips:
		return choices.FieldCantrips
	case shared.ChoiceSpells:
		return choices.FieldSpells
	case shared.ChoiceEquipment:
		return choices.FieldEquipment
	case shared.ChoiceAbilityScores:
		return choices.FieldAbilityScores
	case shared.ChoiceToolProficiency:
		return choices.FieldInstruments // Map tools to instruments for now
	default:
		return choices.Field(string(category)) // Fallback to string conversion
	}
}

// extractChoiceValues extracts string values from a ChoiceData for validation
func extractChoiceValues(choice ChoiceData) []string {
	switch choice.Category {
	case shared.ChoiceSkills:
		values := make([]string, len(choice.SkillSelection))
		for i, skill := range choice.SkillSelection {
			values[i] = string(skill)
		}
		return values
	case shared.ChoiceLanguages:
		values := make([]string, len(choice.LanguageSelection))
		for i, lang := range choice.LanguageSelection {
			values[i] = string(lang)
		}
		return values
	case shared.ChoiceFightingStyle:
		if choice.FightingStyleSelection != nil && *choice.FightingStyleSelection != "" {
			return []string{*choice.FightingStyleSelection}
		}
	case shared.ChoiceEquipment:
		return choice.EquipmentSelection
	case shared.ChoiceSpells:
		return choice.SpellSelection
	case shared.ChoiceCantrips:
		return choice.CantripSelection
	case shared.ChoiceExpertise:
		return choice.ExpertiseSelection
	case shared.ChoiceToolProficiency:
		return choice.ToolProficiencySelection
	}
	return []string{}
}

// ConvertToChoiceData converts new Choice types back to old ChoiceData
// This is temporary for backward compatibility during migration
func ConvertToChoiceData(choice Choice) ChoiceData {
	data := ChoiceData{
		Category: choice.GetCategory(),
		Source:   choice.GetSource(),
		ChoiceID: choice.GetChoiceID(),
	}

	switch c := choice.(type) {
	case NameChoice:
		name := c.Name
		data.NameSelection = &name

	case SkillChoice:
		data.SkillSelection = c.Skills

	case LanguageChoice:
		data.LanguageSelection = c.Languages

	case AbilityScoreChoice:
		data.AbilityScoreSelection = &c.AbilityScores

	case FightingStyleChoice:
		data.FightingStyleSelection = &c.FightingStyle

	case EquipmentChoice:
		data.EquipmentSelection = c.Equipment

	case RaceSelectionChoice:
		data.RaceSelection = &RaceChoice{
			RaceID:    c.Race,
			SubraceID: c.Subrace,
		}

	case ClassSelectionChoice:
		data.ClassSelection = &ClassChoice{
			ClassID: c.Class,
		}

	case BackgroundChoice:
		data.BackgroundSelection = &c.Background

	case SpellChoice:
		data.SpellSelection = c.Spells

	case CantripChoice:
		data.CantripSelection = c.Cantrips

	case ExpertiseChoice:
		data.ExpertiseSelection = c.Expertise

	case TraitChoice:
		data.TraitSelection = c.Traits

	case ToolProficiencyChoice:
		data.ToolProficiencySelection = c.Tools

	case InstrumentProficiencyChoice:
		// Map back to tool proficiency for now
		data.ToolProficiencySelection = c.Instruments
	}

	return data
}
