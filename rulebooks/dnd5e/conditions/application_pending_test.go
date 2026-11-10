// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type pendingFixture struct {
	sheets   []*character.Character
	bus      events.EventBus
	binding  *conditions.ApplicationBinding
	prepared *conditions.PreparedApplication
	owner    *conditions.LinkedCondition
	spec     conditions.ApplicationSpec
}

func (s *applicationBindingSuite) pendingApplication() pendingFixture {
	sheets := s.sheets()
	for i := range sheets {
		sheets[i] = s.reloadConditions(sheets[i])
	}
	spec := applicationSpec()
	spec.Recipients[0].MemberID = "one"
	second := spec.Recipients[0]
	second.MemberID = "two"
	spec.Recipients = append(spec.Recipients, second)
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "pending", CoordinatorID: "caster", Origin: dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster"}, Spec: spec, Existing: bindingParticipants(sheets)})
	s.Require().NoError(err)
	created, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	s.Require().NoError(err)
	bus := events.NewEventBus()
	binding := s.bind(sheets, bus)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(context.Background(), dndevents.ConditionAppliedEvent{Target: sheets[0], Condition: created.Condition}))
	return pendingFixture{sheets: sheets, bus: bus, binding: binding, prepared: prepared, owner: created.Condition.(*conditions.LinkedCondition), spec: spec}
}

func (s *applicationBindingSuite) deliverPending(w pendingFixture, index int) conditions.RecipientCandidate {
	created, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: w.prepared, SpecIndex: index})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(w.bus).Publish(context.Background(), dndevents.ConditionAppliedEvent{Target: w.sheets[index+1], Condition: created.Condition}))
	_, err = w.binding.FinalizeRecipient(context.Background(), &conditions.FinalizeRecipientInput{ApplicationID: "pending", Key: created.Candidate.Key, Delivered: true})
	s.Require().NoError(err)
	return created.Candidate
}

func (s *applicationBindingSuite) TestPendingDeliveryIsSaveableAndResumesByExactKey() {
	w := s.pendingApplication()
	ctx := context.Background()
	s.Require().NoError(w.binding.CheckComplete(), "a valid pending entry is not an orphan")
	s.Empty(w.owner.LinkedApplicationInfo().LiveMembers)
	s.Len(w.owner.LinkedApplicationInfo().PendingMembers, 2)
	s.deliverPending(w, 0)
	s.Require().NoError(w.binding.CheckComplete())
	s.True(w.sheets[0].IsDirty())
	s.Len(w.owner.LinkedApplicationInfo().LiveMembers, 1)
	s.Len(w.owner.LinkedApplicationInfo().PendingMembers, 1)
	s.Require().NoError(w.binding.Close(ctx))
	for i, sheet := range w.sheets {
		data, err := sheet.ToData()
		s.Require().NoError(err)
		w.sheets[i], err = character.Load(ctx, data)
		s.Require().NoError(err)
	}
	w.bus = events.NewEventBus()
	w.binding = s.bind(w.sheets, w.bus)
	resumed, err := conditions.ResumeApplication(&conditions.ResumeApplicationInput{ID: "pending", Spec: w.spec, Existing: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	w.prepared = resumed
	w.owner = w.sheets[0].GetConditions()[0].(*conditions.LinkedCondition)
	roll, err := w.sheets[1].DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindSavingThrow})
	s.Require().NoError(err)
	s.Require().Len(roll.Contributions, 1)
	s.Equal("1d4", roll.Contributions[0].Dice)
	s.False(w.sheets[0].IsDirty(), "the restored owner starts clean")
	s.deliverPending(w, 1)
	s.True(w.sheets[0].IsDirty(), "finalization after resume must be persisted")
	s.Require().NoError(w.binding.CheckComplete())
	s.Len(w.owner.LinkedApplicationInfo().LiveMembers, 2)
	s.Empty(w.owner.LinkedApplicationInfo().PendingMembers)
	s.Require().NoError(w.binding.Close(ctx))
}

func (s *applicationBindingSuite) TestNoEffectFinalizationDoesNotInventMemberLoss() {
	w := s.pendingApplication()
	ctx := context.Background()
	for index := range w.spec.Recipients {
		candidate, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: w.prepared, SpecIndex: index})
		s.Require().NoError(err)
		result, err := w.binding.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: "pending", Key: candidate.Candidate.Key})
		s.Require().NoError(err)
		s.Zero(result.Before)
		s.Zero(result.After)
	}
	s.Require().NoError(w.binding.CheckComplete())
	s.Require().Len(w.sheets[0].GetConditions(), 1)
	s.Empty(w.owner.LinkedApplicationInfo().PendingMembers)
	s.Empty(w.owner.LinkedApplicationInfo().LiveMembers)
	s.Require().NoError(w.binding.Close(ctx))
}

func (s *applicationBindingSuite) TestUnfinalizedActualDeliveryIsNotSaveable() {
	w := s.pendingApplication()
	ctx := context.Background()
	candidate, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: w.prepared, SpecIndex: 0})
	s.Require().NoError(err)
	_, err = w.binding.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: "pending", Key: candidate.Candidate.Key, Delivered: true})
	s.Require().Error(err, "a prospective candidate is not delivery evidence")
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: w.sheets[1], Condition: candidate.Condition}))
	s.Error(w.binding.CheckComplete())
	_, err = w.binding.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: "pending", Key: candidate.Candidate.Key})
	s.Require().Error(err, "an actual attachment cannot be declared absent")
	_, err = w.binding.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: "pending", Key: candidate.Candidate.Key, Delivered: true})
	s.Require().NoError(err)
	s.NoError(w.binding.CheckComplete())
	s.Require().NoError(w.binding.Close(ctx))
}

func (s *applicationBindingSuite) TestForeignOrFinalizedRecipientCannotJoin() {
	w := s.pendingApplication()
	ctx := context.Background()
	candidate := s.deliverPending(w, 0)
	for _, key := range []string{candidate.Key, "foreign"} {
		before := w.owner.LinkedApplicationInfo()
		_, err := w.binding.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: "pending", Key: key, Delivered: true})
		s.Require().Error(err)
		s.Equal(before, w.owner.LinkedApplicationInfo())
	}
	s.Require().NoError(w.binding.Close(ctx))
}

func (s *applicationBindingSuite) TestEndingWithPendingDeliveriesCannotBeResurrected() {
	w := s.pendingApplication()
	ctx := context.Background()
	s.deliverPending(w, 0)
	waiting, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: w.prepared, SpecIndex: 1})
	s.Require().NoError(err)
	address := w.owner.ConditionAddress()
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(w.bus).Publish(ctx, dndevents.ConditionRemovedEvent{MemberID: address.MemberID, ConditionRef: address.ConditionRef, SourceID: address.SourceID, ApplicationID: "pending", Reason: "recast"}))
	s.Empty(w.sheets[1].GetConditions())
	s.Error(dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: w.sheets[2], Condition: waiting.Condition}))
	_, err = w.binding.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: "pending", Key: waiting.Candidate.Key, Delivered: true})
	s.Require().Error(err)
	_, err = conditions.ResumeApplication(&conditions.ResumeApplicationInput{ID: "pending", Spec: w.spec, Existing: bindingParticipants(w.sheets)})
	s.Require().Error(err)
	s.Require().NoError(w.binding.Close(ctx))
}
