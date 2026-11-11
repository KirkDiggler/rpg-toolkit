// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// BoundBehavior is one materialized attachment and its addressed completion
// vocabulary. Expansion/source resolution belongs to its producer, not play.
type BoundBehavior struct {
	Instance    behavior.InstanceSpec
	Completions map[behavior.AnswerKey]machine.Beat
}

// BehaviorInput is an actor's ordered canonical composition. No instances means
// no ordinary policy. BodyInstance defaults only when there is one instance;
// state-bound blocking/posts with multiple instances must name their body source.
type BehaviorInput struct {
	Instances    []BoundBehavior
	BodyInstance string
}

// BehaviorData retains original materialized configuration separately from
// private progress. Loading it never consults current author source or expands
// a sequence. There is no second live creature-machine blob.
type BehaviorData struct {
	Definition   string                `json:"definition"`
	Instances    []BoundBehavior       `json:"instances"`
	BodyInstance string                `json:"body_instance,omitempty"`
	Progress     behavior.ComposerData `json:"progress"`
	// SymbolsRecorded is diagnostic bookkeeping for this immutable definition.
	SymbolsRecorded bool `json:"symbols_recorded,omitempty"`
}

// MarshalJSON preserves the canonical recipe encoding when EncounterData is
// written through encoding/json v1 by a host. The encoding is deterministic:
// a recipe's `states` and `table` maps come out in sorted key order, so an
// unchanged member writes the same bytes on every save, the promise the
// snapshot makes and every byte or digest comparison at the storage boundary
// stands on (rpg-project#897).
func (d BehaviorData) MarshalJSON() ([]byte, error) {
	type plain BehaviorData
	return json.Marshal(plain(d), json.Deterministic(true))
}

// UnmarshalJSON restores canonical scalar types in recipes before the loader
// validates configuration and progress. Invalid JSON is refused.
func (d *BehaviorData) UnmarshalJSON(raw []byte) error {
	type plain BehaviorData
	return json.Unmarshal(raw, (*plain)(d))
}

// Delegated asks the encounter to make the ordinary behavior selection. It is
// not executable and cannot accompany an already-selected Decision.Pick.
type Delegated struct{}

func (Delegated) isTurnIntent() {}

// BehaviorDriver delegates ordinary choice without rolling. A compulsion/custom
// wrapper may return its concrete intent first through the same Driver seam.
type BehaviorDriver struct{}

// Act delegates; the encounter owns the actor's composition and opportunity.
func (BehaviorDriver) Act(MonsterView) (Decision, error) {
	return Decision{Intent: Delegated{}}, nil
}

type behaviorRuntime struct {
	definition      string
	input           BehaviorInput
	composer        *behavior.Composer
	byID            map[string]int
	symbolsRecorded bool
}

func newBehaviorRuntime(in *BehaviorInput, progress *behavior.ComposerData) (*behaviorRuntime, error) {
	owned := BehaviorInput{}
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("behaviors: %w", err)
		}
		if err := json.Unmarshal(raw, &owned); err != nil {
			return nil, fmt.Errorf("behaviors: %w", err)
		}
	}
	if owned.BodyInstance == "" && len(owned.Instances) == 1 {
		owned.BodyInstance = owned.Instances[0].Instance.ID
	}
	vocab := machineVocabulary()
	completionBeats := map[machine.Beat]bool{}
	config := behavior.ComposerConfig{Vocabulary: vocab}
	byID := make(map[string]int, len(owned.Instances))
	for i, bound := range owned.Instances {
		spec := bound.Instance
		if _, duplicate := byID[spec.ID]; duplicate {
			return nil, fmt.Errorf("behaviors: duplicate attachment %q", spec.ID)
		}
		byID[spec.ID] = i
		// Generic machine/composer validation knows structure, not the
		// encounter's action authority. Observe-only inputs cannot request a
		// pick, even conditionally; barred is the explicit targeted exception.
		for _, name := range slices.Sorted(maps.Keys(spec.Recipe.States)) {
			for index, transition := range spec.Recipe.States[name].Transitions {
				if transition.On == beatTime || transition.On == beatTurn || transition.On == beatBarred || transition.On == beatApproached {
					continue
				}
				for _, act := range transition.Do {
					if act.Pick != "" {
						return nil, fmt.Errorf("behaviors %q state %q transition %d: selection is only authorized on time, turn, barred or approached, not %q", spec.ID, name, index, transition.On)
					}
				}
			}
		}
		for key, rows := range spec.Table {
			if isOptionKey(key) || IsRetiredTableKey(key) {
				return nil, fmt.Errorf("behaviors %q: invalid ordinary table key %q: %w", spec.ID, key, ErrBadAnswer)
			}
			for index, row := range rows {
				if err := validateEntry(key, index, row); err != nil {
					return nil, fmt.Errorf("behaviors %q: %w", spec.ID, err)
				}
			}
		}
		for beat, definition := range vocabularyFor(&spec.Recipe).Beats {
			vocab.Beats[beat] = definition
		}
		for name, definition := range vocabularyFor(&spec.Recipe).Facts {
			vocab.Facts[name] = definition
		}
		for key, on := range bound.Completions {
			if on == "" || len(spec.Table[key]) == 0 {
				return nil, fmt.Errorf("behaviors %q: completion %q needs an existing table and beat", spec.ID, key)
			}
			if _, exists := vocab.Beats[on]; exists && !completionBeats[on] {
				return nil, fmt.Errorf("behaviors %q: completion %q collides with occurrence %q", spec.ID, key, on)
			}
			vocab.Beats[on] = machine.BeatSpec{}
			completionBeats[on] = true
			if err := validateCompletionRows(spec.Table[key]); err != nil {
				return nil, fmt.Errorf("behaviors %q completion %q: %w", spec.ID, key, err)
			}
		}
		config.Instances = append(config.Instances, spec)
	}
	if owned.BodyInstance != "" {
		if _, exists := byID[owned.BodyInstance]; !exists {
			return nil, fmt.Errorf("behaviors: unknown body instance %q", owned.BodyInstance)
		}
	}
	var composer *behavior.Composer
	var err error
	if progress == nil {
		composer, err = behavior.NewComposer(&config)
	} else {
		composer, err = behavior.LoadComposer(&behavior.LoadComposerInput{Config: config, Data: *progress})
	}
	if err != nil {
		return nil, fmt.Errorf("behaviors: %w", err)
	}
	raw, err := json.Marshal(owned, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	return &behaviorRuntime{input: owned, composer: composer, byID: byID, definition: hex.EncodeToString(digest[:])}, nil
}

func (b *behaviorRuntime) bodyRecipe() *machine.Recipe {
	if b == nil || b.input.BodyInstance == "" {
		return nil
	}
	return &b.input.Instances[b.byID[b.input.BodyInstance]].Instance.Recipe
}

func (m *memberRecord) bodyState() string {
	if m.behaviors == nil || m.behaviors.input.BodyInstance == "" {
		return ""
	}
	state, err := m.behaviors.composer.StateOf(&behavior.InstanceStateInput{ID: m.behaviors.input.BodyInstance})
	if err != nil {
		panic(err) // installation/load validated the immutable body identity
	}
	return state.State
}

func (b *behaviorRuntime) toData() *BehaviorData {
	if b == nil || len(b.input.Instances) == 0 {
		return nil
	}
	// The configuration is plain canonical JSON already validated at ingress.
	// Round-tripping here detaches all recipe/table/reference data from callers.
	raw, err := json.Marshal(b.input)
	if err != nil {
		panic(err)
	}
	var detached BehaviorInput
	if err := json.Unmarshal(raw, &detached); err != nil {
		panic(err)
	}
	return &BehaviorData{Definition: b.definition, Instances: detached.Instances, BodyInstance: detached.BodyInstance, Progress: b.composer.ToData(), SymbolsRecorded: b.symbolsRecorded}
}

func (e *Encounter) useBehaviors(m *memberRecord, in *BehaviorInput, data *BehaviorData, blocking []string) error {
	var progress *behavior.ComposerData
	if data != nil {
		in = &BehaviorInput{Instances: data.Instances, BodyInstance: data.BodyInstance}
		progress = &data.Progress
	}
	b, err := newBehaviorRuntime(in, progress)
	if err != nil {
		return fmt.Errorf("member %q: %w: %w", m.ID, ErrBadMember, err)
	}
	if data != nil && data.Definition != b.definition {
		return fmt.Errorf("member %q: original behavior configuration changed: %w", m.ID, ErrInvalidData)
	}
	if err := validateBlockingStates(b.bodyRecipe(), blocking); err != nil {
		return fmt.Errorf("member %q: %w: %w", m.ID, ErrBadMember, err)
	}
	if data != nil {
		b.symbolsRecorded = data.SymbolsRecorded
	}
	m.behaviors = b
	m.BlockingStates = slices.Clone(blocking)
	return nil
}

type behaviorOpportunity struct {
	on    machine.Beat
	event any
	now   uint64

	// subject is the member a targeted reaction answers (rpg-project#923):
	// the mover who bumped ([beatBarred]) or was hailed ([beatApproached]).
	// A line picked on it is spoken TO them ([answeredBy.to]). Empty for
	// time, turn and every other opportunity.
	subject MemberID
}

type selectedBehavior struct {
	decision Decision
	instance string
	work     *BehaviorWork
	trace    *behaviorDecisionTrace
}

func (e *Encounter) selectDecision(m *memberRecord, view MonsterView, opportunity behaviorOpportunity) (selectedBehavior, error) {
	d, err := e.driver.Act(view)
	if err != nil {
		return selectedBehavior{}, err
	}
	tracing := m.behaviors != nil && len(m.behaviors.input.Instances) > 0
	if _, delegated := d.Intent.(Delegated); !delegated {
		out := selectedBehavior{decision: d}
		if tracing {
			out.trace = &behaviorDecisionTrace{on: opportunity.on, now: opportunity.now, operation: behaviorTraceDriver}
		}
		return out, nil
	}
	if d.Pick != nil {
		return selectedBehavior{}, fmt.Errorf("delegation with an already-selected pick: %w", ErrBadIntent)
	}
	env := e.envFor(m.ID)
	env.event = opportunity.event
	chosen, err := m.behaviors.composer.Decide(context.Background(), &behavior.DecideInput{
		On: opportunity.on, Event: opportunity.event, Env: env, Now: opportunity.now,
		Facts: factsFromView(view), Temper: view.Temper, Die: e.roller, Trace: tracing,
	})
	if err != nil {
		if errors.Is(err, behavior.ErrNoDie) {
			return selectedBehavior{}, fmt.Errorf("behavior choice: %w", ErrNoRoller)
		}
		return selectedBehavior{}, err
	}
	if env.err != nil {
		return selectedBehavior{}, env.err
	}
	if err := e.behaviorStepped(m, chosen.Steps); err != nil {
		return selectedBehavior{}, err
	}
	var trace *behaviorDecisionTrace
	if tracing {
		trace = &behaviorDecisionTrace{on: opportunity.on, now: opportunity.now, operation: behaviorTraceDecide, outcomes: chosen.Outcomes}
	}
	if chosen.Pick == nil {
		return selectedBehavior{decision: Decision{Intent: Pass{}}, trace: trace}, nil
	}
	translated, err := IntentForAnswer(&IntentForAnswerInput{View: view, Answer: chosen.Pick.Answer})
	if err != nil {
		return selectedBehavior{}, err
	}
	work, err := m.behaviors.workFor(chosen)
	if err != nil {
		return selectedBehavior{}, err
	}
	return selectedBehavior{decision: Decision{Intent: translated.Intent, Pick: chosen.Pick}, instance: chosen.Instance, work: work.Work, trace: trace}, nil
}

func (e *Encounter) behaviorStepped(m *memberRecord, steps []behavior.InstanceStep) error {
	for _, result := range steps {
		step := result.Step
		if result.ID != m.behaviors.input.BodyInstance || step.To == "" {
			continue
		}
		if err := e.appendStateBeat(m.ID, step); err != nil {
			return err
		}
		from, had := m.postIn(step.From)
		to, has := m.postIn(step.To)
		if m.blocksIn(step.From) != m.blocksIn(step.To) || !samePost(from, had, to, has) {
			if cell, placed := e.canvas.GetEntityPosition(string(m.ID)); placed {
				e.noteChanged(cell)
				e.blockingTurned = true
			}
		}
	}
	e.relistBehavior(m)
	return nil
}

func behaviorInputOf(data *BehaviorData) *BehaviorInput {
	if data == nil {
		return nil
	}
	return &BehaviorInput{Instances: data.Instances, BodyInstance: data.BodyInstance}
}

// behaviorReadsRoutes carries the route-on-demand contract across canonical
// attachments. Before selection, any attachment may supply the winning table,
// so use their conservative union without stepping guards or choosing twice.
// No authored policy retains the full view for an opaque host driver.
func (m *memberRecord) behaviorReadsRoutes() bool {
	if m.behaviors == nil || len(m.behaviors.input.Instances) == 0 {
		return true
	}
	for _, bound := range m.behaviors.input.Instances {
		for _, rows := range bound.Instance.Table {
			if tableReadsRoutes(rows) {
				return true
			}
		}
	}
	return false
}

func (m *memberRecord) behaviorNamesThreat() bool {
	if m.behaviors == nil {
		return false
	}
	for _, bound := range m.behaviors.input.Instances {
		for _, rows := range bound.Instance.Table {
			if tableNamesThreat(rows) {
				return true
			}
		}
	}
	return false
}
