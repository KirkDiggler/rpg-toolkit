package constants_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
)

type SkillsTestSuite struct {
	suite.Suite
}

func (s *SkillsTestSuite) TestSkillConstants() {
	tests := []struct {
		name     string
		skill    constants.Skill
		expected string
		display  string
		ability  constants.Ability
	}{
		{
			name:     "athletics skill",
			skill:    constants.SkillAthletics,
			expected: "athletics",
			display:  "Athletics",
			ability:  constants.STR,
		},
		{
			name:     "acrobatics skill",
			skill:    constants.SkillAcrobatics,
			expected: "acrobatics",
			display:  "Acrobatics",
			ability:  constants.DEX,
		},
		{
			name:     "arcana skill",
			skill:    constants.SkillArcana,
			expected: "arcana",
			display:  "Arcana",
			ability:  constants.INT,
		},
		{
			name:     "insight skill",
			skill:    constants.SkillInsight,
			expected: "insight",
			display:  "Insight",
			ability:  constants.WIS,
		},
		{
			name:     "persuasion skill",
			skill:    constants.SkillPersuasion,
			expected: "persuasion",
			display:  "Persuasion",
			ability:  constants.CHA,
		},
		{
			name:     "sleight of hand skill",
			skill:    constants.SkillSleightOfHand,
			expected: "sleight-of-hand",
			display:  "Sleight of Hand",
			ability:  constants.DEX,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Equal(tc.expected, string(tc.skill))
			s.Equal(tc.display, tc.skill.Display())
			s.Equal(tc.ability, tc.skill.Ability())
		})
	}
}

func (s *SkillsTestSuite) TestSkillDisplay_UnknownSkill() {
	unknown := constants.Skill("unknown")
	s.Equal("unknown", unknown.Display())
	s.Equal(constants.Ability(""), unknown.Ability())
}

func (s *SkillsTestSuite) TestAllSkills() {
	skills := constants.AllSkills()
	s.Len(skills, 18) // D&D 5e has 18 skills

	// Verify no duplicates
	seen := make(map[constants.Skill]bool)
	for _, skill := range skills {
		s.False(seen[skill], "Duplicate skill found: %s", skill)
		seen[skill] = true
	}
}

func (s *SkillsTestSuite) TestSkillAbilityMapping() {
	// Test that all skills map to valid abilities
	skills := constants.AllSkills()
	validAbilities := map[constants.Ability]bool{
		constants.STR: true,
		constants.DEX: true,
		constants.CON: true,
		constants.INT: true,
		constants.WIS: true,
		constants.CHA: true,
	}

	for _, skill := range skills {
		ability := skill.Ability()
		s.True(validAbilities[ability], "Skill %s maps to invalid ability %s", skill, ability)
	}
}

func TestSkillsTestSuite(t *testing.T) {
	suite.Run(t, new(SkillsTestSuite))
}
