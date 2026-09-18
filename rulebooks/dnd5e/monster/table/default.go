// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package table

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// DefaultTable is one table of authored entries, carried as the text an
// author would have written.
//
// It is deliberately not a parsed structure. The rulebook's default and the
// author's orders have to be the same kind of thing for the layering to work
// — rulebook, then faction, then placement, nearest key winning wholesale —
// and the cheapest way to make them the same kind of thing is for both to be
// the grammar an author writes.
type DefaultTable struct {
	// Source is the CONTENTS of an `on:` mapping, as YAML: the trigger keys
	// and their entries, with no `on:` header of their own. That is what
	// dungeonspec's CompileTable takes, and it is what an author writes
	// under `on:` on a faction or a placement — so the rulebook's default
	// and the author's orders reach the compiler as the same text.
	//
	// This package does not parse it.
	Source string
}

// generic is the table every monster kind answers with today.
//
// Its order is its argument. A creature that has answered `flee` runs on its
// own time for three rounds, which is why the front room's goblin needs no
// table of its own — `flee` lands the deed and this entry does the running.
// A creature that was struck comes for whoever struck it, which is what the
// retaliator preset did with a grudge and a patience number, now in the
// author's sight as `within: 3`. Failing both, it fights what it can see,
// walks toward what it remembers, and holds. `hold` is last and
// unconditional, so the table always has an answer and a creature with
// nothing to do does nothing rather than nothing-in-particular.
//
// There is no `attacked` trigger to go with the `attacked` condition: what a
// creature does about being hit is decided when it next has time, not when
// the blow lands (design §2).
const generic = `time:
  - { when: { fled: { within: 3 } },      away: actor,     weight: 3 }
  - { when: { attacked: { within: 3 } },  attack: attacker, weight: 3 }
  - { when: { enemy: seen },              attack: enemy }
  - { when: { enemy: remembered },        toward: enemy }
  - { hold: {} }
`

// Default returns the rulebook's default table for a monster kind, as the
// contents of an `on:` mapping in the dungeonspec grammar — what
// dungeonspec's CompileTable takes. This is why a placement with no `on:`
// still fights.
//
// One generic table stands behind every ref this slice: a thug, a goblin and
// a skeleton all answer with it, because what told them apart before was a
// preset in Go and the slice that deletes the presets has not yet been given
// a reason to tell them apart again. The lookup takes a ref anyway, because
// that is the door a kind-specific table comes in through when a use case
// brings one — and the caller already holds the monster's ref, so the door
// costs it nothing to stand there.
//
// Never (nil, nil). A valid ref always gets a table. An invalid one is
// refused rather than answered: a caller asking for the table of nothing has
// a bug, and handing it the generic table would hide the bug behind a
// creature that fights.
func Default(ref core.Ref) (*DefaultTable, error) {
	if err := ref.IsValid(); err != nil {
		return nil, fmt.Errorf("no default table for %q: %w", ref.String(), err)
	}

	return &DefaultTable{Source: generic}, nil
}
