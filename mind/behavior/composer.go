// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

var (
	// ErrBadComposition reports a definition or binding the composer cannot run.
	ErrBadComposition = errors.New("behavior: invalid composition")
	// ErrSelectionRequired reports an observation whose transition requests
	// a table pick. Use Decide for an action-selection opportunity instead.
	ErrSelectionRequired = errors.New("behavior: input requires selection")
	// ErrUnknownInstance reports an observation addressed to no attachment.
	ErrUnknownInstance = errors.New("behavior: unknown instance")
)

// InstanceSpec is a materialized behavior binding. ID identifies this
// attachment, not its definition; Recipe.Ref names the definition. Table keys
// are the picks the recipe requests. Values are resolved by the producer before
// construction, never sampled or templated by the runtime composer.
type InstanceSpec struct {
	ID     string         `json:"id"`
	Recipe machine.Recipe `json:"recipe"`
	Table  Table          `json:"table"`
}

// ComposerConfig supplies independent instances in highest-priority-first
// order and the vocabulary their recipes consume. Empty Instances is legal.
// Fact readers must be nonnil and should be pure; the composer neither owns
// their external state nor rolls back their side effects.
type ComposerConfig struct {
	Instances  []InstanceSpec
	Vocabulary machine.Vocabulary
}

// Composer owns independent machine progress and one ordered action-selection
// lane. It returns decisions, never executes words, publishes, or advances a
// clock. It is not safe for concurrent use; construct with NewComposer or
// LoadComposer rather than using the zero value.
type Composer struct {
	instances []composedInstance
	beats     map[machine.Beat]machine.BeatSpec
	byID      map[string]int
}

type composedInstance struct {
	spec       InstanceSpec
	definition string
	program    *machine.Program
	blob       machine.Blob
}

// NewComposer copies and validates the materialized definitions, compiles their
// machines, and creates private initial blobs. It refuses duplicate/empty IDs,
// nil fact readers, invalid recipes/rows, missing picked tables and transitions
// requesting multiple picks. Nil input returns ErrNilInput; other refusals wrap
// ErrBadComposition and any underlying compiler/row error.
func NewComposer(in *ComposerConfig) (*Composer, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	for _, name := range slices.Sorted(maps.Keys(in.Vocabulary.Facts)) {
		if in.Vocabulary.Facts[name].Read == nil {
			return nil, fmt.Errorf("%w: fact %q has no reader", ErrBadComposition, name)
		}
	}
	out := &Composer{beats: maps.Clone(in.Vocabulary.Beats), byID: make(map[string]int, len(in.Instances))}
	for _, spec := range in.Instances {
		_, exists := out.byID[spec.ID]
		if spec.ID == "" || exists {
			return nil, fmt.Errorf("%w: empty or duplicate instance ID %q", ErrBadComposition, spec.ID)
		}
		instance, err := compileInstance(spec, in.Vocabulary)
		if err != nil {
			return nil, fmt.Errorf("%w: instance %q: %w", ErrBadComposition, spec.ID, err)
		}
		out.byID[spec.ID] = len(out.instances)
		out.instances = append(out.instances, instance)
	}
	return out, nil
}

func compileInstance(spec InstanceSpec, vocabulary machine.Vocabulary) (composedInstance, error) {
	// Copy before traversing: JSON rejects cyclic producer data. Validate the
	// original typed recipe as well, so numeric decoding cannot coerce a bad
	// Go scalar into a valid machine value.
	raw, err := json.Marshal(spec, json.Deterministic(true))
	if err != nil {
		return composedInstance{}, err
	}
	var owned InstanceSpec
	if err := json.Unmarshal(raw, &owned); err != nil {
		return composedInstance{}, err
	}
	program, err := spec.Recipe.Compile(vocabulary)
	if err != nil {
		return composedInstance{}, err
	}
	for _, key := range slices.Sorted(maps.Keys(spec.Table)) {
		if err := checkCompositionRows(spec.Table[key]); err != nil {
			return composedInstance{}, fmt.Errorf("table %q: %w", key, err)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(spec.Recipe.States)) {
		for i, transition := range spec.Recipe.States[name].Transitions {
			picks := 0
			for _, act := range transition.Do {
				if act.Pick == "" {
					continue
				}
				picks++
				if _, exists := owned.Table[AnswerKey(act.Pick)]; !exists {
					return composedInstance{}, fmt.Errorf("state %q transition %d picks absent table %q", name, i, act.Pick)
				}
			}
			if picks > 1 {
				return composedInstance{}, fmt.Errorf("state %q transition %d requests multiple picks", name, i)
			}
		}
	}
	digest := sha256.Sum256(raw)
	return composedInstance{
		spec: owned, definition: hex.EncodeToString(digest[:]), program: program, blob: program.NewBlob(),
	}, nil
}

func checkCompositionRows(rows []Answer) error {
	for i := range rows {
		row := &rows[i]
		if row.Weight < 1 {
			return fmt.Errorf("row %d has nonpositive weight", i)
		}
		if err := row.Validate(); err != nil {
			return fmt.Errorf("row %d: %w", i, err)
		}
		if row.isGroup() {
			if err := checkCompositionRows(row.Rows); err != nil {
				return fmt.Errorf("row %d: %w", i, err)
			}
			continue
		}
		words := 0
		for _, present := range []bool{
			row.Hold, row.Attack != nil, row.Toward != nil, row.Dash != nil,
			row.Away != nil, row.Search != nil, row.Flee, row.Fact != "",
		} {
			if present {
				words++
			}
		}
		if words > 1 || (words == 0 && row.Say == "") {
			return fmt.Errorf("row %d needs one outcome word or a line, not %d words", i, words)
		}
	}
	return nil
}

// ObserveInput is one delivered data-only occurrence. Env supplies the caller's
// facts, Event the beat-specific value, and Now the caller's clock reading.
type ObserveInput struct {
	// Instance addresses completion/feedback to one attachment. Empty broadcasts
	// a shared occurrence; it does not mean an unnamed instance.
	Instance string
	On       machine.Beat
	Event    any
	Env      any
	Now      uint64
	// Trace requests one-pass evaluation and commit explanations.
	Trace bool
}

// InstanceStep identifies one fired machine transition committed by a verb.
// It is an explanation value, not a request for another state mutation.
type InstanceStep struct {
	ID   string
	Step machine.Stepped
}

// ObserveOutput names fired, committed transitions. A successful observation
// may also consume a due wake without firing, as machine.Step does; persist
// ToData after successful verbs rather than treating Steps as a dirty flag.
type ObserveOutput struct {
	Steps []InstanceStep
	// Outcomes are present only when tracing was requested.
	Outcomes []InstanceOutcome
}

// Observe steps the addressed instance, or all instances for a broadcast,
// without rolling. An unknown address returns ErrUnknownInstance. A firing Pick returns
// ErrSelectionRequired without committing any blob; actions are never silently
// dropped. Nil input returns ErrNilInput and an unknown beat ErrUnknownBeat.
func (c *Composer) Observe(in *ObserveInput) (*ObserveOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	proposals, err := c.preview(in)
	if err != nil {
		return nil, err
	}
	for i, proposal := range proposals {
		if proposal.pick != "" {
			return nil, fmt.Errorf("%w: instance %q on %q", ErrSelectionRequired, c.instances[i].spec.ID, in.On)
		}
	}
	out := &ObserveOutput{}
	for i := range proposals {
		c.commit(i, &proposals[i], &out.Steps)
	}
	if in.Trace {
		out.Outcomes = c.instanceOutcomes(proposals)
	}
	return out, nil
}

// DecideInput describes one authorized selection opportunity, not an executed
// action. Facts/Temper/Die feed the existing picker; Env/Event/Now feed machines.
// The caller owns all knowledge projection, budgets and actual clock advancement.
type DecideInput struct {
	On     machine.Beat
	Event  any
	Env    any
	Now    uint64
	Facts  Facts
	Temper Temper
	Die    Die
	// Trace requests one-pass evaluation and commit explanations.
	Trace bool
}

// DecideOutput carries one selected table result and committed transitions.
// Instance and Pick are empty/nil when no instance requests a selection.
// Deferred names lower-priority requesting instances whose progress is kept.
// A no-candidate hold is selected but does not consume that instance's step;
// an explicitly selected Hold row does. Nothing here executes the answer.
type DecideOutput struct {
	Instance string
	Pick     *PickOutput
	Steps    []InstanceStep
	Deferred []string
	// Outcomes distinguish proposals, selection, commits and nondelivery.
	// They are absent when tracing was not requested.
	Outcomes []InstanceOutcome
}

// Decide previews all machines, selects the first requesting instance, and
// rolls only its table. Data-only transitions commit alongside the winner;
// losing action candidates retain their complete original blobs. A no-candidate
// hold also retains its requesting blob. Errors commit no composer state, but
// cannot undo caller-reader side effects or an external die call.
//
// Nil input returns ErrNilInput. Unknown beats and picker errors retain their
// sentinels. A selected missing die returns ErrNoDie only when a row is eligible.
func (c *Composer) Decide(ctx context.Context, in *DecideInput) (*DecideOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	proposals, err := c.preview(&ObserveInput{On: in.On, Event: in.Event, Env: in.Env, Now: in.Now, Trace: in.Trace})
	if err != nil {
		return nil, err
	}
	out := &DecideOutput{}
	selected := -1
	for i, proposal := range proposals {
		if proposal.pick == "" {
			continue
		}
		if selected == -1 {
			selected = i
		} else {
			out.Deferred = append(out.Deferred, c.instances[i].spec.ID)
		}
	}
	if selected >= 0 {
		instance := &c.instances[selected]
		key := proposals[selected].pick
		// Facts can include caller callbacks that inspect a row. They must
		// not receive a writable alias into our immutable configuration.
		rows, err := copyCompositionValue(instance.spec.Table[key])
		if err != nil {
			return nil, fmt.Errorf("instance %q table: %w", instance.spec.ID, err)
		}
		pick, err := Pick(ctx, &PickInput{
			Key: KeyTime, Table: Table{KeyTime: rows},
			Facts: in.Facts, Temper: in.Temper, Die: in.Die,
		})
		if err != nil {
			return nil, fmt.Errorf("instance %q: %w", instance.spec.ID, err)
		}
		pick.Key = key
		// Pick's selected Answer contains pointers into its input table. Copy
		// before returning it so a caller cannot mutate the stored definition.
		owned, err := copyCompositionValue(*pick)
		if err != nil {
			return nil, fmt.Errorf("instance %q result: %w", instance.spec.ID, err)
		}
		out.Instance, out.Pick = instance.spec.ID, &owned
		proposals[selected].selected = true
	}
	for i := range proposals {
		if proposals[i].pick != "" && (i != selected || out.Pick.Entry == -1) {
			continue
		}
		c.commit(i, &proposals[i], &out.Steps)
	}
	if in.Trace {
		out.Outcomes = c.instanceOutcomes(proposals)
	}
	return out, nil
}

type compositionProposal struct {
	touched   bool
	committed bool
	selected  bool
	skipped   uint8
	blob      machine.Blob
	step      machine.Stepped
	pick      AnswerKey
	traced    *machine.TraceStepOutput
}

func (c *Composer) preview(in *ObserveInput) ([]compositionProposal, error) {
	if _, exists := c.beats[in.On]; !exists {
		return nil, fmt.Errorf("%w: %q", machine.ErrUnknownBeat, in.On)
	}
	first, limit := 0, len(c.instances)
	if in.Instance != "" {
		index, exists := c.byID[in.Instance]
		if !exists {
			return nil, fmt.Errorf("%w: %q", ErrUnknownInstance, in.Instance)
		}
		first, limit = index, index+1
	}
	out := make([]compositionProposal, len(c.instances))
	if in.Trace && in.Instance != "" {
		for i := range out {
			if i < first || i >= limit {
				out[i].skipped = traceNotAddressed
			}
		}
	}
	for i := first; i < limit; i++ {
		instance := &c.instances[i]
		// The actor may be due because a different instance is awake. A timed
		// delivery does not spend this instance's future wake or continuation.
		// Non-timed observations remain deliverable to sleeping instances.
		if c.beats[in.On].Timed && instance.blob.WakeAt != nil && in.Now < *instance.blob.WakeAt {
			out[i].skipped = traceSleeping
			continue
		}
		blob := cloneCompositionBlob(instance.blob)
		var step machine.Stepped
		var traced *machine.TraceStepOutput
		var err error
		if in.Trace {
			traced, err = instance.program.TraceStep(&machine.TraceStepInput{Blob: &blob, On: in.On, Event: in.Event, Env: in.Env, Now: in.Now})
			if err == nil {
				step = traced.Step
			}
		} else {
			step, err = instance.program.Step(&blob, in.On, in.Event, in.Env, in.Now)
		}
		if err != nil {
			return nil, fmt.Errorf("instance %q: %w", instance.spec.ID, err)
		}
		out[i] = compositionProposal{touched: true, blob: blob, step: step, traced: traced}
		for _, act := range step.Acts {
			if act.Pick != "" {
				out[i].pick = AnswerKey(act.Pick)
			}
		}
	}
	return out, nil
}

func (c *Composer) commit(i int, proposal *compositionProposal, steps *[]InstanceStep) {
	if !proposal.touched {
		return
	}
	c.instances[i].blob = proposal.blob
	proposal.committed = true
	if proposal.step.Fired {
		*steps = append(*steps, InstanceStep{ID: c.instances[i].spec.ID, Step: proposal.step})
	}
}

func copyCompositionValue[T any](value T) (T, error) {
	var out T
	raw, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	return out, nil
}
