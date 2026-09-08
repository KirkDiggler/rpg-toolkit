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
	condition := NewTrueStrikeCondition(s.casterID, s.targetID, "")
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

// The clock moved to the owner (rpg-project#407 R8): True Strike is a
// concentration cantrip, and the concentrating condition counts the turn ends.
// A count here as well would be two clocks answering one question, and the
// caster's advantage would vanish under them at whichever came first.
func (s *TrueStrikeConditionSuite) TestItCountsNoTurnEndsOfItsOwn() {
	condition := s.applied()

	var removed []dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removed = append(removed, event)
			return nil
		})
	s.Require().NoError(err)

	s.endTurn(s.casterID, 1)
	s.endTurn(s.casterID, 2)
	s.endTurn(s.casterID, 3)

	s.True(condition.IsApplied(), "nothing but its owner or the fight ends this")
	s.Empty(removed, "and it publishes no removal of its own on a turn end")
	s.Require().Len(s.attack(s.casterID, s.targetID).AdvantageSources, 1,
		"the advantage is still there, which is the whole point")
}

func (s *TrueStrikeConditionSuite) TestSomebodyElsesTurnEndDoesNothingEither() {
	condition := s.applied()

	s.endTurn(s.targetID, 1)
	s.endTurn("fighter-2", 1)

	s.True(condition.IsApplied())
}

func (s *TrueStrikeConditionSuite) TestItEndsWithTheFight() {
	condition := s.applied()

	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: s.casterID}))

	s.False(condition.IsApplied())
}

func (s *TrueStrikeConditionSuite) TestItRoundTripsThroughJSON() {
	condition := NewTrueStrikeCondition(s.casterID, s.targetID, "")

	raw, err := condition.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)

	back, ok := loaded.(*TrueStrikeCondition)
	s.Require().True(ok)
	s.Equal(s.casterID, back.MemberID)
	s.Equal(s.targetID, back.TargetID)
	s.Equal(refs.Spells.TrueStrike().String(), back.SourceRef)
	s.Equal(refs.Conditions.TrueStrike(), back.Ref())
}

func (s *TrueStrikeConditionSuite) TestItsSourceIsTheSpell() {
	s.Run("named by the caller", func() {
		out, err := CreateFromRef(&CreateFromRefInput{
			Ref:       refs.Conditions.TrueStrike().String(),
			MemberID:  s.casterID,
			Config:    json.RawMessage(`{"target_id":"goblin-1"}`),
			SourceRef: refs.Spells.TrueStrike().String(),
		})
		s.Require().NoError(err)

		built, ok := out.Condition.(*TrueStrikeCondition)
		s.Require().True(ok)
		s.Equal(refs.Spells.TrueStrike().String(), built.SourceRef)
	})

	s.Run("and defaulted when nobody named one", func() {
		s.Equal(refs.Spells.TrueStrike().String(),
			NewTrueStrikeCondition(s.casterID, s.targetID, "").SourceRef,
			"only True Strike applies this condition, so the blank is never the answer")
	})

	s.Run("even on a blob written before the field existed", func() {
		condition := &TrueStrikeCondition{}
		s.Require().NoError(condition.loadJSON(json.RawMessage(
			`{"ref":{"module":"dnd5e","type":"conditions","id":"true_strike"},` +
				`"member_id":"bard-1","target_id":"goblin-1","turn_ends_left":2}`)))
		s.Equal(refs.Spells.TrueStrike().String(), condition.SourceRef)
	})
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
