// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// SkillProficiencyTestSuite covers the sheet's answer to "how well is this
// character trained in this skill" — the fact the untrained rule is built on
// (rpg-project#457 R2, ideas/shenanigans/front-room-goblin.md).
type SkillProficiencyTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSkillProficiencySuite(t *testing.T) {
	suite.Run(t, new(SkillProficiencyTestSuite))
}

func (s *SkillProficiencyTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SkillProficiencyTestSuite) sheet(skillMap map[skills.Skill]shared.ProficiencyLevel) *Character {
	char, err := Load(s.ctx, &Data{ID: "talker", Level: 1, Skills: skillMap})
	s.Require().NoError(err)

	return char
}

// A skill the character never took reads NotProficient — the zero value, and
// the whole reason this returns one value instead of a (level, ok) pair.
func (s *SkillProficiencyTestSuite) TestAnUntakenSkillIsNotProficient() {
	char := s.sheet(map[skills.Skill]shared.ProficiencyLevel{
		skills.Intimidation: shared.Proficient,
	})

	s.Equal(shared.NotProficient, char.SkillProficiency(skills.Persuasion))
	s.Equal(shared.NotProficient, char.SkillProficiency(skills.Deception))
}

// A sheet with no skill map at all answers the same way, rather than panicking
// on a nil map or inventing an "unknown" third state.
func (s *SkillProficiencyTestSuite) TestASheetWithNoSkillsIsUntrainedInEverything() {
	char := s.sheet(nil)

	s.Equal(shared.NotProficient, char.SkillProficiency(skills.Intimidation))
	s.Equal(shared.NotProficient, char.SkillProficiency(skills.Perception))
}

// Training and expertise come back as themselves, so a caller can tell the
// two apart — which the modifier cannot: a +3 is a trained +1 or an untrained
// +3, and reversing the number is the mistake this read exists to prevent.
func (s *SkillProficiencyTestSuite) TestTrainingAndExpertiseAreDistinguishable() {
	char := s.sheet(map[skills.Skill]shared.ProficiencyLevel{
		skills.Intimidation: shared.Proficient,
		skills.Persuasion:   shared.Expert,
	})

	s.Equal(shared.Proficient, char.SkillProficiency(skills.Intimidation))
	s.Equal(shared.Expert, char.SkillProficiency(skills.Persuasion))
}

// The read states TRAINING and applies no rule to it. A character with a high
// Charisma and no Persuasion has a positive modifier and is still untrained;
// the sheet says so and says nothing about what that costs.
func (s *SkillProficiencyTestSuite) TestAGoodModifierDoesNotMakeSomebodyTrained() {
	char, err := Load(s.ctx, &Data{
		ID: "natural", Level: 1,
		AbilityScores: shared.AbilityScores{abilities.CHA: 18},
	})
	s.Require().NoError(err)

	s.Positive(char.GetSkillModifier(skills.Persuasion), "a +4 Charisma still helps")
	s.Equal(shared.NotProficient, char.SkillProficiency(skills.Persuasion),
		"a good modifier is not training, and only the training answers the rule")
}
