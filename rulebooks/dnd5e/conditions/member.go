// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
)

// member returns a participant's combat-facing sheet out of the installed
// cast, and whether the question could be answered at all.
//
// It wraps [gamectx.Cast.Member] and is named for what it does, not for what
// its callers mean by it. The caller supplies the ID, so nothing in this
// signature makes the lookup self-referential; a name promising otherwise
// would be claiming an intent the compiler cannot hold anyone to.
//
// # What its callers mean by it
//
// A condition reads ITSELF out of the cast, by passing its own character's ID.
// That is the read law: an effect reads the world, the other participants, and
// itself through one channel, with no second arrangement for the sheet it
// happens to be attached to. It looks itself up exactly as it would look up
// the creature standing next to it.
//
// What that replaced was a handle passed in at attach time — an owner the
// loader had to remember to hand over, wired bespoke in two loaders, silently
// absent whenever either forgot. A condition that reads the cast cannot be
// half-wired: the cast is installed by one door, on every path that folds
// anything.
//
// # Not-in-the-cast is "cannot answer", never "no"
//
// Two returns rather than one, and the second is not decoration. "This sheet
// says no shield" and "nobody could tell me whose sheet this is" are different
// facts, and a caller that collapses them invents a rule out of missing data.
//
// The caller must NOT turn a false second return into an error.
// Character.EffectiveAC folds the AC chain, and an erroring contributor takes
// every OTHER contributor down with it — which is how a barbarian ended up
// fighting at 10+DEX with Unarmored Defense attached and nothing logged. Leave
// the chain untouched instead: absent from the answer is recoverable, a
// poisoned fold is not.
func member(ctx context.Context, memberID string) (combat.Combatant, bool) {
	cast, ok := gamectx.CastOf(ctx)
	if !ok {
		return nil, false
	}

	return cast.Member(memberID)
}
