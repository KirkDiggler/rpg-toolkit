// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

// untrained_test.go is the untrained rule's whole surface (rpg-project#457
// R2). Every scene here fails if untrainedSource's body is emptied, and NO
// scene anywhere else in this module does — that is the claim the ruling
// actually made ("flipping to RAW would be an easy refactor"), and this file
// is what makes it falsifiable.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// The two faces this suite rolls. They are far apart on purpose: a straight
// roll and a disadvantaged one must not be able to produce the same number,
// or a scene asserting "it rolled twice and took the lower" would pass on a
// single die.
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

// talker is proficient in Intimidation, expert in Athletics, and trained in
// NOTHING else — so Persuasion is the untrained skill every scene reaches
// for, and CHA 14 (+2) makes their untrained modifier positive, which is what
// stops a scene passing because the total happened to be low.
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
			skills.Athletics:    shared.Expert,
		},
	}
}

func (s *UntrainedTestSuite) check(untrained bool, approaches ...encounter.CheckApproach) *CheckOutput {
	out, err := MakeCheck(s.ctx, &CheckInput{
		Character:  s.talker(),
		Approaches: approaches,
		Roller:     s.roller,
		Untrained:  untrained,
	})
	s.Require().NoError(err)

	return out
}

// THE HEADLINE. A character with no Persuasion rolls the verb at
// disadvantage, and the result says WHO imposed it — "Untrained", by name,
// the way the same list already says "Raging" for advantage.
func (s *UntrainedTestSuite) TestAnUntrainedSkillVerbRollsAtDisadvantage() {
	out := s.check(true, route(string(skills.Persuasion), 10))

	s.Require().Equal(untrainedLow, out.Result.Roll,
		"only a rolled-twice-take-lower can produce this die")
	s.Require().Equal(untrainedLow+2, out.Result.Total, "CHA 14 is +2, and untrained adds no proficiency")

	die := dieOf(s.T(), out.Calculation)
	s.Equal("2d20", die.Notation, "two faces were thrown and both reach the log")
	s.Require().Len(die.KeptIndices, 1)
	s.Require().NotNil(die.Keep)
	s.Equal(dnd5eEvents.KeepDisadvantage, die.Keep.Rule)
	s.Require().Len(die.Keep.Imposed, 1)
	s.Equal("Untrained", die.Keep.Imposed[0].Name)
	s.Equal("rule", die.Keep.Imposed[0].Label,
		"nobody carries it, nobody granted it, nothing dispels it")
	s.Equal(refs.Rules.Untrained().String(), die.Keep.Imposed[0].Ref.String())
	s.Equal(seekerID, die.Keep.Imposed[0].SourceID)
	s.Empty(die.Keep.Granted)
}

// The control that makes the headline mean something: the SAME verb, the same
// roller, a character who took the skill — a straight roll and no source.
func (s *UntrainedTestSuite) TestATrainedCheckerRollsStraight() {
	out := s.check(true, route(string(skills.Intimidation), 10))

	s.Require().Equal(untrainedHigh, out.Result.Roll)
	s.Require().Equal(untrainedHigh+4, out.Result.Total, "CHA +2 and proficiency +2")
	s.Equal("1d20", dieOf(s.T(), out.Calculation).Notation, "a trained check throws one die")
	s.Nil(keepOf(s.T(), out.Calculation), "and records no rule over it")
}

// Expertise is training too: the rule asks whether the character is
// NotProficient, not whether they are exactly Proficient.
func (s *UntrainedTestSuite) TestExpertiseIsTraining() {
	out := s.check(true, route(string(skills.Athletics), 10))

	s.Require().Equal(untrainedHigh, out.Result.Roll)
	s.Nil(keepOf(s.T(), out.Calculation))
}

// THE SCOPE, AND THE LETTER. Search and Unlock are not skill verbs: they
// leave the field false and roll as the 2014 book says, even for a character
// with no training in the skill the route names.
func (s *UntrainedTestSuite) TestAVerbThatDoesNotTakeTheRuleRollsTheLetter() {
	out := s.check(false, route(string(skills.Persuasion), 10))

	s.Require().Equal(untrainedHigh, out.Result.Roll, "RAW puts no penalty on an untrained check")
	s.Nil(keepOf(s.T(), out.Calculation), "one die, and no rule recorded over it")
}

// A bare-ability route has no training to lack. Shoving a door with Strength
// is not a skill, so the rule finds no skill to ask about and does nothing —
// which is also what stops it firing on every check that resolves an ability.
func (s *UntrainedTestSuite) TestABareAbilityRouteTakesNoRule() {
	out := s.check(true, route(string(abilities.STR), 10))

	s.Require().Equal(untrainedHigh, out.Result.Roll)
	s.Nil(keepOf(s.T(), out.Calculation))
}

// The rule reaches ONLY the applied route. A list where the untrained skill
// loses the selection rolls straight, because the check that happened is the
// trained one — and the source list is per check, not per listed approach.
func (s *UntrainedTestSuite) TestOnlyTheAppliedRouteIsAsked() {
	out := s.check(true,
		route(string(skills.Persuasion), 10),
		route(string(skills.Intimidation), 10),
	)

	s.Require().Equal(string(skills.Intimidation), out.Applied.Ability,
		"proficiency makes Intimidation the better route at the same DC")
	s.Require().Equal(untrainedHigh, out.Result.Roll)
	s.Nil(keepOf(s.T(), out.Calculation))
}

// The die it lands on is the ONE the calculation reports, and the source
// rides in the result's own list rather than as a component.
//
// THAT IS WHERE EVERY ADVANTAGE AND DISADVANTAGE LIVES — Raging's advantage is
// not a component either, because neither adds a number: they change which of
// two faces is kept, and the calculation records the face that was kept. A
// build that put "Untrained" in the components would have to invent a zero
// modifier for it, which is a number nobody rolled.
func (s *UntrainedTestSuite) TestTheSourceIsOnTheDieNotInTheArithmetic() {
	out := s.check(true, route(string(skills.Persuasion), 10))

	s.Require().NotNil(out.Calculation)
	s.Require().Equal(untrainedLow+2, out.Calculation.Total)
	for _, component := range out.Calculation.Components {
		s.NotEqual("Untrained", component.Source.Name,
			"a rule that keeps the lower face adds no number to add")
	}

	keep := keepOf(s.T(), out.Calculation)
	s.Require().NotNil(keep, "and the die it decided is where it is recorded")
	s.Require().Len(keep.Imposed, 1)
	s.Equal("Untrained", keep.Imposed[0].Name)
}
