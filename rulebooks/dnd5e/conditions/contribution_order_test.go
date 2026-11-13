// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type contributionOrderSuite struct{ suite.Suite }

func TestContributionOrderSuite(t *testing.T) { suite.Run(t, new(contributionOrderSuite)) }

type orderedGiftInput struct {
	ID, User, Group, Dice string
	Polarity              effects.Polarity
	Turns                 int
	Resistance            bool
}

func (s *contributionOrderSuite) world() *definitionWorld {
	w, err := newDefinitionWorld()
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(w.close()) })
	return w
}
func (s *contributionOrderSuite) give(w *definitionWorld, in orderedGiftInput) {
	benefits := []effects.PartSpec{{Key: "roll", Effect: effects.AddToRoll{Amount: effects.SignedDice{Notation: in.Dice, Polarity: in.Polarity}, Rolls: []contributions.RollKind{contributions.RollKindAttack, contributions.RollKindSavingThrow}, Selection: generic.SelectionRule{Mode: generic.OldestApplicable, Group: in.Group}}, OnTerminal: generic.EndPart}}
	if in.Resistance {
		benefits = append(benefits, effects.PartSpec{Key: "resistance", Effect: effects.Resistance{Types: []damage.Type{damage.Slashing}}, OnTerminal: generic.EndPart})
	}
	definitions, err := conditions.CompileComposition(&conditions.CompileCompositionInput{ID: in.ID, Spec: conditions.CompositionSpec{Name: in.ID, Coordinator: conditions.DefinitionUser, Benefits: benefits, Maintenance: []effects.PartSpec{
		{Key: "duration", Effect: effects.Duration{TurnEnds: in.Turns}, OnTerminal: generic.EndApplication},
		{Key: "rest", Effect: effects.EndOn{Beat: effects.BeatRest, Reason: "rest"}, OnTerminal: generic.EndApplication},
	}}})
	s.Require().NoError(err)
	source := newFixtureSource(in.ID, in.User, in.ID)
	grant, err := useDefinition(context.Background(), definitionUseInput{Source: source, RecipientID: "B", ApplicationID: in.ID, Definitions: definitions.Definitions, Participants: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	s.Require().NoError(w.apply(grant.Prepared))
}
func (s *contributionOrderSuite) selected(w *definitionWorld) []string {
	out, err := w.holder("B").DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: contributions.RollKindAttack})
	s.Require().NoError(err)
	var sources []string
	for _, term := range out.Contributions {
		sources = append(sources, term.Source.SourceID)
	}
	return sources
}

func (s *contributionOrderSuite) TestCharacterOrderSurvivesReloadAndWinnerRemoval() {
	w := s.world()
	s.give(w, orderedGiftInput{ID: "z-old", User: "A", Group: "bless", Dice: "1d4", Polarity: effects.Positive, Turns: 5})
	s.give(w, orderedGiftInput{ID: "a-new", User: "C", Group: "bless", Dice: "1d6", Polarity: effects.Positive, Turns: 5})
	s.give(w, orderedGiftInput{ID: "bane", User: "B", Group: "bane", Dice: "1d4", Polarity: effects.Negative, Turns: 5})
	s.Equal([]string{"z-old", "bane"}, s.selected(w))
	s.Require().NoError(w.turnEnd("C"))
	before, err := w.maintenance("C", "a-new")
	s.Require().NoError(err)
	s.Require().NotNil(before.Duration)
	s.Equal(4, before.Duration.Remaining)
	s.Require().NoError(w.reload())
	// Change runtime registration order without changing the persisted holder
	// order. Selection cannot acquire a second age authority from subscriptions.
	current := w.holder("B").GetConditions()
	for _, condition := range current {
		s.Require().NoError(condition.Remove(context.Background(), w.bus))
	}
	for i := len(current) - 1; i >= 0; i-- {
		s.Require().NoError(current[i].Apply(context.Background(), w.bus))
	}
	s.Equal([]string{"z-old", "bane"}, s.selected(w))
	s.Require().NoError(w.rest("A"))
	s.Equal([]string{"a-new", "bane"}, s.selected(w))
	after, err := w.maintenance("C", "a-new")
	s.Require().NoError(err)
	s.Equal(4, after.Duration.Remaining, "becoming selected does not restart a timer")
	s.Require().NoError(w.binding.CheckComplete())
	s.Require().NoError(w.reload())
	s.Equal([]string{"a-new", "bane"}, s.selected(w))
}

func (s *contributionOrderSuite) TestNonselectedExpiryDoesNotRevive() {
	w := s.world()
	s.give(w, orderedGiftInput{ID: "old", User: "A", Group: "bless", Dice: "1d4", Polarity: effects.Positive, Turns: 5})
	s.give(w, orderedGiftInput{ID: "new", User: "C", Group: "bless", Dice: "1d6", Polarity: effects.Positive, Turns: 1})
	s.Equal([]string{"old"}, s.selected(w))
	s.Require().NoError(w.turnEnd("C"))
	s.Empty(w.holder("C").GetConditions(), "a nonselected application's own duration still advances")
	s.Equal([]string{"old"}, s.selected(w))
	s.Require().NoError(w.reload())
	s.Require().NoError(w.rest("A"))
	s.Empty(s.selected(w), "removing the winner cannot revive an already-expired loser")
	s.Empty(w.holder("B").GetConditions())
	s.Require().NoError(w.binding.CheckComplete())
}

func (s *contributionOrderSuite) TestMixedConditionLosesOnlyOneContribution() {
	w := s.world()
	s.give(w, orderedGiftInput{ID: "old", User: "A", Group: "bless", Dice: "1d4", Polarity: effects.Positive, Turns: 5})
	s.give(w, orderedGiftInput{ID: "mixed", User: "C", Group: "bless", Dice: "1d6", Polarity: effects.Positive, Turns: 5, Resistance: true})
	s.Equal([]string{"old"}, s.selected(w))
	result, err := w.incoming("B", damage.Slashing, 9)
	s.Require().NoError(err)
	s.Equal(4, result.Types[0].Taken, "losing roll selection does not suspend sibling resistance")
	s.Require().NoError(w.rest("A"))
	s.Equal([]string{"mixed"}, s.selected(w))
	selected, err := w.holder("B").DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: contributions.RollKindAttack})
	s.Require().NoError(err)
	s.Equal("1d6", selected.Contributions[0].Dice)
	result, err = w.incoming("B", damage.Slashing, 9)
	s.Require().NoError(err)
	s.Equal(4, result.Types[0].Taken)
}
