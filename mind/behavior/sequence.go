// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// ErrBadSequence reports invalid source, values, references or emitted data.
// It wraps the underlying parameter/compiler error when one is available.
var ErrBadSequence = errors.New("behavior: invalid sequence")

// SequenceEnd determines the final item's explicit $next destination.
// It never advances progress by itself.
type SequenceEnd string

const (
	// SequenceRepeat makes the final $next point to the first item's initial state.
	SequenceRepeat SequenceEnd = "repeat"
	// SequenceStop makes the final $next point to a compiler-owned terminal state.
	SequenceStop SequenceEnd = "stop"
)

// SequenceDefinition is one reusable, data-authored finite body. Body contains
// exactly recipe (without its own ref) and table, using their existing JSON
// shapes with whole-value $item/$param/$next/$done reference objects. The
// producer owns domain validation and defaults; this tool supplies neither.
type SequenceDefinition struct {
	Ref        core.Ref        `json:"ref"`
	Parameters []ParameterSpec `json:"parameters,omitzero"`
	End        SequenceEnd     `json:"end"`
	Body       jsontext.Value  `json:"body"`
}

// ExpandSequenceInput supplies one attachment's finite items, already chosen
// scalar values and vocabulary. Items are opaque JSON, not routes interpreted
// by this package. Empty items and an unspecified End are refused.
type ExpandSequenceInput struct {
	ID         string
	Definition SequenceDefinition
	Items      []jsontext.Value
	Parameters map[string]any
	Vocabulary machine.Vocabulary
}

// SequenceSource identifies an authored local name and its zero-based item.
// Compiler-owned terminal states have no authored source entry.
type SequenceSource struct {
	Item int
	Name string
}

// ExpandSequenceOutput is detached materialized configuration, never a running
// instance. Vocabulary includes private generated completion beats. A caller
// uses the selected table key to find its completion beat and addresses actual
// completion to the selected instance. No completion is raised during binding.
type ExpandSequenceOutput struct {
	Instance     InstanceSpec
	Vocabulary   machine.Vocabulary
	Completions  map[AnswerKey]machine.Beat
	StateSources map[string]SequenceSource
	TableSources map[AnswerKey]SequenceSource
}

type sequenceBody struct {
	Recipe machine.Recipe `json:"recipe"`
	Table  Table          `json:"table"`
}

const sequenceTerminal = "sequence/end"

// ExpandSequence copies a finite authored body into ordinary states/tables,
// qualifies its local references and validates the result with NewComposer.
// It rolls nothing, calls no fact readers and changes no inputs or runtime
// state. Nil input returns ErrNilInput; other refusals wrap ErrBadSequence.
func ExpandSequence(in *ExpandSequenceInput) (*ExpandSequenceOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	out, err := expandSequence(in)
	if err != nil {
		return nil, fmt.Errorf("%w: attachment %q: %w", ErrBadSequence, in.ID, err)
	}
	return out, nil
}

func expandSequence(in *ExpandSequenceInput) (*ExpandSequenceOutput, error) {
	if in.ID == "" {
		return nil, errors.New("missing attachment ID")
	}
	if _, err := core.ParseString(in.Definition.Ref.String()); err != nil {
		return nil, fmt.Errorf("definition ref: %w", err)
	}
	if in.Definition.End != SequenceRepeat && in.Definition.End != SequenceStop {
		return nil, errors.New("end must be repeat or stop")
	}
	if len(in.Items) == 0 {
		return nil, errors.New("items must be a nonempty finite list")
	}
	checked, err := CheckParameters(&CheckParametersInput{Schema: in.Definition.Parameters, Values: in.Parameters})
	if err != nil {
		return nil, err
	}
	parameters := make(map[string]jsontext.Value, len(checked.Values))
	for name, value := range checked.Values {
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return nil, marshalErr
		}
		parameters[name] = raw
	}
	template, err := readSequenceTemplate(in.Definition.Body)
	if err != nil {
		return nil, err
	}
	initials, err := sequenceInitials(in.Items, parameters, template.initial)
	if err != nil {
		return nil, err
	}
	out := newSequenceOutput(in)
	out.Instance.Recipe.Initial = sequenceName(0, initials[0])
	for i, item := range in.Items {
		next := sequenceTerminal
		if i+1 < len(initials) {
			next = sequenceName(i+1, initials[i+1])
		} else if in.Definition.End == SequenceRepeat {
			next = sequenceName(0, initials[0])
		}
		if err := expandSequenceItem(in.ID, i, item, next, parameters, template, out); err != nil {
			return nil, fmt.Errorf("item[%d]: %w", i, err)
		}
	}
	if in.Definition.End == SequenceStop {
		out.Instance.Recipe.States[sequenceTerminal] = machine.State{Terminal: true}
	}
	if _, err := NewComposer(&ComposerConfig{Instances: []InstanceSpec{out.Instance}, Vocabulary: out.Vocabulary}); err != nil {
		return nil, err
	}
	return out, nil
}

func newSequenceOutput(in *ExpandSequenceInput) *ExpandSequenceOutput {
	vocab := machine.Vocabulary{Beats: make(map[machine.Beat]machine.BeatSpec), Facts: maps.Clone(in.Vocabulary.Facts)}
	maps.Copy(vocab.Beats, in.Vocabulary.Beats)
	return &ExpandSequenceOutput{
		Instance: InstanceSpec{ID: in.ID, Recipe: machine.Recipe{Ref: in.Definition.Ref,
			States: make(map[string]machine.State), Bag: make(machine.Bag)}, Table: make(Table)},
		Vocabulary: vocab, Completions: make(map[AnswerKey]machine.Beat),
		StateSources: make(map[string]SequenceSource), TableSources: make(map[AnswerKey]SequenceSource),
	}
}

func sequenceInitials(items []jsontext.Value, parameters map[string]jsontext.Value, initial jsontext.Value) ([]string, error) {
	out := make([]string, len(items))
	for i, item := range items {
		var validated jsontext.Value
		if err := json.Unmarshal(item, &validated); err != nil {
			return nil, fmt.Errorf("item[%d]: %w", i, err)
		}
		r := sequenceResolver{item: validated, parameters: parameters}
		raw, err := r.value(initial, "body.recipe.initial")
		if err != nil {
			return nil, fmt.Errorf("item[%d]: %w", i, err)
		}
		if err := json.Unmarshal(raw, &out[i]); err != nil {
			return nil, fmt.Errorf("item[%d] initial: %w", i, err)
		}
		if out[i] == "" {
			return nil, fmt.Errorf("item[%d]: missing initial state", i)
		}
	}
	return out, nil
}

func sequenceName(item int, name string) string {
	return fmt.Sprintf("item/%d/%s", item, url.PathEscape(name))
}

func expandSequenceItem(id string, index int, item jsontext.Value, next string,
	parameters map[string]jsontext.Value, template *sequenceTemplate, out *ExpandSequenceOutput) error {
	done := make(map[string]string, len(template.tables))
	for _, name := range slices.Sorted(maps.Keys(template.tables)) {
		key := AnswerKey(sequenceName(index, name))
		beat := machine.Beat("sequence/" + url.PathEscape(id) + "/" + string(key))
		if _, exists := out.Vocabulary.Beats[beat]; exists {
			return fmt.Errorf("completion beat collision %q", beat)
		}
		done[name] = string(beat)
		out.Vocabulary.Beats[beat] = machine.BeatSpec{}
		out.Completions[key] = beat
	}
	r := sequenceResolver{item: item, parameters: parameters, next: next, done: done}
	raw, err := r.value(template.body, "body")
	if err != nil {
		return err
	}
	var body sequenceBody
	if err := json.Unmarshal(raw, &body, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	if _, exists := body.Recipe.States[body.Recipe.Initial]; !exists {
		return fmt.Errorf("initial %q is not a declared local state", body.Recipe.Initial)
	}
	for _, key := range slices.Sorted(maps.Keys(body.Recipe.Bag)) {
		out.Instance.Recipe.Bag[sequenceName(index, key)] = body.Recipe.Bag[key]
	}
	for _, name := range slices.Sorted(maps.Keys(body.Recipe.States)) {
		state := body.Recipe.States[name]
		if err := scopeSequenceState(&state, body, index, next); err != nil {
			return fmt.Errorf("state %q: %w", name, err)
		}
		key := sequenceName(index, name)
		out.Instance.Recipe.States[key] = state
		out.StateSources[key] = SequenceSource{Item: index, Name: name}
	}
	for _, name := range slices.Sorted(maps.Keys(body.Table)) {
		key := AnswerKey(sequenceName(index, string(name)))
		out.Instance.Table[key] = body.Table[name]
		out.TableSources[key] = SequenceSource{Item: index, Name: string(name)}
	}
	return nil
}

func scopeSequenceState(state *machine.State, body sequenceBody, index int, next string) error {
	for i := range state.Transitions {
		t := &state.Transitions[i]
		if t.To != "" {
			if _, local := body.Recipe.States[t.To]; local {
				t.To = sequenceName(index, t.To)
			} else if t.To != next {
				return fmt.Errorf("unknown local destination %q", t.To)
			}
		}
		if err := scopeSequenceGuard(t.When, body.Recipe.Bag, index); err != nil {
			return err
		}
		for j := range t.Do {
			a := &t.Do[j]
			if a.Pick != "" {
				if _, ok := body.Table[AnswerKey(a.Pick)]; !ok {
					return fmt.Errorf("unknown local table %q", a.Pick)
				}
				a.Pick = sequenceName(index, a.Pick)
			}
			if a.Set != nil {
				if err := scopeSequenceBagKey(&a.Set.Key, body.Recipe.Bag, index); err != nil {
					return err
				}
			}
			for k := range a.Clear {
				if err := scopeSequenceBagKey(&a.Clear[k], body.Recipe.Bag, index); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func scopeSequenceBagKey(key *string, bag machine.Bag, index int) error {
	if _, exists := bag[*key]; !exists {
		return fmt.Errorf("unknown local bag key %q", *key)
	}
	*key = sequenceName(index, *key)
	return nil
}

func scopeSequenceGuard(g *machine.Guard, bag machine.Bag, index int) error {
	if g == nil {
		return nil
	}
	if g.Flag != "" {
		if err := scopeSequenceBagKey(&g.Flag, bag, index); err != nil {
			return err
		}
	}
	if err := scopeSequenceGuard(g.Not, bag, index); err != nil {
		return err
	}
	for i := range g.All {
		if err := scopeSequenceGuard(&g.All[i], bag, index); err != nil {
			return err
		}
	}
	for i := range g.Any {
		if err := scopeSequenceGuard(&g.Any[i], bag, index); err != nil {
			return err
		}
	}
	return nil
}
