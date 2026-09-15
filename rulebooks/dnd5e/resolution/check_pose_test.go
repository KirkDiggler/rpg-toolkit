// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

const guidedCheckerID = "rogue-1"

type CheckPoseTestSuite struct {
	suite.Suite

	ctx    context.Context
	roller *scriptedRoller
}

func (s *CheckPoseTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.roller = &scriptedRoller{single: straightRoll}
}

// checker is a lone WIS 12 (+1) rogue, proficient in Perception (+3), with
// PROFICIENCYBonus 2 — arithmetic chosen so a offered 1d4 lands on a visibly
// different total from the straight roll.
func (s *CheckPoseTestSuite) checker(conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID:       guidedCheckerID,
		PlayerID: "player-1",
		Name:     "Fen",
		Level:    1,
		ClassID:  classes.Rogue,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 14, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 10,
		},
		HitPoints:        9,
		MaxHitPoints:     9,
		ProficiencyBonus: 2,
		Skills:           map[skills.Skill]shared.ProficiencyLevel{skills.Perception: shared.Proficient},
		Conditions:       conds,
	}
}

func (s *CheckPoseTestSuite) guided() json.RawMessage {
	condition, err := conditions.NewGuidedCondition(conditions.NewGuidedConditionInput{
		MemberID: guidedCheckerID, SourceID: "cleric-1", SourceRef: refs.Spells.Guidance(),
	})
	s.Require().NoError(err)
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	return raw
}

func (s *CheckPoseTestSuite) check(data *character.Data) (*CheckOutput, error) {
	return MakeCheck(s.ctx, &CheckInput{
		Character:  data,
		Approaches: []encounter.CheckApproach{route(string(skills.Perception), 10)},
		Roller:     s.roller,
	})
}

// TestAPlainCheckerNeverPoses is the no-offer regression: nothing about
// ordinary checks changes when nobody holds a die.
func (s *CheckPoseTestSuite) TestAPlainCheckerNeverPoses() {
	out, err := s.check(s.checker())
	s.Require().NoError(err)

	s.Nil(out.Posed)
	s.Require().NotNil(out.Result)
	s.Equal(straightRoll+3, out.Result.Total, "WIS +1, proficiency +2")
	s.Require().NotNil(out.Calculation)
	s.Equal(out.Result.Total, out.Calculation.Total, "the sourced total agrees with the bare one")
}

// TestAGuidedCheckerPoses is the whole mechanism: the checker's own held die
// stops the check instead of finishing it.
func (s *CheckPoseTestSuite) TestAGuidedCheckerPoses() {
	out, err := s.check(s.checker(s.guided()))
	s.Require().NoError(err)

	s.Require().Nil(out.Result, "a posed check carries no result yet")
	s.Require().NotNil(out.Posed)
	s.Equal(guidedCheckerID, out.Posed.Ask.Audience)
	s.Equal(refs.Conditions.Guided().String(), out.Posed.Ask.Offer.Ref.String())
	s.Equal(conditions.GuidedDie, out.Posed.Ask.Offer.Die)
	s.Equal(straightRoll, out.Posed.Ask.Roll)
	s.Equal(straightRoll+3, out.Posed.Ask.Total, "the offer is against the pre-offer total")
	s.NotEmpty(out.Posed.Frozen)
}

// TestSpendingAppendsTheDieAndEndsTheCondition is the spend half: the die
// joins the total exactly once, and the reloaded condition hears its own
// die was taken.
func (s *CheckPoseTestSuite) TestSpendingAppendsTheDieAndEndsTheCondition() {
	posed, err := s.check(s.checker(s.guided()))
	s.Require().NoError(err)
	s.Require().NotNil(posed.Posed)

	dieRoller := &scriptedRoller{single: 2}
	out, err := ResumeCheck(s.ctx, &CheckResumeInput{
		Frozen:    posed.Posed.Frozen,
		Answer:    OfferSpend,
		Character: s.checker(s.guided()),
		Roller:    dieRoller,
	})
	s.Require().NoError(err)

	s.Require().NotNil(out.Result)
	s.Equal(straightRoll, out.Result.Roll, "the frozen d20, not a re-roll")
	s.Equal(straightRoll+3+2, out.Result.Total, "the base total plus the offered face")
	s.False(out.Result.Success, "3+3+2=8 still misses DC 10 — the die helped without saving the roll")
	s.Require().NotNil(out.Calculation)
	s.Equal(out.Result.Total, out.Calculation.Total)
	s.Require().Len(out.Calculation.Components, 3, "d20, modifier, and the offered die")

	s.Require().NotNil(out.DirtyCharacter, "the spent condition's removal is a sheet change")
}

// TestKeepingLeavesTheDieUnspent is the decline half: nothing is rolled or
// published, so the frozen d20 stands alone and the condition survives for
// the checker's next check.
func (s *CheckPoseTestSuite) TestKeepingLeavesTheDieUnspent() {
	posed, err := s.check(s.checker(s.guided()))
	s.Require().NoError(err)
	s.Require().NotNil(posed.Posed)

	out, err := ResumeCheck(s.ctx, &CheckResumeInput{
		Frozen:    posed.Posed.Frozen,
		Answer:    OfferKeep,
		Character: s.checker(s.guided()),
	})
	s.Require().NoError(err)

	s.Require().NotNil(out.Result)
	s.Equal(straightRoll+3, out.Result.Total, "unchanged from the frozen total")
	s.Require().Len(out.Calculation.Components, 2, "no offered-die component was appended")
	s.Nil(out.DirtyCharacter, "declining touches nothing on the sheet")
}

// TestResumeRefusesAnUnansweredOrForeignAnswer pins the same refusals
// [NewStrikeResumed] pins, adapted to checks.
func (s *CheckPoseTestSuite) TestResumeRefusesABadAnswer() {
	posed, err := s.check(s.checker(s.guided()))
	s.Require().NoError(err)

	_, err = ResumeCheck(s.ctx, &CheckResumeInput{
		Frozen: posed.Posed.Frozen, Answer: "maybe", Character: s.checker(s.guided()),
	})
	s.Require().ErrorIs(err, ErrNotOffered)
}

func (s *CheckPoseTestSuite) TestResumeRefusesGarbageFrozenBytes() {
	_, err := ResumeCheck(s.ctx, &CheckResumeInput{
		Frozen: []byte("not json"), Answer: OfferKeep, Character: s.checker(s.guided()),
	})
	s.Require().ErrorIs(err, ErrBadFrozen)
}

func (s *CheckPoseTestSuite) TestResumeRefusesTheWrongCheckersSheet() {
	posed, err := s.check(s.checker(s.guided()))
	s.Require().NoError(err)

	other := s.checker()
	other.ID = "somebody-else"
	_, err = ResumeCheck(s.ctx, &CheckResumeInput{
		Frozen: posed.Posed.Frozen, Answer: OfferKeep, Character: other,
	})
	s.Require().ErrorIs(err, ErrBadParticipant)
}

func TestCheckPoseSuite(t *testing.T) {
	suite.Run(t, new(CheckPoseTestSuite))
}
