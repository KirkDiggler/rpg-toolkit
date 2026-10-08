// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

var _ contributions.ActionAssessor = (*RagingCondition)(nil)

// AssessAction answers whether Rage's damage bonus applies to the framed
// action. It reads the frame — the attacker's barbarian levels included — and
// this rage's own owner; it never
// applies, sustains or ends Rage. Defensive resistance and the save/check
// advantages are not part of this answer.
func (r *RagingCondition) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	return ragingDamageRule{owner: r.CharacterID}.AssessAction(in)
}

// ragingDamageRule holds only the facts Rage's damage predicate uses, not the
// condition, so asking it cannot touch the live rage.
type ragingDamageRule struct {
	owner string
}

// rageDamageBonus is Rage's melee damage bonus for a barbarian level: +2
// through 8th, +3 from 9th, +4 from 16th.
func rageDamageBonus(level int) int {
	switch {
	case level < 9:
		return 2
	case level < 16:
		return 3
	default:
		return 4
	}
}

func (r ragingDamageRule) AssessAction(in *contributions.AssessActionInput) (*contributions.AssessActionOutput, error) {
	frame, err := frameOf(in, "raging")
	if err != nil {
		return nil, err
	}
	answer := func(state contributions.Applicability, reason string) *contributions.AssessActionOutput {
		return &contributions.AssessActionOutput{Answer: contributions.Answer{
			Decision:      contributions.Decision{Applicability: state, Reason: reason},
			Participation: contributions.ContributesNow,
		}}
	}
	if frame.Actor != r.owner {
		return answer(contributions.DoesNotApply, "Rage modifies its recipient's attacks"), nil
	}
	level, err := actorClassLevel(frame, classes.Barbarian, "raging")
	if err != nil {
		return nil, err
	}
	weapon, weaponKnown := frame.Action.WeaponPool.Get()
	melee, meleeKnown := frame.Action.Melee.Get()
	ability, abilityKnown := frame.Action.Ability.Get()
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
		return answer(contributions.Depends, "Depends on the attack's weapon and ability"), nil
	}
	out := answer(contributions.Applies, "The melee weapon attack uses Strength")
	bonus := rageDamageBonus(level)
	out.Answer.Benefit = fmt.Sprintf("+%d damage", bonus)
	out.Answer.Damage = []contributions.DamageChange{{
		PoolID: contributions.PrimaryWeaponPool,
		Source: contributions.Source{Ref: refs.Conditions.Raging(), Name: "Raging"},
		Fixed:  &bonus,
	}}
	return out, nil
}
