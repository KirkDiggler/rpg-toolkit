// Package character provides D&D 5e character creation and management functionality
package character

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Validator handles D&D 5e specific validation rules
type Validator struct {
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateDraft checks if the draft is valid in its current state
func (v *Validator) ValidateDraft(draft *Draft, raceData *race.Data, classData *class.Data,
	backgroundData *shared.Background) []ValidationError {
	errors := []ValidationError{}

	// Name validation
	if draft.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: "name is required",
		})
	}

	// Race validation if selected
	if raceChoice, ok := draft.Choices[shared.ChoiceRace].(RaceChoice); ok {
		if err := v.ValidateRaceChoice(raceChoice, raceData); err != nil {
			errors = append(errors, ValidationError{
				Field:   "race",
				Message: err.Error(),
			})
		}
	}

	// Ability scores validation if set
	if scores, ok := draft.Choices[shared.ChoiceAbilityScores].(shared.AbilityScores); ok {
		if err := v.ValidateAbilityScores(scores); err != nil {
			errors = append(errors, ValidationError{
				Field:   "ability_scores",
				Message: err.Error(),
			})
		}
	}

	// Skills validation if selected
	if skills, ok := draft.Choices[shared.ChoiceSkills].([]string); ok {
		if err := v.ValidateSkillSelection(draft, skills, classData, backgroundData); err != nil {
			errors = append(errors, ValidationError{
				Field:   "skills",
				Message: err.Error(),
			})
		}
	}

	return errors
}

// ValidateRaceChoice validates race and subrace selection
func (v *Validator) ValidateRaceChoice(choice RaceChoice, raceData *race.Data) error {
	if raceData == nil {
		return fmt.Errorf("race data is required")
	}

	if choice.RaceID != raceData.ID {
		return fmt.Errorf("race choice does not match provided race data")
	}

	// Validate subrace if provided
	if choice.SubraceID != "" {
		// Check if subrace exists in race data
		found := false
		for _, subrace := range raceData.Subraces {
			if subrace.ID == choice.SubraceID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid subrace %s for race %s", choice.SubraceID, raceData.Name)
		}
	}

	return nil
}

// ValidateAbilityScores validates ability score assignments
func (v *Validator) ValidateAbilityScores(scores shared.AbilityScores) error {
	// Standard array: 15, 14, 13, 12, 10, 8
	// Point buy: 27 points, scores 8-15 before racial modifiers
	// Rolled: Each score 3-18

	// Check minimum and maximum values
	if scores.Strength < 3 || scores.Strength > 20 {
		return fmt.Errorf("strength must be between 3 and 20")
	}
	if scores.Dexterity < 3 || scores.Dexterity > 20 {
		return fmt.Errorf("dexterity must be between 3 and 20")
	}
	if scores.Constitution < 3 || scores.Constitution > 20 {
		return fmt.Errorf("constitution must be between 3 and 20")
	}
	if scores.Intelligence < 3 || scores.Intelligence > 20 {
		return fmt.Errorf("intelligence must be between 3 and 20")
	}
	if scores.Wisdom < 3 || scores.Wisdom > 20 {
		return fmt.Errorf("wisdom must be between 3 and 20")
	}
	if scores.Charisma < 3 || scores.Charisma > 20 {
		return fmt.Errorf("charisma must be between 3 and 20")
	}

	// TODO: Validate based on character creation method (point buy, standard array, rolled)

	return nil
}

// ValidateSkillSelection validates skill proficiency choices
func (v *Validator) ValidateSkillSelection(_ *Draft, skills []string, classData *class.Data,
	backgroundData *shared.Background) error {
	if classData == nil {
		return fmt.Errorf("class data is required for skill validation")
	}

	if backgroundData == nil {
		return fmt.Errorf("background data is required for skill validation")
	}

	// Check that the number of skills matches what the class allows
	if len(skills) != classData.SkillProficiencyCount {
		return fmt.Errorf("must choose exactly %d skills, got %d", classData.SkillProficiencyCount, len(skills))
	}

	// Check that all selected skills are valid options for the class
	for _, skill := range skills {
		found := false
		for _, option := range classData.SkillOptions {
			if skill == option {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("skill %s is not available for this class", skill)
		}
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, skill := range skills {
		if seen[skill] {
			return fmt.Errorf("duplicate skill selection: %s", skill)
		}
		seen[skill] = true
	}

	return nil
}

// ValidateEquipmentChoice validates equipment selections
func (v *Validator) ValidateEquipmentChoice(_ *Draft, _ []string) error {
	// TODO: Validate equipment choices based on class and background options
	return nil
}

// ValidateSpellSelection validates spell and cantrip choices
func (v *Validator) ValidateSpellSelection(_ *Draft, _ []string, _ []string) error {
	// TODO: Implement spell validation when spell data is available

	return nil
}
