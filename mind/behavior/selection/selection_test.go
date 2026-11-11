package selection_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/selection"
)

type SelectionSuite struct{ suite.Suite }

func TestSelectionSuite(t *testing.T) { suite.Run(t, new(SelectionSuite)) }

// A failed/unknown requirement must not be rescued by a large preference.
func (s *SelectionSuite) TestRequirementsAndWeights() {
	pool, err := selection.Evaluate(&selection.EvaluateInput{Candidates: []selection.Candidate{
		{ID: "search", Weight: 2, Requirements: []selection.Requirement{{ID: "lead", Verdict: selection.Met, Reason: "remembered route"}}, Factors: []selection.Factor{{ID: "persistent", Multiplier: 3}}},
		{ID: "hold", Weight: 4},
		{ID: "bribe", Weight: 1000, Requirements: []selection.Requirement{{ID: "offered", Verdict: selection.Unmet, Reason: "no offer"}}},
		{ID: "accuse", Weight: 1000, Requirements: []selection.Requirement{{ID: "identity", Verdict: selection.Unknown, Reason: "not observed"}}},
	}})
	s.Require().NoError(err)
	out, err := pool.Inspect()
	s.Require().NoError(err)
	s.Equal(2, out.Eligible)
	s.Equal(10, out.Total)
	s.Equal([]selection.Row{
		{ID: "search", BaseWeight: 2, Requirements: []selection.Requirement{{ID: "lead", Verdict: selection.Met, Reason: "remembered route"}}, Factors: []selection.Factor{{ID: "persistent", Multiplier: 3}}, Eligible: true, Weight: 6},
		{ID: "hold", BaseWeight: 4, Eligible: true, Weight: 4},
		{ID: "bribe", BaseWeight: 1000, Requirements: []selection.Requirement{{ID: "offered", Verdict: selection.Unmet, Reason: "no offer"}}},
		{ID: "accuse", BaseWeight: 1000, Requirements: []selection.Requirement{{ID: "identity", Verdict: selection.Unknown, Reason: "not observed"}}},
	}, out.Rows)
}

func (s *SelectionSuite) TestMalformedCandidates() {
	cases := []struct {
		name string
		rows []selection.Candidate
	}{
		{"empty identity", []selection.Candidate{{Weight: 1}}},
		{"blank identity", []selection.Candidate{{ID: " \t", Weight: 1}}},
		{"duplicate identity", []selection.Candidate{{ID: "a", Weight: 1}, {ID: "a", Weight: 2}}},
		{"negative base", []selection.Candidate{{ID: "a", Weight: -1}}},
		{"missing requirement identity", []selection.Candidate{{ID: "a", Requirements: []selection.Requirement{{Verdict: selection.Met}}}}},
		{"duplicate requirement", []selection.Candidate{{ID: "a", Requirements: []selection.Requirement{{ID: "key", Verdict: selection.Met}, {ID: "key", Verdict: selection.Unmet}}}}},
		{"invalid verdict", []selection.Candidate{{ID: "a", Requirements: []selection.Requirement{{ID: "key", Verdict: selection.Verdict(99)}}}}},
		{"missing factor identity", []selection.Candidate{{ID: "a", Factors: []selection.Factor{{Multiplier: 1}}}}},
		{"duplicate factor", []selection.Candidate{{ID: "a", Factors: []selection.Factor{{ID: "m", Multiplier: 1}, {ID: "m", Multiplier: 2}}}}},
		{"negative factor", []selection.Candidate{{ID: "a", Factors: []selection.Factor{{ID: "m", Multiplier: -1}}}}},
		{"invalid excluded row", []selection.Candidate{{ID: "a", Weight: -1, Requirements: []selection.Requirement{{ID: "key", Verdict: selection.Unmet}}}}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			pool, err := selection.Evaluate(&selection.EvaluateInput{Candidates: tc.rows})
			s.Nil(pool)
			s.ErrorIs(err, selection.ErrInvalidCandidate)
		})
	}
	pool, err := selection.Evaluate(nil)
	s.Nil(pool)
	s.ErrorIs(err, selection.ErrNilInput)
}

func (s *SelectionSuite) TestOverflowIsRefused() {
	maxInt := int(^uint(0) >> 1)
	cases := []struct {
		name string
		rows []selection.Candidate
	}{
		{"product", []selection.Candidate{{ID: "a", Weight: maxInt, Factors: []selection.Factor{{ID: "m", Multiplier: 2}}}}},
		{"sum", []selection.Candidate{{ID: "a", Weight: maxInt}, {ID: "b", Weight: 1}}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			pool, err := selection.Evaluate(&selection.EvaluateInput{Candidates: tc.rows})
			s.Nil(pool)
			s.ErrorIs(err, selection.ErrOverflow)
		})
	}
	// A rejected row carries evidence, not a contribution to the die.
	pool, err := selection.Evaluate(&selection.EvaluateInput{Candidates: []selection.Candidate{
		{ID: "excluded", Weight: maxInt, Requirements: []selection.Requirement{{ID: "gate", Verdict: selection.Unmet}}, Factors: []selection.Factor{{ID: "m", Multiplier: 2}}},
		{ID: "safe", Weight: 3, Factors: []selection.Factor{{ID: "first", Multiplier: 2}, {ID: "second", Multiplier: 4}}},
	}})
	s.Require().NoError(err)
	out, err := pool.Inspect()
	s.Require().NoError(err)
	s.Equal(24, out.Total)
	s.Zero(out.Rows[0].Weight)
}

func (s *SelectionSuite) TestInspectionIsDetached() {
	input := &selection.EvaluateInput{Candidates: []selection.Candidate{{ID: "search", Weight: 2,
		Requirements: []selection.Requirement{{ID: "lead", Verdict: selection.Met, Reason: "known"}}, Factors: []selection.Factor{{ID: "profile", Multiplier: 3}}}}}
	pool, err := selection.Evaluate(input)
	s.Require().NoError(err)
	input.Candidates[0].ID = "changed"
	input.Candidates[0].Requirements[0].Verdict = selection.Unmet
	input.Candidates[0].Factors[0].Multiplier = 99
	first, err := pool.Inspect()
	s.Require().NoError(err)
	s.Equal("search", first.Rows[0].ID)
	s.Equal(selection.Met, first.Rows[0].Requirements[0].Verdict)
	s.Equal(6, first.Total)
	first.Rows[0].Requirements[0].Reason = "changed"
	first.Rows[0].Factors[0].Multiplier = 99
	first.Rows[0].Eligible = false
	first.Total = 999
	second, err := pool.Inspect()
	s.Require().NoError(err)
	s.Equal("known", second.Rows[0].Requirements[0].Reason)
	s.Equal(3, second.Rows[0].Factors[0].Multiplier)
	s.True(second.Rows[0].Eligible)
	s.Equal(6, second.Total)
}

func (s *SelectionSuite) TestEmptyAndUninitializedEvaluationsDiffer() {
	pool, err := selection.Evaluate(&selection.EvaluateInput{})
	s.Require().NoError(err)
	out, err := pool.Inspect()
	s.Require().NoError(err)
	s.Empty(out.Rows)
	s.Zero(out.Eligible)
	s.Zero(out.Total)
	for _, invalid := range []*selection.Evaluation{nil, {}} {
		out, err = invalid.Inspect()
		s.Nil(out)
		s.ErrorIs(err, selection.ErrNoEvaluation)
	}
}
