// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func (s *partsSuite) TestUntilEmptyRequiresActualLastMemberLoss() {
	part := s.bind(partInput("members", dndfx.KindUntilEmpty, `{}`))
	var reasons []string
	assembly, err := fx.Assemble(fx.AssemblyInput{Key: "application-1", Parts: []fx.PartInput{part},
		Lifecycle: fx.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, end fx.AssemblyEnd) error {
			reasons = append(reasons, end.Reason)
			return nil
		}}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	for _, fact := range []dndevents.ApplicationMembersChangedEvent{
		{ApplicationID: "application-1", Owner: partOwner(), Before: 0, After: 0},
		{ApplicationID: "application-1", Owner: partOwner(), Before: 2, After: 1},
		{ApplicationID: "other", Owner: partOwner(), Before: 1, After: 0},
		{ApplicationID: "application-1", Owner: dndevents.ConditionAddress{MemberID: "other", ConditionRef: partOwner().ConditionRef, SourceID: "application-1"}, Before: 1, After: 0},
		{ApplicationID: "application-1", Owner: dndevents.ConditionAddress{MemberID: "maintainer", ConditionRef: partOwner().ConditionRef, SourceID: "other"}, Before: 1, After: 0},
	} {
		s.Require().NoError(dndevents.ApplicationMembersChangedTopic.On(bus).Publish(ctx, fact))
		s.Empty(reasons)
	}
	s.Require().NoError(dndevents.ApplicationMembersChangedTopic.On(bus).Publish(ctx,
		dndevents.ApplicationMembersChangedEvent{ApplicationID: "application-1", Owner: partOwner(), Before: 1, After: 0}))
	s.Equal([]string{"spell_ended"}, reasons)
	s.False(assembly.IsApplied())
}

func (s *partsSuite) TestEndOnPolicyIsExplicitAndHolderScoped() {
	part := s.bind(partInput("combat", dndfx.KindEndOn, `{"beat":"combat_end","reason":"combat_end"}`))
	var reasons []string
	assembly, err := fx.Assemble(fx.AssemblyInput{Key: "application-1", Parts: []fx.PartInput{part},
		Lifecycle: fx.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, end fx.AssemblyEnd) error {
			reasons = append(reasons, end.Reason)
			return nil
		}}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(dndevents.CombatEndTopic.On(bus).Publish(ctx, dndevents.CombatEndEvent{SubjectID: "other"}))
	s.Empty(reasons)
	s.Require().NoError(dndevents.CombatEndTopic.On(bus).Publish(ctx, dndevents.CombatEndEvent{SubjectID: "maintainer"}))
	s.Equal([]string{"combat_end"}, reasons)
	for _, params := range []string{`{}`, `{"beat":"unknown","reason":"end"}`, `{"beat":"rest","reason":""}`} {
		got, err := dndfx.BindPart(partInput("invalid", dndfx.KindEndOn, params))
		s.Error(err)
		s.Nil(got)
	}
}
