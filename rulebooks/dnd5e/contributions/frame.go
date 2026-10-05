// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package contributions

import (
	"fmt"
	"math"
	"slices"

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
type ActionFacts struct {
	Roll       Fact[RollKind]
	Ability    Fact[abilities.Ability]
	Melee      Fact[bool]
	WeaponPool Fact[bool]
	Advantage  Fact[bool]
}

// PairFacts carries facts in the From→To direction. DistanceCells is in grid
// cells as the producer measured it. No reverse fact is inferred.
type PairFacts struct {
	From          string
	To            string
	DistanceCells Fact[float64]
	Stance        Fact[Stance]
}

// Frame is everything a rule may read when it answers. Resolution builds it
// once per action and target: from the actor's knowledge for information, from
// authoritative state for execution. Complete is an explicit guarantee that
// Pairs covers every relevant member; a partial set of sightings never is.
// The zero frame is invalid.
type Frame struct {
	Actor    string
	Target   Fact[string]
	Action   ActionFacts
	Pairs    []PairFacts
	Complete bool
}

// Validate refuses a frame no rule can read: no actor, an unknown roll kind,
// a known but empty target, malformed or duplicate pairs, an impossible known
// distance or an unrecognised known stance. Unknown facts are valid.
func (f Frame) Validate() error {
	if f.Actor == "" {
		return fmt.Errorf("frame requires an actor")
	}
	if _, known := f.Action.Roll.Get(); !known {
		return fmt.Errorf("frame requires a known roll kind")
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
	return nil
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

// Clone detaches the frame's pairs; every fact is already a scalar value.
func (f Frame) Clone() Frame {
	f.Pairs = slices.Clone(f.Pairs)
	return f
}
