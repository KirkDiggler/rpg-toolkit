// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func (s *applicationBindingSuite) reloadConditions(sheet *character.Character, stored ...json.RawMessage) *character.Character {
	data, err := sheet.ToData()
	s.Require().NoError(err)
	data.Conditions = stored
	loaded, err := character.Load(context.Background(), data)
	s.Require().NoError(err)
	return loaded
}

func (s *applicationBindingSuite) TestForeignAndCompetingOwnersAreRefused() {
	sheets := s.sheets()
	data, err := sheets[1].ToData()
	s.Require().NoError(err)
	var linked conditions.LinkedConditionData
	s.Require().NoError(json.Unmarshal(data.Conditions[0], &linked))
	linked.Bindings.Owner.MemberID = "two"
	raw, err := json.Marshal(linked)
	s.Require().NoError(err)
	sheets[1] = s.reloadConditions(sheets[1], raw)
	_, err = conditions.BindApplications(&conditions.BindApplicationsInput{Participants: bindingParticipants(sheets), Bus: events.NewEventBus()})
	s.Require().Error(err)
	s.Contains(err.Error(), "foreign application or owner")

	sheets = s.sheets()
	raging, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: "one",
		Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "one"}, Config: json.RawMessage(`{"damage_bonus":{"fixed":2}}`)})
	s.Require().NoError(err)
	rage, err := raging.Condition.ToJSON()
	s.Require().NoError(err)
	sheets[1] = s.reloadConditions(sheets[1], rage)
	for _, index := range []int{0, 2} {
		id := sheets[index].GetID()
		owner := s.blob(id, conditions.LinkedConditionConfig{ApplicationID: id + "-app", AddressSourceID: id + "-app", Role: conditions.LinkedCoordinator,
			Bindings: &conditions.LinkedBindings{Name: "Timed effect", Parts: []dndfx.PartDeclaration{
				{Key: "clock", Kind: dndfx.KindDuration, Params: json.RawMessage(`{"turn_ends":2}`), OnTerminal: fx.EndApplication},
			}, Members: []conditions.LinkedMemberBinding{{Key: "rage", Mode: conditions.LinkAdopted,
				Address: dndevents.ConditionAddress{MemberID: "one", ConditionRef: refs.Conditions.Raging().String()}}}}})
		sheets[index] = s.reloadConditions(sheets[index], owner)
	}
	_, err = conditions.BindApplications(&conditions.BindApplicationsInput{Participants: bindingParticipants(sheets), Bus: events.NewEventBus()})
	s.Require().Error(err)
	s.Contains(err.Error(), "claimed by multiple owners", "the test must reach competing ownership, not fail on a foreign backlink")
}

func (s *applicationBindingSuite) TestKeeperIgnoresPriorInvocationEvenWithoutBinding() {
	sheets := s.sheets()
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(character.Attach(ctx, sheets[1], bus))
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{
		MemberID: "one", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "caster", ApplicationID: "prior", Reason: "late",
	}))
	s.Require().Len(sheets[1].GetConditions(), 1)
}

func (s *applicationBindingSuite) TestSelfRecipientAndIndependentPayloadKeepTheirOwnLifetimes() {
	sheets := s.sheets()
	ctx, bus := context.Background(), events.NewEventBus()
	binding := s.bind(sheets, bus)
	spec := applicationSpec()
	spec.Recipients = append(spec.Recipients, conditions.RecipientSpec{Ref: refs.Conditions.Dodging().String(), Lifetime: conditions.LifetimeIndependent})
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "new-app", CoordinatorID: "caster",
		Origin: dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster"}, Spec: spec, Existing: bindingParticipants(sheets)})
	s.Require().NoError(err)
	old, _ := prepared.Replacement()
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{
		MemberID: old.MemberID, ConditionRef: old.ConditionRef, SourceID: old.SourceID, Reason: "recast",
	}))
	var receipts []conditions.DeliveryReceipt
	for _, delivery := range []struct{ sheet, spec int }{{0, 0}, {1, 0}, {1, 1}} {
		created, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: delivery.spec, MemberID: sheets[delivery.sheet].GetID()})
		s.Require().NoError(err)
		s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{
			Target: sheets[delivery.sheet], Source: dndevents.ConditionSourceSpell, Condition: created.Condition,
		}))
		receipts = append(receipts, created.Receipt)
	}
	sealed, err := conditions.SealApplication(&conditions.SealApplicationInput{Prepared: prepared, Delivered: receipts, Binding: binding})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{
		Target: sheets[0], Source: dndevents.ConditionSourceSpell, Condition: sealed.Condition,
	}))
	s.Require().NoError(binding.CheckComplete())
	self := receipts[0].Address
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{
		MemberID: self.MemberID, ConditionRef: self.ConditionRef, SourceID: self.SourceID, ApplicationID: "new-app", Reason: "consumed",
	}))
	owner := sealed.Condition.(*conditions.LinkedCondition)
	s.Require().Len(owner.LinkedApplicationInfo().LiveMembers, 1)
	s.Require().Len(sheets[0].GetConditions(), 1, "losing self's benefit does not end the remaining recipient")
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "caster"}))
	s.Empty(sheets[0].GetConditions())
	s.Require().Len(sheets[1].GetConditions(), 1)
	s.Equal(refs.Conditions.Dodging().String(), sheets[1].GetConditions()[0].Ref().String())
	s.Require().NoError(binding.Close(ctx))
}
