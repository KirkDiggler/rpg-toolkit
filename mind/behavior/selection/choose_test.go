package selection_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/selection"
)

type ChoiceSuite struct{ suite.Suite }

type drawContextKey struct{}

func TestChoiceSuite(t *testing.T) { suite.Run(t, new(ChoiceSuite)) }

type fixedDie struct {
	face    int
	err     error
	sizes   []int
	context context.Context
}

func (d *fixedDie) Roll(ctx context.Context, sides int) (int, error) {
	d.context = ctx
	d.sizes = append(d.sizes, sides)
	return d.face, d.err
}

func (s *ChoiceSuite) pool() *selection.Evaluation {
	pool, err := selection.Evaluate(&selection.EvaluateInput{Candidates: []selection.Candidate{
		{ID: "deliberate-only", Weight: 1, Factors: []selection.Factor{{ID: "zero", Multiplier: 0}}},
		{ID: "search", Weight: 2, Factors: []selection.Factor{{ID: "profile", Multiplier: 3}}, Requirements: []selection.Requirement{{ID: "lead", Verdict: selection.Met}}},
		{ID: "hold", Weight: 4},
		{ID: "secret", Weight: 1000, Requirements: []selection.Requirement{{ID: "known", Verdict: selection.Unknown}}},
	}})
	s.Require().NoError(err)
	return pool
}

func (s *ChoiceSuite) TestEveryFaceUsesAuthoredOrder() {
	pool := s.pool()
	for face := 1; face <= 10; face++ {
		die := &fixedDie{face: face}
		ctx := context.WithValue(context.Background(), drawContextKey{}, "draw context")
		out, err := selection.Draw(ctx, &selection.DrawInput{Evaluation: pool, Die: die})
		s.Require().NoError(err)
		want, index := "search", 1
		if face >= 7 {
			want, index = "hold", 2
		}
		s.Equal(want, out.Choice.Row.ID)
		s.Equal(index, out.Choice.Index)
		s.Equal(face, out.Roll)
		s.Equal(10, out.Total)
		s.Equal([]int{10}, die.sizes)
		s.Equal(ctx, die.context)
	}
}

func (s *ChoiceSuite) TestExplicitChoiceDoesNotNeedRandomness() {
	pool := s.pool()
	out, err := selection.Choose(&selection.ChooseInput{Evaluation: pool, ID: "deliberate-only"})
	s.Require().NoError(err)
	s.Equal(0, out.Index)
	s.True(out.Row.Eligible)
	s.Zero(out.Row.Weight)
	_, err = selection.Choose(&selection.ChooseInput{Evaluation: pool, ID: "secret"})
	s.ErrorIs(err, selection.ErrIneligible)
	_, err = selection.Choose(&selection.ChooseInput{Evaluation: pool, ID: "absent"})
	s.ErrorIs(err, selection.ErrUnknownID)
}

func (s *ChoiceSuite) TestSelectionRefusals() {
	ctx := context.Background()
	out, err := selection.Draw(ctx, nil)
	s.Nil(out)
	s.ErrorIs(err, selection.ErrNilInput)
	chosen, err := selection.Choose(nil)
	s.Nil(chosen)
	s.ErrorIs(err, selection.ErrNilInput)
	for _, e := range []*selection.Evaluation{nil, {}} {
		out, err = selection.Draw(ctx, &selection.DrawInput{Evaluation: e})
		s.Nil(out)
		s.ErrorIs(err, selection.ErrNoEvaluation)
		chosen, err = selection.Choose(&selection.ChooseInput{Evaluation: e, ID: "a"})
		s.Nil(chosen)
		s.ErrorIs(err, selection.ErrNoEvaluation)
	}
	die := &fixedDie{face: 1}
	for _, rows := range [][]selection.Candidate{nil, {{ID: "unmet", Weight: 10, Requirements: []selection.Requirement{{ID: "gate", Verdict: selection.Unmet}}}}} {
		pool, e := selection.Evaluate(&selection.EvaluateInput{Candidates: rows})
		s.Require().NoError(e)
		out, err = selection.Draw(ctx, &selection.DrawInput{Evaluation: pool, Die: die})
		s.Nil(out)
		s.ErrorIs(err, selection.ErrNoCandidates)
	}
	zero, e := selection.Evaluate(&selection.EvaluateInput{Candidates: []selection.Candidate{{ID: "zero"}}})
	s.Require().NoError(e)
	out, err = selection.Draw(ctx, &selection.DrawInput{Evaluation: zero, Die: die})
	s.Nil(out)
	s.ErrorIs(err, selection.ErrZeroWeight)
	s.Empty(die.sizes, "empty or zero pools consume no entropy")
	out, err = selection.Draw(ctx, &selection.DrawInput{Evaluation: s.pool()})
	s.Nil(out)
	s.ErrorIs(err, selection.ErrNoDie)
	out, err = selection.Draw(nil, &selection.DrawInput{Evaluation: s.pool(), Die: die}) //nolint:staticcheck // SA1012: intentional nil-context refusal control.
	s.Nil(out)
	s.ErrorIs(err, selection.ErrNilContext)
	s.Empty(die.sizes)
}

func (s *ChoiceSuite) TestInvalidFacesAndDieErrors() {
	for _, face := range []int{-1, 0, 11} {
		die := &fixedDie{face: face}
		out, err := selection.Draw(context.Background(), &selection.DrawInput{Evaluation: s.pool(), Die: die})
		s.Nil(out)
		s.ErrorIs(err, selection.ErrBadRoll)
		s.Equal([]int{10}, die.sizes)
	}
	sentinel := errors.New("entropy unavailable")
	die := &fixedDie{err: sentinel}
	out, err := selection.Draw(context.Background(), &selection.DrawInput{Evaluation: s.pool(), Die: die})
	s.Nil(out)
	s.ErrorIs(err, sentinel)
	s.Equal([]int{10}, die.sizes)
}

func (s *ChoiceSuite) TestChoiceEvidenceIsDetached() {
	pool := s.pool()
	chosen, err := selection.Choose(&selection.ChooseInput{Evaluation: pool, ID: "search"})
	s.Require().NoError(err)
	chosen.Row.Requirements[0].Verdict = selection.Unmet
	chosen.Row.Factors[0].Multiplier = 0
	die := &fixedDie{face: 6}
	drawn, err := selection.Draw(context.Background(), &selection.DrawInput{Evaluation: pool, Die: die})
	s.Require().NoError(err)
	s.Equal("search", drawn.Choice.Row.ID)
	s.Equal(6, drawn.Choice.Row.Weight)
	drawn.Choice.Row.Factors[0].Multiplier = 99
	again, err := selection.Choose(&selection.ChooseInput{Evaluation: pool, ID: "search"})
	s.Require().NoError(err)
	s.Equal(selection.Met, again.Row.Requirements[0].Verdict)
	s.Equal(3, again.Row.Factors[0].Multiplier)
	s.Equal([]int{10}, die.sizes, "inspection and explicit selection never rolled")
}
