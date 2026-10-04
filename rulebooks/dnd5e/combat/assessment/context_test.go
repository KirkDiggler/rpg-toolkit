package assessment_test

import (
	"math"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/assessment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/stretchr/testify/suite"
)

type contextSuite struct{ suite.Suite }

func TestContextSuite(t *testing.T) { suite.Run(t, new(contextSuite)) }

func (s *contextSuite) fixture() assessment.Context {
	return assessment.Context{
		Observer: "actor",
		Members:  []assessment.MemberFacts{{ID: "target"}, {ID: "neighbor"}},
		Pairs: []assessment.PairFacts{{
			From: "target", To: "neighbor", DistanceCells: contributions.Known(1.0),
			Hostile: contributions.Known(true), CanSee: contributions.Known(false),
		}},
	}
}

func (s *contextSuite) TestPairFactsPreserveDirectionAndUnknown() {
	facts := s.fixture()
	s.Require().NoError(facts.Validate())
	out, err := facts.Pair(&assessment.PairInput{From: "target", To: "neighbor"})
	s.Require().NoError(err)
	distance, known := out.Facts.DistanceCells.Get()
	s.True(known)
	s.Equal(1.0, distance)
	hostile, known := out.Facts.Hostile.Get()
	s.True(known)
	s.True(hostile)
	sees, known := out.Facts.CanSee.Get()
	s.True(known)
	s.False(sees)

	reverse, err := facts.Pair(&assessment.PairInput{From: "neighbor", To: "target"})
	s.Require().NoError(err)
	_, known = reverse.Facts.Hostile.Get()
	s.False(known, "relationship is not inferred from the reverse pair")
	_, known = reverse.Facts.CanSee.Get()
	s.False(known)
	_, known = reverse.Facts.DistanceCells.Get()
	s.False(known, "the fact consumer must not calculate missing geometry")
	s.False(facts.Complete)
}

func (s *contextSuite) TestObserverOnlyIsValidButNotACompleteNeighborhood() {
	facts := assessment.Context{Observer: "actor"}
	s.Require().NoError(facts.Validate())
	s.False(facts.Complete)
	out, err := facts.Pair(&assessment.PairInput{From: "actor", To: "hidden"})
	s.Error(err)
	s.Nil(out)
	out, err = facts.Pair(nil)
	s.Error(err)
	s.Nil(out)
}

func (s *contextSuite) TestInvalidFactSetsAreRefused() {
	for name, mutate := range map[string]func(*assessment.Context){
		"no observer":        func(c *assessment.Context) { c.Observer = "" },
		"duplicate observer": func(c *assessment.Context) { c.Members[0].ID = c.Observer },
		"duplicate member":   func(c *assessment.Context) { c.Members[1].ID = c.Members[0].ID },
		"missing identity":   func(c *assessment.Context) { c.Members[0].ID = "" },
		"undeclared pair":    func(c *assessment.Context) { c.Pairs[0].To = "hidden" },
		"self pair":          func(c *assessment.Context) { c.Pairs[0].To = c.Pairs[0].From },
		"duplicate pair":     func(c *assessment.Context) { c.Pairs = append(c.Pairs, c.Pairs[0]) },
		"negative distance":  func(c *assessment.Context) { c.Pairs[0].DistanceCells = contributions.Known(-1.0) },
		"nonfinite distance": func(c *assessment.Context) { c.Pairs[0].DistanceCells = contributions.Known(math.NaN()) },
		"infinite distance":  func(c *assessment.Context) { c.Pairs[0].DistanceCells = contributions.Known(math.Inf(1)) },
	} {
		s.Run(name, func() {
			facts := s.fixture()
			mutate(&facts)
			s.Error(facts.Validate())
			out, err := facts.Pair(&assessment.PairInput{From: "target", To: "neighbor"})
			s.Error(err)
			s.Nil(out)
		})
	}
}

func (s *contextSuite) TestReturnedFactsAndClonesAreDetached() {
	facts := s.fixture()
	cloned := facts.Clone()
	cloned.Members[0].ID = "changed"
	cloned.Pairs[0].Hostile = contributions.Known(false)
	s.Equal("target", facts.Members[0].ID)
	out, err := facts.Pair(&assessment.PairInput{From: "target", To: "neighbor"})
	s.Require().NoError(err)
	out.Facts.Hostile = contributions.Known(false)
	out, err = facts.Pair(&assessment.PairInput{From: "target", To: "neighbor"})
	s.Require().NoError(err)
	hostile, known := out.Facts.Hostile.Get()
	s.True(known)
	s.True(hostile)
}
