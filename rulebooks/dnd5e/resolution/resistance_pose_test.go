// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ResistancePoseTestSuite proves the architecture Resistance needs: a save
// gated cast can pose on an offer held against the saver's own throw, and
// resuming it finishes the SAME cast from exactly where it stopped — for a
// single target and, propagated one Request layer further out, for a target
// beyond the one that posed.
//
// Bane is the vehicle rather than Resistance's own cast content, which does
// not exist until a later slice in this plan: [poseContest]/[poseCast] are
// generic over WHICH condition offered the die, and Bane is already a
// shipped, multi-target, save-gated cast this suite does not have to build.
type ResistancePoseTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestResistancePoseSuite(t *testing.T) {
	suite.Run(t, new(ResistancePoseTestSuite))
}

func (s *ResistancePoseTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ResistancePoseTestSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx
	return fixtures
}

// resistanceJSON is one Resistance die already on a sheet, [baneOwnerJSON]'s
// own pattern applied to [conditions.ResistanceCondition].
func (s *ResistancePoseTestSuite) resistanceJSON(memberID, casterID string) json.RawMessage {
	condition, err := conditions.NewResistanceCondition(conditions.NewResistanceConditionInput{
		MemberID: memberID, SourceID: casterID, SourceRef: refs.Spells.Resistance(),
	})
	s.Require().NoError(err)
	stored, err := condition.ToJSON()
	s.Require().NoError(err)
	return stored
}

// TestASaveNobodyOffersAnythingOnIsUnchanged is the regression that matters
// most: the save-offer chain now folds on every save-gated cast, and a save
// with no offer against it must produce exactly what it produced before this
// existed — no pose, ordinary outcome.
func (s *ResistancePoseTestSuite) TestASaveNobodyOffersAnythingOnIsUnchanged() {
	fixtures := s.fixtures()
	roller := &countingCastRoller{facedRoller: facedRoller{d20: 10, other: 4}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{heroID}, Roller: roller,
	})
	s.Require().NoError(err)

	out, err := fixtures.resolve(fixtures.saver(14), machine, baneCost(), baneCaster(1, 2))
	s.Require().NoError(err)
	s.Require().Nil(out.Posed, "nobody offered anything, so nothing was asked")
	outcome := out.Outcome.(CastOutcome)
	s.Require().Len(outcome.Targets, 1)
}

// TestASingleTargetSavePosesAndResumes is the single-`Request`-layer case:
// Sacred Flame's own shape, exercised here through Bane's one-target arm.
func (s *ResistancePoseTestSuite) TestASingleTargetSavePosesAndResumes() {
	fixtures := s.fixtures()
	saver := fixtures.saver(14, s.resistanceJSON(heroID, "cleric-1"))
	roller := &countingCastRoller{facedRoller: facedRoller{d20: 10, other: 4}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{heroID}, Roller: roller,
	})
	s.Require().NoError(err)

	out, err := fixtures.resolve(saver, machine, baneCost(), baneCaster(1, 2))
	s.Require().NoError(err)
	s.Require().Nil(out.Outcome, "a posed machine produced nothing yet")
	s.Require().NotNil(out.Posed)
	s.Equal(heroID, out.Posed.Ask.Audience)
	s.Equal(conditions.ResistanceName, out.Posed.Ask.Offer.Name)
	s.Equal(refs.Conditions.Resistance().String(), out.Posed.Ask.Offer.Ref.String())
	s.Equal([]string{"spend", "keep"}, out.Posed.Ask.Options)
	frozenTotal := out.Posed.Ask.Total

	s.Run("spend appends the die and finishes the cast", func() {
		resumed, err := NewCastResumed(&CastResumeInput{
			Frozen: append([]byte(nil), out.Posed.Frozen...), Answer: OfferSpend,
			Roller: facedRoller{d20: 1, other: 3},
		})
		s.Require().NoError(err)

		resumedOut, err := fixtures.resolve(saver, resumed, nil, baneCaster(1, 2))
		s.Require().NoError(err)
		s.Require().Nil(resumedOut.Posed, "a resumed save finishes")
		outcome := resumedOut.Outcome.(CastOutcome)
		s.Require().Len(outcome.Targets, 1)
		s.Equal(frozenTotal+3, outcome.Targets[0].Save.Save.Result.Total, "roll plus bonus plus the face")
	})

	s.Run("keep leaves the total alone", func() {
		resumed, err := NewCastResumed(&CastResumeInput{
			Frozen: append([]byte(nil), out.Posed.Frozen...), Answer: OfferKeep,
		})
		s.Require().NoError(err)

		resumedOut, err := fixtures.resolve(saver, resumed, nil, baneCaster(1, 2))
		s.Require().NoError(err)
		s.Require().Nil(resumedOut.Posed)
		outcome := resumedOut.Outcome.(CastOutcome)
		s.Equal(frozenTotal, outcome.Targets[0].Save.Save.Result.Total, "no face joined it")
	})
}

// TestAPosedTargetLeavesLaterTargetsToRun is the two-`Request`-layer case
// Bane's own multi-target shape needs: target 0 poses, and resuming must
// still resolve target 1's OWN save rather than stopping after the one that
// suspended.
func (s *ResistancePoseTestSuite) TestAPosedTargetLeavesLaterTargetsToRun() {
	fixtures := s.fixtures()
	saver := fixtures.saver(14, s.resistanceJSON(heroID, "cleric-1"))
	roller := &countingCastRoller{facedRoller: facedRoller{d20: 10, other: 4}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{heroID, wolfID}, Roller: roller,
	})
	s.Require().NoError(err)

	out, err := fixtures.resolve(saver, machine, baneCost(), baneCaster(1, 2))
	s.Require().NoError(err)
	s.Require().NotNil(out.Posed, "hero (target 0) holds the die")

	resumed, err := NewCastResumed(&CastResumeInput{
		Frozen: out.Posed.Frozen, Answer: OfferKeep, Roller: roller,
	})
	s.Require().NoError(err)

	resumedOut, err := fixtures.resolve(saver, resumed, nil, baneCaster(1, 2))
	s.Require().NoError(err)
	s.Require().Nil(resumedOut.Posed)
	outcome := resumedOut.Outcome.(CastOutcome)
	s.Require().Len(outcome.Targets, 2, "the wolf's own save still ran after resume")
	s.Equal(heroID, outcome.Targets[0].TargetID)
	s.Equal(wolfID, outcome.Targets[1].TargetID)
}
