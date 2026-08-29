// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// self returns this condition's OWN combat-facing sheet, and whether the
// question could be asked at all.
//
// # Self is a member
//
// An effect reads the world, the other participants, and ITSELF through one
// channel. There is no second arrangement for the sheet a condition happens to
// be attached to: it looks itself up in the cast by its own ID, exactly as it
// would look up the creature it is standing next to. That is the whole of the
// read law, and this function is the one place it is spelled.
//
// What it replaces is a handle passed in at attach time — an owner the loader
// had to remember to hand over, wired bespoke in two loaders, silently absent
// when either forgot. A condition that reads the cast cannot be half-wired: the
// cast is installed by one door on every path that folds anything.
//
// # Not-in-the-cast is "cannot answer", never "no"
//
// Two returns rather than one, and the second is not decoration. "This sheet
// says no shield" and "nobody could tell me whose sheet this is" are different
// facts, and a caller that collapses them invents a rule out of missing data.
//
// The caller must NOT turn a false second return into an error. Character
// .EffectiveAC folds the AC chain, and an erroring contributor takes every
// OTHER contributor down with it — which is how a barbarian ended up fighting
// at 10+DEX with Unarmored Defense attached and nothing logged. Leave the chain
// untouched instead: absent from the answer is recoverable, a poisoned fold is
// not.
func self(ctx context.Context, ownID string) (combat.Combatant, bool) {
	cast, ok := gamectx.CastOf(ctx)
	if !ok {
		return nil, false
	}

	return cast.Member(ownID)
}
