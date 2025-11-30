package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// MonkFinalizeSuite tests Monk-specific finalization
type MonkFinalizeSuite struct {
	suite.Suite
	eventBus events.EventBus
}

// SetupTest runs before each test
func (s *MonkFinalizeSuite) SetupTest() {
	s.eventBus = events.NewEventBus()
}

// TestMonkWithUnarmoredDefenseAndMartialArts tests that a Monk gets both
// Unarmored Defense and Martial Arts conditions when finalized
func (s *MonkFinalizeSuite) TestMonkWithUnarmoredDefenseAndMartialArts() {
	// Create a new draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-monk-draft",
		PlayerID: "player-1",
	})
	s.Require().NoError(err)
	s.Require().NotNil(draft)

	// Set name
	err = draft.SetName(&SetNameInput{
		Name: "Chen Wei the Swift",
	})
	s.Require().NoError(err)

	// Set race (Human)
	err = draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Elvish},
		},
	})
	s.Require().NoError(err)

	// Set class (Monk)
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Monk,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Acrobatics,
				skills.Stealth,
			},
			Equipment: []EquipmentChoiceSelection{
				{ChoiceID: choices.MonkWeaponsPrimary, OptionID: choices.MonkWeaponShortsword},
				{ChoiceID: choices.MonkPack, OptionID: choices.MonkPackDungeoneer},
			},
		},
	})
	s.Require().NoError(err)

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Hermit,
	})
	s.Require().NoError(err)

	// Set ability scores (DEX and WIS focused for Monk)
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 16,
			abilities.CON: 14,
			abilities.INT: 8,
			abilities.WIS: 15,
			abilities.CHA: 12,
		},
		Method: "standard-array",
	})
	s.Require().NoError(err)

	// Finalize to character
	char, err := draft.ToCharacter(context.Background(), "monk-1", s.eventBus)
	s.Require().NoError(err, "Failed to convert draft to character")
	s.Require().NotNil(char)

	// Verify character was created correctly
	s.Equal("Chen Wei the Swift", char.GetName())
	data := char.ToData()
	s.Equal(classes.Monk, data.ClassID)

	// Verify both conditions were applied (Unarmored Defense and Martial Arts)
	charConditions := char.GetConditions()
	s.Require().Len(charConditions, 2, "Character should have exactly 2 conditions (Unarmored Defense and Martial Arts)")

	// Find and verify UnarmoredDefenseCondition
	var udCondition *conditions.UnarmoredDefenseCondition
	var maCondition *conditions.MartialArtsCondition

	for _, cond := range charConditions {
		switch c := cond.(type) {
		case *conditions.UnarmoredDefenseCondition:
			udCondition = c
		case *conditions.MartialArtsCondition:
			maCondition = c
		}
	}

	// Verify Unarmored Defense
	s.Require().NotNil(udCondition, "Character should have UnarmoredDefenseCondition")
	s.Equal(conditions.UnarmoredDefenseMonk, udCondition.Type, "Unarmored Defense should be Monk type (WIS-based)")
	s.Equal("monk-1", udCondition.CharacterID)
	s.Equal("monk:unarmored_defense", udCondition.Source)

	// Verify Martial Arts
	s.Require().NotNil(maCondition, "Character should have MartialArtsCondition")
	s.Equal("monk-1", maCondition.CharacterID)
	s.Equal("1d4", maCondition.GetUnarmedDamage(), "Level 1 Monk should have 1d4 unarmed strike")
}

// TestMonkUnarmoredDefenseACCalculation tests that the Monk's Unarmored Defense
// correctly calculates AC using DEX + WIS
func (s *MonkFinalizeSuite) TestMonkUnarmoredDefenseACCalculation() {
	// Create a draft with specific ability scores to test AC calculation
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-monk-ac",
		PlayerID: "player-2",
	})
	s.Require().NoError(err)

	// Set name
	err = draft.SetName(&SetNameInput{Name: "Test AC Monk"})
	s.Require().NoError(err)

	// Set race
	err = draft.SetRace(&SetRaceInput{
		RaceID: races.Human,
		Choices: RaceChoices{
			Languages: []languages.Language{languages.Common},
		},
	})
	s.Require().NoError(err)

	// Set class
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Monk,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Acrobatics,
				skills.Insight,
			},
			Equipment: []EquipmentChoiceSelection{
				{
					ChoiceID:           choices.MonkWeaponsPrimary,
					OptionID:           choices.MonkWeaponSimple,
					CategorySelections: []shared.EquipmentID{weapons.Quarterstaff},
				},
				{ChoiceID: choices.MonkPack, OptionID: choices.MonkPackExplorer},
			},
		},
	})
	s.Require().NoError(err)

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Acolyte,
	})
	s.Require().NoError(err)

	// Set ability scores: DEX 16 (+3), WIS 14 (+2) = AC 15
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 16, // +3 modifier
			abilities.CON: 12,
			abilities.INT: 8,
			abilities.WIS: 14, // +2 modifier
			abilities.CHA: 10,
		},
		Method: "standard-array",
	})
	s.Require().NoError(err)

	// Finalize to character
	char, err := draft.ToCharacter(context.Background(), "monk-ac-test", s.eventBus)
	s.Require().NoError(err)
	s.Require().NotNil(char)

	// Get the UnarmoredDefenseCondition
	charConditions := char.GetConditions()
	var udCondition *conditions.UnarmoredDefenseCondition
	for _, cond := range charConditions {
		if ud, ok := cond.(*conditions.UnarmoredDefenseCondition); ok {
			udCondition = ud
			break
		}
	}
	s.Require().NotNil(udCondition)

	// Calculate expected AC: 10 + DEX(3) + WIS(2) = 15
	// Note: We need to use the character's final ability scores (with racial bonuses)
	charData := char.ToData()
	calculatedAC := udCondition.CalculateAC(charData.AbilityScores)
	s.Equal(15, calculatedAC, "Unarmored Defense AC should be 10 + DEX(+3) + WIS(+2) = 15")
}

// TestMonkMartialArtsDieLevels tests that the Martial Arts damage die
// scales correctly with monk level
func (s *MonkFinalizeSuite) TestMonkMartialArtsDieLevels() {
	testCases := []struct {
		level       int
		expectedDie string
	}{
		{1, "1d4"},
		{4, "1d4"},
		{5, "1d6"},
		{10, "1d6"},
		{11, "1d8"},
		{16, "1d8"},
		{17, "1d10"},
		{20, "1d10"},
	}

	for _, tc := range testCases {
		s.Run("Level "+string(rune('0'+tc.level/10))+string(rune('0'+tc.level%10)), func() {
			// Create martial arts condition at the specified level
			maCondition := conditions.NewMartialArtsCondition(conditions.MartialArtsConditionConfig{
				CharacterID: "test-monk",
				Level:       tc.level,
			})

			s.Equal(tc.expectedDie, maCondition.GetUnarmedDamage(),
				"Level %d monk should have %s unarmed damage", tc.level, tc.expectedDie)
		})
	}
}

func TestMonkFinalizeSuite(t *testing.T) {
	suite.Run(t, new(MonkFinalizeSuite))
}
