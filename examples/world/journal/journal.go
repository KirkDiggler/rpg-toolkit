// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package journal

import (
	"errors"
	"slices"
)

// EntityID names something in the world. The journal never resolves one; it
// only records which ids took part in a fact and which ids saw it happen.
type EntityID string

// Kind names what happened — "assault", "infiltration", "unmasked". Content
// declares its own kinds; the journal treats every kind the same way.
type Kind string

// Approach names how an actor went about an attempt — a skill, a tactic, a
// spell. It is an opaque string here on purpose: the journal must not know
// that "stealth" is a D&D 5e skill, only that the resolver was handed it.
type Approach string

// ErrEmptyKind reports a fact appended without a Kind. A kind-less fact can
// never be folded by any declared derivation, so it is refused rather than
// silently recorded and ignored.
var ErrEmptyKind = errors.New("journal: fact has no kind")

// ErrEmptyActor reports a fact appended without an Actor. Attribution is not
// optional: an unattributed fact would produce derived state no one is
// answerable for.
var ErrEmptyActor = errors.New("journal: fact has no actor")

// Audience is the set of entities that witnessed a fact.
//
// An empty audience is a real and useful value: it is the quiet kill, the
// unseen entry. The fact happened and the log holds it, but no one's fold
// will ever reach it, so the world behaves as if it did not.
type Audience []EntityID

// Includes reports whether any of the given ids witnessed the fact.
//
// Callers pass the observer plus every group the observer belongs to, which is
// how group-grain audiences reach their members: a fact witnessed by the camp
// is witnessed by everyone in it.
func (a Audience) Includes(ids ...EntityID) bool {
	for _, id := range ids {
		if slices.Contains(a, id) {
			return true
		}
	}
	return false
}

// Outcome is what the composer's resolver decided about an attempt.
//
// It is deliberately thin. Margin is how far past (or short of) the difficulty
// the attempt landed, and Detail is free text for a transcript. Neither is
// interpreted by anything in this module — a fold may branch on Succeeded and
// nothing else.
type Outcome struct {
	// Contested reports whether this fact came from an attempt at all.
	// Declared facts — the ones content states as simply true — leave it
	// false, and Succeeded is then meaningless rather than accidentally false.
	Contested bool

	// Succeeded reports whether a contested attempt beat its difficulty.
	Succeeded bool

	// Margin is the signed distance from the difficulty.
	Margin int

	// Detail carries a human-readable trace of how the outcome was reached.
	Detail string
}

// Fact is one thing that happened, attributed to an actor and scoped to an
// audience.
//
// Actor and Subject are the two positions every declared derivation folds on:
// the actor is who did it, the subject is who or what it was done to. A fact
// with no subject is legal — an actor may simply act.
type Fact struct {
	// Seq is the append position, assigned by [Journal.Append] starting at 1.
	// It is the world's only clock: folds run in Seq order.
	Seq int

	// Kind names what happened.
	Kind Kind

	// Actor is the attribution — who did this.
	Actor EntityID

	// Subject is who or what it was done to, if anyone.
	Subject EntityID

	// Audience is who witnessed it.
	Audience Audience

	// Outcome is how the attempt was resolved, if this fact came from one.
	Outcome Outcome
}

// Journal is the append-only log.
//
// It hands out copies and offers no way to edit or remove what it holds. The
// zero value is not usable; call [New].
type Journal struct {
	facts []Fact
}

// New returns an empty journal.
func New() *Journal {
	return &Journal{}
}

// Append records a fact and returns it with its Seq assigned.
//
// Returns [ErrEmptyKind] or [ErrEmptyActor] for a fact that no fold could ever
// use. The journal is otherwise incurious: it does not check that the entities
// exist, that the audience is plausible, or that the fact is true.
func (j *Journal) Append(f Fact) (Fact, error) {
	if f.Kind == "" {
		return Fact{}, ErrEmptyKind
	}
	if f.Actor == "" {
		return Fact{}, ErrEmptyActor
	}

	f.Seq = len(j.facts) + 1
	f.Audience = slices.Clone(f.Audience)
	j.facts = append(j.facts, f)

	return j.copyOf(len(j.facts) - 1), nil
}

// Len returns how many facts have been recorded.
func (j *Journal) Len() int {
	return len(j.facts)
}

// All returns every fact in append order.
//
// The result is a copy down to each fact's audience, so a caller holding it
// cannot reach back into the log.
func (j *Journal) All() []Fact {
	out := make([]Fact, len(j.facts))
	for i := range j.facts {
		out[i] = j.copyOf(i)
	}

	return out
}

// WitnessedBy returns, in append order, every fact whose audience includes at
// least one of the given ids.
//
// Passing no ids returns nothing, which is the honest answer: an observer that
// is nobody witnessed nothing. This is the whole of the knowledge model — one
// filter, applied before the fold.
func (j *Journal) WitnessedBy(ids ...EntityID) []Fact {
	var out []Fact
	for i := range j.facts {
		if j.facts[i].Audience.Includes(ids...) {
			out = append(out, j.copyOf(i))
		}
	}

	return out
}

func (j *Journal) copyOf(i int) Fact {
	f := j.facts[i]
	f.Audience = slices.Clone(f.Audience)

	return f
}
