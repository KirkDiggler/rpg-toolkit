// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"fmt"
	"strings"
)

// Refusal is one problem [Recipe.Compile] found (rpg-project#997): its kind,
// where in the recipe it is, as data, and the sentence that says it.
// Compile still joins every refusal into one error; a caller finds each with
// errors.As over the joined error (errors.Join's Unwrap() []error), reads
// [Refusal.Where] to place it (a studio marks the step, a content compiler
// reports it at the step's own path), and tests its kind with errors.Is,
// which [Refusal.Unwrap] answers.
//
// The text is exactly what it always was: "<kind>: <where>: <message>".
type Refusal struct {
	// Where is the place in the recipe the refusal is about.
	Where Where
	// Kind is the sentinel the refusal is one of ([ErrUnknownBeat],
	// [ErrBadGuard], …).
	Kind error
	// Message is the sentence about this refusal, without kind or place.
	Message string
}

// Error is the refusal as it has always read: kind, place, sentence.
func (r *Refusal) Error() string {
	return fmt.Sprintf("%v: %s: %s", r.Kind, r.Where, r.Message)
}

// Unwrap is the refusal's kind, so errors.Is(err, ErrBadGuard) holds.
func (r *Refusal) Unwrap() error { return r.Kind }

// Where is a place in a recipe, as fields a caller builds its own path from
// without parsing text. Exactly one of three: a bag key ([Where.Bag]), the
// recipe's initial state ([Where.Initial]), or a state ([Where.State]) —
// the state itself when [Where.Transition] is -1, else one of its steps,
// and inside that step [Where.Leaf] when the refusal is about a guard node
// or an act. [Where.String] renders the path the refusal's text shows.
type Where struct {
	// Bag is the bag key a bag refusal is about; empty otherwise.
	Bag string
	// Initial is true for a refusal of the recipe's initial state.
	Initial bool
	// State is the state the refusal is about; empty for a bag or initial
	// refusal.
	State string
	// Transition is the step's index within State, or -1 for the state
	// itself, a bag key or the initial state.
	Transition int
	// Leaf is the place inside the step, in the recipe's own spelling:
	// "when", "when.all[0].fact", "when.not.scope", "do[1]". Empty for the
	// step (or the state) itself.
	Leaf string
}

// String is the path the refusal's text names: "bag.<key>", "initial",
// "states.<s>", "states.<s>.transitions[<k>]" and, inside a step,
// ".<leaf>".
func (w Where) String() string {
	switch {
	case w.Bag != "":
		return "bag." + w.Bag
	case w.Initial:
		return "initial"
	case w.Transition < 0:
		return "states." + w.State
	}
	s := fmt.Sprintf("states.%s.transitions[%d]", w.State, w.Transition)
	if w.Leaf != "" {
		s += "." + w.Leaf
	}
	return s
}

// The constructors every refusal's place is made with: a place is never
// left to be spelled by hand, so none can be missing.

// atBag is a bag key.
func atBag(key string) Where { return Where{Bag: key, Transition: -1} }

// atInitial is the recipe's initial state.
func atInitial() Where { return Where{Initial: true, Transition: -1} }

// atState is a state itself.
func atState(name string) Where { return Where{State: name, Transition: -1} }

// atStep is one step of a state.
func atStep(name string, i int) Where { return Where{State: name, Transition: i} }

// in is a place inside the step, the suffix in the recipe's own spelling
// (".when", ".all", "[2]", ".fact", ".do[1]") appended to what is already
// there.
func (w Where) in(suffix string) Where {
	if w.Leaf == "" {
		suffix = strings.TrimPrefix(suffix, ".")
	}
	w.Leaf += suffix
	return w
}
