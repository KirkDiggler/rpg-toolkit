// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	fx "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func (s *partsSuite) resistanceTaken(bus events.EventBus, kind damage.Type) (int, []dndevents.DamageMultiplier) {
	amount := 9
	cause := core.Ref{Module: "example", Type: "actions", ID: "incoming"}
	frame := attackFrame(0)
	frame.Actor = "enemy"
	frame.Target = contributions.Known("recipient")
	sent, err := dndevents.NewIncomingDamageEvent(dndevents.IncomingDamageInput{
		TargetID: "recipient", SourceID: "enemy", Frame: frame,
		Dealt: []dndevents.DamageComponent{{Source: dndevents.DamageSourceWeapon, DamageType: kind,
			Roll: dndevents.RollComponent{Source: dndevents.RollSource{Ref: &cause, Name: "Incoming"}, Modifier: &amount}}},
	})
	s.Require().NoError(err)
	ctx := context.Background()
	chain, err := dndevents.IncomingDamageChain.On(bus).PublishWithChain(ctx, sent.Clone(), events.NewStagedChain[*dndevents.IncomingDamageEvent](combat.ModifierStages))
	s.Require().NoError(err)
	folded, err := chain.Execute(ctx, sent.Clone())
	s.Require().NoError(err)
	s.Require().NoError(folded.CheckUnaltered(sent))
	settled, err := combat.SettleDamage(&combat.SettleDamageInput{Dealt: sent.Dealt(), Reductions: folded.Reductions, Multipliers: folded.Multipliers})
	s.Require().NoError(err)
	s.Require().Len(settled.Types, 1)
	return settled.Types[0].Taken, folded.Multipliers
}

// This is local part/meaning proof. Actual linked-application ending belongs to
// the later transport task; Remove here deliberately tests only local cleanup.
func (s *partsSuite) TestResistanceMeaningIsSharedWithRaging() {
	input := partInput("resistance", dndfx.KindResistance, `{"types":["bludgeoning","piercing","slashing"]}`)
	input.HolderID = "recipient"
	input.Declaration.OnTerminal = fx.EndPart
	ward, err := fx.Assemble(fx.AssemblyInput{Key: "ward-recipient", Parts: []fx.PartInput{s.bind(input)}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(ward.Apply(ctx, bus))
	taken, multipliers := s.resistanceTaken(bus, damage.Slashing)
	s.Equal(4, taken)
	s.Require().Len(multipliers, 1)
	s.Equal("maintainer", multipliers[0].Source.SourceID)
	fire, _ := s.resistanceTaken(bus, damage.Fire)
	s.Equal(9, fire)

	raging, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref: refs.Conditions.Raging().String(), MemberID: "recipient",
		Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "recipient"},
		Config: json.RawMessage(`{"damage_bonus":{"fixed":3}}`),
	})
	s.Require().NoError(err)
	s.Require().NoError(raging.Condition.Apply(ctx, bus))
	taken, multipliers = s.resistanceTaken(bus, damage.Slashing)
	s.Equal(4, taken, "two resistance sources do not quarter damage")
	s.Require().Len(multipliers, 2)
	s.Require().NoError(ward.Remove(ctx, bus))
	taken, multipliers = s.resistanceTaken(bus, damage.Slashing)
	s.Equal(4, taken)
	s.Require().Len(multipliers, 1)
	s.Equal("recipient", multipliers[0].Source.SourceID)
	s.Require().NoError(raging.Condition.Remove(ctx, bus))
	taken, multipliers = s.resistanceTaken(bus, damage.Slashing)
	s.Equal(9, taken)
	s.Empty(multipliers)
}
