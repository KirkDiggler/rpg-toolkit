// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// CastTargetRule names who a cast may be pointed at. Two rules, because two is
// what the content has: a cast that needs no target at all and a cast that
// names one creature.
type CastTargetRule string

const (
	// CastTargetSelf is a cast with no target but the caster.
	CastTargetSelf CastTargetRule = "self"

	// CastTargetOneCreature is the existing creature target kind. MinTargets
	// and MaxTargets carry cardinality, including Bane's one-to-three range.
	CastTargetOneCreature CastTargetRule = "one_creature"

	// CastTargetArea is a cast whose recipients the ENGINE derives, from a
	// shape the content declares. The caller names nobody.
	//
	// A RULE OF ITS OWN rather than a nil-check on [CastProfile.Area], and the
	// cost — arms in three closed switches — is the point. The alternative is
	// precedence between two fields: CastTargetSelf with a non-nil Area, read
	// as "self, except not really". Resolution's self arm rewrites the target
	// list to the caster, so a spell that must never hit the caster would be
	// resolved against them by a rule that looked right. One value meaning two
	// things is how that happens quietly; [CastProfile.Validate] binds the two
	// so neither can appear without the other.
	CastTargetArea CastTargetRule = "area"
)

// CastRecipient names which of a cast's two parties one delivered condition
// lands on.
//
// A cast has at most two parties and a condition lands on one of them. True
// Strike names a creature and puts its condition on the CASTER; Vicious Mockery
// names a creature and puts its condition on the TARGET. Without this field the
// two are indistinguishable in content, and resolution would have to know which
// spell it was holding — exactly the identity dispatch ADR-0045 forbids.
type CastRecipient string

const (
	// CastRecipientCaster puts the condition on whoever cast.
	CastRecipientCaster CastRecipient = "caster"

	// CastRecipientTarget puts the condition on the creature the cast named.
	CastRecipientTarget CastRecipient = "target"
)

// CastProfile declares everything a cast-resolution machine needs, and names no
// spell.
//
// # Nothing here says which spell this is
//
// A warlock's Eldritch Blast, a monster's innate cast, and a bard's cantrip all
// declare a profile rather than asking resolution for a case (ADR-0045). The
// arm resolution reads is [CastProfile.Save]: a profile with a gate is a save
// contested before anything is delivered, and a profile without one delivers
// straight away.
//
// # The price is not here
//
// What a cast costs is [Definition.Cost], compiled by whoever mints the
// declaration, because the same profile is free for a monster's innate cast and
// an action for a player's.
type CastProfile struct {
	// RangeFeet is how far the cast reaches. A self-targeted cast still
	// declares one, because the range is what a UI draws.
	RangeFeet int `json:"range_feet"`

	// Target is what kind of recipient may be named. Cardinality is declared
	// separately so one creature kind can support both single- and multi-target casts.
	Target CastTargetRule `json:"target"`

	// MinTargets and MaxTargets bound the ordered target list. Self-targeted
	// casts declare zero; creature-targeted casts require at least one.
	MinTargets int `json:"min_targets"`
	MaxTargets int `json:"max_targets"`

	// Save is the gate the target contests the whole cast with, or nil for a
	// cast that lands without a roll. Negated-on-success only: a successful
	// save against a cantrip negates every consequence, and half-on-success
	// arrives with a spell that has one.
	Save *saves.SaveGate `json:"save,omitempty"`

	// Damage is what the cast deals when it lands. Never marked with
	// [damage.AddsAttackAbilityModifier]: no attack roll happens here, so
	// there is no attack ability to add.
	Damage []damage.Damage `json:"damage,omitempty"`

	// Effects are the conditions the cast delivers when it lands.
	Effects []CastEffect `json:"effects,omitempty"`

	// Area is the shape this cast covers, or nil for a cast that names its
	// recipients instead. Non-nil exactly when Target is [CastTargetArea].
	//
	// A pointer for [CastConcentration]'s reason: a shape beside a target rule
	// that means nothing unless the rule says "area" is a zero value that
	// lies. Nil is "this cast names its targets"; non-nil is the whole answer.
	Area *CastArea `json:"area,omitempty"`

	// Concentration is how long the caster must hold this cast together, or
	// nil for a cast that needs no concentration at all.
	//
	// A POINTER RATHER THAN A BOOL, because a bool beside a duration that
	// means nothing when the bool is false is a zero value that lies. Nil is
	// "no concentration"; non-nil is the whole answer.
	Concentration *CastConcentration `json:"concentration,omitempty"`
}

// CastConcentration is what a concentration cast declares: how long the caster
// holds it, if nothing takes it away first.
//
// ONE CLOCK, ON THE OWNER. The effects a concentration spell leaves behind do
// not each count their own turn ends — the concentrating condition counts, and
// they end when it does. Two clocks would be two answers to one question.
type CastConcentration struct {
	// TurnEnds is how many of the caster's turn ends the spell survives.
	// Turn ends rather than minutes because turn end is the only duration
	// boundary this stack has.
	TurnEnds int `json:"turn_ends"`

	// SkipFirstTurnEnd gives a newly created owner one persisted grace boundary.
	// It is used when the cast resolves during the caster's current turn so that
	// turn does not consume one of the declared subsequent turn ends.
	SkipFirstTurnEnd bool `json:"skip_first_turn_end,omitempty"`
}

// CastEffect declares one condition a cast delivers, and who receives it.
type CastEffect struct {
	// Recipient is which party the condition lands on.
	Recipient CastRecipient `json:"recipient"`

	// Ref names the condition to build. Always a dnd5e:conditions ref.
	Ref core.Ref `json:"ref"`

	// Parameters are the condition's own configuration, opaque here.
	Parameters json.RawMessage `json:"parameters,omitempty"`

	// CounterpartKey names the parameter whoever builds this condition must
	// fill with the cast's OTHER party — the named target when the condition
	// lands on the caster, the caster when it lands on the target. Empty when
	// the condition needs no such binding.
	//
	// Declared rather than inferred because the two conditions this slice
	// ships spell it differently and mean different things by it: True Strike
	// is keyed to the target it grants advantage against ("target_id"),
	// Vicious Mockery records the bard who imposed it ("source_id"). A
	// convention that guessed one key would silently drop the other.
	CounterpartKey string `json:"counterpart_key,omitempty"`
}

// Validate reports whether the profile declares a reachable range, a known
// target rule, a contestable gate, and at least one consequence.
//
// A cast with no damage and no condition is refused rather than resolved into a
// no-op: it would mint a row at the door that delivered nothing, which is the
// affordance-with-nothing-behind-it this stack keeps finding.
func (p CastProfile) Validate() error {
	if p.RangeFeet <= 0 {
		return fmt.Errorf("cast must declare a positive range")
	}

	switch p.Target {
	case CastTargetSelf:
		if p.MinTargets != 0 || p.MaxTargets != 0 {
			return fmt.Errorf("self-targeted cast must declare zero targets")
		}
	case CastTargetOneCreature:
		if p.MinTargets < 1 {
			return fmt.Errorf("creature-targeted cast must require at least one target")
		}
		if p.MaxTargets < p.MinTargets {
			return fmt.Errorf("cast maximum targets must be at least its minimum")
		}
	case CastTargetArea:
		// Zero targets for the same reason a self cast declares zero: these
		// bound what the CALLER may name, and the caller names nobody. Who
		// receives an area cast is a different question, answered by the
		// engine from the shape.
		if p.MinTargets != 0 || p.MaxTargets != 0 {
			return fmt.Errorf("area cast must declare zero targets")
		}
	default:
		return fmt.Errorf("unknown cast target rule %q", p.Target)
	}

	// The two halves are bound in both directions, so neither an area rule with
	// no shape nor a shape no rule reads can reach a machine.
	if p.Target == CastTargetArea && p.Area == nil {
		return fmt.Errorf("area cast must declare an area")
	}
	if p.Target != CastTargetArea && p.Area != nil {
		return fmt.Errorf("cast declares an area but its target rule is %q", p.Target)
	}
	if p.Area != nil {
		if err := p.Area.Validate(); err != nil {
			return fmt.Errorf("cast area is invalid: %w", err)
		}
	}

	if p.Save != nil {
		if p.Save.OnSuccess != saves.Negated {
			return fmt.Errorf("cast save must negate the cast on success")
		}
		if p.Save.Recurrence != saves.RecurrenceNone {
			return fmt.Errorf("cast save must not recur")
		}
		if err := p.Save.Validate(); err != nil {
			return fmt.Errorf("cast save is invalid: %w", err)
		}
	}

	if len(p.Damage) == 0 && len(p.Effects) == 0 {
		return fmt.Errorf("cast must declare damage or a delivered condition")
	}
	if len(p.Damage) > 0 {
		if err := damage.Validate(p.Damage); err != nil {
			return fmt.Errorf("cast damage declaration is invalid: %w", err)
		}
		for _, pool := range p.Damage {
			if pool.HasProperty(damage.AddsAttackAbilityModifier) {
				return fmt.Errorf("cast damage must not be marked with the attack ability modifier")
			}
		}
	}

	for index, effect := range p.Effects {
		if err := effect.validate(p.Target); err != nil {
			return fmt.Errorf("cast effect %d is invalid: %w", index, err)
		}
	}

	if p.Concentration != nil && p.Concentration.TurnEnds <= 0 {
		// A declared concentration that expires before it begins is an
		// affordance with nothing behind it: the caster would hold a spell
		// the clock had already ended. Nil says "no concentration"; this
		// field is only reached by a profile that said there is some.
		return fmt.Errorf("cast concentration must last at least one turn end")
	}

	return nil
}

// Clone returns a deep copy of the profile's mutable gate, damage, and effect
// declarations.
func (p CastProfile) Clone() CastProfile {
	clone := p
	if p.Save != nil {
		save := *p.Save
		save.Abilities = append([]abilities.Ability(nil), p.Save.Abilities...)
		clone.Save = &save
	}
	if p.Damage != nil {
		clone.Damage = make([]damage.Damage, len(p.Damage))
		copy(clone.Damage, p.Damage)
		for index := range p.Damage {
			clone.Damage[index].Properties = append([]damage.Property(nil), p.Damage[index].Properties...)
		}
	}
	if p.Effects != nil {
		clone.Effects = make([]CastEffect, len(p.Effects))
		for index, effect := range p.Effects {
			clone.Effects[index] = effect.Clone()
		}
	}
	if p.Area != nil {
		area := *p.Area
		clone.Area = &area
	}
	if p.Concentration != nil {
		concentration := *p.Concentration
		clone.Concentration = &concentration
	}
	return clone
}

// validate reports whether this effect names a D&D 5e condition, a known
// recipient, and a binding the cast's target rule can actually satisfy.
func (e CastEffect) validate(target CastTargetRule) error {
	switch e.Recipient {
	case CastRecipientCaster, CastRecipientTarget:
	default:
		return fmt.Errorf("unknown cast effect recipient %q", e.Recipient)
	}
	if target == CastTargetSelf {
		if e.Recipient != CastRecipientCaster {
			return fmt.Errorf("a self-targeted cast delivers only to the caster")
		}
		if e.CounterpartKey != "" {
			return fmt.Errorf("a self-targeted cast has no counterpart to bind")
		}
	}
	if err := e.Ref.IsValid(); err != nil {
		return fmt.Errorf("condition ref is invalid: %w", err)
	}
	if e.Ref.Module != dnd5eModule || e.Ref.Type != conditionType {
		return fmt.Errorf("condition ref must use %s:%s, got %s", dnd5eModule, conditionType, e.Ref.String())
	}
	if len(e.Parameters) > 0 && !json.Valid(e.Parameters) {
		return fmt.Errorf("condition parameters must be valid JSON")
	}
	return nil
}

// Clone returns a deep copy of the effect's opaque parameters.
func (e CastEffect) Clone() CastEffect {
	clone := e
	clone.Parameters = append(json.RawMessage(nil), e.Parameters...)
	return clone
}
