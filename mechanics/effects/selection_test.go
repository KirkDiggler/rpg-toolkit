// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
)

type contributionSelectionSuite struct{ suite.Suite }

func TestContributionSelectionSuite(t *testing.T) { suite.Run(t, new(contributionSelectionSuite)) }

// Sorting by ID or keeping the newest candidate would select a-new instead.
func (s *contributionSelectionSuite) TestOldestPerGroup() {
	out, err := effects.SelectContributions(&effects.SelectContributionsInput{Candidates: []effects.SelectionCandidate{
		{ID: "z-old", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "boost"}},
		{ID: "a-new", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "boost"}},
		{ID: "penalty", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "penalty"}},
	}})
	s.Require().NoError(err)
	s.Equal([]effects.SelectionDecision{
		{ID: "z-old", Selected: true},
		{ID: "a-new", SelectedInstead: "z-old"},
		{ID: "penalty", Selected: true},
	}, out.Decisions)
}

// Treating an absent group as one global exclusion group would lose gift-b.
func (s *contributionSelectionSuite) TestAllApplicable() {
	out, err := effects.SelectContributions(&effects.SelectContributionsInput{Candidates: []effects.SelectionCandidate{
		{ID: "gift-a", Rule: effects.SelectionRule{Mode: effects.AllApplicable}},
		{ID: "old", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "one"}},
		{ID: "gift-b", Rule: effects.SelectionRule{Mode: effects.AllApplicable}},
		{ID: "new", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "one"}},
	}})
	s.Require().NoError(err)
	s.Equal([]effects.SelectionDecision{
		{ID: "gift-a", Selected: true}, {ID: "old", Selected: true},
		{ID: "gift-b", Selected: true}, {ID: "new", SelectedInstead: "old"},
	}, out.Decisions)
}

// Retaining the previous winner would exclude b after a is absent.
func (s *contributionSelectionSuite) TestRemovalRecomputes() {
	candidates := []effects.SelectionCandidate{
		{ID: "a", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "one"}},
		{ID: "b", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "one"}},
		{ID: "c", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "two"}},
	}
	first, err := effects.SelectContributions(&effects.SelectContributionsInput{Candidates: candidates})
	s.Require().NoError(err)
	s.Equal("a", first.Decisions[1].SelectedInstead)
	next, err := effects.SelectContributions(&effects.SelectContributionsInput{Candidates: candidates[1:]})
	s.Require().NoError(err)
	s.Equal([]effects.SelectionDecision{{ID: "b", Selected: true}, {ID: "c", Selected: true}}, next.Decisions)
	s.Equal("a", first.Decisions[1].SelectedInstead, "a previous answer remains an observation of its own input")
	s.Len(candidates, 3, "selection never removes an input candidate")
}

// Skipping validation after a group is claimed would silently accept bad input.
func (s *contributionSelectionSuite) TestWholeInputRefusal() {
	for _, tc := range []struct {
		name      string
		candidate effects.SelectionCandidate
	}{
		{"blank-id", effects.SelectionCandidate{Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "claimed"}}},
		{"whitespace-id", effects.SelectionCandidate{ID: " \t", Rule: effects.SelectionRule{Mode: effects.AllApplicable}}},
		{"duplicate-id", effects.SelectionCandidate{ID: "first", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "other"}}},
		{"missing-mode", effects.SelectionCandidate{ID: "bad", Rule: effects.SelectionRule{Group: "claimed"}}},
		{"unknown-mode", effects.SelectionCandidate{ID: "bad", Rule: effects.SelectionRule{Mode: "newest", Group: "claimed"}}},
		{"missing-group", effects.SelectionCandidate{ID: "bad", Rule: effects.SelectionRule{Mode: effects.OldestApplicable}}},
		{"blank-group", effects.SelectionCandidate{ID: "bad", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: " \t"}}},
		{"all-cannot-claim-group", effects.SelectionCandidate{ID: "bad", Rule: effects.SelectionRule{Mode: effects.AllApplicable, Group: "claimed"}}},
	} {
		s.Run(tc.name, func() {
			out, err := effects.SelectContributions(&effects.SelectContributionsInput{Candidates: []effects.SelectionCandidate{
				{ID: "first", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "claimed"}},
				tc.candidate,
			}})
			s.Require().Error(err)
			s.Nil(out, "a bad later candidate must not produce a partial selection")
		})
	}
}

func (s *contributionSelectionSuite) TestEmptyAndUnknownPolicy() {
	out, err := effects.SelectContributions(nil)
	s.Require().Error(err)
	s.Nil(out)
	out, err = effects.SelectContributions(&effects.SelectContributionsInput{})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Empty(out.Decisions)
	out, err = effects.SelectContributions(&effects.SelectContributionsInput{Candidates: []effects.SelectionCandidate{{ID: "only", Rule: effects.SelectionRule{Mode: "unknown"}}}})
	s.Require().Error(err)
	s.Nil(out, "an unknown policy must not default to selecting everything")
}

// Definition compilation must validate a policy before any candidate IDs exist.
func (s *contributionSelectionSuite) TestUnboundRuleValidation() {
	s.Require().Error(effects.ValidateSelectionRule(nil))
	for _, rule := range []effects.SelectionRule{
		{Mode: effects.AllApplicable},
		{Mode: effects.OldestApplicable, Group: "a-group"},
	} {
		s.NoError(effects.ValidateSelectionRule(&effects.ValidateSelectionRuleInput{Rule: rule}))
	}
	for _, rule := range []effects.SelectionRule{
		{},
		{Mode: "unknown"},
		{Mode: effects.OldestApplicable},
		{Mode: effects.OldestApplicable, Group: " \t"},
		{Mode: effects.AllApplicable, Group: "not-allowed"},
	} {
		s.Error(effects.ValidateSelectionRule(&effects.ValidateSelectionRuleInput{Rule: rule}))
	}
}

func (s *contributionSelectionSuite) TestInputAndOutputOwnership() {
	candidates := []effects.SelectionCandidate{
		{ID: "first", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "group"}},
		{ID: "second", Rule: effects.SelectionRule{Mode: effects.OldestApplicable, Group: "group"}},
	}
	before := slices.Clone(candidates)
	out, err := effects.SelectContributions(&effects.SelectContributionsInput{Candidates: candidates})
	s.Require().NoError(err)
	s.Equal(before, candidates)
	candidates[0].ID = "changed-input"
	s.Equal("first", out.Decisions[0].ID)
	s.Equal("first", out.Decisions[1].SelectedInstead)
	out.Decisions[0] = effects.SelectionDecision{ID: "changed-output"}
	again, err := effects.SelectContributions(&effects.SelectContributionsInput{Candidates: before})
	s.Require().NoError(err)
	s.Equal([]effects.SelectionDecision{{ID: "first", Selected: true}, {ID: "second", SelectedInstead: "first"}}, again.Decisions)
	s.Equal("changed-input", candidates[0].ID)
}
