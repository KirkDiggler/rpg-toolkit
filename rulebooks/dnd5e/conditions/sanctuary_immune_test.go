// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type SanctuaryImmuneSuite struct{ suite.Suite }

func TestSanctuaryImmuneSuite(t *testing.T) { suite.Run(t, new(SanctuaryImmuneSuite)) }

func (s *SanctuaryImmuneSuite) condition(source string) *SanctuaryImmuneCondition {
	s.T().Helper()
	condition, err := NewSanctuaryImmuneCondition(NewSanctuaryImmuneConditionInput{
		MemberID: "goblin-1", SourceID: source, SourceRef: refs.Spells.Sanctuary(),
	})
	s.Require().NoError(err)
	return condition
}

func (s *SanctuaryImmuneSuite) TestRoundTripAndDisplay() {
	condition := s.condition("cleric-a")
	s.Same(refs.Conditions.SanctuaryImmune(), condition.Ref())
	s.Equal("dnd5e:conditions:sanctuary_immune", condition.Ref().String())
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)
	s.Equal(condition.ConditionAddress(), ConditionAddressOf("goblin-1", loaded))
	back, ok := loaded.(*SanctuaryImmuneCondition)
	s.Require().True(ok)
	s.Equal(refs.Spells.Sanctuary(), back.SourceRef)
	s.Equal(SanctuaryImmuneTurnEnds, back.TurnEndsLeft, "a fresh grant's full count survives the round trip")
	display, ok := DisplayFor(*condition.Ref())
	s.Require().True(ok)
	s.Equal("Sanctuary Immune", display.Name)
}

func (s *SanctuaryImmuneSuite) TestRequiresAttackerAndBlockedCaster() {
	for _, input := range []NewSanctuaryImmuneConditionInput{
		{SourceID: "cleric", SourceRef: refs.Spells.Sanctuary()},
		{MemberID: "goblin-1", SourceRef: refs.Spells.Sanctuary()},
		{MemberID: "goblin-1", SourceID: "cleric"},
		{MemberID: "goblin-1", SourceID: "cleric", SourceRef: refs.Spells.Bless()},
	} {
		_, err := NewSanctuaryImmuneCondition(input)
		s.Error(err)
	}
}

func (s *SanctuaryImmuneSuite) TestItEndsWithTheFight() {
	ctx := context.Background()
	bus := events.NewEventBus()
	condition := s.condition("cleric-a")
	s.Require().NoError(condition.Apply(ctx, bus))

	var removed []dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removed = append(removed, event)
			return nil
		})
	s.Require().NoError(err)

	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(bus).Publish(ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: "goblin-1"}))

	s.Require().Len(removed, 1)
	s.Equal("goblin-1", removed[0].MemberID)
	s.Equal("cleric-a", removed[0].SourceID)
	s.Equal("combat ended", removed[0].Reason)
	s.False(condition.IsApplied())
}

func (s *SanctuaryImmuneSuite) TestSomebodyElsesFightEndingLeavesIt() {
	ctx := context.Background()
	bus := events.NewEventBus()
	condition := s.condition("cleric-a")
	s.Require().NoError(condition.Apply(ctx, bus))

	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(bus).Publish(ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: "someone-else"}))

	s.True(condition.IsApplied())
}

func (s *SanctuaryImmuneSuite) TestItEndsOnAnyRest() {
	ctx := context.Background()
	bus := events.NewEventBus()
	condition := s.condition("cleric-a")
	s.Require().NoError(condition.Apply(ctx, bus))

	var removed []dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removed = append(removed, event)
			return nil
		})
	s.Require().NoError(err)

	s.Require().NoError(dnd5eEvents.RestTopic.On(bus).Publish(ctx, dnd5eEvents.RestEvent{
		CharacterID: "goblin-1",
	}))

	s.Require().Len(removed, 1)
	s.Equal("rest", removed[0].Reason)
	s.False(condition.IsApplied())
}

// TestItExpiresAfterItsOwnTurnEndsRunOut pins the actual settled number
// (SanctuaryImmuneTurnEnds, 20 — deliberately longer than Sanctuary's own
// 10-turn ward, so the immunity outliving the spell that granted it is
// something a live table can actually observe) and the off-by-one BladeWard
// already established: the count is the HOLDER's own turn ends, not a raw
// event count.
func (s *SanctuaryImmuneSuite) TestItExpiresAfterItsOwnTurnEndsRunOut() {
	ctx := context.Background()
	bus := events.NewEventBus()
	condition := s.condition("cleric-a")
	s.Require().NoError(condition.Apply(ctx, bus))

	var removed []dnd5eEvents.ConditionRemovedEvent
	_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removed = append(removed, event)
			return nil
		})
	s.Require().NoError(err)

	for range SanctuaryImmuneTurnEnds - 1 {
		s.Require().NoError(dnd5eEvents.TurnEndTopic.On(bus).Publish(ctx,
			dnd5eEvents.TurnEndEvent{SubjectID: "goblin-1", Round: 1}))
	}
	s.Empty(removed, "the immunity survives every turn end but its last")
	s.True(condition.IsApplied())

	s.Require().NoError(dnd5eEvents.TurnEndTopic.On(bus).Publish(ctx,
		dnd5eEvents.TurnEndEvent{SubjectID: "goblin-1", Round: 1}))
	s.Require().Len(removed, 1)
	s.Equal("expired", removed[0].Reason)
	s.Equal("cleric-a", removed[0].SourceID)
	s.False(condition.IsApplied())
}

func (s *SanctuaryImmuneSuite) TestSomebodyElsesTurnEndDoesNotSpendIt() {
	ctx := context.Background()
	bus := events.NewEventBus()
	condition := s.condition("cleric-a")
	s.Require().NoError(condition.Apply(ctx, bus))

	for range SanctuaryImmuneTurnEnds + 5 {
		s.Require().NoError(dnd5eEvents.TurnEndTopic.On(bus).Publish(ctx,
			dnd5eEvents.TurnEndEvent{SubjectID: "someone-else", Round: 1}))
	}

	s.True(condition.IsApplied(), "other members' turns do not own this clock")
	s.Equal(SanctuaryImmuneTurnEnds, condition.TurnEndsLeft)
}

func (s *SanctuaryImmuneSuite) TestApplyingTwiceIsRefused() {
	ctx := context.Background()
	bus := events.NewEventBus()
	condition := s.condition("cleric-a")
	s.Require().NoError(condition.Apply(ctx, bus))
	s.Require().Error(condition.Apply(ctx, bus))
}
