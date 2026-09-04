// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package scenarios

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// FieldType is the kind of answer a scenario field wants, and the reason a
// builder can render a picker for it without knowing what the field means.
//
// A CLOSED SET, unlike [Field.Kind] below. A client must be able to render
// every field type it is handed, so a type it has never heard of is a
// picker it cannot draw — the opposite trade from the kind vocabulary.
type FieldType string

const (
	// FieldEntityRef is a reference to something in the dungeon, by id. The
	// picker lists candidates; which ones is [Field.Kind]'s answer.
	FieldEntityRef FieldType = "entity_ref"

	// FieldCheck is an authored check — approaches and DCs. No scenario
	// declares one yet; the type exists because the descriptor is what the
	// builder renders and adding a form control is not a rules change.
	FieldCheck FieldType = "check"
)

// Field is one question on a scenario's form.
type Field struct {
	// Key is the binding's key in the dungeon file, e.g. "artifact".
	Key string

	// Label is what the form shows above the control.
	Label string

	// Type is the control to render. See [FieldType].
	Type FieldType

	// Kind narrows an entity reference to what may answer it — "prop",
	// "exit", "door", "monster". An OPEN STRING, deliberately not an enum:
	// the descriptor is content, and a scenario that wants to bind a region
	// should not need a toolkit release to say so. A client that meets a
	// kind it cannot narrow by lists everything and lets the refusal do the
	// work, which is a worse form and not a broken one.
	Kind string

	// Guidance is the constructor's own refusal text, VERBATIM — the same
	// sentence [Scenario.New] returns when this field is wrong.
	//
	// One sentence, two jobs, and that is the point: the words that tell an
	// author what to fill in are the words that tell them what they got
	// wrong, so the two can never drift into disagreeing. The pinning test
	// asserts it.
	Guidance string
}

// Declared is what a scenario resolves to once its form is filled in: the
// endings the encounter runs, and the ids it bound.
type Declared struct {
	// Endings are declared verbatim on [encounter.SetupInput.Endings],
	// alongside whatever the host declares itself. Never empty for a
	// scenario that constructed.
	Endings []encounter.EndingInput

	// Artifact is the placement the party is there to recover, when this
	// scenario binds one. Empty otherwise.
	Artifact encounter.PropID

	// Exit is the way out that counts as escaping, when this scenario binds
	// one. Empty otherwise.
	Exit encounter.ExitID
}

// Scenario is one thing a dungeon can be for.
type Scenario interface {
	// ID is the scenario's key in a dungeon's `scenarios:` map.
	ID() string

	// Name is what the builder shows in the picker.
	Name() string

	// Fields are the form, in the order it is rendered.
	Fields() []Field

	// New validates one filled-in form against the dungeon it is bound to,
	// and returns what the run declares. Every refusal is in form-filler
	// words and names the field it is about.
	//
	// NOTHING IS DEFAULTED. A missing binding is a refusal, never an
	// assumption (rpg-toolkit#1033).
	New(cfg map[string]string, compiled *DungeonFacts) (Declared, error)
}

// DungeonFacts is everything a scenario is allowed to ask about the dungeon
// it is being bound to: which ids exist, and which props can be picked up.
//
// # Why not the whole Compiled
//
// This package cannot import dungeonspec — dungeonspec imports encounter and
// this package would close a cycle the moment dungeonspec wanted to validate
// against a descriptor. More to the point, a scenario has no business with
// the geometry: it binds ids, and the two questions it asks about them are
// here. The caller builds this from a [dungeonspec.Compiled] in one place.
type DungeonFacts struct {
	// Props is every placed prop's id to whether it is holdable. A prop with
	// no id is not in here — a scenario cannot bind a thing with no name.
	Props map[encounter.PropID]bool

	// Exits is every authored exit's id.
	Exits map[encounter.ExitID]bool
}

// FactsFrom builds the facts a scenario may ask about, from a compiled
// dungeon's field. The ONE place a Compiled is narrowed to what a scenario
// sees, so no scenario can reach past it.
func FactsFrom(field encounter.FieldInput) *DungeonFacts {
	facts := &DungeonFacts{
		Props: make(map[encounter.PropID]bool, len(field.Props)),
		Exits: make(map[encounter.ExitID]bool, len(field.Exits)),
	}
	for _, p := range field.Props {
		if p.ID != "" {
			facts.Props[p.ID] = p.Holdable
		}
	}
	for _, ex := range field.Exits {
		facts.Exits[ex.ID] = true
	}
	return facts
}

// registry is every scenario this build knows, by id.
var registry = map[string]Scenario{}

// register adds a scenario to the registry. Called from each scenario's own
// init, so adding a scenario is adding a file.
func register(s Scenario) { registry[s.ID()] = s }

// All returns every scenario this build knows, sorted by id — what an
// authoring service serves as ListScenarios, and what the pinning test runs
// over so a later scenario cannot skip it.
func All() []Scenario {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]Scenario, 0, len(ids))
	for _, id := range ids {
		out = append(out, registry[id])
	}
	return out
}

// Lookup returns the scenario with an id, and whether this build has one.
// A caller that reads `scenarios:` from a dungeon file uses this; an id this
// build does not know is refused by the caller, never guessed at.
func Lookup(id string) (Scenario, bool) {
	s, ok := registry[id]
	return s, ok
}
