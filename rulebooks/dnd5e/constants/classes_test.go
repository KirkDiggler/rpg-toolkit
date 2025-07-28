package constants_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
)

type ClassesTestSuite struct {
	suite.Suite
}

func (s *ClassesTestSuite) TestClassConstants() {
	tests := []struct {
		name        string
		class       constants.Class
		expected    string
		display     string
		hitDice     int
		primaryStat constants.Ability
	}{
		{
			name:        "barbarian class",
			class:       constants.ClassBarbarian,
			expected:    "barbarian",
			display:     "Barbarian",
			hitDice:     12,
			primaryStat: constants.STR,
		},
		{
			name:        "wizard class",
			class:       constants.ClassWizard,
			expected:    "wizard",
			display:     "Wizard",
			hitDice:     6,
			primaryStat: constants.INT,
		},
		{
			name:        "rogue class",
			class:       constants.ClassRogue,
			expected:    "rogue",
			display:     "Rogue",
			hitDice:     8,
			primaryStat: constants.DEX,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Equal(tc.expected, string(tc.class))
			s.Equal(tc.display, tc.class.Display())
			s.Equal(tc.hitDice, tc.class.HitDice())
			s.Equal(tc.primaryStat, tc.class.PrimaryStat())
		})
	}
}

func (s *ClassesTestSuite) TestClassDisplay_UnknownClass() {
	unknown := constants.Class("unknown")
	s.Equal("unknown", unknown.Display())
	s.Equal(0, unknown.HitDice())
	s.Equal(constants.Ability(""), unknown.PrimaryStat())
}

func (s *ClassesTestSuite) TestAllClassHitDice() {
	// Test all classes have valid hit dice
	classes := []constants.Class{
		constants.ClassBarbarian,
		constants.ClassBard,
		constants.ClassCleric,
		constants.ClassDruid,
		constants.ClassFighter,
		constants.ClassMonk,
		constants.ClassPaladin,
		constants.ClassRanger,
		constants.ClassRogue,
		constants.ClassSorcerer,
		constants.ClassWarlock,
		constants.ClassWizard,
	}

	for _, class := range classes {
		s.Run(string(class), func() {
			hitDice := class.HitDice()
			s.Contains([]int{6, 8, 10, 12}, hitDice, "Hit dice should be 6, 8, 10, or 12")
		})
	}
}

func TestClassesTestSuite(t *testing.T) {
	suite.Run(t, new(ClassesTestSuite))
}
