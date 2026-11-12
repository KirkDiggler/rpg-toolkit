// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package effects_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type combatActivitySuite struct{ suite.Suite }

func TestCombatActivitySuite(t *testing.T) { suite.Run(t, new(combatActivitySuite)) }
func (s *combatActivitySuite) definition(skip bool, withClock bool, reasons *[]string) fx.AssemblyInput {
	params := `{"skip_first":false}`
	if skip {
		params = `{"skip_first":true}`
	}
	part, err := dndfx.BindPart(partInput("activity", dndfx.KindCombatActivity, params))
	s.Require().NoError(err)
	def := fx.AssemblyInput{Key: "activity", Parts: []fx.PartInput{part.Part}, Lifecycle: fx.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, e fx.AssemblyEnd) error {
		*reasons = append(*reasons, e.Reason)
		return nil
	}}}
	if withClock {
		clock, err := dndfx.BindPart(partInput("clock", dndfx.KindDuration, `{"turn_ends":10,"skip_first":false}`))
		s.Require().NoError(err)
		def.Parts = append(def.Parts, clock.Part)
	}
	return def
}
func (s *combatActivitySuite) start(skip, clock bool) (*fx.Assembly, events.EventBus, *[]string, fx.AssemblyInput) {
	reasons := []string{}
	def := s.definition(skip, clock, &reasons)
	a, err := fx.Assemble(def)
	s.Require().NoError(err)
	bus := events.NewEventBus()
	s.Require().NoError(a.Apply(context.Background(), bus))
	s.T().Cleanup(func() { s.NoError(a.Remove(context.Background(), bus)) })
	return a, bus, &reasons, def
}
func (s *combatActivitySuite) end(bus events.EventBus, who string) {
	s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(context.Background(), dndevents.TurnEndEvent{SubjectID: who}))
}
func (s *combatActivitySuite) attack(bus events.EventBus, who string) {
	_, err := dndevents.PostAttackRollChain.On(bus).PublishWithChain(context.Background(), &dndevents.PostAttackRollEvent{AttackerID: who}, events.NewStagedChain[*dndevents.PostAttackRollEvent](combat.ModifierStages))
	s.Require().NoError(err)
}
func (s *combatActivitySuite) TestFirstEndGraceDoesNotSkipDuration() {
	a, bus, reasons, _ := s.start(true, true)
	s.end(bus, "other")
	s.Empty(*reasons)
	s.end(bus, "maintainer")
	s.Empty(*reasons)
	s.Equal(9, a.Snapshot().Parts["clock"].Bag["remaining"])
	for range 8 {
		s.attack(bus, "maintainer")
		s.end(bus, "maintainer")
		s.Empty(*reasons)
	}
	s.attack(bus, "maintainer")
	s.end(bus, "maintainer")
	s.Equal([]string{"duration"}, *reasons)
}
func (s *combatActivitySuite) TestMissAndPositiveAppliedDamageSustain() {
	_, bus, reasons, _ := s.start(false, false)
	s.attack(bus, "maintainer")
	s.end(bus, "maintainer")
	s.Empty(*reasons)
	s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(context.Background(), &dndevents.DamageTakenEvent{MemberID: "maintainer", Amount: 1}))
	s.end(bus, "maintainer")
	s.Empty(*reasons)
	s.end(bus, "maintainer")
	s.Equal([]string{"no_combat_activity"}, *reasons)
}
func (s *combatActivitySuite) TestZeroForeignAndLegacyDamageDoNotSustain() {
	for _, mode := range []string{"zero", "foreign", "legacy", "foreign attack"} {
		s.Run(mode, func() {
			_, bus, reasons, _ := s.start(false, false)
			switch mode {
			case "zero":
				s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(context.Background(), &dndevents.DamageTakenEvent{MemberID: "maintainer"}))
			case "foreign":
				s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(context.Background(), &dndevents.DamageTakenEvent{MemberID: "other", Amount: 8}))
			case "legacy":
				s.Require().NoError(dndevents.DamageReceivedTopic.On(bus).Publish(context.Background(), dndevents.DamageReceivedEvent{TargetID: "maintainer", Amount: 8}))
			case "foreign attack":
				s.attack(bus, "other")
			}
			s.end(bus, "maintainer")
			s.Equal([]string{"no_combat_activity"}, *reasons)
		})
	}
}
func (s *combatActivitySuite) TestFlagsClearAndSurviveReload() {
	a, bus, reasons, def := s.start(true, false)
	s.end(bus, "maintainer")
	s.attack(bus, "maintainer")
	snapshot := a.Snapshot()
	s.Require().NoError(a.Remove(context.Background(), bus))
	restored, err := fx.RestoreAssembly(fx.RestoreAssemblyInput{Definition: def, Snapshot: &snapshot})
	s.Require().NoError(err)
	bus2 := events.NewEventBus()
	s.Require().NoError(restored.Apply(context.Background(), bus2))
	s.T().Cleanup(func() { s.NoError(restored.Remove(context.Background(), bus2)) })
	s.end(bus2, "maintainer")
	s.Empty(*reasons)
	s.end(bus2, "maintainer")
	s.Equal([]string{"no_combat_activity"}, *reasons)
}
func (s *combatActivitySuite) TestActivityDoesNotRequestConcentrationOrApplyDamage() {
	_, bus, reasons, _ := s.start(false, false)
	instructions := 0
	_, err := dndevents.DamageReceivedTopic.On(bus).Subscribe(context.Background(), func(context.Context, dndevents.DamageReceivedEvent) error { instructions++; return nil })
	s.Require().NoError(err)
	fact := &dndevents.DamageTakenEvent{MemberID: "maintainer", Amount: 6}
	s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(context.Background(), fact))
	s.Empty(fact.FollowUps)
	s.Equal(6, fact.Amount)
	s.Zero(instructions)
	s.end(bus, "maintainer")
	s.Empty(*reasons)
}
func (s *combatActivitySuite) TestEndOnHolderTurnStartAndZeroHP() {
	for _, beat := range []string{"turn_start", "dropped_to_zero"} {
		s.Run(beat, func() {
			part, err := dndfx.BindPart(partInput("end", dndfx.KindEndOn, `{"beat":"`+beat+`","reason":"done"}`))
			s.Require().NoError(err)
			reasons := []string{}
			a, err := fx.Assemble(fx.AssemblyInput{Key: "end", Parts: []fx.PartInput{part.Part}, Lifecycle: fx.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, e fx.AssemblyEnd) error {
				reasons = append(reasons, e.Reason)
				return nil
			}}})
			s.Require().NoError(err)
			bus := events.NewEventBus()
			s.Require().NoError(a.Apply(context.Background(), bus))
			publish := func(who string) {
				if beat == "turn_start" {
					s.Require().NoError(dndevents.TurnStartTopic.On(bus).Publish(context.Background(), dndevents.TurnStartEvent{SubjectID: who}))
				} else {
					s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(context.Background(), &dndevents.DamageTakenEvent{MemberID: who, Amount: 5, DroppedToZero: true}))
				}
			}
			publish("other")
			s.Empty(reasons)
			publish("maintainer")
			publish("maintainer")
			s.Equal([]string{"done"}, reasons)
		})
	}
}
