// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/table"
)

// creaturetable.go is where the three layers of a creature's policy meet
// (rpg-project#465, ideas/creature-table/design.md §1).
//
// The author's two — the faction's orders and the placement's — are laid by
// the compiler and reach this seam as one table. The rulebook's default for
// the monster's KIND is the third, and it goes UNDERNEATH: this is the only
// place in the stack that holds both a monster's ref and the compiler that
// reads the rulebook's own grammar, which is why the fold happens here rather
// than in either of them.
//
// Nothing here decides anything. What a creature does was written by an
// author; what its kind does by default is rulebook content; what its
// temperament means is rulebook content. This file resolves three lookups and
// refuses loudly when one of them fails.

// foldedTable is the whole table one spawned monster fights from: the
// rulebook's default for its kind with the author's orders laid over, key by
// key, the nearer layer winning wholesale.
//
// THE DEFAULT IS WHY A PLACEMENT WITH NO `on:` STILL FIGHTS. Before the table,
// that creature was driven by a preset named on its sheet; now it is driven by
// its kind's own shipped table, which an author can read and override one key
// at a time.
//
// OVERRIDING A KEY COSTS THE ENTRIES UNDER IT, and that cost is deliberate
// (design §1): there is no merging of entry lists, so an author never has to
// reason about what was added to what. A placement that writes `time:` to walk
// a thug to the front room has given up the default's `time:` entries for
// fighting, and the file shows it.
//
// A REF THE RULEBOOK CANNOT ANSWER FOR REFUSES THE SPAWN. So does a default
// table that will not compile. Both are this repository's own bug rather than
// an author's, and a creature placed with no policy at all would stand there
// looking like a design choice.
func foldedTable(ref *core.Ref, authored encounter.Table) (encounter.Table, error) {
	// A SHEET WITH NO REF IS THIS PACKAGE'S OWN BUG, not an author's: a
	// monster is instantiated FROM a ref, so one that reached here without
	// keeping it could not be asked what kind it is. Refused by name rather
	// than dereferenced, and refused rather than defaulted to the generic
	// table — a creature fighting from a default nobody could trace back to a
	// kind is the silent degrade this slice exists to avoid.
	if ref == nil {
		return nil, fmt.Errorf("default table: the sheet names no ref: %w", ErrBadRef)
	}

	def, err := table.Default(*ref)
	if err != nil {
		return nil, fmt.Errorf("default table: %w", err)
	}

	base, err := dungeonspec.CompileTable(def.Source)
	if err != nil {
		return nil, fmt.Errorf("default table for %q: %w", ref.String(), err)
	}

	return encounter.Layer(base, authored), nil
}

// resolvedTemper fills in what a temperament WORD means, from the rulebook
// that ships the numbers, and hands the composition a temperament it can
// actually multiply by.
//
// THE COMPOSITION CANNOT LOOK THIS UP ITSELF. A profile is content — the
// design's §3 table, in percent, tuned by a walk — and it lives in
// rulebooks/dnd5e beside the default tables. The composition may not import a
// rulebook (its C1), so the caller that stands between the two fills the
// numbers in. That caller is this seam.
//
// THREE SHAPES IN, AND THEY ARE NOT THREE CASES OF ONE THING:
//
//   - An authored WORD: the profile is looked up now and the creature is that
//     temperament from its first turn.
//   - A faction's MIX: every word in it gets its profile, and the composition
//     deals one at Join through the world's own dice with the faction as the
//     die's entity. Dealing here instead would put a roll in a seam that owns
//     no dice and write no beat for it, and "which goblin came out the coward"
//     is a thing the streamer is meant to see.
//   - NOTHING: nothing. The zero temperament is a soldier — every word at 100
//     — so a monster nobody gave one and a monster authored `temper: soldier`
//     are one creature rather than two states that have to agree.
//
// AN UNKNOWN WORD REFUSES, in a mix exactly as on a placement. Answering
// "solider" with a soldier's profile is the silent degrade this whole slice
// exists to avoid: the placement would play perfectly well and nobody would
// ever learn the word never landed.
func resolvedTemper(in encounter.Temper) (encounter.Temper, error) {
	// A MIX AND A WORD CANNOT BOTH BE ANSWERED, and the word wins by never
	// reaching here with a mix beside it: the compiler resolves a placement's
	// own word before its faction's mix is ever consulted. Checked in the
	// order the compiler resolves them, so the two cannot disagree.
	if in.Word != "" {
		profile, err := profileOf(in.Word)
		if err != nil {
			return encounter.Temper{}, err
		}

		return encounter.Temper{Word: in.Word, Profile: profile}, nil
	}

	if len(in.Mix) == 0 {
		return encounter.Temper{}, nil
	}

	profiles := make(map[string]encounter.TemperProfile, len(in.Mix))
	for word := range in.Mix {
		profile, err := profileOf(word)
		if err != nil {
			return encounter.Temper{}, fmt.Errorf("in the faction's mix: %w", err)
		}
		profiles[word] = profile
	}

	return encounter.Temper{Mix: in.Mix, Profiles: profiles}, nil
}

// profileOf is one word's weight profile, in the composition's own shape.
//
// THE EMPTY WORD NEVER REACHES IT. [resolvedTemper] answers "no temperament"
// before asking, because the rulebook's parser deliberately refuses the empty
// string: an author who wrote nothing never named a temperament, and a parser
// that answered "soldier" to "" could not tell them apart from one who wrote
// "solider".
func profileOf(word string) (encounter.TemperProfile, error) {
	parsed, err := table.ParseTemper(word)
	if err != nil {
		return encounter.TemperProfile{}, fmt.Errorf("temper: %w", err)
	}

	profile, err := parsed.Profile()
	if err != nil {
		return encounter.TemperProfile{}, fmt.Errorf("temper: %w", err)
	}

	return encounter.TemperProfile{
		Attack: profile.Attack,
		Toward: profile.Toward,
		Away:   profile.Away,
		Flee:   profile.Flee,
		Hold:   profile.Hold,
	}, nil
}
