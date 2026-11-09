// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func (s *applicationBindingSuite) TestNamedRecipientRestEndsOnlyThatAttachment() {
	sheets := s.sheets()
	ctx, bus := context.Background(), events.NewEventBus()
	binding := s.bind(sheets, bus)
	spec := applicationSpec()
	spec.Recipients[0].Ref = refs.Conditions.Blessed().String()
	spec.Recipients[0].Parts = nil // registry-owned named composition
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "named", CoordinatorID: "caster",
		Origin: dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster"}, Spec: spec, Existing: bindingParticipants(sheets)})
	s.Require().NoError(err)
	old, _ := prepared.Replacement()
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{MemberID: old.MemberID, ConditionRef: old.ConditionRef, SourceID: old.SourceID, Reason: "recast"}))
	var receipts []conditions.DeliveryReceipt
	for _, sheet := range sheets[1:] {
		created, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0, MemberID: sheet.GetID()})
		s.Require().NoError(err)
		s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: sheet, Condition: created.Condition, Source: dndevents.ConditionSourceSpell}))
		receipts = append(receipts, created.Receipt)
	}
	sealed, err := conditions.SealApplication(&conditions.SealApplicationInput{Prepared: prepared, Delivered: receipts, Binding: binding})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: sheets[0], Condition: sealed.Condition, Source: dndevents.ConditionSourceSpell}))
	s.Require().NoError(binding.CheckComplete())
	s.Require().NoError(dndevents.RestTopic.On(bus).Publish(ctx, dndevents.RestEvent{CharacterID: "one", RestType: coreResources.ResetShortRest}))
	s.Empty(sheets[1].GetConditions())
	s.Require().Len(sheets[2].GetConditions(), 1)
	s.Require().Len(sheets[0].GetConditions(), 1)
	s.Require().Len(sealed.Condition.(*conditions.LinkedCondition).LinkedApplicationInfo().LiveMembers, 1)
	s.Require().NoError(binding.CheckComplete())
	s.Require().NoError(dndevents.RestTopic.On(bus).Publish(ctx, dndevents.RestEvent{CharacterID: "two", RestType: coreResources.ResetLongRest}))
	s.Empty(sheets[2].GetConditions())
	s.Empty(sheets[0].GetConditions(), "only actual last-child loss ends the shared hold")
	s.Require().NoError(binding.Close(ctx))
}
