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
		{Applicability: contributions.Depends, Reason: "Depends on the target"},
	} {
		s.Require().NoError(decision.Validate())
	}
}

func (s *decisionSuite) TestDecisionRequiresReasonAndKnownApplicability() {
	for _, decision := range []contributions.Decision{
		{},
		{Applicability: contributions.Applies},
		{Applicability: contributions.Applies, Reason: " "},
		{Applicability: "needs_context", Reason: "The retired pending answer"},
		{Applicability: "unsupported", Reason: "Not an eligibility answer"},
	} {
		s.Error(decision.Validate(), "%+v", decision)
	}
}
