package constants_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
)

type AbilitiesTestSuite struct {
	suite.Suite
}

func (s *AbilitiesTestSuite) TestAbilityConstants() {
	tests := []struct {
		name     string
		ability  constants.Ability
		expected string
		display  string
	}{
		{
			name:     "strength constant",
			ability:  constants.STR,
			expected: "str",
			display:  "Strength",
		},
		{
			name:     "dexterity constant",
			ability:  constants.DEX,
			expected: "dex",
			display:  "Dexterity",
		},
		{
			name:     "constitution constant",
			ability:  constants.CON,
			expected: "con",
			display:  "Constitution",
		},
		{
			name:     "intelligence constant",
			ability:  constants.INT,
			expected: "int",
			display:  "Intelligence",
		},
		{
			name:     "wisdom constant",
			ability:  constants.WIS,
			expected: "wis",
			display:  "Wisdom",
		},
		{
			name:     "charisma constant",
			ability:  constants.CHA,
			expected: "cha",
			display:  "Charisma",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Equal(tc.expected, string(tc.ability))
			s.Equal(tc.display, tc.ability.Display())
		})
	}
}

func (s *AbilitiesTestSuite) TestAbilityDisplay_UnknownAbility() {
	unknown := constants.Ability("unknown")
	s.Equal("unknown", unknown.Display())
}

func (s *AbilitiesTestSuite) TestAllAbilities() {
	abilities := constants.AllAbilities()
	s.Len(abilities, 6)

	// Verify standard order
	s.Equal(constants.STR, abilities[0])
	s.Equal(constants.DEX, abilities[1])
	s.Equal(constants.CON, abilities[2])
	s.Equal(constants.INT, abilities[3])
	s.Equal(constants.WIS, abilities[4])
	s.Equal(constants.CHA, abilities[5])
}

func TestAbilitiesTestSuite(t *testing.T) {
	suite.Run(t, new(AbilitiesTestSuite))
}
