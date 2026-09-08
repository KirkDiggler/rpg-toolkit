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

// ViciousMockeryConditionSuite is rpg-project#405's done-when for the rider a
// failed save delivers: exactly one attack roll takes the disadvantage, and it
// is gone at the end of the mocked creature's next turn.
type ViciousMockeryConditionSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	mockedID string
	bardID   string
}

func TestViciousMockeryConditionSuite(t *testing.T) {
	suite.Run(t, new(ViciousMockeryConditionSuite))
}

func (s *ViciousMockeryConditionSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.mockedID = "goblin-1"
	s.bardID = "bard-1"
}

// applied returns a live condition on the bus.
func (s *ViciousMockeryConditionSuite) applied() *ViciousMockeryCondition {
	condition := NewViciousMockeryCondition(s.mockedID, s.bardID)
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	return condition
}

// attack publishes one attack down the chain and returns the folded event.
func (s *ViciousMockeryConditionSuite) attack(attackerID string) dnd5eEvents.AttackChainEvent {
	event := dnd5eEvents.AttackChainEvent{AttackerID: attackerID, TargetID: "somebody"}
	staged := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)

	modified, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(s.ctx, event, staged)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)
	return folded
}

func (s *ViciousMockeryConditionSuite) TestItImposesDisadvantageOnExactlyOneAttackRoll() {
	condition := s.applied()

	first := s.attack(s.mockedID)

	s.Require().Len(first.DisadvantageSources, 1)
	s.Equal(refs.Conditions.ViciousMockery(), first.DisadvantageSources[0].SourceRef)
	s.Equal(s.bardID, first.DisadvantageSources[0].SourceID, "attributed to the bard who imposed it")
	s.False(condition.IsApplied())

	second := s.attack(s.mockedID)
	s.Empty(second.DisadvantageSources, "the next attack roll is unaffected")
}

func (s *ViciousMockeryConditionSuite) TestSomebodyElsesAttackIsUntouched() {
	condition := s.applied()

	folded := s.attack("orc-2")

	s.Empty(folded.DisadvantageSources)
	s.True(condition.IsApplied())
}

func (s *ViciousMockeryConditionSuite) TestItEndsAtTheMockedCreaturesNextTurnEnd() {
	condition := s.applied()

	var removed *dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removed = &event
			return nil
		})
	s.Require().NoError(err)

	s.Require().NoError(dnd5eEvents.TurnEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.TurnEndEvent{SubjectID: s.mockedID, Round: 1}))

	s.False(condition.IsApplied())
	s.Require().NotNil(removed)
	s.Equal(s.mockedID, removed.MemberID)
	s.Equal("expired", removed.Reason)
}

func (s *ViciousMockeryConditionSuite) TestTheBardsOwnTurnEndDoesNotEndIt() {
	condition := s.applied()

	s.Require().NoError(dnd5eEvents.TurnEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.TurnEndEvent{SubjectID: s.bardID, Round: 1}))

	s.True(condition.IsApplied(), "the boundary RAW names is the mocked creature's own turn")
}

func (s *ViciousMockeryConditionSuite) TestItEndsWithTheFight() {
	condition := s.applied()

	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: s.mockedID}))

	s.False(condition.IsApplied())
}

func (s *ViciousMockeryConditionSuite) TestItRoundTripsThroughJSON() {
	raw, err := NewViciousMockeryCondition(s.mockedID, s.bardID).ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)

	back, ok := loaded.(*ViciousMockeryCondition)
	s.Require().True(ok)
	s.Equal(s.mockedID, back.MemberID)
	s.Equal(s.bardID, back.SourceID)
	s.Equal(refs.Conditions.ViciousMockery(), back.Ref())
}

func (s *ViciousMockeryConditionSuite) TestTheFactoryRefusesAnUnattributedInsult() {
	_, err := CreateFromRef(&CreateFromRefInput{
		Ref:      refs.Conditions.ViciousMockery().String(),
		MemberID: s.mockedID,
	})

	s.Require().ErrorContains(err, "source_id")
}

func (s *ViciousMockeryConditionSuite) TestTheFactoryBuildsItFromItsCounterpartKey() {
	out, err := CreateFromRef(&CreateFromRefInput{
		Ref:      refs.Conditions.ViciousMockery().String(),
		MemberID: s.mockedID,
		Config:   json.RawMessage(`{"source_id":"bard-1"}`),
	})

	s.Require().NoError(err)
	built, ok := out.Condition.(*ViciousMockeryCondition)
	s.Require().True(ok)
	s.Equal(s.mockedID, built.MemberID)
	s.Equal(s.bardID, built.SourceID)
}
