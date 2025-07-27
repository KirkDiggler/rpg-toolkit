// Package character provides D&D 5e character creation and management functionality
package character

import (
	"errors"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Draft represents a character in progress
type Draft struct {
	ID        string
	PlayerID  string
	Name      string
	Choices   map[shared.ChoiceCategory]any
	Progress  DraftProgress
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DraftProgress tracks completion of character creation steps
type DraftProgress struct {
	flags uint32
}

// ToCharacter converts a completed draft into a playable character
// This method validates the draft is complete and creates a fully initialized character
func (d *Draft) ToCharacter(raceData *race.Data, classData *class.Data,
	backgroundData *shared.Background) (*Character, error) {
	// Validate we have all required data
	if raceData == nil || classData == nil || backgroundData == nil {
		return nil, errors.New("race, class, and background data are required")
	}

	// Check if draft is complete enough to build
	if !d.isComplete() {
		return nil, errors.New("draft is incomplete - missing required choices")
	}

	// Validate the draft with external data
	validator := NewValidator()
	errors := validator.ValidateDraft(d, raceData, classData, backgroundData)
	if len(errors) > 0 {
		return nil, fmt.Errorf("validation failed: %v", errors)
	}

	// Compile the character using the same logic as builder
	return d.compileCharacter(raceData, classData, backgroundData)
}

// isComplete checks if the draft has all required fields to create a character
func (d *Draft) isComplete() bool {
	required := ProgressName | ProgressRace | ProgressClass | ProgressBackground | ProgressAbilityScores
	return d.Progress.flags&required == required
}

// compileCharacter creates a character from the draft data
func (d *Draft) compileCharacter(raceData *race.Data, classData *class.Data,
	backgroundData *shared.Background) (*Character, error) {
	// Start with base character data
	charData := Data{
		ID:           d.ID,
		PlayerID:     d.PlayerID,
		Name:         d.Name,
		Level:        1, // Starting level
		RaceID:       raceData.ID,
		ClassID:      classData.ID,
		BackgroundID: backgroundData.ID,
	}

	// Extract subrace ID if present
	if raceChoice, ok := d.Choices[shared.ChoiceRace].(RaceChoice); ok && raceChoice.SubraceID != "" {
		charData.SubraceID = raceChoice.SubraceID
	}

	// Get ability scores from choices
	if scores, ok := d.Choices[shared.ChoiceAbilityScores].(shared.AbilityScores); ok {
		charData.AbilityScores = scores

		// Apply racial ability score improvements
		applyAbilityScoreIncreases(&charData.AbilityScores, raceData.AbilityScoreIncreases)

		// Apply subrace ability score improvements if applicable
		if raceChoice, ok := d.Choices[shared.ChoiceRace].(RaceChoice); ok && raceChoice.SubraceID != "" {
			// Find the subrace data
			for _, subrace := range raceData.Subraces {
				if subrace.ID == raceChoice.SubraceID {
					applyAbilityScoreIncreases(&charData.AbilityScores, subrace.AbilityScoreIncreases)
					break
				}
			}
		}
	}

	// Calculate HP
	charData.MaxHitPoints = classData.HitDice + ((charData.AbilityScores.Constitution - 10) / 2)
	charData.HitPoints = charData.MaxHitPoints

	// Physical characteristics from race
	charData.Speed = raceData.Speed
	charData.Size = raceData.Size

	// Skills
	charData.Skills = make(map[string]int)
	if skills, ok := d.Choices[shared.ChoiceSkills].([]string); ok {
		for _, skill := range skills {
			charData.Skills[skill] = int(shared.Proficient)
		}
	}
	// Add background skills
	for _, skill := range backgroundData.SkillProficiencies {
		charData.Skills[skill] = int(shared.Proficient)
	}

	// Languages
	charData.Languages = append([]string{}, raceData.Languages...)
	charData.Languages = append(charData.Languages, backgroundData.Languages...)
	// TODO(#106): Add language choices

	// Proficiencies
	charData.Proficiencies = shared.Proficiencies{
		Armor:   classData.ArmorProficiencies,
		Weapons: append(classData.WeaponProficiencies, raceData.WeaponProficiencies...),
		Tools:   append(classData.ToolProficiencies, backgroundData.ToolProficiencies...),
	}

	// Saving throws
	charData.SavingThrows = make(map[string]int)
	for _, save := range classData.SavingThrows {
		charData.SavingThrows[save] = int(shared.Proficient)
	}

	// Store choices made
	charData.Choices = []ChoiceData{}
	for category, choice := range d.Choices {
		// Determine the source based on the choice category
		source := "player" // Default for player-made choices
		switch category {
		case shared.ChoiceRace, shared.ChoiceSubrace, shared.ChoiceLanguages:
			// Languages can come from race or background, but if it's a choice, it's usually race
			source = "race"
		case shared.ChoiceClass, shared.ChoiceSkills, shared.ChoiceFightingStyle,
			shared.ChoiceSpells, shared.ChoiceCantrips, shared.ChoiceEquipment:
			source = "class"
		case shared.ChoiceBackground:
			source = "background"
		case shared.ChoiceAbilityScores, shared.ChoiceName:
			source = "player"
		}

		// Store all choices, not just shared.ChoiceData
		charData.Choices = append(charData.Choices, ChoiceData{
			Category:  string(category),
			Source:    source,
			Selection: choice,
		})
	}

	charData.CreatedAt = time.Now()
	charData.UpdatedAt = time.Now()

	// Create the character domain object
	return LoadCharacterFromData(charData, raceData, classData, backgroundData)
}

// LoadDraftFromData creates a Draft from persistent data
func LoadDraftFromData(data DraftData) (*Draft, error) {
	if data.ID == "" {
		return nil, errors.New("draft ID is required")
	}

	return &Draft{
		ID:        data.ID,
		PlayerID:  data.PlayerID,
		Name:      data.Name,
		Choices:   data.Choices,
		Progress:  DraftProgress{flags: data.ProgressFlags},
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}, nil
}

// ToData converts the draft to its persistent representation
func (d *Draft) ToData() DraftData {
	return DraftData{
		ID:            d.ID,
		PlayerID:      d.PlayerID,
		Name:          d.Name,
		Choices:       d.Choices,
		ProgressFlags: d.Progress.flags,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

// GetProgress returns information about the draft's completion status
func (d *Draft) GetProgress() DraftProgress {
	return d.Progress
}

// IsComplete returns true if the draft has all required fields to create a character
func (d *Draft) IsComplete() bool {
	return d.isComplete()
}

// Helper methods for DraftProgress

func (p *DraftProgress) setFlag(flag uint32) {
	p.flags |= flag
}

func (p *DraftProgress) hasFlag(flag uint32) bool {
	return p.flags&flag != 0
}

// applyAbilityScoreIncreases applies ability score increases to the given scores
func applyAbilityScoreIncreases(scores *shared.AbilityScores, increases map[string]int) {
	for ability, bonus := range increases {
		switch ability {
		case shared.AbilityStrength:
			scores.Strength += bonus
		case shared.AbilityDexterity:
			scores.Dexterity += bonus
		case shared.AbilityConstitution:
			scores.Constitution += bonus
		case shared.AbilityIntelligence:
			scores.Intelligence += bonus
		case shared.AbilityWisdom:
			scores.Wisdom += bonus
		case shared.AbilityCharisma:
			scores.Charisma += bonus
		}
	}
}
