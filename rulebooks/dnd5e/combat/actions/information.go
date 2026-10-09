// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// Grip names how the attacking weapon is held. It is a fact of the assembled
// attack, never derived from a weapon name.
type Grip string

const (
	// GripNone means the attack carries no weapon context.
	GripNone Grip = ""
	// GripOneHanded is a weapon held in one hand.
	GripOneHanded Grip = "one-handed"
	// GripTwoHanded is a weapon held in both hands.
	GripTwoHanded Grip = "two-handed"
	// GripOffHand is the bonus attack of two-weapon fighting.
	GripOffHand Grip = "off-hand"
)

// DescribeInput supplies the already-assembled action, not a raw weapon lookup.
type DescribeInput struct {
	Definition Definition
}

// DescribeOutput is an action's own information: authored prose and typed base
// facts. No field is a formatted display string; hosts render the facts.
type DescribeOutput struct {
	// Description is Definition.Description verbatim. It may be empty.
	Description string
	// Facts are the typed base facts of the action.
	Facts BaseFacts
}

// BaseFacts states what an action does before any contextual effect folds in.
// A fact a host chooses not to show is still stated here: a modifier that does
// not join the damage is an AbilityFact with Participates false, not an absent
// field.
type BaseFacts struct {
	// Damage lists attack pools, then cast pools, in declared order. Nil when
	// the action declares none.
	Damage []DamageFact
	// Grip is GripNone unless the attack carries weapon context.
	Grip Grip
	// Melee is a copy of the attack's melee delivery, nil when ranged.
	Melee *MeleeDelivery
	// Ranged is a copy of the attack's ranged delivery, nil when melee.
	Ranged *RangedDelivery
	// Cast states a cast's facts. Nil unless Definition.Cast is set.
	Cast *CastFacts
}

// CastFacts states a cast's own facts. A DC is stated as a number only when its
// source is static; otherwise it is stated unknown, never guessed.
type CastFacts struct {
	RangeFeet       int
	Targets         TargetsFact
	Save            *SaveFact
	DamageIfInjured []DamageFact
	Effects         []EffectFact
	Healing         *HealingFact
	Area            *CastArea
	Concentration   *CastConcentration
}

// TargetsFact is the cast's targeting rule and cardinality.
type TargetsFact struct {
	Rule CastTargetRule
	Min  int
	Max  int
}

// SaveFact is the gate a cast is contested with.
type SaveFact struct {
	Abilities []abilities.Ability
	// DC is meaningful only when DCKnown.
	DC int
	// DCKnown is false for any non-static DC source; DC is then zero.
	DCKnown    bool
	OnSuccess  saves.SaveEffect
	Recurrence saves.Recurrence
}

// EffectFact is one condition a cast delivers.
type EffectFact struct {
	Ref       core.Ref
	Recipient CastRecipient
	// OnFailedSave is true when a save gates delivery of this condition.
	OnFailedSave bool
}

// HealingFact is a cast's healing pool and its sourced fixed contributions.
type HealingFact struct {
	Dice      string
	Modifiers []ModifierFact
}

// ModifierFact is one fixed contribution with its content-authored source name.
type ModifierFact struct {
	Name   string
	Amount int
}

// DamageFact is one damage pool.
type DamageFact struct {
	Dice      string
	FlatBonus int
	Type      damage.Type
	// Ability is nil when the pool does not add the attack ability modifier.
	Ability *AbilityFact
}

// AbilityFact is the attack ability's contribution to a pool.
type AbilityFact struct {
	Ability abilities.Ability
	// Modifier is the ability modifier. A present zero is a real zero.
	Modifier int
	// Participates is damage.IncludesAbilityModifier's answer for this attack.
	Participates bool
}

// Describe states a validated definition's information without modifying it,
// asking a target, rolling, spending or folding effects. Whether an ability
// modifier joins damage is damage.IncludesAbilityModifier's answer, the same
// one execution consumes. Nil or malformed input is an error; absent prose is
// not invented.
func Describe(in *DescribeInput) (*DescribeOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("describe action: input is required")
	}
	if err := in.Definition.Validate(); err != nil {
		return nil, fmt.Errorf("describe action: %w", err)
	}
	out := &DescribeOutput{Description: in.Definition.Description}
	attack := in.Definition.Attack
	if attack == nil && in.Definition.Cast != nil {
		attack = in.Definition.Cast.Attack
	}
	if attack != nil {
		out.Facts.Damage = append(out.Facts.Damage, attackDamageFacts(attack)...)
		out.Facts.Grip = gripOf(attack)
		if attack.Delivery.Melee != nil {
			melee := *attack.Delivery.Melee
			out.Facts.Melee = &melee
		}
		if attack.Delivery.Ranged != nil {
			ranged := *attack.Delivery.Ranged
			out.Facts.Ranged = &ranged
		}
	}
	if cast := in.Definition.Cast; cast != nil {
		facts := castFacts(cast)
		out.Facts.Cast = &facts
		for _, pool := range cast.Damage {
			out.Facts.Damage = append(out.Facts.Damage, plainDamageFact(pool))
		}
	}
	return out, nil
}

func gripOf(attack *AttackProfile) Grip {
	switch {
	case attack.IsOffHandAttack:
		return GripOffHand
	case attack.Weapon == nil:
		return GripNone
	case attack.Weapon.TwoHanded:
		return GripTwoHanded
	default:
		return GripOneHanded
	}
}

func attackDamageFacts(attack *AttackProfile) []DamageFact {
	facts := make([]DamageFact, 0, len(attack.Damage))
	for _, pool := range attack.Damage {
		fact := plainDamageFact(pool)
		if attack.Ability != nil && pool.HasProperty(damage.AddsAttackAbilityModifier) {
			fact.Ability = &AbilityFact{
				Ability:  attack.Ability.Ability,
				Modifier: attack.Ability.Modifier,
				Participates: damage.IncludesAbilityModifier(damage.AbilityModifierInput{
					Modifier: attack.Ability.Modifier, OffHand: attack.IsOffHandAttack,
				}),
			}
		}
		facts = append(facts, fact)
	}
	return facts
}

func plainDamageFact(pool damage.Damage) DamageFact {
	return DamageFact{Dice: pool.Dice, FlatBonus: pool.FlatBonus, Type: pool.Type}
}

func castFacts(cast *CastProfile) CastFacts {
	facts := CastFacts{
		RangeFeet: cast.RangeFeet,
		Targets:   TargetsFact{Rule: cast.Target, Min: cast.MinTargets, Max: cast.MaxTargets},
	}
	if gate := cast.Save; gate != nil {
		save := &SaveFact{
			Abilities:  append([]abilities.Ability(nil), gate.Abilities...),
			OnSuccess:  gate.OnSuccess,
			Recurrence: gate.Recurrence,
		}
		if gate.DC != nil && gate.DC.Kind() == saves.DCKindStatic {
			save.DC = gate.DC.DC(saves.DCInput{})
			save.DCKnown = true
		}
		facts.Save = save
	}
	for _, pool := range cast.DamageIfInjured {
		facts.DamageIfInjured = append(facts.DamageIfInjured, plainDamageFact(pool))
	}
	for _, effect := range cast.Effects {
		facts.Effects = append(facts.Effects, EffectFact{
			Ref: effect.Ref, Recipient: effect.Recipient, OnFailedSave: cast.Save != nil,
		})
	}
	if cast.Healing != nil {
		healed := &HealingFact{Dice: cast.Healing.Dice}
		for _, modifier := range cast.Healing.Modifiers {
			healed.Modifiers = append(healed.Modifiers, ModifierFact{Name: modifier.Source.Name, Amount: modifier.Amount})
		}
		facts.Healing = healed
	}
	if cast.Area != nil {
		area := *cast.Area
		facts.Area = &area
	}
	if cast.Concentration != nil {
		concentration := *cast.Concentration
		facts.Concentration = &concentration
	}
	return facts
}
