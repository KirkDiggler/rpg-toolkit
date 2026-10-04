package contributions_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/stretchr/testify/suite"
)

type decisionSuite struct{ suite.Suite }

func TestDecisionSuite(t *testing.T) { suite.Run(t, new(decisionSuite)) }

func (s *decisionSuite) TestThreeAnswersRemainDistinct() {
	for _, decision := range []contributions.Decision{
		{Applicability: contributions.Applies, Reason: "Known qualifying neighbor"},
		{Applicability: contributions.DoesNotApply, Reason: "Already used this turn"},
		{Applicability: contributions.NeedsContext, Reason: "Choose a target",
			Needs: []contributions.Need{{Kind: contributions.NeedTarget}}},
	} {
		s.Require().NoError(decision.Validate())
	}
}

func (s *decisionSuite) TestInvalidAnswersAreErrorsNotUnknown() {
	for _, decision := range []contributions.Decision{
		{},
		{Applicability: contributions.Applies, Reason: " "},
		{Applicability: "unsupported", Reason: "Not an eligibility answer"},
		{Applicability: contributions.NeedsContext, Reason: "No actual need supplied"},
		{Applicability: contributions.Applies, Reason: "Contradictory",
			Needs: []contributions.Need{{Kind: contributions.NeedTarget}}},
		{Applicability: contributions.DoesNotApply, Reason: "Contradictory",
			Needs: []contributions.Need{{Kind: contributions.NeedTarget}}},
		{Applicability: contributions.NeedsContext, Reason: "Invalid need",
			Needs: []contributions.Need{{Kind: "arbitrary_predicate"}}},
	} {
		s.Error(decision.Validate(), "%+v", decision)
	}
}

func (s *decisionSuite) TestPairNeedsNameBothDistinctSubjects() {
	for _, kind := range []contributions.NeedKind{
		contributions.NeedDistance, contributions.NeedRelationship, contributions.NeedSight,
	} {
		s.Require().NoError((contributions.Need{Kind: kind, Subject: "actor", Other: "target"}).Validate())
		s.Error((contributions.Need{Kind: kind, Subject: "actor"}).Validate())
		s.Error((contributions.Need{Kind: kind, Subject: "actor", Other: "actor"}).Validate())
	}
	for _, kind := range []contributions.NeedKind{
		contributions.NeedUniverse, contributions.NeedTargetEffects, contributions.NeedReactionReadiness, contributions.NeedActionFacts,
	} {
		s.Require().NoError((contributions.Need{Kind: kind, Subject: "target"}).Validate())
		s.Error((contributions.Need{Kind: kind}).Validate())
		s.Error((contributions.Need{Kind: kind, Subject: "target", Other: "extra"}).Validate())
	}
	s.Error((contributions.Need{Kind: contributions.NeedTarget, Other: "extra"}).Validate())
}

func (s *decisionSuite) TestDetachedDecisionNeeds() {
	original := contributions.Decision{Applicability: contributions.NeedsContext,
		Reason: "Target context is incomplete",
		Needs:  []contributions.Need{{Kind: contributions.NeedTargetEffects, Subject: "target"}},
	}
	cloned := original.Clone()
	s.Equal(original, cloned)
	cloned.Needs[0].Subject = "changed"
	s.Equal("target", original.Needs[0].Subject)
}

func (s *decisionSuite) TestFacetsAreClosedAndHaveNoDefault() {
	for _, facet := range []contributions.Facet{contributions.AttackRoll, contributions.DamageNormal,
		contributions.DamageCritical, contributions.SpellDC, contributions.Healing} {
		s.True(facet.Valid())
	}
	s.False(contributions.Facet("").Valid())
	s.False(contributions.Facet("final_hp_loss").Valid())
}
