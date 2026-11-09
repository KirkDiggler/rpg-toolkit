// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func applicationSpec() conditions.ApplicationSpec {
	return conditions.ApplicationSpec{Name: "Bless", CoordinatorParts: []dndfx.PartDeclaration{
		{Key: "hold", Kind: dndfx.KindConcentration, OnTerminal: fx.EndApplication},
		{Key: "duration", Kind: dndfx.KindDuration, Params: json.RawMessage(`{"turn_ends":1}`), OnTerminal: fx.EndApplication},
		{Key: "empty", Kind: dndfx.KindUntilEmpty, OnTerminal: fx.EndApplication},
	}, Recipients: []conditions.RecipientSpec{{Ref: refs.Conditions.LinkedEffects().String(), Name: "Blessed", AddressSourceID: "caster", Lifetime: conditions.LifetimeShared,
		Parts: []dndfx.PartDeclaration{{Key: "dice", Kind: dndfx.KindRollModifier,
			Params: json.RawMessage(`{"dice":"1d4","group":"bless","rolls":["attack","saving_throw"]}`), OnTerminal: fx.EndPart}},
	}}}
}

func (s *applicationBindingSuite) TestPreparationIsPureAndOnlyDeliveredMembersSeal() {
	sheets := s.sheets()
	ctx, bus := context.Background(), events.NewEventBus()
	binding := s.bind(sheets, bus)
	var removed []dndevents.ConditionRemovedEvent
	_, err := dndevents.ConditionRemovedTopic.On(bus).Subscribe(ctx, func(_ context.Context, event dndevents.ConditionRemovedEvent) error {
		removed = append(removed, event)
		return nil
	})
	s.Require().NoError(err)
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{
		ID: "next", CoordinatorID: "caster", Origin: dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster"},
		Spec: applicationSpec(), Existing: bindingParticipants(sheets),
	})
	s.Require().NoError(err)
	s.Empty(removed)
	for _, sheet := range sheets {
		s.Require().Len(sheet.GetConditions(), 1, "preparation does not displace or apply")
	}
	replacement, exists := prepared.Replacement()
	s.True(exists)
	s.Equal("app", replacement.SourceID)
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{
		MemberID: replacement.MemberID, ConditionRef: replacement.ConditionRef, SourceID: replacement.SourceID, Reason: "recast",
	}))
	var receipts []conditions.DeliveryReceipt
	for _, sheet := range sheets[1:] {
		created, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0, MemberID: sheet.GetID()})
		s.Require().NoError(err)
		s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{
			Target: sheet, Source: dndevents.ConditionSourceSpell, Condition: created.Condition,
		}))
		receipts = append(receipts, created.Receipt)
	}
	s.Error(binding.CheckComplete(), "unsealed recipients cannot leave as a successful result")
	_, err = conditions.SealApplication(&conditions.SealApplicationInput{Prepared: prepared, Delivered: receipts[:1], Binding: binding})
	s.Error(err, "sealing must not abandon the second delivered recipient")
	sealed, err := conditions.SealApplication(&conditions.SealApplicationInput{Prepared: prepared, Delivered: receipts, Binding: binding})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{
		Target: sheets[0], Source: dndevents.ConditionSourceSpell, Condition: sealed.Condition,
	}))
	s.Require().NoError(binding.CheckComplete())
	s.Error(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{
		MemberID: "one", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "caster", ApplicationID: "app", Reason: "late-old-fact",
	}), "a prior application's fact must not remove the replacement")
	s.Require().Len(sheets[1].GetConditions(), 1)
	owner := sealed.Condition.(*conditions.LinkedCondition)
	s.Require().Len(owner.LinkedApplicationInfo().LiveMembers, 2)
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "caster"}))
	for _, sheet := range sheets {
		s.Empty(sheet.GetConditions())
	}
	s.Require().NoError(binding.Close(ctx))
}

func (s *applicationBindingSuite) TestPreparationOwnsBindingsAndRefusesInvalidSpecifications() {
	sheets := s.sheets()
	base := conditions.PrepareApplicationInput{ID: "next", CoordinatorID: "caster",
		Origin: dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster"}, Spec: applicationSpec(), Existing: bindingParticipants(sheets)}
	prepared, err := conditions.PrepareApplication(&base)
	s.Require().NoError(err)
	base.Spec.Recipients[0].Parts[0].Params[0] = '!'
	_, err = conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0, MemberID: "one"})
	s.Require().NoError(err, "preparation owns declaration bytes")
	_, err = conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0, MemberID: "outsider"})
	s.Error(err)
	for _, change := range []func(*conditions.PrepareApplicationInput){
		func(in *conditions.PrepareApplicationInput) { in.ID = "app" },
		func(in *conditions.PrepareApplicationInput) { in.CoordinatorID = "outsider" },
		func(in *conditions.PrepareApplicationInput) { in.Origin.EntityID = "" },
		func(in *conditions.PrepareApplicationInput) { in.Spec.Recipients[0].Ref = "dnd5e:conditions:unknown" },
		func(in *conditions.PrepareApplicationInput) {
			in.Spec.Recipients[0].Lifetime = conditions.LifetimeIndependent
		},
		func(in *conditions.PrepareApplicationInput) {
			in.Spec.Recipients[0].Parts[0].Params = json.RawMessage(`{"dice":"bad","group":"bless","rolls":["attack"]}`)
		},
		func(in *conditions.PrepareApplicationInput) {
			in.Spec.Recipients[0].Parts = append(in.Spec.Recipients[0].Parts, in.Spec.Recipients[0].Parts[0])
		},
	} {
		input := base
		input.Spec = applicationSpec()
		change(&input)
		got, err := conditions.PrepareApplication(&input)
		s.Error(err)
		s.Nil(got)
	}
	for _, sheet := range sheets {
		s.Require().Len(sheet.GetConditions(), 1)
	}
}

func (s *applicationBindingSuite) TestZeroDeliveryDoesNotInventLastChildLoss() {
	sheets := s.sheets()
	ctx, bus := context.Background(), events.NewEventBus()
	binding := s.bind(sheets, bus)
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "empty", CoordinatorID: "caster",
		Origin: dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster"}, Spec: applicationSpec(), Existing: bindingParticipants(sheets)})
	s.Require().NoError(err)
	old, _ := prepared.Replacement()
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{
		MemberID: old.MemberID, ConditionRef: old.ConditionRef, SourceID: old.SourceID, Reason: "recast",
	}))
	sealed, err := conditions.SealApplication(&conditions.SealApplicationInput{Prepared: prepared, Binding: binding})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{
		Target: sheets[0], Source: dndevents.ConditionSourceSpell, Condition: sealed.Condition,
	}))
	s.Require().NoError(binding.CheckComplete())
	s.Require().Len(sheets[0].GetConditions(), 1)
	s.Empty(sealed.Condition.(*conditions.LinkedCondition).LinkedApplicationInfo().LiveMembers)
	s.Require().NoError(binding.Close(ctx))
}
