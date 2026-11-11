// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/selection"
)

// OptionEvaluationSuite pins the one pool the offer and the door share
// (rpg-project#921): one row per option in authored order, weight zero (a
// line is chosen, never drawn), and one named verdict per `requires` item
// (rpg-project#922), each reason naming only the requirement's kind.
type OptionEvaluationSuite struct {
	suite.Suite
}

func TestOptionEvaluationSuite(t *testing.T) { suite.Run(t, new(OptionEvaluationSuite)) }

// TestOnePoolOneRowPerOption judges the arms that need no board: no
// requirement holds, a `not` over nil never holds, a `not` over that holds
// again, and a pair holds only when both do.
func (s *OptionEvaluationSuite) TestOnePoolOneRowPerOption() {
	e := &Encounter{}
	options := []Option{
		{ID: "plain"},
		{ID: "never", Requires: []Trigger{TriggerNot{}}},
		{ID: "twice", Requires: []Trigger{TriggerNot{Inner: TriggerNot{}}}},
		{ID: "pair", Requires: []Trigger{TriggerNot{Inner: TriggerNot{}}, TriggerNot{}}},
	}

	evaluation, err := e.evaluateOptions("alice", options, nil)
	s.Require().NoError(err)
	inspection, err := evaluation.Inspect()
	s.Require().NoError(err)

	s.Require().Len(inspection.Rows, len(options))
	want := []struct {
		eligible     bool
		requirements []selection.Requirement
	}{
		{true, nil},
		{false, []selection.Requirement{{ID: "requires:0:not", Verdict: selection.Unmet, Reason: "not"}}},
		{true, []selection.Requirement{{ID: "requires:0:not", Verdict: selection.Met, Reason: "not"}}},
		{false, []selection.Requirement{
			{ID: "requires:0:not", Verdict: selection.Met, Reason: "not"},
			{ID: "requires:1:not", Verdict: selection.Unmet, Reason: "not"},
		}},
	}
	for i, row := range inspection.Rows {
		s.Equal(options[i].ID, row.ID, "authored order")
		s.Zero(row.BaseWeight, "%s: chosen, never drawn", row.ID)
		s.Equal(want[i].eligible, row.Eligible, row.ID)
		s.Equal(want[i].requirements, row.Requirements, "%s: one named verdict per requirement", row.ID)
	}
}

// TestADuplicateIDIsAnErrorNeverASilentOffer: the field validator refuses
// duplicate ids, so one reaching the pool is a construction defect, surfaced
// rather than swallowed.
func (s *OptionEvaluationSuite) TestADuplicateIDIsAnErrorNeverASilentOffer() {
	e := &Encounter{}
	_, err := e.evaluateOptions("alice", []Option{{ID: "same"}, {ID: "same"}}, nil)
	s.ErrorIs(err, selection.ErrInvalidCandidate)

	_, err = e.offered("alice", []Option{{ID: "same"}, {ID: "same"}})
	s.ErrorIs(err, selection.ErrInvalidCandidate)
}

// TestAWhitespaceIDIsRefusedAtConstruction: an id of only spaces is no id,
// refused by the field validator with the blank-id message, so it can never
// reach the pool that would refuse it there.
func (s *OptionEvaluationSuite) TestAWhitespaceIDIsRefusedAtConstruction() {
	err := validateOptions([]Option{{ID: "  ", Say: "Evening.", Success: []Answer{{Weight: 1, Say: "Evening."}}}})
	s.ErrorContains(err, "options[0] has no id")
}
