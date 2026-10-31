// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/stretchr/testify/suite"
)

type ActivitySuite struct{ suite.Suite }

func TestActivitySuite(t *testing.T) { suite.Run(t, new(ActivitySuite)) }

func (s *ActivitySuite) TestPickedAnswerCarriesActivityWithoutChangingWeights() {
	rows := []behavior.Answer{
		{Weight: 3, Say: "Let me think.", Activity: &behavior.ActivityCue{Ref: "future:activity:ponder", Cycles: 2}},
		{Weight: 1, Say: "No."},
	}
	for _, face := range []int{1, 300, 301, 400} {
		die := &facedDie{face: face}
		got, err := behavior.Pick(context.Background(), &behavior.PickInput{
			Key: "success", Table: behavior.Table{"success": rows}, Die: die,
		})
		s.Require().NoError(err)
		s.Require().NotNil(got)
		s.Equal(400, die.of)
		if face <= 300 {
			s.Equal("Let me think.", got.Answer.Say)
			s.Require().NotNil(got.Answer.Activity)
			s.Equal("future:activity:ponder", got.Answer.Activity.Ref)
			s.Equal(uint32(2), got.Answer.Activity.Cycles)
		} else {
			s.Equal("No.", got.Answer.Say)
			s.Nil(got.Answer.Activity, "a different selected answer never borrows the cue")
		}
	}
}

func (s *ActivitySuite) TestActivityRequiresLineReferenceAndCycles() {
	for _, row := range []behavior.Answer{
		{Weight: 1, Activity: &behavior.ActivityCue{Ref: "thoughtful", Cycles: 2}},
		{Weight: 1, Say: "Hmm.", Activity: &behavior.ActivityCue{Cycles: 2}},
		{Weight: 1, Say: "Hmm.", Activity: &behavior.ActivityCue{Ref: "thoughtful"}},
	} {
		s.ErrorIs(row.Validate(), behavior.ErrBadActivity)
	}
	valid := behavior.Answer{Weight: 1, Say: "Hmm.",
		Activity: &behavior.ActivityCue{Ref: "future:activity:unknown", Cycles: 2}}
	s.NoError(valid.Validate(), "this primitive does not resolve an asset catalog")
	legacy := behavior.Answer{Weight: 1, Say: "Hello."}
	s.NoError(legacy.Validate())
	s.Nil(legacy.Activity)
}

func (s *ActivitySuite) TestNestedPickRetainsActivityOnlyOnItsSelectedLeaf() {
	group := behavior.Answer{Weight: 1, Rows: []behavior.Answer{
		{Weight: 1, Say: "Hmm.", Activity: &behavior.ActivityCue{Ref: "thoughtful", Cycles: 3}},
	}}
	s.Require().NoError(group.Validate())
	got, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: "success", Table: behavior.Table{"success": {group}}, Die: &facedDie{face: 1},
	})
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal([]int{0, 0}, got.Path)
	s.Require().NotNil(got.Answer.Activity)
	s.Equal(uint32(3), got.Answer.Activity.Cycles)
	group.Activity = &behavior.ActivityCue{Ref: "thoughtful", Cycles: 2}
	s.ErrorIs(group.Validate(), behavior.ErrBadGroup, "the group's answer is its selected leaf")
	group.Activity = nil
	group.Rows[0].Activity.Cycles = 0
	s.ErrorIs(group.Validate(), behavior.ErrBadActivity, "nested rows use the same validation")
}
