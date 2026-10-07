// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type IncomingDamageSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestIncomingDamageSuite(t *testing.T) {
	suite.Run(t, new(IncomingDamageSuite))
}

func (s *IncomingDamageSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func eight() *int { v := 8; return &v }

func (s *IncomingDamageSuite) input() dnd5eEvents.IncomingDamageInput {
	return dnd5eEvents.IncomingDamageInput{
		TargetID: "ogre-1",
		SourceID: "fighter-1",
		Dealt: []dnd5eEvents.DamageComponent{{
			Source: dnd5eEvents.DamageSourceWeapon,
			Roll: dnd5eEvents.RollComponent{
				Source:   dnd5eEvents.RollSource{Ref: refs.Weapons.Longsword(), Name: "Longsword"},
				Modifier: eight(),
			},
			DamageType: damage.Slashing,
		}},
		Frame: contributions.Frame{
			Actor:  "fighter-1",
			Target: contributions.Known("ogre-1"),
			Action: contributions.ActionFacts{
				Roll:       contributions.Known(contributions.RollKindAttack),
				WeaponPool: contributions.Known(true),
			},
		},
	}
}

func resistance() dnd5eEvents.DamageMultiplier {
	return dnd5eEvents.DamageMultiplier{
		Category:   dnd5eEvents.DamageSourceCondition,
		Source:     dnd5eEvents.RollSource{Ref: refs.Conditions.Raging(), Name: "Raging"},
		DamageType: damage.Slashing,
		Factor:     dnd5eEvents.DamageFactorResistance,
	}
}

// fold publishes sent with one subscriber whose modifier is answer, executes
// over a clone the way the target step does, and checks the result.
func (s *IncomingDamageSuite) fold(
	sent *dnd5eEvents.IncomingDamageEvent,
	answer func(context.Context, *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error),
) (*dnd5eEvents.IncomingDamageEvent, error) {
	_, err := dnd5eEvents.IncomingDamageChain.On(s.bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, _ *dnd5eEvents.IncomingDamageEvent, c chain.Chain[*dnd5eEvents.IncomingDamageEvent],
		) (chain.Chain[*dnd5eEvents.IncomingDamageEvent], error) {
			return c, c.Add(combat.StageFinal, "answer", answer)
		})
	s.Require().NoError(err)
	modified, err := dnd5eEvents.IncomingDamageChain.On(s.bus).PublishWithChain(
		s.ctx, sent.Clone(), events.NewStagedChain[*dnd5eEvents.IncomingDamageEvent](combat.ModifierStages))
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, sent.Clone())
	s.Require().NoError(err)
	return folded, folded.CheckUnaltered(sent)
}

// An answer appended to the fold is accepted.
func (s *IncomingDamageSuite) TestAnAppendedAnswerIsAccepted() {
	sent, err := dnd5eEvents.NewIncomingDamageEvent(s.input())
	s.Require().NoError(err)

	folded, err := s.fold(sent, func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
		e.Multipliers = append(e.Multipliers, resistance())
		return e, nil
	})
	s.Require().NoError(err)
	s.Equal([]dnd5eEvents.DamageMultiplier{resistance()}, folded.Multipliers)
}

// A target answer that changes a dealt component is refused: the only way to
// is to hand back a different event, and the step sees it.
func (s *IncomingDamageSuite) TestAFoldThatChangesADealtComponentIsRefused() {
	sent, err := dnd5eEvents.NewIncomingDamageEvent(s.input())
	s.Require().NoError(err)

	_, err = s.fold(sent, func(_ context.Context, _ *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
		in := s.input()
		four := 4
		in.Dealt[0].Roll.Modifier = &four
		return dnd5eEvents.NewIncomingDamageEvent(in)
	})
	s.Require().ErrorIs(err, dnd5eEvents.ErrTargetAnswerAltered)
}

// Removing a dealt component is refused the same way.
func (s *IncomingDamageSuite) TestAFoldThatRemovesADealtComponentIsRefused() {
	sent, err := dnd5eEvents.NewIncomingDamageEvent(s.input())
	s.Require().NoError(err)

	_, err = s.fold(sent, func(_ context.Context, _ *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
		in := s.input()
		in.Dealt = nil
		return dnd5eEvents.NewIncomingDamageEvent(in)
	})
	s.Require().ErrorIs(err, dnd5eEvents.ErrTargetAnswerAltered)
}

// An answer present before the fold opened — a reaction's, settled in its
// window — cannot be rewritten by the fold.
func (s *IncomingDamageSuite) TestAFoldThatRewritesAnEarlierAnswerIsRefused() {
	in := s.input()
	in.Multipliers = []dnd5eEvents.DamageMultiplier{resistance()}
	sent, err := dnd5eEvents.NewIncomingDamageEvent(in)
	s.Require().NoError(err)

	_, err = s.fold(sent, func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
		e.Multipliers[0].Factor = dnd5eEvents.DamageFactorImmunity
		return e, nil
	})
	s.Require().ErrorIs(err, dnd5eEvents.ErrTargetAnswerAltered)
}

// THE HANDLER-TIME PROBE. A handler is handed the published event and its
// answer lists are writable. A handler that rewrites a reaction's resistance
// into immunity there must reach nothing the step settles: the step publishes
// a clone, so the folded answer and the settlement keep the resistance.
func (s *IncomingDamageSuite) TestAHandlerCannotRewriteAnEarlierAnswer() {
	in := s.input()
	in.Multipliers = []dnd5eEvents.DamageMultiplier{resistance()}
	sent, err := dnd5eEvents.NewIncomingDamageEvent(in)
	s.Require().NoError(err)

	_, err = dnd5eEvents.IncomingDamageChain.On(s.bus).SubscribeWithChain(s.ctx,
		func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent, c chain.Chain[*dnd5eEvents.IncomingDamageEvent],
		) (chain.Chain[*dnd5eEvents.IncomingDamageEvent], error) {
			e.Multipliers[0].Factor = dnd5eEvents.DamageFactorImmunity
			return c, nil
		})
	s.Require().NoError(err)
	modified, err := dnd5eEvents.IncomingDamageChain.On(s.bus).PublishWithChain(
		s.ctx, sent.Clone(), events.NewStagedChain[*dnd5eEvents.IncomingDamageEvent](combat.ModifierStages))
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, sent.Clone())
	s.Require().NoError(err)
	s.Require().NoError(folded.CheckUnaltered(sent))

	settled, err := combat.SettleDamage(&combat.SettleDamageInput{
		Dealt: sent.Dealt(), Reductions: folded.Reductions, Multipliers: folded.Multipliers,
	})
	s.Require().NoError(err)
	s.Equal([]dnd5eEvents.DamageMultiplier{resistance()}, folded.Multipliers)
	_, total := settled.FinalDamage()
	s.Equal(4, total, "8 resisted lands as 4; the handler's immunity reached nothing")
}

// A malformed appended answer is refused by the check.
func (s *IncomingDamageSuite) TestAMalformedAppendedAnswerIsRefused() {
	sent, err := dnd5eEvents.NewIncomingDamageEvent(s.input())
	s.Require().NoError(err)

	_, err = s.fold(sent, func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
		quarter := resistance()
		quarter.Factor = 0.25
		e.Multipliers = append(e.Multipliers, quarter)
		return e, nil
	})
	s.Require().ErrorIs(err, dnd5eEvents.ErrMalformedTargetAnswer)
}

// The read-only accessors hand out copies.
func (s *IncomingDamageSuite) TestDealtAndFrameAreCopies() {
	sent, err := dnd5eEvents.NewIncomingDamageEvent(s.input())
	s.Require().NoError(err)

	dealt := sent.Dealt()
	*dealt[0].Roll.Modifier = 99
	frame := sent.Frame()
	frame.Actor = "someone-else"

	s.Equal(8, sent.Dealt()[0].Total())
	s.Equal("fighter-1", sent.Frame().Actor)
	s.Equal([]damage.Type{damage.Slashing}, sent.DealtTypes())
}

// The constructor refuses what the step must never publish.
func (s *IncomingDamageSuite) TestConstructionRefusals() {
	half := 0.5
	cases := map[string]func(*dnd5eEvents.IncomingDamageInput){
		"no target":                    func(in *dnd5eEvents.IncomingDamageInput) { in.TargetID = "" },
		"no source":                    func(in *dnd5eEvents.IncomingDamageInput) { in.SourceID = "" },
		"a frame actor not the source": func(in *dnd5eEvents.IncomingDamageInput) { in.Frame.Actor = "rogue-1" },
		"a frame not knowing the target": func(in *dnd5eEvents.IncomingDamageInput) {
			in.Frame.Target = contributions.Unknown[string]()
		},
		"an invalid frame": func(in *dnd5eEvents.IncomingDamageInput) {
			in.Frame.Action.Roll = contributions.Unknown[contributions.RollKind]()
		},
		"a dealt component carrying a multiplier": func(in *dnd5eEvents.IncomingDamageInput) { in.Dealt[0].Multiplier = &half },
		"a dealt component with no type":          func(in *dnd5eEvents.IncomingDamageInput) { in.Dealt[0].DamageType = "" },
		"a malformed early answer": func(in *dnd5eEvents.IncomingDamageInput) {
			in.Reductions = []dnd5eEvents.DamageReduction{{
				Source: dnd5eEvents.RollSource{Ref: refs.Features.DeflectMissiles()}, DamageType: damage.Slashing, Modifier: 2,
			}}
		},
	}
	for name, mutate := range cases {
		s.Run(name, func() {
			in := s.input()
			mutate(&in)
			_, err := dnd5eEvents.NewIncomingDamageEvent(in)
			s.Error(err)
		})
	}
}

// Every malformed answer, on either Validate path, wraps the one sentinel
// resolution classifies a rule defect by.
func (s *IncomingDamageSuite) TestMalformedAnswersWrapTheSentinel() {
	in := s.input()
	in.Reductions = []dnd5eEvents.DamageReduction{{
		Source: dnd5eEvents.RollSource{Ref: refs.Features.DeflectMissiles()}, DamageType: damage.Slashing, Modifier: 2,
	}}
	_, err := dnd5eEvents.NewIncomingDamageEvent(in)
	s.Require().ErrorIs(err, dnd5eEvents.ErrMalformedTargetAnswer)

	unnamed := resistance()
	unnamed.Source = dnd5eEvents.RollSource{}
	s.Require().ErrorIs(unnamed.Validate(), dnd5eEvents.ErrMalformedTargetAnswer)
	untyped := resistance()
	untyped.DamageType = ""
	s.Require().ErrorIs(untyped.Validate(), dnd5eEvents.ErrMalformedTargetAnswer)
}
