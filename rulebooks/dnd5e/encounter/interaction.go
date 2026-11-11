// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"crypto/sha256"
	"encoding/hex"
	stdjson "encoding/json"
	json "encoding/json/v2"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// interaction.go is THE ENTRANCE REMEMBERS (rpg-project#922, Gate S2 of
// #769; design #916 R2 and R3): an encounter-owned interaction instance per
// authored interaction, with its progress on `mind/machine`.
//
// # One owner, independent of any speaker
//
// #916 R3: "The encounter owns an authored interaction instance
// independently of any speaker or guard. Conversation state and admission
// progress are that one instance." So the instance belongs to the RUN, not to
// a member: changing speaker, speaking to another guard, backing away or
// reloading leaves it where it was (R2). Its state is a requirement an option
// can name ([TriggerInteraction]), and the settled verdict of a line spoken
// there may advance it ([Option.Advance]); which flavour row answered does
// not.
//
// # Definition and progress stay distinct
//
// As a member's behaviors are ([BehaviorData]): the bound definition is kept
// as authored, canonical, with its digest, and the progress is one
// [machine.Blob]. Load compiles the definition again and refuses a digest
// that does not match it, or a blob naming a state the recipe lacks.
//
// # The existing machine, a vocabulary of its own
//
// No second conversation engine: the recipe is a [machine.Recipe] compiled
// against a vocabulary built from the recipe itself — every `on:` it names is
// a beat, and it reads no fact. An interaction reads no world in this slice,
// so a guard with a `fact` or `scope` leaf is refused (the machine's own
// compile refuses both against this vocabulary); `flag` and `rounds` are
// allowed. Its beats are never added to [machineVocabulary], so no member's
// recipe compiles against them. An interaction rolls nothing and nobody wakes
// it, so a `pick` or a `wake_in` act is refused at the door.

// InteractionID names one authored interaction instance in a run — the key
// an option's [TriggerInteraction] and [Advance] name it by.
type InteractionID string

// InteractionInput is one authored interaction: its id and its recipe.
//
// Carried on [FieldInput.Interactions] so a host that launches a compiled
// dungeon passes it with the field it already passes; it is NOT site data
// (site.go): the definition and the progress are the run's, persisted on
// [EncounterData.Interactions], and never in the site's hash.
type InteractionInput struct {
	// ID names the interaction. REQUIRED non-empty and unique in the run.
	ID InteractionID

	// Recipe is the interaction as a machine: its states, and the beats a
	// line's verdict lands on it. REQUIRED: a valid ref, an initial state,
	// no `pick` or `wake_in` act, and no guard that reads a fact or a scope.
	Recipe machine.Recipe

	// Approach is how near a carrier must see a character come to address
	// them (address.go, rpg-project#923). Nil: nobody is ever hailed, and
	// the definition's digest is what it was before the field existed.
	Approach *InteractionApproach `json:",omitzero"`

	// Outcomes is what follows the interaction ENTERING a state, keyed by
	// that state (recognition.go, rpg-project#928): authored once here, so
	// every line that leads to the state, on any carrier, writes the same
	// thing. Each key is a state of the recipe and names at least one
	// [Consequence]. Nil: nothing follows, and the definition's digest is
	// what it was before the field existed.
	Outcomes map[string][]Consequence `json:",omitzero"`
}

// InteractionData is the persistent representation of one interaction
// instance: the definition, canonical, with its digest, and its progress.
type InteractionData struct {
	// ID is [InteractionInput.ID].
	ID InteractionID `json:"id"`

	// Definition is the lower-case hex SHA-256 of the canonical definition
	// (the id and the recipe). Load refuses a recipe that does not hash to it.
	Definition string `json:"definition"`

	// Recipe is the canonical JSON of the interaction's [machine.Recipe].
	Recipe stdjson.RawMessage `json:"recipe"`

	// Blob is the interaction's progress: its state and bag.
	Blob machine.Blob `json:"blob"`

	// Approach is [InteractionInput.Approach]; part of the definition the
	// digest covers. Absent for none.
	Approach *InteractionApproach `json:"approach,omitzero"`

	// Outcomes is [InteractionInput.Outcomes]; part of the definition the
	// digest covers. Absent for none.
	Outcomes map[string][]Consequence `json:"outcomes,omitzero"`

	// Address is who the interaction addresses now (address.go), absent for
	// nobody. Load refuses one naming a member or guard the run lacks.
	Address *InteractionAddress `json:"address,omitzero"`

	// Revision is the conversation's revision (interaction_hold.go, rpg-project#924):
	// one or more, bumped whenever the progress moves and whenever a
	// reservation is placed, settled or released, and never by an address.
	// A record saved before the field existed carries none and loads at one,
	// the revision every instance starts at.
	Revision uint64 `json:"revision,omitzero"`

	// Reservation is the checked line waiting on its speaker's roll decision
	// (interaction_hold.go), absent for none. At most one per interaction. Load refuses
	// one naming a member the run lacks, or a revision not below [Revision].
	Reservation *InteractionReservation `json:"reservation,omitzero"`
}

// MarshalJSON writes the record through encoding/json v2, so the recipe and
// the blob keep their canonical encodings when a host writes [EncounterData]
// through encoding/json v1 — [BehaviorData]'s reason. Deterministic for the
// same reason as [BehaviorData.MarshalJSON]: the blob's bag is a map.
func (d InteractionData) MarshalJSON() ([]byte, error) {
	type plain InteractionData
	return json.Marshal(plain(d), json.Deterministic(true))
}

// UnmarshalJSON reads the record through encoding/json v2, so the blob's bag
// lands whole numbers as int. Invalid JSON is refused.
func (d *InteractionData) UnmarshalJSON(raw []byte) error {
	type plain InteractionData
	return json.Unmarshal(raw, (*plain)(d))
}

// interactionRecord is one live interaction instance: the owned definition,
// its digest, the compiled program, and the progress.
type interactionRecord struct {
	input      InteractionInput
	definition string
	program    *machine.Program
	blob       machine.Blob

	// address is who the interaction addresses now; nil for nobody.
	address *InteractionAddress

	// revision is the conversation's revision, one or more (interaction_hold.go).
	revision uint64

	// reservation is the checked line waiting on its roll; nil for none.
	reservation *InteractionReservation
}

// hasState reports whether the interaction's recipe declares state.
func (r *interactionRecord) hasState(state string) bool {
	_, ok := r.input.Recipe.States[state]
	return ok
}

// hears reports whether some state of the interaction's recipe listens for
// beat — the only beats an [Advance] may name.
func (r *interactionRecord) hears(beat machine.Beat) bool {
	for _, s := range r.input.Recipe.States {
		for _, t := range s.Transitions {
			if t.On == beat {
				return true
			}
		}
	}
	return false
}

// newInteractionRecord owns, checks and compiles one interaction definition,
// starting it in its initial state. A refusal names the interaction; the
// caller adds the sentinel of the door it came in through.
func newInteractionRecord(in InteractionInput) (*interactionRecord, error) {
	if in.ID == "" {
		return nil, fmt.Errorf("an interaction has no id")
	}
	raw, err := json.Marshal(in.Recipe, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("interaction %q: %w", in.ID, err)
	}
	owned := InteractionInput{ID: in.ID, Approach: cloneApproach(in.Approach), Outcomes: cloneOutcomes(in.Outcomes)}
	if err := json.Unmarshal(raw, &owned.Recipe); err != nil {
		return nil, fmt.Errorf("interaction %q: %w", in.ID, err)
	}
	if err := validateApproach(in.ID, owned.Approach); err != nil {
		return nil, err
	}
	if err := owned.Recipe.Ref.IsValid(); err != nil {
		return nil, fmt.Errorf("interaction %q recipe ref: %w", in.ID, err)
	}
	for _, name := range slices.Sorted(maps.Keys(owned.Recipe.States)) {
		for i, t := range owned.Recipe.States[name].Transitions {
			for _, act := range t.Do {
				// AN INTERACTION ROLLS NOTHING AND NOBODY WAKES IT: a pick
				// names a table no instance has, and a wake arms a clock no
				// registry asks. Both would be authored and never happen.
				if act.Pick != "" {
					return nil, fmt.Errorf("interaction %q state %q transition %d: an interaction picks nothing", in.ID, name, i)
				}
				if act.WakeIn != 0 {
					return nil, fmt.Errorf("interaction %q state %q transition %d: nothing wakes an interaction", in.ID, name, i)
				}
			}
		}
	}
	states := make(map[string]bool, len(owned.Recipe.States))
	for name := range owned.Recipe.States {
		states[name] = true
	}
	if err := validateOutcomes(in.ID, states, owned.Outcomes); err != nil {
		return nil, err
	}
	program, err := owned.Recipe.Compile(interactionVocabulary(&owned.Recipe))
	if err != nil {
		return nil, fmt.Errorf("interaction %q: %w", in.ID, err)
	}
	digest, err := interactionDigest(owned)
	if err != nil {
		return nil, fmt.Errorf("interaction %q: %w", in.ID, err)
	}

	return &interactionRecord{input: owned, definition: digest, program: program, blob: program.NewBlob(), revision: 1}, nil
}

// CheckInteraction validates one authored interaction exactly as Setup
// installs it — a valid recipe ref, no `pick` or `wake_in` act, no guard
// that reads a fact or a scope, and a recipe the machine compiles — without
// constructing an encounter, for an authoring dialect that refuses it at the
// author's own path, as [CheckBehaviors] does for behaviors. Its states and
// the beats it listens for are returned so the dialect can judge an option
// naming them.
//
// Errors: ErrNilInput for a nil input; every refusal wraps ErrNoField, the
// door an interaction comes in through.
func CheckInteraction(in *InteractionInput) (*CheckInteractionOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("check interaction: %w", ErrNilInput)
	}
	record, err := newInteractionRecord(*in)
	if err != nil {
		return nil, fmt.Errorf("check interaction: %w: %w", err, ErrNoField)
	}
	out := &CheckInteractionOutput{States: slices.Sorted(maps.Keys(record.input.Recipe.States))}
	out.Beats = slices.Sorted(maps.Keys(interactionVocabulary(&record.input.Recipe).Beats))
	topics := grantedTopics(map[InteractionID]*interactionRecord{record.input.ID: record})
	out.Topics = slices.Sorted(maps.Keys(topics))
	return out, nil
}

// CheckInteractionOutput is what an authoring dialect may judge an option
// against: every state the interaction's recipe declares, every beat it
// listens for, and every topic its outcomes grant (rpg-project#928) — what
// a `recognized` requirement may name — each sorted.
type CheckInteractionOutput struct {
	States []string
	Beats  []machine.Beat
	Topics []TopicID
}

// interactionDigest is the hex SHA-256 of the canonical definition.
func interactionDigest(in InteractionInput) (string, error) {
	raw, err := json.Marshal(in, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// interactionVocabulary is the vocabulary an interaction compiles against:
// every beat its own recipe is `on:`, each carrying the round it lands at so
// a `rounds` leaf can be answered, and no fact.
func interactionVocabulary(r *machine.Recipe) machine.Vocabulary {
	v := machine.Vocabulary{Facts: map[machine.FactName]machine.FactSpec{}, Beats: map[machine.Beat]machine.BeatSpec{}}
	for _, s := range r.States {
		for _, t := range s.Transitions {
			v.Beats[t.On] = machine.BeatSpec{Round: interactionRound}
		}
	}
	return v
}

// interactionRound reads the round an interaction beat lands at: the event
// [Encounter.advanceInteraction] steps with.
func interactionRound(ev any) (uint64, bool) {
	round, ok := ev.(uint64)
	return round, ok
}

// compileInteractions builds every authored interaction of a run, refusing
// a duplicate id and any definition [newInteractionRecord] refuses. Empty
// for none, so a run without interactions holds and writes nothing.
func compileInteractions(inputs []InteractionInput) (map[InteractionID]*interactionRecord, error) {
	out := make(map[InteractionID]*interactionRecord, len(inputs))
	for _, in := range inputs {
		if _, dup := out[in.ID]; dup {
			return nil, fmt.Errorf("interaction %q is declared twice", in.ID)
		}
		record, err := newInteractionRecord(in)
		if err != nil {
			return nil, err
		}
		out[in.ID] = record
	}
	return out, nil
}

// interactionsDataFrom renders the run's interactions for the blob, in id
// order; nil for none, so a run without interactions writes no key.
func interactionsDataFrom(interactions map[InteractionID]*interactionRecord) []InteractionData {
	if len(interactions) == 0 {
		return nil
	}
	out := make([]InteractionData, 0, len(interactions))
	for _, id := range slices.Sorted(maps.Keys(interactions)) {
		r := interactions[id]
		raw, err := json.Marshal(r.input.Recipe, json.Deterministic(true))
		if err != nil {
			panic(err) // owned and compiled at the door: it marshaled once already
		}
		blob := r.blob
		blob.Bag = r.blob.Bag.Clone()
		if r.blob.WakeAt != nil {
			at := *r.blob.WakeAt
			blob.WakeAt = &at
		}
		if r.blob.Anchor != nil {
			at := *r.blob.Anchor
			blob.Anchor = &at
		}
		data := InteractionData{
			ID: id, Definition: r.definition, Recipe: raw, Blob: blob, Approach: cloneApproach(r.input.Approach),
			Outcomes: cloneOutcomes(r.input.Outcomes),
			Revision: r.revision, Reservation: cloneReservation(r.reservation),
		}
		if r.address != nil {
			address := *r.address
			data.Address = &address
		}
		out = append(out, data)
	}
	return out
}

// interactionsFromData rebuilds the run's interactions from the blob:
// every definition compiled again, a digest that does not match its
// definition refused, and a blob whose state the recipe lacks refused. Each
// refusal is the caller's to wrap in [ErrInvalidData].
func interactionsFromData(data []InteractionData) (map[InteractionID]*interactionRecord, error) {
	out := make(map[InteractionID]*interactionRecord, len(data))
	for _, d := range data {
		if _, dup := out[d.ID]; dup {
			return nil, fmt.Errorf("interaction %q is saved twice", d.ID)
		}
		in := InteractionInput{ID: d.ID, Approach: d.Approach, Outcomes: d.Outcomes}
		if err := json.Unmarshal(d.Recipe, &in.Recipe); err != nil {
			return nil, fmt.Errorf("interaction %q recipe: %w", d.ID, err)
		}
		record, err := newInteractionRecord(in)
		if err != nil {
			return nil, err
		}
		if record.definition != d.Definition {
			return nil, fmt.Errorf("interaction %q: the saved definition does not match its recipe", d.ID)
		}
		if d.Blob.State != "" && !record.hasState(d.Blob.State) {
			return nil, fmt.Errorf("interaction %q is saved in state %q, which its recipe does not declare", d.ID, d.Blob.State)
		}
		record.blob = d.Blob
		if d.Address != nil {
			address := *d.Address
			record.address = &address
		}
		if record.blob.State == "" {
			record.blob.State = record.input.Recipe.Initial
		}
		// A RECORD FROM BEFORE THE REVISION loads at one, where every
		// instance starts: zero is never an offered revision (interaction_hold.go).
		if d.Revision > 0 {
			record.revision = d.Revision
		}
		if d.Reservation != nil {
			if err := validateReservationShape(d.ID, *d.Reservation, record.revision); err != nil {
				return nil, err
			}
			record.reservation = cloneReservation(d.Reservation)
		}
		out[d.ID] = record
	}
	return out, nil
}

// validateOptionInteractions refuses a line that names an interaction this
// run does not have, a state its recipe does not declare, a recognition on a
// topic no interaction's outcome grants (rpg-project#928), or an advance on a
// beat no state of its recipe listens for — each a requirement nobody can
// ever meet or a verdict that can never land, refused at the door it came
// in through. The caller adds the sentinel.
func validateOptionInteractions(interactions map[InteractionID]*interactionRecord, options []Option) error {
	granted := grantedTopics(interactions)
	for _, o := range options {
		if err := validateRecognizedTopics(granted, fmt.Sprintf("option %q", o.ID), o.Requires); err != nil {
			return err
		}
		for _, requires := range o.Requires {
			inner, _ := unwrapNot(requires)
			t, ok := inner.(TriggerInteraction)
			if !ok {
				continue
			}
			record, ok := interactions[t.Interaction]
			if !ok {
				return fmt.Errorf("option %q requires interaction %q, and the run has no such interaction", o.ID, t.Interaction)
			}
			for _, state := range t.In {
				if !record.hasState(state) {
					return fmt.Errorf("option %q requires interaction %q in state %q, which its recipe does not declare",
						o.ID, t.Interaction, state)
				}
			}
		}
		if o.Advance == nil {
			continue
		}
		record, ok := interactions[o.Advance.Interaction]
		if !ok {
			return fmt.Errorf("option %q advances interaction %q, and the run has no such interaction", o.ID, o.Advance.Interaction)
		}
		for _, beat := range []machine.Beat{o.Advance.Success, o.Advance.Failure} {
			if beat != "" && !record.hears(beat) {
				return fmt.Errorf("option %q advances interaction %q on %q, which no state of its recipe listens for",
					o.ID, o.Advance.Interaction, beat)
			}
		}
	}
	return nil
}

// validatePropInteractions asks [validateOptionInteractions] of every
// prop's lines on the field, refused with [ErrNoField], the door a prop
// comes in through. Asked once the run's interactions are built, at Setup
// and at Load: a prop's lines are site data, and the site knows no run.
func validatePropInteractions(interactions map[InteractionID]*interactionRecord, f *field) error {
	for _, p := range f.props {
		if err := validateOptionInteractions(interactions, p.Options); err != nil {
			return fmt.Errorf("prop %q: %w: %w", p.ID, err, ErrNoField)
		}
	}
	for i := range f.placed {
		if err := validateOptionInteractions(interactions, f.placed[i].options); err != nil {
			return fmt.Errorf("prop %q: %w: %w", f.placed[i].id, err, ErrNoField)
		}
	}
	return nil
}

// interactionIn reports whether the run's interaction id is in one of the
// states named — the read [TriggerInteraction] makes. An interaction the run
// does not have is in no state: the door refuses a line naming one, so this
// is the fail-closed answer for a value that could only arrive by hand.
func (e *Encounter) interactionIn(id InteractionID, states []string) bool {
	record, ok := e.interactions[id]
	if !ok {
		return false
	}
	return slices.Contains(states, record.blob.State)
}
