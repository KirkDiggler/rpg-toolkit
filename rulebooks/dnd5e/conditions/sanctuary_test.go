// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
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
		MemberID: "ward", SourceID: source, SourceRef: refs.Spells.Sanctuary(), SaveDC: 13,
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
	s.Equal(13, back.SaveDC, "the ward keeps the DC it was cast with")
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
		input.SaveDC = 13
		_, err := NewSanctuaryCondition(input)
		s.Error(err)
	}
}

// A ward with no DC is unreadable, and a save against DC 0 always succeeds,
// so the constructor refuses it rather than building a ward that protects
// nobody (rpg-toolkit#1965).
func (s *SanctuarySuite) TestRefusesAWardWithNoSaveDC() {
	for _, dc := range []int{0, -1} {
		_, err := NewSanctuaryCondition(NewSanctuaryConditionInput{
			MemberID: "ward", SourceID: "cleric", SourceRef: refs.Spells.Sanctuary(), SaveDC: dc,
		})
		s.Require().ErrorContains(err, "spell save DC", "DC %d", dc)
	}
}

// WardSaveDC hands back the ward's own DC.
func (s *SanctuarySuite) TestWardSaveDCIsTheWardsOwn() {
	dc, err := s.condition("cleric-a").WardSaveDC()
	s.Require().NoError(err)
	s.Equal(13, dc)
}

// A ward stored before wards kept a DC still LOADS — refusing would brick the
// whole sheet until a long rest — but it loads as SaveDC 0 and the ward itself
// refuses to answer a DC. The decision is pinned here so neither half can
// drift: load stays lenient, the reading stays fail-closed.
func (s *SanctuarySuite) TestAWardStoredWithoutADCLoadsButRefusesItsDC() {
	loaded, err := LoadJSON(json.RawMessage(`{
		"ref":{"module":"dnd5e","type":"conditions","id":"sanctuary"},
		"member_id":"ward","source_id":"cleric",
		"source_ref":{"module":"dnd5e","type":"spells","id":"sanctuary"}
	}`))
	s.Require().NoError(err)
	ward, ok := loaded.(*SanctuaryCondition)
	s.Require().True(ok)
	s.Zero(ward.SaveDC)

	_, err = ward.WardSaveDC()
	s.Require().ErrorIs(err, ErrWardWithoutDC)
}

// The factory reads the DC from the parameter the cast effect's SaveDCKey
// names, and a config without one is refused the same way.
func (s *SanctuarySuite) TestTheFactoryTakesTheDCFromConfig() {
	built, err := CreateFromRef(&CreateFromRefInput{
		Ref: refs.Conditions.Sanctuary().String(), MemberID: "ward", SourceRef: refs.Spells.Sanctuary().String(),
		Config: json.RawMessage(`{"source_id":"cleric","save_dc":14}`),
	})
	s.Require().NoError(err)
	ward, ok := built.Condition.(*SanctuaryCondition)
	s.Require().True(ok)
	s.Equal(14, ward.SaveDC)

	_, err = CreateFromRef(&CreateFromRefInput{
		Ref: refs.Conditions.Sanctuary().String(), MemberID: "ward", SourceRef: refs.Spells.Sanctuary().String(),
		Config: json.RawMessage(`{"source_id":"cleric"}`),
	})
	s.Require().ErrorContains(err, "spell save DC")
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
