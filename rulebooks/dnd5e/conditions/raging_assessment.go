// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/assessment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// AssessmentSnapshot captures Rage's outgoing-damage rule without its live bus,
// duration or receiver pointer. Defensive resistance and save/check advantages
// remain outside this outgoing, pre-defense calculation. Invalid binding identity
// is an error; reading the snapshot never applies, sustains or consumes Rage.
func (r *RagingCondition) AssessmentSnapshot(in *assessment.BindInput) (*assessment.BindOutput, error) {
	if r == nil || in == nil || in.ID == "" || in.OwnerID == "" ||
		in.OwnerID != r.CharacterID || in.Order < 0 {
		return nil, fmt.Errorf("raging snapshot requires its owner and a valid binding identity")
	}
	return &assessment.BindOutput{Binding: assessment.RuleBinding{
		ID: in.ID, Order: in.Order,
		Source: contributions.CloneSource(contributions.Source{Ref: refs.Conditions.Raging(), Name: "Raging"}),
		Address: &dndevents.ConditionAddress{
			MemberID: r.CharacterID, ConditionRef: refs.Conditions.Raging().String(),
		},
		Coverage: []assessment.Coverage{
			{Facet: contributions.AttackRoll, Support: assessment.NotRelevant, Reason: "Rage does not modify the attack roll"},
			{Facet: contributions.DamageNormal, Support: assessment.Supported, Reason: "Rage can add weapon damage"},
			{Facet: contributions.DamageCritical, Support: assessment.Supported, Reason: "Rage can add weapon damage"},
			{Facet: contributions.SpellDC, Support: assessment.NotRelevant, Reason: "Rage does not modify caster save DC"},
			{Facet: contributions.Healing, Support: assessment.NotRelevant, Reason: "Rage does not modify healing"},
		},
		Damage: ragingDamageRule{owner: r.CharacterID, bonus: r.DamageBonus},
	}}, nil
}

// A value containing only the facts the owning predicate uses, not the condition.
type ragingDamageRule struct {
	owner string
	bonus int
}

func (r ragingDamageRule) AssessDamage(in *assessment.AssessDamageInput) (*assessment.AssessDamageOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("raging damage assessment requires input")
	}
	answer := func(state contributions.Applicability, reason string) *assessment.AssessDamageOutput {
		return &assessment.AssessDamageOutput{Decision: contributions.Decision{Applicability: state, Reason: reason}}
	}
	if in.Frame.ActorID != r.owner {
		return answer(contributions.DoesNotApply, "Rage modifies its recipient's attacks"), nil
	}
	weapon, weaponKnown := in.Frame.HasWeaponPool.Get()
	melee, meleeKnown := in.Frame.Melee.Get()
	ability, abilityKnown := in.Frame.Ability.Get()
	if weaponKnown && !weapon {
		return answer(contributions.DoesNotApply, "Rage requires a weapon damage pool"), nil
	}
	if meleeKnown && !melee {
		return answer(contributions.DoesNotApply, "Rage's damage bonus requires a melee weapon attack"), nil
	}
	if abilityKnown && ability != abilities.STR {
		return answer(contributions.DoesNotApply, "Rage's damage bonus requires Strength"), nil
	}
	if !weaponKnown || !meleeKnown || !abilityKnown {
		out := answer(contributions.NeedsContext, "The attack's effective weapon and ability facts are not established")
		out.Decision.Needs = []contributions.Need{{Kind: contributions.NeedActionFacts, Subject: r.owner}}
		return out, nil
	}
	out := answer(contributions.Applies, "The melee weapon attack uses Strength")
	bonus := r.bonus
	out.Changes = []assessment.DamageChange{{
		PoolID: assessment.PrimaryWeaponPool,
		Source: contributions.CloneSource(contributions.Source{Ref: refs.Conditions.Raging(), Name: "Raging"}),
		Fixed:  &bonus,
	}}
	return out, nil
}
