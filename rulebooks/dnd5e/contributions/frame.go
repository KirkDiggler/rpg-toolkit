// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions

import (
	"fmt"
	"math"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
)

// Stance is the relationship between two members on the disposition graph at
// the moment of asking: hostile, neutral, allied, or no side. It is not a
// boolean and it is not cached.
type Stance string

const (
	// StanceHostile means the two members are enemies.
	StanceHostile Stance = "hostile"
	// StanceNeutral means the two members are neither enemies nor allies.
	StanceNeutral Stance = "neutral"
	// StanceAllied means the two members are allies.
	StanceAllied Stance = "allied"
	// StanceNone means the two members have no side toward each other: one
	// of them belongs to no faction. It is a known fact and is not hostile.
	// It is never reported as neutral — neutral is a real disposition that
	// can turn hostile or allied — and it is distinct from an unknown Fact.
	StanceNone Stance = "none"
)

// ActionFacts are the action's effective facts, settled from the assembled
// action before any rule is asked. Ability Known("") means the action declares
// no governing ability, which is a stat block's honest answer.
//
// The weapon facts describe the weapon the attack is made with, not how it is
// delivered: a ranged weapon is a weapon of a ranged category, and Melee says
// only that the attack reaches as a melee attack. An attack made with no
// weapon — a spell attack — knows every weapon fact as its zero value.
type ActionFacts struct {
	Roll       Fact[RollKind]
	Ability    Fact[abilities.Ability]
	Melee      Fact[bool]
	WeaponPool Fact[bool]
	Advantage  Fact[bool]

	// AbilityModifier is the static modifier of the action's governing
	// ability as assembled; 0 when it declares none.
	AbilityModifier Fact[int]
	// Weapon is the canonical ref of the weapon the attack is made with,
	// including the unarmed strike; Known("") when there is none.
	Weapon Fact[string]
	// WeaponSlot is the hand the weapon is held in; Known("") when the
	// attack names no hand, as a stat block does not.
	WeaponSlot Fact[string]
	// Finesse says the weapon has the finesse property.
	Finesse Fact[bool]
	// RangedWeapon says the weapon is of a ranged category.
	RangedWeapon Fact[bool]
	// TwoHanded says the weapon is wielded in both hands, because it
	// requires it or a versatile weapon is gripped that way.
	TwoHanded Fact[bool]
	// OffHandWeapon says the attacker's other hand holds a weapon.
	OffHandWeapon Fact[bool]
	// OffHandAttack says this is the two-weapon fighting bonus attack.
	OffHandAttack Fact[bool]
	// Opportunity says this is an opportunity attack.
	Opportunity Fact[bool]
}

// PairFacts carries facts in the From→To direction. DistanceCells is in grid
// cells as the producer measured it. Sees says From can see To. No reverse fact
// is inferred.
type PairFacts struct {
	From          string
	To            string
	DistanceCells Fact[float64]
	Stance        Fact[Stance]
	Sees          Fact[bool]
}

// HeldCondition is one condition a member holds: its canonical condition ref
// and its source qualifier ("" when it has none).
type HeldCondition struct {
	Ref      string
	SourceID string
}

// MemberHeld lists what one member holds, in the order the producer reported.
// A member present with an empty list is KNOWN to hold nothing; a member
// absent from Frame.Held is unknown.
type MemberHeld struct {
	Member     string
	Conditions []HeldCondition
}

// Frame is everything a rule may read when it answers. Resolution builds it
// once per action and target: from the actor's knowledge for information, from
// authoritative state for execution. Complete is an explicit guarantee that
// Pairs covers every relevant member; a partial set of sightings never is.
// Held lists the conditions members are known to hold; see [Frame.HeldBy].
// The zero frame is invalid.
type Frame struct {
	Actor    string
	Target   Fact[string]
	Action   ActionFacts
	Pairs    []PairFacts
	Complete bool
	Held     []MemberHeld
}

// Validate refuses a frame no rule can read: no actor, a roll kind that is
// unknown or not one of the RollKind values, a known ability that is neither
// one of the six nor the declared none (Known("")), a known weapon that is
// neither a ref nor the declared none (Known("")), a known but empty target,
// malformed or duplicate pairs, an impossible known distance or an
// unrecognised known stance, or a malformed held list — an empty or repeated
// member, a condition that is not a ref, or one condition listed twice for a
// member. Unknown facts are valid; a known value that is
// not a real value is an error, never a negative answer.
func (f Frame) Validate() error {
	if f.Actor == "" {
		return fmt.Errorf("frame requires an actor")
	}
	roll, known := f.Action.Roll.Get()
	if !known {
		return fmt.Errorf("frame requires a known roll kind")
	}
	switch roll {
	case RollKindAttack, RollKindSavingThrow:
	default:
		return fmt.Errorf("unknown roll kind %q", roll)
	}
	if ability, known := f.Action.Ability.Get(); known && ability != "" &&
		!slices.Contains(abilities.AllAbilities(), ability) {
		return fmt.Errorf("unknown ability %q", ability)
	}
	if weapon, known := f.Action.Weapon.Get(); known && weapon != "" {
		if _, err := core.ParseString(weapon); err != nil {
			return fmt.Errorf("frame weapon %q is not a ref: %w", weapon, err)
		}
	}
	if target, known := f.Target.Get(); known && target == "" {
		return fmt.Errorf("frame target is known but empty")
	}
	seen := make(map[[2]string]bool, len(f.Pairs))
	for _, pair := range f.Pairs {
		key := [2]string{pair.From, pair.To}
		if pair.From == "" || pair.To == "" || pair.From == pair.To || seen[key] {
			return fmt.Errorf("invalid or duplicate frame pair %q to %q", pair.From, pair.To)
		}
		seen[key] = true
		if distance, known := pair.DistanceCells.Get(); known &&
			(distance < 0 || math.IsNaN(distance) || math.IsInf(distance, 0)) {
			return fmt.Errorf("invalid distance for frame pair %q to %q", pair.From, pair.To)
		}
		if stance, known := pair.Stance.Get(); known {
			switch stance {
			case StanceHostile, StanceNeutral, StanceAllied, StanceNone:
			default:
				return fmt.Errorf("unknown stance %q for frame pair %q to %q", stance, pair.From, pair.To)
			}
		}
	}
	members := make(map[string]bool, len(f.Held))
	for _, held := range f.Held {
		if held.Member == "" || members[held.Member] {
			return fmt.Errorf("invalid or repeated held member %q", held.Member)
		}
		members[held.Member] = true
		listed := make(map[HeldCondition]bool, len(held.Conditions))
		for _, condition := range held.Conditions {
			if _, err := core.ParseString(condition.Ref); err != nil {
				return fmt.Errorf("member %q holds %q, which is not a ref: %w", held.Member, condition.Ref, err)
			}
			if listed[condition] {
				return fmt.Errorf("member %q holds %q@%q twice", held.Member, condition.Ref, condition.SourceID)
			}
			listed[condition] = true
		}
	}
	return nil
}

// HeldBy returns what member holds and whether that is known. A member the
// frame does not list is unknown, never a member holding nothing.
func (f Frame) HeldBy(member string) ([]HeldCondition, bool) {
	for _, held := range f.Held {
		if held.Member == member {
			return held.Conditions, true
		}
	}
	return nil, false
}

// Pair returns the facts for the directed pair. A pair the frame does not
// carry comes back with every fact unknown, never as a negative.
func (f Frame) Pair(from, to string) PairFacts {
	for _, pair := range f.Pairs {
		if pair.From == from && pair.To == to {
			return pair
		}
	}
	return PairFacts{From: from, To: to}
}

// Clone detaches the frame's pairs and held lists; every fact is already a
// scalar value.
func (f Frame) Clone() Frame {
	f.Pairs = slices.Clone(f.Pairs)
	if f.Held != nil {
		held := make([]MemberHeld, len(f.Held))
		for i, member := range f.Held {
			held[i] = MemberHeld{Member: member.Member, Conditions: slices.Clone(member.Conditions)}
		}
		f.Held = held
	}
	return f
}
