// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"fmt"
	"slices"
)

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
// verdict; `hold`, `attack`, `toward`, `away` and `search` are what a
// creature does with TIME. A `when` is a `time` word too — a social verdict is already the
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

	// When is the list of conditions this entry is on the table under, and
	// EVERY ONE MUST HOLD (R8 of rpg-project#569); empty is a standing order.
	// See [When] — an entry whose conditions do not all hold is not a
	// candidate for this roll at all.
	//
	// A LIST BECAUSE ROWS ARE ROLLED, NOT ORDERED. "Storm the keep once the
	// guards have turned, but not while a loyalist is in sight" cannot be
	// said by putting one row above another; it is two conditions on one
	// row — `[{ fact: guards-turned }, { not: { enemy: seen } }]` — and each
	// When in the list still names exactly one thing.
	When []When

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

	// Search walks on from a LOST trail ([EnemyLost]): the cell the creature
	// last saw its quarry on, out along the heading the row names, for
	// Distance cells (rpg-project#668 R4, rpg-project#679 R10). Where that
	// cell is, what each heading reads from the creature's testimony and
	// when the search is over are the caller's; this module carries the
	// number and the word.
	//
	// OMITTED WHEN NIL, so a row that does not search marshals exactly as it
	// did before the word existed and no compiled table moves.
	Search *Search `json:",omitempty"`

	// Rows makes this row a GROUP: its answer is a table, rolled when the
	// group is picked (rpg-project#670, R12). A group is a candidate when its
	// own When all hold and at least one of its rows is a candidate (R15); it
	// loads its one Weight × the percent of [Answer.As], so a two-row branch
	// takes one share beside a one-row sibling, not two; and when the die
	// lands on it, a second die rolls over its own candidates ([Pick]).
	//
	// A GROUP CARRIES NO WORD. Its answer is its rows, so a group row sets no
	// Hold, Attack, Toward, Away, Flee, Search, Say or Fact, and holds at
	// least one row ([Answer.Validate], ErrBadGroup). Groups nest, to
	// [MaxGroupDepth] (R14).
	//
	// THIS MODULE HOLDS THE TREE, NEVER A NAME. A bundle is the dialect's
	// word; by the time a table reaches [Pick] every `use:` has been resolved
	// into this arm with its parameters substituted, and nothing here looks
	// a bundle up.
	//
	// Omitted when empty, with As, From and Params, so a word row marshals
	// exactly as it did before groups existed.
	Rows []Answer `json:",omitempty"`

	// As is the temperament word a group counts as, one of [AsWords], or
	// empty for none, which loads at 100 (R13). A group has no word of its
	// own, so a coward's pursuer is whatever its author says it is: `as:
	// toward` loads it at the coward's toward percent. Only a group carries
	// it.
	As string `json:",omitempty"`

	// From is the bundle a group was compiled from, FOR THE RECORD ONLY: the
	// table beat and the panel name the group by it. Nothing here reads it.
	From string `json:",omitempty"`

	// Params is the parameters a group's bundle was used with, as written,
	// FOR THE RECORD ONLY: they were substituted into Rows at compile.
	// Nothing here reads them.
	Params map[string]string `json:",omitempty"`
}

// MaxGroupDepth is how deep groups nest: a group inside a group inside a
// group, to this many levels (R14: unbounded in principle, refused only by
// this guard). [Answer.Validate] refuses a deeper one, and [Pick], which
// never validates, treats a group past it as absent, so a table built in Go
// that holds itself is a dead row rather than a recursion without end.
const MaxGroupDepth = 8

// AsWords is every word a group's [Answer.As] may name: the five words a
// [TemperProfile] carries a percent for. Exported for the dialect's refusal.
var AsWords = []string{"attack", "toward", "away", "flee", "hold"}

// Validate refuses what this module's own types cannot carry: a [When] in the
// list that does not name exactly one condition ([When.Validate],
// ErrBadWhen), and a [Selector] naming no target or more than one, a `path`
// on a word other than `toward`, `sound` on `attack`, or `sound` on a row
// whose list names no `heard` condition (ErrBadSelector, R10), and a `search`
// distance below 1 or a heading not in [SearchHeadings] (ErrBadSearch), and
// a group row ([Answer.Rows]) that carries a word, a search, a line or a fact
// of its own, holds no rows, counts `as` a word not in [AsWords], or nests
// past [MaxGroupDepth] (ErrBadGroup). Every row inside a group is validated
// as a row, and its refusal names which row of the group it is. Each refusal
// is a sentence for the form, wrapping its sentinel.
//
// THE REST IS THE CALLER'S DOOR, as it always was: the weight, the
// one-outcome-word rule among a word row's words, which word is legal under
// which key, and which enemy,
// deed and selector words this build reads are refused by the dialect that
// decoded the file, which calls this first.
func (a *Answer) Validate() error {
	return a.validate(0)
}

// validate is [Answer.Validate] for a row nested inside `nest` groups.
func (a *Answer) validate(nest int) error {
	if a.isGroup() {
		return a.validateGroup(nest)
	}
	for i := range a.When {
		if err := a.When[i].Validate(); err != nil {
			return err
		}
	}
	if a.Search != nil && a.Search.Distance < 1 {
		return fmt.Errorf("`search` distance %d: a search walks at least one cell: %w",
			a.Search.Distance, ErrBadSearch)
	}
	if a.Search != nil && !a.Search.Heading.sealed() {
		return fmt.Errorf("`search` heading %q: a search follows went, faced, left or right: %w",
			a.Search.Heading, ErrBadSearch)
	}
	for _, slot := range []struct {
		word string
		sel  *Selector
	}{{"attack", a.Attack}, {"toward", a.Toward}, {"away", a.Away}} {
		if slot.sel == nil {
			continue
		}
		if err := slot.sel.validate(slot.word); err != nil {
			return err
		}
		if slot.sel.Word == SelectorSound && !a.listens() {
			return fmt.Errorf("`sound` needs a `heard` condition on the row: "+
				"the sound it walks to is the one the row listened for: %w", ErrBadSelector)
		}
	}

	return nil
}

// isGroup reports whether this row is a group: it holds rows, or carries
// something only a group carries. A row with `as` and no rows is a group
// with no rows, refused as one, not a word row with a stray field.
func (a *Answer) isGroup() bool {
	return a.Rows != nil || a.As != "" || a.From != "" || a.Params != nil
}

// validateGroup is the group arm's door: no word of its own, at least one
// row, `as` a temperament word or nothing, no deeper than [MaxGroupDepth],
// its own When, and every row inside, each refusal naming the row it is in.
func (a *Answer) validateGroup(nest int) error {
	if nest+1 > MaxGroupDepth {
		return fmt.Errorf("a group nested %d deep; groups nest at most %d deep: %w",
			nest+1, MaxGroupDepth, ErrBadGroup)
	}
	if carried := a.words(); len(carried) > 0 {
		return fmt.Errorf("a group carries %s; a group's answer is its rows, so the word goes on a row "+
			"inside it: %w", andList(carried), ErrBadGroup)
	}
	if len(a.Rows) == 0 {
		return fmt.Errorf("a group holds no rows; a group is a table, and a table with no rows can never "+
			"be picked: %w", ErrBadGroup)
	}
	if a.As != "" && !slices.Contains(AsWords, a.As) {
		return fmt.Errorf("a group counts `as` %q; it counts as one of attack, toward, away, flee or hold, "+
			"or as nothing: %w", a.As, ErrBadGroup)
	}
	for i := range a.When {
		if err := a.When[i].Validate(); err != nil {
			return err
		}
	}
	for i := range a.Rows {
		if err := a.Rows[i].validate(nest + 1); err != nil {
			return fmt.Errorf("row %d of the group: %w", i+1, err)
		}
	}

	return nil
}

// words names every word, search, line and fact this row carries, as an
// author writes each.
func (a *Answer) words() []string {
	var out []string
	for _, w := range []struct {
		name string
		set  bool
	}{
		{"`hold`", a.Hold}, {"`attack`", a.Attack != nil}, {"`toward`", a.Toward != nil},
		{"`away`", a.Away != nil}, {"`flee`", a.Flee}, {"`search`", a.Search != nil},
		{"`say`", a.Say != ""}, {"`fact`", a.Fact != ""},
	} {
		if w.set {
			out = append(out, w.name)
		}
	}

	return out
}

// Search is `search: { distance: N, heading: <word> }`: how far on from a
// lost trail the creature walks before it gives up (rpg-project#668 R4), and
// which way it walks (rpg-project#679 R10).
//
// A NUMBER AND A WORD, NOTHING ELSE. The trail, the bearing each heading word
// reads and the cell N on are the caller's to derive from what the creature
// holds, each time the row fires — the search holds no state, as a route
// holds no state.
type Search struct {
	// Distance is how many cells on the search walks. AT LEAST 1, refused
	// otherwise. The dialect fills an omitted distance before it gets here,
	// so nothing down here has to know what "omitted" meant.
	Distance int

	// Heading is which way the search walks: one of [SearchHeadings],
	// refused otherwise. The dialect fills an omitted heading with
	// [HeadingWent] before it gets here, so the persisted row always names
	// its word and a reloaded table never holds a wordless search.
	Heading SearchHeading
}

// SearchHeading is the sealed word a `search` names for its way on
// (rpg-project#679 R10): the row's choice of heading, not the engine's.
type SearchHeading string

const (
	// HeadingWent follows the way the quarry WENT: the bearing of the last
	// step the creature saw it take. The dialect's default.
	HeadingWent SearchHeading = "went"

	// HeadingFaced follows the way the quarry FACED when last seen: the
	// behaviour every search had before the word existed.
	HeadingFaced SearchHeading = "faced"

	// HeadingLeft turns a quarter left off the held heading (went when it
	// was seen, faced otherwise). A table that wants a guess writes a left
	// row and a right row and lets the roll pick.
	HeadingLeft SearchHeading = "left"

	// HeadingRight turns a quarter right off the held heading, the mirror of
	// [HeadingLeft].
	HeadingRight SearchHeading = "right"
)

// SearchHeadings is every value `search: { heading: }` takes. Exported for
// the dialect's refusal, [AnswerKeys]'s reason.
var SearchHeadings = []SearchHeading{HeadingWent, HeadingFaced, HeadingLeft, HeadingRight}

// sealed reports whether h is one of [SearchHeadings].
func (h SearchHeading) sealed() bool {
	for _, w := range SearchHeadings {
		if h == w {
			return true
		}
	}

	return false
}

// listens reports whether this row's `when` list names a `heard` condition
// at its top level. A `heard` inside a `not` does not count: that row holds
// when the creature heard NOTHING, so there is no sound for it to walk to.
func (a *Answer) listens() bool {
	for i := range a.When {
		if a.When[i].Heard != nil {
			return true
		}
	}

	return false
}
