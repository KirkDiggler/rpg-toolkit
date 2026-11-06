// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

func rageAttackFrame(actor, target string) contributions.Frame {
	return contributions.Frame{
		Actor: actor, Target: contributions.Known(target),
		Action: contributions.ActionFacts{
			Roll: contributions.Known(contributions.RollKindAttack), Ability: contributions.Known(abilities.STR),
			Melee: contributions.Known(true), WeaponPool: contributions.Known(true),
		},
	}
}

func baseDamage(amount int, kind damage.Type) []dndevents.DamageComponent {
	ref := core.Ref{Module: "example", Type: "actions", ID: "strike"}
	return []dndevents.DamageComponent{{
		Source: dndevents.DamageSourceWeapon, DamageType: kind,
		Roll: dndevents.RollComponent{Source: dndevents.RollSource{Ref: &ref, Name: "Example strike"}, Modifier: &amount},
	}}
}

// The harness plays the caller's fold role using real event/chain contracts.
// It is not a claim that an actual encounter/session or character sheet ran.
func rageDamage(ctx context.Context, bus events.EventBus, actor string, ability abilities.Ability, melee, weapon bool) (
	*dndevents.DamageChainEvent, error,
) {
	frame := rageAttackFrame(actor, "target")
	frame.Action.Ability = contributions.Known(ability)
	frame.Action.Melee = contributions.Known(melee)
	frame.Action.WeaponPool = contributions.Known(weapon)
	input := dndevents.DamageChainInput{
		AttackerID: actor, TargetID: "target", Frame: frame,
		Components: baseDamage(10, damage.Slashing), WeaponDamageType: damage.Slashing,
	}
	c, err := dndevents.DamageChain.On(bus).PublishWithChain(ctx, dndevents.NewDamageChainEvent(input),
		events.NewStagedChain[*dndevents.DamageChainEvent](combat.ModifierStages))
	if err != nil {
		return nil, err
	}
	return c.Execute(ctx, dndevents.NewDamageChainEvent(input))
}

func rageIncoming(ctx context.Context, bus events.EventBus, target string, kind damage.Type) (*combat.SettleDamageOutput, error) {
	sent, err := dndevents.NewIncomingDamageEvent(dndevents.IncomingDamageInput{
		TargetID: target, SourceID: "enemy", Frame: rageAttackFrame("enemy", target), Dealt: baseDamage(9, kind),
	})
	if err != nil {
		return nil, err
	}
	c, err := dndevents.IncomingDamageChain.On(bus).PublishWithChain(ctx, sent.Clone(),
		events.NewStagedChain[*dndevents.IncomingDamageEvent](combat.ModifierStages))
	if err != nil {
		return nil, err
	}
	folded, err := c.Execute(ctx, sent.Clone())
	if err != nil {
		return nil, err
	}
	if err := folded.CheckUnaltered(sent); err != nil {
		return nil, err
	}
	return combat.SettleDamage(&combat.SettleDamageInput{
		Dealt: sent.Dealt(), Reductions: folded.Reductions, Multipliers: folded.Multipliers,
	})
}

func rageSave(ctx context.Context, bus events.EventBus, holder string, ability abilities.Ability) (*dndevents.SavingThrowChainEvent, error) {
	event := &dndevents.SavingThrowChainEvent{SaverID: holder, Ability: ability, DC: 10}
	c, err := dndevents.SavingThrowChain.On(bus).PublishWithChain(ctx, event,
		events.NewStagedChain[*dndevents.SavingThrowChainEvent](combat.ModifierStages))
	if err != nil {
		return nil, err
	}
	return c.Execute(ctx, &dndevents.SavingThrowChainEvent{SaverID: holder, Ability: ability, DC: 10})
}

func rageCheck(ctx context.Context, bus events.EventBus, holder string, skill skills.Skill) (*dndevents.AbilityCheckChainEvent, error) {
	event := &dndevents.AbilityCheckChainEvent{CheckerID: holder, Skill: skill, DC: 10}
	c, err := dndevents.AbilityCheckChain.On(bus).PublishWithChain(ctx, event,
		events.NewStagedChain[*dndevents.AbilityCheckChainEvent](combat.ModifierStages))
	if err != nil {
		return nil, err
	}
	return c.Execute(ctx, &dndevents.AbilityCheckChainEvent{CheckerID: holder, Skill: skill, DC: 10})
}
