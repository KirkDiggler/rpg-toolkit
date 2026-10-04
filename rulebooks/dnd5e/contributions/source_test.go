package contributions_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type sourceSuite struct{ suite.Suite }

func TestSourceSuite(t *testing.T) { suite.Run(t, new(sourceSuite)) }

func (s *sourceSuite) TestDetachedSourceAndEventAlias() {
	original := contributions.Source{
		Ref:  &core.Ref{Module: "dnd5e", Type: "conditions", ID: "blessed"},
		Name: "Bless", Label: "Attack roll", SourceID: "caster",
	}
	cloned := contributions.CloneSource(original)
	s.Equal(original, cloned)
	s.NotSame(original.Ref, cloned.Ref)
	cloned.Ref.ID = "changed"
	s.Equal("blessed", original.Ref.ID)
	// Passing both directions requires aliases, not a similar second DTO.
	eventSource := dndevents.CloneRollSource(original)
	leafSource := contributions.CloneSource(eventSource)
	s.Equal(original, leafSource)
	s.NotSame(original.Ref, leafSource.Ref)
}

func (s *sourceSuite) TestAbsentRefRemainsAbsent() {
	original := contributions.Source{Name: "A sourced fact"}
	s.Equal(original, contributions.CloneSource(original))
	s.Nil(contributions.CloneSource(original).Ref)
}

func (s *sourceSuite) TestUnresolvedDiceAndKindAliases() {
	value := contributions.DiceContribution{
		Source: contributions.Source{Name: "Bane", SourceID: "caster"},
		Dice:   "1d4", Subtract: true,
	}
	eventOutput := dndevents.DescribeRollContributionsOutput{
		Contributions: []dndevents.DiceContribution{value},
	}
	eventInput := dndevents.DescribeRollContributionsInput{Kind: contributions.RollKindAttack}
	s.Equal(value, eventOutput.Contributions[0])
	s.Equal(dndevents.RollKindAttack, eventInput.Kind)
	s.Equal(dndevents.RollKindSavingThrow, contributions.RollKindSavingThrow)
}
