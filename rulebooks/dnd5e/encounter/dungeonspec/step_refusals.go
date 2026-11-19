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
