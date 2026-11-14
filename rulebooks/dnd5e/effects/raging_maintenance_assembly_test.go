// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type ragingMaintenanceAssemblySuite struct{ suite.Suite }

func TestRagingMaintenanceAssemblySuite(t *testing.T) {
	suite.Run(t, new(ragingMaintenanceAssemblySuite))
}

type maintenanceRig struct {
	bus              events.EventBus
	assembly         *generic.Assembly
	ends             []generic.AssemblyEnd
	observedOutputs  int
	reactionsAllowed bool
}

func (s *ragingMaintenanceAssemblySuite) rig(scope generic.EndScope) *maintenanceRig {
	compiled, err := dndfx.CompileParts(&dndfx.CompilePartsInput{Parts: []dndfx.PartSpec{
		{Key: "resistance", Effect: dndfx.Resistance{Types: []damage.Type{damage.Slashing}}, OnTerminal: generic.EndPart},
		{Key: "activity", Effect: dndfx.CombatActivity{SkipFirst: true}, OnTerminal: scope},
		{Key: "clock", Effect: dndfx.RoundDuration{Rounds: 10, ExpiredReason: "duration_expired", ResetReason: "clock_reset"}, OnTerminal: scope},
	}})
	s.Require().NoError(err)
	var parts []generic.PartInput
	for _, declaration := range compiled.Parts {
		in := partInput(declaration.Key, declaration.Kind, "")
		in.Declaration = declaration
		bound, err := dndfx.BindPart(in)
		s.Require().NoError(err)
		parts = append(parts, bound.Part)
	}
	r := &maintenanceRig{bus: events.NewEventBus()}
	r.assembly, err = generic.Assemble(generic.AssemblyInput{Key: "maintenance", Parts: parts, Lifecycle: generic.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, end generic.AssemblyEnd) error {
		r.ends = append(r.ends, end)
		outputs := r.assembly.CurrentOutputs()
		r.observedOutputs = len(outputs)
		for _, out := range outputs {
			r.reactionsAllowed = r.reactionsAllowed || out.Output.CanReact()
		}
		return nil
	}}})
	s.Require().NoError(err)
	s.Require().NoError(r.assembly.Apply(context.Background(), r.bus))
	s.T().Cleanup(func() { s.NoError(r.assembly.Remove(context.Background(), r.bus)) })
	return r
}
func (s *ragingMaintenanceAssemblySuite) turn(r *maintenanceRig, round int) {
	s.Require().NoError(dndevents.TurnEndTopic.On(r.bus).Publish(context.Background(), dndevents.TurnEndEvent{SubjectID: "maintainer", Round: round}))
}
func (s *ragingMaintenanceAssemblySuite) miss(r *maintenanceRig) {
	_, err := dndevents.PostAttackRollChain.On(r.bus).PublishWithChain(context.Background(), &dndevents.PostAttackRollEvent{AttackerID: "maintainer", WouldHit: false}, events.NewStagedChain[*dndevents.PostAttackRollEvent](combat.ModifierStages))
	s.Require().NoError(err)
}
func (s *ragingMaintenanceAssemblySuite) TestFirstEndGraceAndPositiveAppliedDamageOrMiss() {
	r := s.rig(generic.EndAttachment)
	s.turn(r, 1)
	s.Empty(r.ends)
	s.Require().NoError(dndevents.DamageTakenTopic.On(r.bus).Publish(context.Background(), &dndevents.DamageTakenEvent{MemberID: "maintainer", Amount: 6}))
	s.turn(r, 2)
	s.Empty(r.ends)
	s.miss(r)
	s.turn(r, 3)
	s.Empty(r.ends)
	s.Require().NoError(dndevents.DamageTakenTopic.On(r.bus).Publish(context.Background(), &dndevents.DamageTakenEvent{MemberID: "maintainer", Amount: 0}))
	s.turn(r, 4)
	s.Require().Len(r.ends, 1)
	s.Equal("no_combat_activity", r.ends[0].Reason)
}
func (s *ragingMaintenanceAssemblySuite) TestIdleWinsAtElapsedAndResetBoundaries() {
	for _, tc := range []struct{ first, last int }{{4, 13}, {7, 1}} {
		for _, scope := range []generic.EndScope{generic.EndAttachment, generic.EndApplication} {
			r := s.rig(scope)
			s.turn(r, tc.first)
			s.Empty(r.ends)
			s.turn(r, tc.last)
			s.Require().Len(r.ends, 1)
			s.Equal("no_combat_activity", r.ends[0].Reason)
			s.Equal(scope, r.ends[0].Scope)
			s.Equal(1, r.observedOutputs, "payload remains observable during terminal reporting")
			s.False(r.reactionsAllowed, "ending does not permit new work")
			s.False(r.assembly.IsApplied())
		}
	}
}
func (s *ragingMaintenanceAssemblySuite) TestActiveSparseClockExpiresWithSameReason() {
	r := s.rig(generic.EndAttachment)
	s.turn(r, 4)
	s.miss(r)
	s.turn(r, 9)
	s.Empty(r.ends)
	s.miss(r)
	s.turn(r, 13)
	s.Require().Len(r.ends, 1)
	s.Equal("duration_expired", r.ends[0].Reason)
	s.Equal(1, r.observedOutputs)
	s.False(r.reactionsAllowed)
}
func (s *ragingMaintenanceAssemblySuite) TestRoundlessGraceIsNotPerpetual() {
	r := s.rig(generic.EndApplication)
	s.turn(r, 0)
	s.Empty(r.ends)
	s.turn(r, 0)
	s.Require().Len(r.ends, 1)
	s.Equal("no_combat_activity", r.ends[0].Reason)
}
