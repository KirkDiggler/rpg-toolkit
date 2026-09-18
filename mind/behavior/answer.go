// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

// answer.go is ONE ROW of a creature's table: how likely it is, when it is
// even on the table, what the creature says, and the one thing it does.
//
// THE WORDS ARE DATA HERE AND NOWHERE ELSE. This module knows that an entry
// carries at most one of them and which one fired; it has no idea what
// attacking IS, where `toward` walks to, or what a fact teaches. A rulebook
// executes them, and the sealed-per-trigger rule — which word is legal under
// which key — is the rulebook's too, because only it knows what its own
// triggers mean.

// Answer is ONE entry in a trigger's table: how likely it is, when it is even
// on the table, what the creature says, and the one thing it does.
//
// EXACTLY ONE OUTCOME WORD, OR NONE. An entry that both teaches a fact and
// sends the creature running is refused at the door rather than ordered here,
// so an author never has to guess which happens first (the design's rule of
// the table). An entry with neither is legal only when it has a line to say:
// a creature that answers and does nothing is a real outcome, and a creature
// that neither speaks nor acts is a row the author wrote for no reason.
//
// THE WORD IS LEGAL PER KEY (design §2). `fact` and `flee` answer a social
// verdict; `hold`, `attack`, `toward` and `away` are what a creature does
// with TIME. A `when` is a `time` word too — a social verdict is already the
// condition. the rulebook refuses the crossings by name.
type Answer struct {
	// Weight is this entry's share of the table. AT LEAST 1, refused
	// otherwise: a zero would be an entry that can never fire, sitting in a
	// file looking like a possibility. Weights are relative and loaded by
	// the creature's temperament before they are summed, so 3 and 1 and 75
	// and 25 both mean the same thing to a soldier.
	//
	// The authoring dialect fills an omitted weight with 1 before it gets
	// here, so every entry that reaches this composition carries its own
	// number and nothing down here has to know what "omitted" meant.
	Weight int

	// Say is the creature's line, VERBATIM. The composition never composes
	// it, never templates it and never translates it — it is carried onto
	// the caller's own beat exactly as the author typed it. Empty means the creature
	// says nothing, which is the common case for a table that only turns a
	// disposition.
	Say string

	// When is the condition this entry is on the table under, or nil for a
	// standing order. See [When] — an entry whose condition does not hold is
	// not a candidate for this roll at all.
	When *When

	// Fact is the world fact every witness learns when this entry fires —
	// what `on: { intimidated: { fact: … } }` used to be, now one entry of
	// one outcome's table. Empty means this entry teaches nothing.
	//
	// THE PLAY RECORD AND THE WORLD JOURNAL ARE BOTH WRITTEN AND NEITHER IS
	// THE OTHER'S CACHE (Kirk, 2026-09-16). The deed the verb landed is the
	// creature's own memory of who leaned on it and goes with the run; this
	// is what the camp comes to know and carries out of it.
	Fact string

	// Flee lands [VerbFled] on this creature, naming whoever just spoke to
	// it, at now — AND NOTHING ELSE.
	//
	// IT DOES NOT STEP. The one-shot directed walk this used to do is gone
	// (rpg-project#465, design §2): the verb that scared the creature pays a
	// round on the world clock, the creature's own `time` table reads
	// `fled: { within: N }`, and the running is `away: actor` on that table
	// — for as many rounds as the author wrote. A creature that keeps
	// running while the party walks after it is what the one-shot could
	// never express.
	Flee bool

	// Hold is `hold`: this creature does nothing with its time. The `time`
	// key's a pass.
	Hold bool

	// Attack strikes the selected member. REFUSED OFF THE TURN CLOCK
	// (the caller): an enemy in reach on the world clock is a fight
	// sight already formed, so a table that reaches this outside a bubble is
	// describing a world that cannot happen.
	Attack *Selector

	// Toward walks toward the selected member's BELIEVED position, or an
	// authored cell, for this turn's movement.
	Toward *Selector

	// Away walks away from the selected member, this turn's movement — the
	// coward's run. Command's and Dissonant Whispers' compelled walks keep
	// their own engine-driven its own router path and do not pass through the
	// table.
	Away *Selector
}
