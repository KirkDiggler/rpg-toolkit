// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// TrueStrikeConditionSuite is rpg-project#405's done-when for the gateless
// half of the cast door: the advantage lands on the named creature and nobody
// else, and it is gone after the caster's next turn.
type TrueStrikeConditionSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	casterID string
	targetID string
}

func TestTrueStrikeConditionSuite(t *testing.T) {
	suite.Run(t, new(TrueStrikeConditionSuite))
}

func (s *TrueStrikeConditionSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.casterID = "bard-1"
	s.targetID = "goblin-1"
}

// applied returns a live condition on the bus.
func (s *TrueStrikeConditionSuite) applied() *TrueStrikeCondition {
	condition := NewTrueStrikeCondition(s.casterID, s.targetID)
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	return condition
}

// attack publishes one attack down the chain and returns the folded event.
func (s *TrueStrikeConditionSuite) attack(attackerID, targetID string) dnd5eEvents.AttackChainEvent {
	event := dnd5eEvents.AttackChainEvent{AttackerID: attackerID, TargetID: targetID}
	staged := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)

	modified, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(s.ctx, event, staged)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)
	return folded
}

// endTurn publishes one turn end for the given subject.
func (s *TrueStrikeConditionSuite) endTurn(subjectID string, round int) {
	s.Require().NoError(dnd5eEvents.TurnEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.TurnEndEvent{SubjectID: subjectID, Round: round}))
}

func (s *TrueStrikeConditionSuite) TestItAppendsAdvantageAgainstTheNamedTarget() {
	condition := s.applied()

	folded := s.attack(s.casterID, s.targetID)

	s.Require().Len(folded.AdvantageSources, 1)
	s.Equal(refs.Conditions.TrueStrike(), folded.AdvantageSources[0].SourceRef)
	s.Equal(s.casterID, folded.AdvantageSources[0].SourceID)
	s.False(condition.IsApplied(), "one attack, then gone")
}

func (s *TrueStrikeConditionSuite) TestItAppendsNothingAgainstAnybodyElse() {
	condition := s.applied()

	folded := s.attack(s.casterID, "orc-2")

	s.Empty(folded.AdvantageSources, "the advantage is good against one creature only")
	s.True(condition.IsApplied(), "and an attack elsewhere does not spend it")
}

func (s *TrueStrikeConditionSuite) TestSomebodyElsesAttackIsUntouched() {
	condition := s.applied()

	folded := s.attack("fighter-2", s.targetID)

	s.Empty(folded.AdvantageSources, "the advantage belongs to the caster")
	s.True(condition.IsApplied())
}

func (s *TrueStrikeConditionSuite) TestItSurvivesTheTurnItWasCastOn() {
	condition := s.applied()

	s.endTurn(s.casterID, 1)

	s.True(condition.IsApplied(),
		"a cantrip costs an action, so the first turn end is the casting turn's")
	s.Require().Len(s.attack(s.casterID, s.targetID).AdvantageSources, 1,
		"and the advantage is there on the next turn, which is the whole point")
}

func (s *TrueStrikeConditionSuite) TestItIsGoneAfterTheCastersNextTurn() {
	condition := s.applied()

	var removed *dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removed = &event
			return nil
		})
	s.Require().NoError(err)

	s.endTurn(s.casterID, 1)
	s.endTurn(s.casterID, 2)

	s.False(condition.IsApplied())
	s.Require().NotNil(removed)
	s.Equal(s.casterID, removed.MemberID)
	s.Equal("expired", removed.Reason)
	s.Empty(s.attack(s.casterID, s.targetID).AdvantageSources)
}

func (s *TrueStrikeConditionSuite) TestSomebodyElsesTurnEndDoesNotCountDown() {
	condition := s.applied()

	s.endTurn(s.targetID, 1)
	s.endTurn("fighter-2", 1)

	s.True(condition.IsApplied())
	s.Equal(TrueStrikeTurnEnds, condition.TurnEndsLeft)
}

func (s *TrueStrikeConditionSuite) TestItEndsWithTheFight() {
	condition := s.applied()

	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: s.casterID}))

	s.False(condition.IsApplied())
}

func (s *TrueStrikeConditionSuite) TestItRoundTripsThroughJSON() {
	condition := NewTrueStrikeCondition(s.casterID, s.targetID)
	condition.TurnEndsLeft = 1

	raw, err := condition.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)

	back, ok := loaded.(*TrueStrikeCondition)
	s.Require().True(ok)
	s.Equal(s.casterID, back.MemberID)
	s.Equal(s.targetID, back.TargetID)
	s.Equal(1, back.TurnEndsLeft)
	s.Equal(refs.Conditions.TrueStrike(), back.Ref())
}

func (s *TrueStrikeConditionSuite) TestAStoredCountOfZeroIsReadAsOneMoreTurn() {
	condition := &TrueStrikeCondition{}
	err := condition.loadJSON(json.RawMessage(
		`{"ref":{"module":"dnd5e","type":"conditions","id":"true_strike"},` +
			`"member_id":"bard-1","target_id":"goblin-1"}`))

	s.Require().NoError(err)
	s.Equal(1, condition.TurnEndsLeft, "a condition that vanished on load would be silent")
}

func (s *TrueStrikeConditionSuite) TestTheFactoryRefusesACastWithNoTarget() {
	_, err := CreateFromRef(&CreateFromRefInput{
		Ref:      refs.Conditions.TrueStrike().String(),
		MemberID: s.casterID,
	})

	s.Require().ErrorContains(err, "target_id")
}

func (s *TrueStrikeConditionSuite) TestTheFactoryBuildsItFromItsCounterpartKey() {
	out, err := CreateFromRef(&CreateFromRefInput{
		Ref:      refs.Conditions.TrueStrike().String(),
		MemberID: s.casterID,
		Config:   json.RawMessage(`{"target_id":"goblin-1"}`),
	})

	s.Require().NoError(err)
	built, ok := out.Condition.(*TrueStrikeCondition)
	s.Require().True(ok)
	s.Equal(s.casterID, built.MemberID)
	s.Equal(s.targetID, built.TargetID)
}
