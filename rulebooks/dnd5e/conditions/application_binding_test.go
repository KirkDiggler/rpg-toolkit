// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type applicationBindingSuite struct{ suite.Suite }

func TestApplicationBindingSuite(t *testing.T) { suite.Run(t, new(applicationBindingSuite)) }

func (s *applicationBindingSuite) blob(member string, config conditions.LinkedConditionConfig) json.RawMessage {
	raw, err := json.Marshal(config)
	s.Require().NoError(err)
	created, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: member,
		Origin: &dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster"}, Config: raw})
	s.Require().NoError(err)
	stored, err := created.Condition.ToJSON()
	s.Require().NoError(err)
	// These fixtures represent an already-delivered persisted graph. Fresh
	// pending construction is exercised through Prepare/CreateApplication instead.
	if config.Role == conditions.LinkedCoordinator {
		var data conditions.LinkedConditionData
		s.Require().NoError(json.Unmarshal(stored, &data))
		data.Runtime.Lifetime.LiveMembers = append([]string{}, data.Runtime.Lifetime.PendingMembers...)
		data.Runtime.Lifetime.PendingMembers = []string{}
		stored, err = json.Marshal(data)
		s.Require().NoError(err)
	}
	return stored
}

func (s *applicationBindingSuite) sheets() []*character.Character { return s.sheetsClock(1, false) }

func (s *applicationBindingSuite) sheetsClock(turns int, skip bool) []*character.Character {
	clock, err := json.Marshal(struct {
		TurnEnds int  `json:"turn_ends"`
		Skip     bool `json:"skip_first"`
	}{turns, skip})
	s.Require().NoError(err)
	owner := dndevents.ConditionAddress{MemberID: "caster", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "app"}
	members := []conditions.LinkedMemberBinding{
		{Key: "one", Address: dndevents.ConditionAddress{MemberID: "one", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "caster"}, Mode: conditions.LinkComposed},
		{Key: "two", Address: dndevents.ConditionAddress{MemberID: "two", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "caster"}, Mode: conditions.LinkComposed},
	}
	ownerBlob := s.blob("caster", conditions.LinkedConditionConfig{ApplicationID: "app", Role: conditions.LinkedCoordinator, AddressSourceID: "app",
		Bindings: &conditions.LinkedBindings{Name: "Bless", Members: members, Parts: []dndfx.PartDeclaration{
			{Key: "hold", Kind: dndfx.KindConcentration, OnTerminal: fx.EndApplication},
			{Key: "duration", Kind: dndfx.KindDuration, Params: clock, OnTerminal: fx.EndApplication},
			{Key: "rest", Kind: dndfx.KindEndOn, Params: json.RawMessage(`{"beat":"rest","reason":"rest"}`), OnTerminal: fx.EndApplication},
			{Key: "combat", Kind: dndfx.KindEndOn, Params: json.RawMessage(`{"beat":"combat_end","reason":"combat_end"}`), OnTerminal: fx.EndApplication},
			{Key: "empty", Kind: dndfx.KindUntilEmpty, OnTerminal: fx.EndApplication},
		}}})
	var sheets []*character.Character
	for _, id := range []string{"caster", "one", "two"} {
		data := &character.Data{ID: id, PlayerID: "player-" + id, Name: id, Level: 1, ProficiencyBonus: 2,
			ClassID: classes.Fighter, RaceID: races.Human, HitPoints: 12, MaxHitPoints: 12,
			AbilityScores: shared.AbilityScores{abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14, abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8}}
		if id == "caster" {
			data.Conditions = []json.RawMessage{ownerBlob}
		} else {
			data.Conditions = []json.RawMessage{s.blob(id, conditions.LinkedConditionConfig{ApplicationID: "app", Role: conditions.LinkedRecipient, AddressSourceID: "caster",
				Bindings: &conditions.LinkedBindings{Name: "Blessed", Owner: &owner, MemberKey: id, Parts: []dndfx.PartDeclaration{
					{Key: "dice", Kind: dndfx.KindRollModifier, Params: json.RawMessage(`{"dice":"1d4","group":"bless","rolls":["attack","saving_throw"]}`), OnTerminal: fx.EndPart},
				}}})}
		}
		sheet, err := character.Load(context.Background(), data)
		s.Require().NoError(err)
		sheets = append(sheets, sheet)
	}
	return sheets
}

func bindingParticipants(sheets []*character.Character) []conditions.ParticipantConditions {
	var participants []conditions.ParticipantConditions
	for _, sheet := range sheets {
		participants = append(participants, conditions.ParticipantConditions{MemberID: sheet.GetID(), Conditions: sheet.GetConditions()})
	}
	return participants
}

func (s *applicationBindingSuite) bind(sheets []*character.Character, bus events.EventBus) *conditions.ApplicationBinding {
	binding, err := conditions.BindApplications(&conditions.BindApplicationsInput{Participants: bindingParticipants(sheets), Bus: bus})
	s.Require().NoError(err)
	for _, sheet := range sheets {
		s.Require().NoError(character.Attach(context.Background(), sheet, bus))
	}
	return binding
}

func (s *applicationBindingSuite) TestEndOrderingAndExternalOwnerFactAreExactlyOnce() {
	for _, external := range []bool{false, true} {
		sheets := s.sheets()
		ctx, bus := context.Background(), events.NewEventBus()
		var order []string
		_, err := dndevents.ConditionRemovedTopic.On(bus).Subscribe(ctx, func(_ context.Context, event dndevents.ConditionRemovedEvent) error {
			order = append(order, "remove:"+event.MemberID)
			return nil
		})
		s.Require().NoError(err)
		_, err = dndevents.ConcentrationEndedTopic.On(bus).Subscribe(ctx, func(_ context.Context, event dndevents.ConcentrationEndedEvent) error {
			order = append(order, "end:"+event.Reason)
			return nil
		})
		s.Require().NoError(err)
		binding := s.bind(sheets, bus)
		if external {
			event := dndevents.ConditionRemovedEvent{MemberID: "caster", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "app", Reason: "recast"}
			s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, event))
			s.Equal([]string{"remove:caster", "remove:one", "remove:two", "end:recast"}, order)
		} else {
			s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "caster"}))
			s.Equal([]string{"remove:one", "remove:two", "end:duration", "remove:caster"}, order)
		}
		for _, sheet := range sheets {
			s.Empty(sheet.GetConditions())
			data, err := sheet.ToData()
			s.Require().NoError(err)
			s.Empty(data.Conditions)
		}
		s.Require().NoError(binding.Close(ctx))
	}
}

func (s *applicationBindingSuite) TestChildRemovalAndDuplicateNoticePreserveOtherMembers() {
	sheets := s.sheets()
	ctx, bus := context.Background(), events.NewEventBus()
	binding := s.bind(sheets, bus)
	lost := dndevents.ConditionRemovedEvent{MemberID: "one", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "caster", Reason: "consumed"}
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, lost))
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, lost))
	s.Require().Len(sheets[0].GetConditions(), 1)
	owner := sheets[0].GetConditions()[0].(*conditions.LinkedCondition)
	s.Equal([]string{"two"}, owner.LinkedApplicationInfo().LiveMembers)
	s.Require().Len(sheets[2].GetConditions(), 1)
	lost.MemberID = "two"
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, lost))
	s.Empty(sheets[0].GetConditions(), "last actual child loss ends until_empty")
	s.Empty(sheets[2].GetConditions())
	s.Require().NoError(binding.Close(ctx))
}

func (s *applicationBindingSuite) TestMissingHolderOrMismatchedSheetFailsBeforeMutation() {
	sheets := s.sheets()
	bus := events.NewEventBus()
	_, err := conditions.BindApplications(&conditions.BindApplicationsInput{Participants: bindingParticipants(sheets[:2]), Bus: bus})
	s.Error(err)
	owner := sheets[0].GetConditions()[0].(*conditions.LinkedCondition)
	s.Equal([]string{"one", "two"}, owner.LinkedApplicationInfo().LiveMembers)
	s.False(owner.IsApplied())
	_, err = owner.ToJSON()
	s.Require().NoError(err)
	participants := bindingParticipants(sheets)
	participants[2].MemberID = "wrong-holder"
	_, err = conditions.BindApplications(&conditions.BindApplicationsInput{Participants: participants, Bus: bus})
	s.Error(err)
	s.Equal([]string{"one", "two"}, owner.LinkedApplicationInfo().LiveMembers)
}

func (s *applicationBindingSuite) TestBindingTeardownDoesNotEndSavedApplications() {
	sheets := s.sheets()
	ctx, bus := context.Background(), events.NewEventBus()
	binding := s.bind(sheets, bus)
	s.Require().NoError(binding.Close(ctx))
	s.Require().NoError(binding.Close(ctx))
	for _, sheet := range sheets {
		s.Require().Len(sheet.GetConditions(), 1)
		data, err := sheet.ToData()
		s.Require().NoError(err)
		s.Require().Len(data.Conditions, 1)
	}
}
