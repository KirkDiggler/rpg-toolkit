// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// interactions.go is the root `interactions:` key (rpg-project#922, Gate S2
// of #769): the entrance that remembers, authored as one native
// `machine.Recipe` per interaction and judged by the engine's own door
// ([encounter.CheckInteraction]) at the author's path.

// interactionShapes judges every root interaction and returns what an option
// may name of each, by id. A refused interaction is kept as a nil entry, so
// a line naming it is not refused a second time for the one defect.
func interactionShapes(s *SingleRoomSpec, add errSink) map[string]*interactionShape {
	if len(s.Interactions) == 0 {
		return nil
	}
	out := make(map[string]*interactionShape, len(s.Interactions))
	for _, id := range sortedKeys(s.Interactions) {
		out[id] = nil
		path := "interactions." + id
		in, ok := interactionInputOf(id, s.Interactions[id], path, add)
		if !ok {
			continue
		}
		checked, err := encounter.CheckInteraction(&in)
		if err != nil {
			add(path+".recipe", err.Error())
			continue
		}
		shape := &interactionShape{states: map[string]bool{}, beats: map[string]bool{}}
		for _, state := range checked.States {
			shape.states[state] = true
		}
		for _, beat := range checked.Beats {
			shape.beats[string(beat)] = true
		}
		out[id] = shape
	}
	return out
}

// interactionInputOf decodes one interaction's recipe with unknown members
// refused and its ref required, reporting a defect at path when it cannot.
func interactionInputOf(id string, spec InteractionSpec, path string, add errSink) (encounter.InteractionInput, bool) {
	if spec.Recipe == nil {
		add(path+".recipe", "an interaction is its recipe, and this names none")
		return encounter.InteractionInput{}, false
	}
	var recipe machine.Recipe
	if err := decodeBehaviorValue(spec.Recipe, &recipe); err != nil {
		add(path+".recipe", err.Error())
		return encounter.InteractionInput{}, false
	}
	if err := recipe.Ref.IsValid(); err != nil {
		add(path+".recipe.ref", err.Error())
		return encounter.InteractionInput{}, false
	}
	in := encounter.InteractionInput{ID: encounter.InteractionID(id), Recipe: recipe}
	if spec.Approach != nil {
		if spec.Approach.Within < 1 {
			add(path+".approach.within", "an approach is a distance of at least 1 cell")
			return encounter.InteractionInput{}, false
		}
		in.Approach = &encounter.InteractionApproach{Within: spec.Approach.Within}
	}
	return in, true
}

// interactionsOf lowers the root interactions into the field's, in id
// order; nil for none. Reached only after validation passed, so a recipe
// that cannot decode is not here to be dropped.
func interactionsOf(s *SingleRoomSpec) []encounter.InteractionInput {
	if len(s.Interactions) == 0 {
		return nil
	}
	out := make([]encounter.InteractionInput, 0, len(s.Interactions))
	for _, id := range sortedKeys(s.Interactions) {
		in, ok := interactionInputOf(id, s.Interactions[id], "", func(string, string) {})
		if ok {
			out = append(out, in)
		}
	}
	return out
}
