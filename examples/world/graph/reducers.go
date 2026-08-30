// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph

import (
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

// Condition selects which outcomes a reducer folds.
//
// The zero value is [Always], so a reducer declared without one folds every
// fact of its kind — which is the right reading for a fact that was simply
// declared rather than attempted.
type Condition int

const (
	// Always folds the fact whatever its outcome, including facts that came
	// from no attempt at all.
	Always Condition = iota

	// Succeeded folds only a contested attempt that beat its difficulty. An
	// uncontested fact is not a success.
	Succeeded

	// Failed folds only a contested attempt that missed. An uncontested fact is
	// not a failure — this is why [journal.Outcome] carries Contested.
	Failed
)

func (c Condition) holds(o journal.Outcome) bool {
	switch c {
	case Succeeded:
		return o.Contested && o.Succeeded
	case Failed:
		return o.Contested && !o.Succeeded
	case Always:
		return true
	default:
		return true
	}
}

// Reducer folds one witnessed fact into derived cells.
//
// The interface is sealed: only declarations in this package implement it. That
// is the mechanism behind "authors declare structure and derivation, never
// methods" — there is no seam through which a path-specific fold could be
// smuggled in as content. The cost is real and deliberate: a derivation shape
// this package does not have is a change to this package.
type Reducer interface {
	reduce(f journal.Fact, s *State)
}

// Occupy declares that a fact puts its Actor into a role over its Subject.
//
// The same declaration serves a truth and a lie. A leader taking command and an
// impostor claiming to have taken command produce the same fact kind; all that
// differs is who witnessed it. That is why the changeling needs no code.
//
// The slot must have been declared. An Occupy naming a role no [Slot] covers is
// recorded in [State.Refusals] rather than quietly creating structure.
type Occupy struct {
	// On is the fact kind that moves the slot.
	On journal.Kind

	// When narrows which outcomes count.
	When Condition

	// Role is the slot the actor takes.
	Role Role
}

func (o Occupy) reduce(f journal.Fact, s *State) {
	if f.Kind != o.On || !o.When.holds(f.Outcome) {
		return
	}
	if f.Subject == "" {
		s.refuse("occupy %q: fact %d has no subject to hold the slot", o.Role, f.Seq)

		return
	}
	if !s.hasSlot(o.Role, f.Subject) {
		s.refuse("occupy %q: %q has no such slot declared", o.Role, f.Subject)

		return
	}
	s.setOccupant(o.Role, f.Subject, f.Actor)
}

// Vacate declares that a fact empties whatever slot of a role its Subject was
// holding.
//
// It is keyed on the occupant rather than the slot's owner because that is what
// the world hands you: the assassin's fact names the leader, not the camp. One
// declaration covers the leader being killed and the impostor being unmasked —
// in both cases somebody stops holding a role, and the fold does not care which
// happened.
type Vacate struct {
	// On is the fact kind that empties the slot.
	On journal.Kind

	// When narrows which outcomes count.
	When Condition

	// Role is the slot to empty.
	Role Role
}

func (v Vacate) reduce(f journal.Fact, s *State) {
	if f.Kind != v.On || !v.When.holds(f.Outcome) {
		return
	}
	if f.Subject == "" {
		s.refuse("vacate %q: fact %d has no subject to remove", v.Role, f.Seq)

		return
	}
	s.vacateHeldBy(v.Role, f.Subject)
}

// Count declares that a fact moves a counter its Subject holds toward the
// faction the Actor acts for.
//
// Pointing the counter at the actor's faction rather than the actor is what
// makes a run of conversations with one paladin add up to the camp's opinion of
// the whole party.
type Count struct {
	// On is the fact kind that moves the counter.
	On journal.Kind

	// When narrows which outcomes count — a botched parley is normally a
	// separate declaration with a negative By.
	When Condition

	// Into names the counter.
	Into Counter

	// By is how far to move it, signed.
	By int
}

func (c Count) reduce(f journal.Fact, s *State) {
	if f.Kind != c.On || !c.When.holds(f.Outcome) {
		return
	}
	if f.Subject == "" {
		s.refuse("count %q: fact %d has no subject to hold the counter", c.Into, f.Seq)

		return
	}
	s.addCount(c.Into, f.Subject, s.FactionOf(f.Actor), c.By)
}

// Raise declares that a fact sets a flag on its Subject.
//
// Flags only go up. There is no reducer that lowers one because unwitnessing
// something is not an event: a camp that has seen an assault has seen it, and
// the way back is a different fact raising a different flag.
type Raise struct {
	// On is the fact kind that raises the flag.
	On journal.Kind

	// When narrows which outcomes count.
	When Condition

	// Flag is what gets set on the subject.
	Flag Flag
}

func (r Raise) reduce(f journal.Fact, s *State) {
	if f.Kind != r.On || !r.When.holds(f.Outcome) {
		return
	}
	if f.Subject == "" {
		s.refuse("raise %q: fact %d has no subject to flag", r.Flag, f.Seq)

		return
	}
	s.raise(r.Flag, f.Subject)
}
