// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type SanctuarySuite struct{ suite.Suite }

func TestSanctuarySuite(t *testing.T) { suite.Run(t, new(SanctuarySuite)) }

func (s *SanctuarySuite) condition(source string) *SanctuaryCondition {
	s.T().Helper()
	condition, err := NewSanctuaryCondition(NewSanctuaryConditionInput{
		MemberID: "ward", SourceID: source, SourceRef: refs.Spells.Sanctuary(),
	})
	s.Require().NoError(err)
	return condition
}

func (s *SanctuarySuite) TestRoundTripAndDisplay() {
	condition := s.condition("cleric-a")
	s.Same(refs.Conditions.Sanctuary(), condition.Ref())
	s.Equal("dnd5e:conditions:sanctuary", condition.Ref().String())
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)
	s.Equal(condition.ConditionAddress(), ConditionAddressOf("ward", loaded))
	back, ok := loaded.(*SanctuaryCondition)
	s.Require().True(ok)
	s.Equal(refs.Spells.Sanctuary(), back.SourceRef)
	display, ok := DisplayFor(*condition.Ref())
	s.Require().True(ok)
	s.Equal("Sanctuary", display.Name)
}

func (s *SanctuarySuite) TestRequiresWardAndCanonicalSource() {
	for _, input := range []NewSanctuaryConditionInput{
		{SourceID: "cleric", SourceRef: refs.Spells.Sanctuary()},
		{MemberID: "ward", SourceRef: refs.Spells.Sanctuary()},
		{MemberID: "ward", SourceID: "cleric"},
		{MemberID: "ward", SourceID: "cleric", SourceRef: refs.Spells.Bless()},
	} {
		_, err := NewSanctuaryCondition(input)
		s.Error(err)
	}
}

func (s *SanctuarySuite) TestApplyAndRemove() {
	ctx := context.Background()
	bus := events.NewEventBus()
	condition := s.condition("cleric-a")

	s.Require().NoError(condition.Apply(ctx, bus))
	s.True(condition.IsApplied())
	s.Require().Error(condition.Apply(ctx, bus), "applying twice is refused")

	s.Require().NoError(condition.Remove(ctx, bus))
	s.False(condition.IsApplied())
	s.Require().NoError(condition.Remove(ctx, bus), "removing an already-removed condition is a no-op")
}

func (s *SanctuarySuite) TestLongRestRemovesIt() {
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
		RestType: coreResources.ResetLongRest, CharacterID: "ward",
	}))

	s.Require().Len(removed, 1)
	s.Equal("ward", removed[0].MemberID)
	s.Equal(refs.Conditions.Sanctuary().String(), removed[0].ConditionRef)
	s.Equal("cleric-a", removed[0].SourceID)
	s.False(condition.IsApplied())
}
