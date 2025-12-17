// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/backgrounds"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// UnarmoredDefenseACTestSuite tests that Unarmored Defense is correctly applied
// to character AC during character creation.
// Issue #450: Character AC should use UnarmoredDefenseCondition.CalculateAC()
// for Barbarians and Monks instead of hardcoded 10 + DEX.
type UnarmoredDefenseACTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *UnarmoredDefenseACTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// SetupSubTest creates a fresh event bus for each subtest in table-driven tests
func (s *UnarmoredDefenseACTestSuite) SetupSubTest() {
	s.bus = events.NewEventBus()
}

func TestUnarmoredDefenseACTestSuite(t *testing.T) {
	suite.Run(t, new(UnarmoredDefenseACTestSuite))
}

// TestUnarmoredDefenseAC tests AC calculation for classes with and without Unarmored Defense
func (s *UnarmoredDefenseACTestSuite) TestUnarmoredDefenseAC() {
	testCases := []struct {
		name          string
		class         classes.Class
		classSkills   []skills.Skill
		background    backgrounds.Background
		scores        shared.AbilityScores
		expectedAC    int
		acExplanation string
	}{
		{
			name:        "Barbarian gets CON bonus to AC",
			class:       classes.Barbarian,
			classSkills: []skills.Skill{skills.Athletics, skills.Intimidation},
			background:  backgrounds.Soldier,
			scores: shared.AbilityScores{
				abilities.STR: 15,
				abilities.DEX: 14, // +2 modifier
				abilities.CON: 16, // +3 modifier
				abilities.INT: 8,
				abilities.WIS: 10,
				abilities.CHA: 10,
			},
			expectedAC:    15,
			acExplanation: "10 + DEX (+2) + CON (+3) = 15",
		},
		{
			name:        "Monk gets WIS bonus to AC",
			class:       classes.Monk,
			classSkills: []skills.Skill{skills.Acrobatics, skills.Stealth},
			background:  backgrounds.Hermit,
			scores: shared.AbilityScores{
				abilities.STR: 10,
				abilities.DEX: 16, // +3 modifier
				abilities.CON: 12,
				abilities.INT: 10,
				abilities.WIS: 14, // +2 modifier
				abilities.CHA: 8,
			},
			expectedAC:    15,
			acExplanation: "10 + DEX (+3) + WIS (+2) = 15",
		},
		{
			name:        "Fighter uses base AC (no Unarmored Defense)",
			class:       classes.Fighter,
			classSkills: []skills.Skill{skills.Athletics, skills.Intimidation},
			background:  backgrounds.Soldier,
			scores: shared.AbilityScores{
				abilities.STR: 16,
				abilities.DEX: 14, // +2 modifier
				abilities.CON: 14, // +2 modifier (not used for AC)
				abilities.INT: 10,
				abilities.WIS: 10,
				abilities.CHA: 10,
			},
			expectedAC:    12,
			acExplanation: "10 + DEX (+2) = 12",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Arrange
			draft := LoadDraftFromData(&DraftData{
				ID:       "test-char",
				PlayerID: "player1",
			})

			s.Require().NoError(draft.SetName(&SetNameInput{Name: "Test Character"}))
			s.Require().NoError(draft.SetRace(&SetRaceInput{RaceID: races.Human}))
			s.Require().NoError(draft.SetClass(&SetClassInput{
				ClassID: tc.class,
				Choices: ClassChoices{Skills: tc.classSkills},
			}))
			s.Require().NoError(draft.SetBackground(&SetBackgroundInput{
				BackgroundID: tc.background,
			}))
			s.Require().NoError(draft.SetAbilityScores(&SetAbilityScoresInput{
				Scores: tc.scores,
			}))

			// Act
			char, err := draft.ToCharacter(s.ctx, "char-test", s.bus)
			s.Require().NoError(err)
			s.Require().NotNil(char)

			// Assert
			data := char.ToData()
			s.Equal(tc.expectedAC, data.ArmorClass,
				"AC should be %s, but got %d", tc.acExplanation, data.ArmorClass)
		})
	}
}
