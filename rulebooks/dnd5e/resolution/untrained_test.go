// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

// untrained_test.go pins the 2014 letter on a skill the character never
// trained: d20 plus the ability modifier, ONE die, no rule recorded over it.
// Proficiency adds its bonus; its absence adds nothing and takes nothing
// away. The house rule that rolled an untrained skill verb at disadvantage
// was deleted on the operator's ruling (rpg-project#569): "no skill just means
// no plus".

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// The two faces this suite's roller holds. They are far apart on purpose: a
// single die and a pair kept low must not be able to produce the same number,
// so a scene asserting "one die" cannot pass on a disadvantaged pair.
const (
	untrainedHigh = 15
	untrainedLow  = 4
)

type UntrainedTestSuite struct {
	suite.Suite

	ctx    context.Context
	roller *scriptedRoller
}

func TestUntrainedSuite(t *testing.T) {
	suite.Run(t, new(UntrainedTestSuite))
}

func (s *UntrainedTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.roller = &scriptedRoller{single: untrainedHigh, pair: []int{untrainedHigh, untrainedLow}}
}

// talker is proficient in Intimidation and trained in NOTHING else — so
// Persuasion is the untrained skill, and CHA 14 (+2) makes the untrained
// modifier positive, which stops a scene passing because the total happened
// to be low.
func (s *UntrainedTestSuite) talker() *character.Data {
	return &character.Data{
		ID:       seekerID,
		PlayerID: "player-1",
		Name:     "Vex",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 14,
		},
		HitPoints:        14,
		MaxHitPoints:     14,
		ProficiencyBonus: 2,
		Skills: map[skills.Skill]shared.ProficiencyLevel{
			skills.Intimidation: shared.Proficient,
		},
	}
}

func (s *UntrainedTestSuite) check(approaches ...encounter.CheckApproach) *CheckOutput {
	out, err := MakeCheck(s.ctx, &CheckInput{
		Character:  s.talker(),
		Approaches: approaches,
		Roller:     s.roller,
	})
	s.Require().NoError(err)

	return out
}

// THE HEADLINE. A character with no Persuasion rolls ONE d20 and adds their
// Charisma modifier — no second face, no keep rule, and no "Untrained" source
// anywhere in the calculation.
func (s *UntrainedTestSuite) TestAnUntrainedSkillRollsOnePlainD20() {
	out := s.check(route(string(skills.Persuasion), 10))

	s.Require().Equal(untrainedHigh, out.Result.Roll,
		"the single die; a pair kept low would have produced the other face")
	s.Require().Equal(untrainedHigh+2, out.Result.Total, "CHA 14 is +2, and no skill adds no plus")

	s.Require().NotNil(out.Calculation)
	s.Equal(untrainedHigh+2, out.Calculation.Total)
	die := dieOf(s.T(), out.Calculation)
	s.Equal("1d20", die.Notation, "one die is thrown")
	s.Nil(die.Keep, "and no rule is recorded over it")
	for _, component := range out.Calculation.Components {
		s.NotEqual("Untrained", component.Source.Name, "no source names the missing training")
	}
}

// The control: the same roller, a skill the character took — the same single
// die, plus the proficiency bonus.
func (s *UntrainedTestSuite) TestATrainedCheckerAddsTheBonus() {
	out := s.check(route(string(skills.Intimidation), 10))

	s.Require().Equal(untrainedHigh, out.Result.Roll)
	s.Require().Equal(untrainedHigh+4, out.Result.Total, "CHA +2 and proficiency +2")
	s.Equal("1d20", dieOf(s.T(), out.Calculation).Notation, "a trained check throws one die")
	s.Nil(keepOf(s.T(), out.Calculation), "and records no rule over it")
}
