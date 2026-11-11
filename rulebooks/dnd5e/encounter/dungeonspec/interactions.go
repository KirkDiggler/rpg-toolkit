// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"

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
		shape := &interactionShape{states: map[string]bool{}, beats: map[string]bool{}, topics: map[string]bool{}}
		for _, topic := range checked.Topics {
			shape.topics[string(topic)] = true
		}
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
	outcomes, ok := outcomesOf(spec.Outcomes, &recipe, path+".outcomes", add)
	if !ok {
		return encounter.InteractionInput{}, false
	}
	in.Outcomes = outcomes
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

// outcomesOf judges and lowers an interaction's `outcomes` (rpg-project#928)
// at the author's own paths: every key a state of the recipe, every state
// naming at least one consequence, every consequence `recognize`, and every
// word of it one of the fixed ones. The engine judges the same again
// ([encounter.CheckInteraction]). Nil for none.
func outcomesOf(
	specs map[string][]OutcomeSpec, recipe *machine.Recipe, path string, add errSink,
) (map[string][]encounter.Consequence, bool) {
	if len(specs) == 0 {
		return nil, true
	}
	ok := true
	fail := func(at, message string) { ok = false; add(at, message) }
	out := make(map[string][]encounter.Consequence, len(specs))
	for _, state := range sortedKeys(specs) {
		at := path + "." + state
		if _, declared := recipe.States[state]; !declared {
			fail(at, fmt.Sprintf("%q is not a state of this interaction's recipe", state))
			continue
		}
		if len(specs[state]) == 0 {
			fail(at, "an outcome lists what follows entering the state: write at least one `recognize`")
			continue
		}
		for i, c := range specs[state] {
			itemAt := fmt.Sprintf("%s[%d]", at, i)
			r := c.Recognize
			if r == nil {
				fail(itemAt, "a consequence is `recognize: { topic, holder, about }`, the one kind there is")
				continue
			}
			if r.Topic == "" {
				fail(itemAt+".recognize.topic", "a recognition names the topic held")
			}
			if r.Holder != string(encounter.RecognitionSpokenTo) {
				fail(itemAt+".recognize.holder", fmt.Sprintf("the holder is `%s`, the creature the line was said to",
					encounter.RecognitionSpokenTo))
			}
			if r.About != string(encounter.RecognitionPartyInSight) {
				fail(itemAt+".recognize.about", fmt.Sprintf("the subjects are `%s`, the speaker's party the holder sees",
					encounter.RecognitionPartyInSight))
			}
			if r.HeardBy != "" && r.HeardBy != string(encounter.RecognitionCarriersWhoWitnessed) {
				fail(itemAt+".recognize.heard_by", fmt.Sprintf("the hearers are `%s`, or omit it for nobody",
					encounter.RecognitionCarriersWhoWitnessed))
			}
			out[state] = append(out[state], encounter.Consequence{Recognize: &encounter.Recognition{
				Topic: encounter.TopicID(r.Topic), Holder: encounter.RecognitionHolder(r.Holder),
				About: encounter.RecognitionSubjects(r.About), HeardBy: encounter.RecognitionHearers(r.HeardBy),
			}})
		}
	}
	return out, ok
}

// grantedTopicsOf is every topic a root interaction's `outcomes` grants, read
// straight off the document: the topics a post's `pass` may recognize on.
func grantedTopicsOf(interactions map[string]InteractionSpec) map[string]bool {
	out := map[string]bool{}
	for _, spec := range interactions {
		for _, cs := range spec.Outcomes {
			for _, c := range cs {
				if c.Recognize != nil && c.Recognize.Topic != "" {
					out[c.Recognize.Topic] = true
				}
			}
		}
	}
	return out
}
