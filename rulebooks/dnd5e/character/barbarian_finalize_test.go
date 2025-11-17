package character

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/suite"
)

// BarbarianFinalizeSuite tests the specific issue with Barbarian finalization
type BarbarianFinalizeSuite struct {
	suite.Suite
	eventBus events.EventBus
}

// SetupTest runs before each test
func (s *BarbarianFinalizeSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
}

// TestCompleteHumanBarbarianFinalization tests creating a complete Human Barbarian
// and successfully finalizing it to a Character
func (s *BarbarianFinalizeSuite) TestCompleteHumanBarbarianFinalization() {
	// Create a new draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-draft-1",
		PlayerID: "player-1",
	})
	s.Require().NoError(err)
	s.Require().NotNil(draft)

	// Set name
	err = draft.SetName(&SetNameInput{
		Name: "Grog the Rager",
	})
	s.Require().NoError(err)
	s.T().Logf("After SetName: progress = %d (0x%X)", draft.Progress(), draft.Progress())

	// Set race (Human with Dwarvish language)
	err = draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Dwarvish},
		},
	})
	s.Require().NoError(err)
	s.T().Logf("After SetRace: progress = %d (0x%X)", draft.Progress(), draft.Progress())
	s.T().Logf("Race choices stored: %+v", draft.Choices())

	// Set class (Barbarian with all required choices)
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Intimidation,
			},
			Equipment: map[choices.ChoiceID]shared.SelectionID{
				choices.BarbarianWeaponsPrimary:   choices.BarbarianWeaponGreataxe,    // "barbarian-weapon-a"
				choices.BarbarianWeaponsSecondary: choices.BarbarianSecondaryHandaxes, // "barbarian-secondary-a"
				choices.BarbarianPack:             choices.BarbarianPackExplorer,      // "barbarian-pack-a"
			},
		},
	})
	s.Require().NoError(err)
	s.T().Logf("After SetClass: progress = %d (0x%X)", draft.Progress(), draft.Progress())
	s.T().Logf("All choices after SetClass:")
	for i, choice := range draft.Choices() {
		s.T().Logf("  %d. category=%s source=%s choiceID=%s optionID=%s",
			i+1, choice.Category, choice.Source, choice.ChoiceID, choice.OptionID)
		if len(choice.EquipmentSelection) > 0 {
			s.T().Logf("     equipment: %v", choice.EquipmentSelection)
		}
		if len(choice.SkillSelection) > 0 {
			s.T().Logf("     skills: %v", choice.SkillSelection)
		}
	}

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
	})
	s.Require().NoError(err)
	s.T().Logf("After SetBackground: progress = %d (0x%X)", draft.Progress(), draft.Progress())

	// Set ability scores
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 15,
			abilities.INT: 8,
			abilities.WIS: 12,
			abilities.CHA: 10,
		},
		Method: "standard-array",
	})
	s.Require().NoError(err)
	s.T().Logf("After SetAbilityScores: progress = %d (0x%X)", draft.Progress(), draft.Progress())

	// Debug: Check if class is complete
	isClassComplete := draft.IsClassComplete()
	s.T().Logf("IsClassComplete() = %v", isClassComplete)

	// Debug: Validate choices explicitly
	err = draft.ValidateChoices()
	s.T().Logf("ValidateChoices() error = %v", err)

	// Check progress flags
	hasName := draft.Progress().Has(ProgressName)
	hasRace := draft.Progress().Has(ProgressRace)
	hasClass := draft.Progress().Has(ProgressClass)
	hasBackground := draft.Progress().Has(ProgressBackground)
	hasAbilityScores := draft.Progress().Has(ProgressAbilityScores)

	s.T().Logf("Progress flags: name=%v race=%v class=%v background=%v ability_scores=%v",
		hasName, hasRace, hasClass, hasBackground, hasAbilityScores)

	// Try to convert to character
	char, err := draft.ToCharacter(context.Background(), "char-1", s.eventBus)
	s.Require().NoError(err, "Failed to convert draft to character")
	s.Require().NotNil(char)

	// Verify character properties
	data := char.ToData()
	s.Equal("Grog the Rager", data.Name)
	s.Equal(races.Human, data.RaceID)
	s.Equal(classes.Barbarian, data.ClassID)
	// TODO: Background is not stored in Character - separate issue tracked separately
	// s.Equal(backgrounds.Soldier, data.BackgroundID)
	s.Equal(1, data.Level)
}

// TestBarbarianClassComplete tests the IsClassComplete method specifically
func (s *BarbarianFinalizeSuite) TestBarbarianClassComplete() {
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-draft-2",
		PlayerID: "player-2",
	})
	s.Require().NoError(err)

	// Set only class
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Intimidation,
			},
			Equipment: map[choices.ChoiceID]shared.SelectionID{
				choices.BarbarianWeaponsPrimary:   choices.BarbarianWeaponGreataxe,
				choices.BarbarianWeaponsSecondary: choices.BarbarianSecondaryHandaxes,
				choices.BarbarianPack:             choices.BarbarianPackExplorer,
			},
		},
	})
	s.Require().NoError(err)

	s.T().Logf("Class choices after SetClass:")
	for i, choice := range draft.Choices() {
		if choice.Source == shared.SourceClass {
			s.T().Logf("  %d. category=%s choiceID=%s optionID=%s",
				i+1, choice.Category, choice.ChoiceID, choice.OptionID)
		}
	}

	// Check if class is complete
	isComplete := draft.IsClassComplete()
	s.T().Logf("IsClassComplete() = %v", isComplete)
	s.True(isComplete, "Barbarian class should be complete with all choices")
}

// TestBarbarianEquipmentValidation tests the equipment validation directly
func (s *BarbarianFinalizeSuite) TestBarbarianEquipmentValidation() {
	// Get Barbarian requirements
	reqs := choices.GetClassRequirements(classes.Barbarian)
	s.Require().NotNil(reqs)
	s.Require().NotNil(reqs.Skills, "Barbarian should have skill requirements")
	s.Require().NotEmpty(reqs.Equipment, "Barbarian should have equipment requirements")

	s.T().Logf("Barbarian requirements:")
	s.T().Logf("  Skills: %d required from %d options", reqs.Skills.Count, len(reqs.Skills.Options))
	s.T().Logf("  Equipment choices: %d", len(reqs.Equipment))
	for i, equipReq := range reqs.Equipment {
		s.T().Logf("    %d. ID=%s label=%q choose=%d options=%d",
			i+1, equipReq.ID, equipReq.Label, equipReq.Choose, len(equipReq.Options))
	}

	// Create submissions matching what SetClass creates
	subs := choices.NewSubmissions()

	// Add skill submission
	subs.Add(choices.Submission{
		Category: shared.ChoiceSkills,
		Source:   shared.SourceClass,
		ChoiceID: choices.BarbarianSkills,
		Values: []shared.SelectionID{
			skills.Athletics,
			skills.Intimidation,
		},
	})

	// Add equipment submissions (matching the test data pattern)
	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.BarbarianWeaponsPrimary,
		OptionID: choices.BarbarianWeaponGreataxe,
		Values: []shared.SelectionID{
			choices.BarbarianWeaponGreataxe,
		},
	})

	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.BarbarianWeaponsSecondary,
		OptionID: choices.BarbarianSecondaryHandaxes,
		Values: []shared.SelectionID{
			choices.BarbarianSecondaryHandaxes,
		},
	})

	subs.Add(choices.Submission{
		Category: shared.ChoiceEquipment,
		Source:   shared.SourceClass,
		ChoiceID: choices.BarbarianPack,
		OptionID: choices.BarbarianPackExplorer,
		Values: []shared.SelectionID{
			choices.BarbarianPackExplorer,
		},
	})

	// Validate
	validator := choices.NewValidator()
	result := validator.Validate(reqs, subs)

	s.T().Logf("Validation result: valid=%v errors=%d", result.Valid, len(result.Errors))
	for i, err := range result.Errors {
		s.T().Logf("  Error %d: category=%s choiceID=%s message=%s",
			i+1, err.Category, err.ChoiceID, err.Message)
	}

	s.True(result.Valid, "Barbarian equipment submissions should be valid")
	s.Empty(result.Errors, "Should have no validation errors")
}

func TestBarbarianFinalizeSuite(t *testing.T) {
	suite.Run(t, new(BarbarianFinalizeSuite))
}
