// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Refusal is one problem [Recipe.Compile] found (rpg-project#997): its kind,
// where in the recipe it is, as data, and the sentence that says it.
// Compile joins every refusal into one error; [Refusals] gets them all back,
// in the order the error's text lists them, and a caller reads
// [Refusal.Where] to place each (a studio marks the step, a content
// compiler reports it at the step's own path) and tests its kind with
// errors.Is, which [Refusal.Unwrap] answers.
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

// Refusals is every [*Refusal] in err, however it is joined or wrapped, in
// the order the error's text lists them (the order Compile found them).
// Nil, or an error holding no refusal, gives none.
func Refusals(err error) []*Refusal {
	var out []*Refusal
	var walk func(error)
	walk = func(e error) {
		switch v := e.(type) {
		case nil:
		case *Refusal:
			out = append(out, v)
		case interface{ Unwrap() []error }:
			for _, kid := range v.Unwrap() {
				walk(kid)
			}
		default:
			walk(errors.Unwrap(e))
		}
	}
	walk(err)
	return out
}

// PlaceKind is what a [Where] names: a bag key, the recipe's initial state,
// a state, or one step of a state. The zero value is [PlaceNone]: no place.
type PlaceKind int

const (
	// PlaceNone is no place at all: the zero [Where]. Compile never refuses
	// at it.
	PlaceNone PlaceKind = iota
	// PlaceBag is a key of the recipe's bag ([Where.Bag]).
	PlaceBag
	// PlaceInitial is the recipe's initial state.
	PlaceInitial
	// PlaceState is a state itself ([Where.State]).
	PlaceState
	// PlaceStep is one step of a state ([Where.State], [Where.Transition]),
	// and inside it [Where.Leaf] when the refusal is about a guard node or
	// an act.
	PlaceStep
)

// Where is a place in a recipe, as data a caller builds its own path from
// without parsing text. What it names is [Where.Kind], set only by the
// package's own constructors: a Where built anywhere else (the zero value
// among them) is [PlaceNone] and renders as no path. The fields a kind does
// not use are empty: Bag only for [PlaceBag]; State for [PlaceState] and
// [PlaceStep]; Transition and Leaf only for [PlaceStep].
type Where struct {
	kind PlaceKind
	// Bag is the bag key, for a [PlaceBag].
	Bag string
	// State is the state, for a [PlaceState] or a [PlaceStep].
	State string
	// Transition is the step's index within State, for a [PlaceStep].
	Transition int
	// Leaf is the place inside the step, for a [PlaceStep], as segments
	// in the recipe's own spelling: {when} {all 1} {not} {flag}, or {do 1}.
	// Empty for the step itself.
	Leaf []Segment
}

// Segment is one key of a place inside a step: a name, and an index when
// the key is a list ("all[1]", "do[2]").
type Segment struct {
	// Name is the key as the recipe spells it: "when", "all", "any",
	// "not", "fact", "flag", "scope", "rounds", "do".
	Name string
	// Index is the list position, when Indexed.
	Index int
	// Indexed is true for a key that names one entry of a list.
	Indexed bool
}

// String is the segment as the path spells it: "when", "all[1]".
func (s Segment) String() string {
	if s.Indexed {
		return fmt.Sprintf("%s[%d]", s.Name, s.Index)
	}
	return s.Name
}

// Kind is what the place names; [PlaceNone] for a Where the package did not
// make.
func (w Where) Kind() PlaceKind { return w.kind }

// String is the path the refusal's text names: "bag.<key>", "initial",
// "states.<s>", "states.<s>.transitions[<k>]" and, inside a step,
// ".<leaf>" ("states.s.transitions[0].when.all[1].not.flag"). No place
// renders "(no place)", which is no path.
func (w Where) String() string {
	switch w.kind {
	case PlaceBag:
		return "bag." + w.Bag
	case PlaceInitial:
		return "initial"
	case PlaceState:
		return "states." + w.State
	case PlaceStep:
		s := fmt.Sprintf("states.%s.transitions[%d]", w.State, w.Transition)
		if len(w.Leaf) > 0 {
			parts := make([]string, len(w.Leaf))
			for i, seg := range w.Leaf {
				parts[i] = seg.String()
			}
			s += "." + strings.Join(parts, ".")
		}
		return s
	}
	return "(no place)"
}

// The constructors every refusal's place is made with, the only code that
// sets a place's kind.

// atBag is a bag key.
func atBag(key string) Where { return Where{kind: PlaceBag, Bag: key} }

// atInitial is the recipe's initial state.
func atInitial() Where { return Where{kind: PlaceInitial} }

// atState is a state itself.
func atState(name string) Where { return Where{kind: PlaceState, State: name} }

// atStep is one step of a state.
func atStep(name string, i int) Where { return Where{kind: PlaceStep, State: name, Transition: i} }

// in is a key inside the step, appended to what is already there.
func (w Where) in(name string) Where {
	w.Leaf = append(slices.Clone(w.Leaf), Segment{Name: name})
	return w
}

// inAt is one entry of a list inside the step ("all[1]", "do[2]").
func (w Where) inAt(name string, i int) Where {
	w.Leaf = append(slices.Clone(w.Leaf), Segment{Name: name, Index: i, Indexed: true})
	return w
}
