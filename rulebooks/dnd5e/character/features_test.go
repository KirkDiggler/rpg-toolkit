package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

type FeaturesTestSuite struct {
	suite.Suite
}

func (s *FeaturesTestSuite) TestBarbarianGetsRageFeature() {
	// Create a draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-barbarian",
		PlayerID: "player1",
	})
	s.Require().NoError(err)

	// Set name
	err = draft.SetName(&SetNameInput{Name: "Conan"})
	s.Require().NoError(err)

	// Set race to human
	err = draft.SetRace(&SetRaceInput{
		RaceID:    races.Human,
		SubraceID: "", // Human has no subrace
		Choices:   RaceChoices{},
	})
	s.Require().NoError(err)

	// Set class to barbarian
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Barbarian,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.Survival,
			}, // Barbarian needs to choose 2 skills
		},
	})
	s.Require().NoError(err)

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices:      BackgroundChoices{},
	})
	s.Require().NoError(err)

	// Set ability scores
	scores := shared.AbilityScores{
		abilities.STR: 15,
		abilities.DEX: 14,
		abilities.CON: 13,
		abilities.INT: 12,
		abilities.WIS: 10,
		abilities.CHA: 8,
	}
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: scores,
		Method: "standard",
	})
	s.Require().NoError(err)

	// Convert to character
	character, err := draft.ToCharacter("char-1")
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Check that the barbarian has features
	features := character.GetFeatures()
	s.Require().Len(features, 1, "Barbarian should have 1 feature at level 1")

	// Verify the rage feature
	rageFeature := character.GetFeature("rage")
	s.Require().NotNil(rageFeature)
	s.Equal("rage", rageFeature.GetID())

	// Test we can also get it by index
	s.Equal(rageFeature, features[0])
}

func (s *FeaturesTestSuite) TestNonBarbarianHasNoFeatures() {
	// Create a draft
	draft, err := NewDraft(&DraftConfig{
		ID:       "test-fighter",
		PlayerID: "player1",
	})
	s.Require().NoError(err)

	// Set name
	err = draft.SetName(&SetNameInput{Name: "Arthur"})
	s.Require().NoError(err)

	// Set race to human
	err = draft.SetRace(&SetRaceInput{
		RaceID:    races.Human,
		SubraceID: "",
		Choices:   RaceChoices{},
	})
	s.Require().NoError(err)

	// Set class to fighter
	err = draft.SetClass(&SetClassInput{
		ClassID: classes.Fighter,
		Choices: ClassChoices{
			Skills: []skills.Skill{
				skills.Athletics,
				skills.History,
			}, // Fighter needs to choose 2 skills
		},
	})
	s.Require().NoError(err)

	// Set background
	err = draft.SetBackground(&SetBackgroundInput{
		BackgroundID: backgrounds.Soldier,
		Choices:      BackgroundChoices{},
	})
	s.Require().NoError(err)

	// Set ability scores
	scores := shared.AbilityScores{
		abilities.STR: 15,
		abilities.DEX: 14,
		abilities.CON: 13,
		abilities.INT: 12,
		abilities.WIS: 10,
		abilities.CHA: 8,
	}
	err = draft.SetAbilityScores(&SetAbilityScoresInput{
		Scores: scores,
		Method: "standard",
	})
	s.Require().NoError(err)

	// Convert to character
	character, err := draft.ToCharacter("char-1")
	s.Require().NoError(err)
	s.Require().NotNil(character)

	// Check that the fighter has no features yet
	// TODO: Fighter should have Second Wind at level 1
	features := character.GetFeatures()
	s.Len(features, 0, "Fighter has no features implemented yet")
}

func TestFeaturesTestSuite(t *testing.T) {
	suite.Run(t, new(FeaturesTestSuite))
}
