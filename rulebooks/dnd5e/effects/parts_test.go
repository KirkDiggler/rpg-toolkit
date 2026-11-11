// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type partsSuite struct{ suite.Suite }

func TestPartsSuite(t *testing.T) { suite.Run(t, new(partsSuite)) }

func partOwner() dndevents.ConditionAddress {
	return dndevents.ConditionAddress{MemberID: "maintainer", ConditionRef: "example:conditions:owner", SourceID: "application-1"}
}

func partInput(key string, kind fx.KindName, params string) *dndfx.BindPartInput {
	return &dndfx.BindPartInput{
		Declaration: dndfx.PartDeclaration{Key: key, Kind: kind, Params: json.RawMessage(params), OnTerminal: fx.EndApplication},
		Ref:         core.Ref{Module: "example", Type: "conditions", ID: "owner"},
		HolderID:    "maintainer", ApplicationID: "application-1", DisplayName: "Maintained protection",
		Origin: dndevents.ConditionOrigin{Ref: core.Ref{Module: "example", Type: "features", ID: "protection"}, EntityID: "maintainer"},
		Owner:  partOwner(),
	}
}

func (s *partsSuite) bind(input *dndfx.BindPartInput) fx.PartInput {
	part, err := dndfx.BindPart(input)
	s.Require().NoError(err)
	return part.Part
}

func (s *partsSuite) TestConcentrationAndDurationAreIndependent() {
	concentration := s.bind(partInput("hold", dndfx.KindConcentration, `{}`))
	duration := s.bind(partInput("clock", dndfx.KindDuration, `{"turn_ends":2,"skip_first":true}`))
	var reasons []string
	assembly, err := fx.Assemble(fx.AssemblyInput{Key: "application-1", Parts: []fx.PartInput{concentration, duration},
		Lifecycle: fx.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, end fx.AssemblyEnd) error {
			reasons = append(reasons, end.Reason)
			return nil
		}}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	for range 2 {
		s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "maintainer"}))
	}
	s.Empty(reasons)
	s.Equal(1, assembly.Snapshot().Parts["clock"].Bag["remaining"])
	s.Equal("active", assembly.Snapshot().Parts["hold"].State)
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "maintainer"}))
	s.Equal([]string{"duration"}, reasons)
	s.False(assembly.IsApplied())
}

func (s *partsSuite) TestConcentrationDescribesOneSaveAndZeroHPDoesNotRoll() {
	part := s.bind(partInput("hold", dndfx.KindConcentration, `{}`))
	var reasons []string
	assembly, err := fx.Assemble(fx.AssemblyInput{Key: "application-1", Parts: []fx.PartInput{part},
		Lifecycle: fx.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, end fx.AssemblyEnd) error {
			reasons = append(reasons, end.Reason)
			// This is transport, not a new round: requesting work here would be a late reaction.
			return nil
		}}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	for _, fact := range []*dndevents.DamageTakenEvent{nil, {MemberID: "other", Amount: 8}, {MemberID: "maintainer"}} {
		s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(ctx, fact))
		if fact != nil {
			s.Empty(fact.FollowUps)
		}
	}
	fact := &dndevents.DamageTakenEvent{MemberID: "maintainer", Amount: 27}
	s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(ctx, fact))
	s.Require().Len(fact.FollowUps, 1)
	s.Equal("maintainer", fact.FollowUps[0].SaverID)
	s.Equal(abilities.CON, fact.FollowUps[0].Ability)
	s.Equal(13, fact.FollowUps[0].DC)
	s.Equal(partOwner(), fact.FollowUps[0].OnFailure.Owner)
	s.Empty(fact.FollowUps[0].OnFailure.Remove, "the owner, not this part, knows the linked recipients")
	s.Equal("example:features:protection", fact.FollowUps[0].Cause.EffectRef.String())
	s.Empty(reasons)
	zero := &dndevents.DamageTakenEvent{MemberID: "maintainer", Amount: 5, DroppedToZero: true}
	s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(ctx, zero))
	s.Empty(zero.FollowUps)
	s.Equal([]string{"caster_down"}, reasons)
}

func (s *partsSuite) TestDurationGraceAndRemainingSurviveRestore() {
	input := partInput("clock", dndfx.KindDuration, `{"turn_ends":2,"skip_first":true}`)
	definition := fx.AssemblyInput{Key: "application-1", Parts: []fx.PartInput{s.bind(input)},
		Lifecycle: fx.AssemblyLifecycle{End: func(context.Context, events.EventBus, fx.AssemblyEnd) error { return nil }}}
	assembly, err := fx.Assemble(definition)
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "other"}))
	before, err := dndfx.InspectPart(&dndfx.InspectPartInput{Declaration: input.Declaration, Snapshot: assembly.Snapshot().Parts["clock"]})
	s.Require().NoError(err)
	s.Require().NotNil(before.Duration)
	s.True(before.Duration.SkipNext)
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "maintainer"}))
	snapshot := assembly.Snapshot()
	s.Require().NoError(assembly.Remove(ctx, bus))
	restored, err := fx.RestoreAssembly(fx.RestoreAssemblyInput{Definition: definition, Snapshot: &snapshot})
	s.Require().NoError(err)
	info, err := dndfx.InspectPart(&dndfx.InspectPartInput{Declaration: input.Declaration, Snapshot: restored.Snapshot().Parts["clock"]})
	s.Require().NoError(err)
	s.Equal(2, info.Duration.Remaining)
	s.False(info.Duration.SkipNext)
	s.Require().NoError(restored.Apply(ctx, bus))
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "maintainer"}))
	s.Equal(1, restored.Snapshot().Parts["clock"].Bag["remaining"])
	s.Require().NoError(restored.Remove(ctx, bus))
}

func (s *partsSuite) TestConcentrationDoesNotRequestNewWorkDuringEnd() {
	duration := s.bind(partInput("clock", dndfx.KindDuration, `{"turn_ends":1}`))
	hold := s.bind(partInput("hold", dndfx.KindConcentration, `{}`))
	var followUps []dndevents.FollowUp
	assembly, err := fx.Assemble(fx.AssemblyInput{Key: "application-1", Parts: []fx.PartInput{duration, hold},
		Lifecycle: fx.AssemblyLifecycle{End: func(ctx context.Context, bus events.EventBus, _ fx.AssemblyEnd) error {
			fact := &dndevents.DamageTakenEvent{MemberID: "maintainer", Amount: 8}
			if err := dndevents.DamageTakenTopic.On(bus).Publish(ctx, fact); err != nil {
				return err
			}
			followUps = append(followUps, fact.FollowUps...)
			return nil
		}}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "maintainer"}))
	s.Empty(followUps)
	s.False(assembly.IsApplied())
}
