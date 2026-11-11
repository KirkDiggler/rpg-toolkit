// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func (s *applicationBindingSuite) TestResumeRefusesChangedBindingsAndCannotCreateAnotherOwner() {
	w := s.pendingApplication()
	ctx := context.Background()
	resumed, err := conditions.ResumeApplication(&conditions.ResumeApplicationInput{ID: "pending", Spec: w.spec, Existing: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	_, err = conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: resumed})
	s.Require().Error(err)
	for _, change := range []func(*conditions.ApplicationSpec){
		func(spec *conditions.ApplicationSpec) { spec.Name = "changed" },
		func(spec *conditions.ApplicationSpec) { spec.Recipients[0].MemberID = "caster" },
		func(spec *conditions.ApplicationSpec) { spec.Recipients[0].AddressSourceID = "changed" },
		func(spec *conditions.ApplicationSpec) { spec.CoordinatorParts = spec.CoordinatorParts[:1] },
	} {
		spec := w.spec
		spec.Recipients = append([]conditions.RecipientSpec(nil), spec.Recipients...)
		change(&spec)
		_, err = conditions.ResumeApplication(&conditions.ResumeApplicationInput{ID: "pending", Spec: spec, Existing: bindingParticipants(w.sheets)})
		s.Require().Error(err)
	}
	s.Require().NoError(w.binding.CheckComplete())
	s.Require().NoError(w.binding.Close(ctx))
}

func (s *applicationBindingSuite) TestRepeatedAppliedFactStillChecksItsGeneration() {
	w := s.pendingApplication()
	ctx := context.Background()
	s.deliverPending(w, 0)
	condition := w.sheets[1].GetConditions()[0]
	err := dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: w.sheets[1], Condition: condition, ApplicationID: "foreign"})
	s.Require().ErrorContains(err, "foreign application")
	s.Require().Len(w.sheets[1].GetConditions(), 1)
	s.Require().NoError(w.binding.CheckComplete())
	s.Require().NoError(w.binding.Close(ctx))
}

func (s *applicationBindingSuite) TestAdoptedPendingDeliveryRequiresItsGeneration() {
	w := s.pendingApplication()
	ctx := context.Background()
	spec := conditions.ApplicationSpec{Name: "Adopted strike", CoordinatorParts: []dndfx.PartDeclaration{{Key: "clock", Kind: dndfx.KindDuration, Params: json.RawMessage(`{"turn_ends":2}`), OnTerminal: generic.EndApplication}},
		Recipients: []conditions.RecipientSpec{{MemberID: "one", Ref: refs.Conditions.TrueStrike().String(), Lifetime: conditions.LifetimeShared, Config: json.RawMessage(`{"target_id":"two"}`)}}}
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "adopted", CoordinatorID: "caster", Origin: dndevents.ConditionOrigin{Ref: *refs.Spells.TrueStrike(), EntityID: "caster"}, Spec: spec, Existing: bindingParticipants(w.sheets)})
	s.Require().NoError(err)
	owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: w.sheets[0], Condition: owner.Condition}))
	candidate, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0})
	s.Require().NoError(err)
	s.Require().Error(dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: w.sheets[1], Condition: candidate.Condition}), "a parent-only backlink cannot infer which pending application delivered it")
	s.Empty(w.sheets[1].GetConditions())
	s.Require().Error(dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: w.sheets[1], Condition: candidate.Condition, ApplicationID: "foreign"}))
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: w.sheets[1], Condition: candidate.Condition, ApplicationID: "adopted"}))
	_, err = w.binding.FinalizeRecipient(ctx, &conditions.FinalizeRecipientInput{ApplicationID: "adopted", Key: candidate.Candidate.Key, Delivered: true})
	s.Require().NoError(err)
	s.Require().NoError(w.binding.CheckComplete())
	s.Require().NoError(dndevents.TurnEndTopic.On(w.bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "caster"}))
	s.Require().NoError(dndevents.TurnEndTopic.On(w.bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "caster"}))
	s.Empty(w.sheets[1].GetConditions(), "the delivered adopted payload is owned and removed on expiry")
	s.Require().NoError(w.binding.CheckComplete())
	s.Require().NoError(w.binding.Close(ctx))
}
