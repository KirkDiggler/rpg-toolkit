// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// step_refusals.go is A REFUSED STEP IS REPORTED AT ITS OWN PATH
// (rpg-project#997, #1001 item 2): a machine's refusal ([machine.Refusals])
// and the encounter's own ([encounter.StepRefusals]) carry the state, the
// step and the leaf inside it as data, and a content compiler reports each
// at the place the author wrote it — a native recipe's
// `behaviorDefinitions.<id>.recipe.states.<S>.transitions[<k>]…`, a states
// block's own keys — not at the creature that uses it. The sentence is the
// refusal's own, unchanged.

// placedRefusal is one refusal of a step, with where it is in the machine.
type placedRefusal struct {
	kind       machine.PlaceKind
	bag, state string
	transition int
	leaf       []machine.Segment
	message    string
}

// placedRefusals is every refusal in err that says where it is: the
// machine's, by [machine.Where.Kind] (a place it did not make is skipped),
// and the encounter's own steps. None when err holds neither, which leaves
// the caller to report err where it always did.
func placedRefusals(err error) []placedRefusal {
	var out []placedRefusal
	for _, r := range machine.Refusals(err) {
		w := r.Where
		if w.Kind() == machine.PlaceNone {
			continue
		}
		out = append(out, placedRefusal{kind: w.Kind(), bag: w.Bag, state: w.State, transition: w.Transition, leaf: w.Leaf, message: r.Message})
	}
	for _, r := range encounter.StepRefusals(err) {
		out = append(out, placedRefusal{kind: machine.PlaceStep, state: r.State, transition: r.Transition, leaf: r.Leaf, message: r.Message})
	}
	return out
}

// leafPath is a leaf as a native recipe's author wrote it: the machine's own
// keys, so `.when.all[1].fact`.
func leafPath(leaf []machine.Segment) string {
	var b strings.Builder
	for _, s := range leaf {
		b.WriteString(".")
		b.WriteString(s.String())
	}
	return b.String()
}

// recipePath is where a refusal is in a native recipe at base
// (`behaviorDefinitions.<id>.recipe`).
func (r placedRefusal) recipePath(base string) string {
	switch r.kind {
	case machine.PlaceBag:
		return base + ".bag." + r.bag
	case machine.PlaceInitial:
		return base + ".initial"
	case machine.PlaceState:
		return base + ".states." + r.state
	}
	return fmt.Sprintf("%s.states.%s.transitions[%d]%s", base, r.state, r.transition, leafPath(r.leaf))
}

// recipeRefusals is err's placed refusals as field errors in the native
// recipe at base; nil when err places none.
func recipeRefusals(err error, base string) []FieldError {
	var out []FieldError
	for _, r := range placedRefusals(err) {
		out = append(out, FieldError{Path: r.recipePath(base), Message: r.message})
	}
	return out
}

// statesPath is where a refusal is in a states block at base (`….states`):
// the dialect compiles a step's `within`/`regions`, `rounds` and `when`
// into one guard, wrapping them in an `all` when there are several, so a
// leaf inside that guard is read back to the key the author wrote.
func (r placedRefusal) statesPath(base string, s *StatesSpec) string {
	switch r.kind {
	case machine.PlaceInitial:
		return base + ".initial"
	case machine.PlaceState:
		return base + "." + r.state
	case machine.PlaceBag, machine.PlaceNone:
		return base
	}
	step := fmt.Sprintf("%s.%s.transitions[%d]", base, r.state, r.transition)
	st, ok := s.States[r.state]
	if !ok || r.transition < 0 || r.transition >= len(st.Transitions) || len(r.leaf) == 0 {
		return step
	}
	t := st.Transitions[r.transition]
	if r.leaf[0].Name != "when" {
		return step + leafPath(r.leaf) // an act: `do[j]` as written
	}
	// The parts the dialect compiled, in its order ([recipeOf]).
	var parts []string
	if t.Within > 0 || t.Regions > 0 {
		parts = append(parts, "scope")
	}
	if t.Rounds > 0 {
		parts = append(parts, "rounds")
	}
	if t.When != nil {
		parts = append(parts, "when")
	}
	rest := r.leaf[1:]
	part := ""
	switch {
	case len(parts) > 1 && len(rest) > 0 && rest[0].Name == "all" && rest[0].Indexed && rest[0].Index < len(parts):
		part, rest = parts[rest[0].Index], rest[1:]
	case len(parts) == 1:
		part = parts[0]
	default:
		return step
	}
	switch part {
	case "scope":
		if t.Within > 0 {
			return step + ".within"
		}
		return step + ".regions"
	case "rounds":
		return step + ".rounds"
	}
	return step + whenPath(t.When, rest)
}

// whenPath is a leaf inside a state's `when` as the author wrote it: a `not`
// is `.not`, and the fact a `when` reads is `.enemy` or `.fact`.
func whenPath(w *WhenSpec, rest []machine.Segment) string {
	path := ".when"
	for _, s := range rest {
		switch {
		case s.Name == "not" && w != nil && w.Not != nil:
			path += ".not"
			w = w.Not
		case s.Name == "fact" && w != nil && w.Enemy != "":
			return path + ".enemy"
		case s.Name == "fact" && w != nil:
			return path + ".fact"
		default:
			return path
		}
	}
	return path
}
