// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package scenarios

import (
	"errors"
	"fmt"
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

	// Convince is the faction the party is there to turn, when this
	// scenario binds one (rpg-project#375). Empty otherwise.
	Convince encounter.FactionID
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

	// Factions is every faction this dungeon has — the two reserved ones and
	// the declared — to what a scenario may ask of it (rpg-project#375).
	Factions map[encounter.FactionID]FactionFacts

	// Reveals is every fact some placed record reveals — the other half of
	// "a hold-out nobody can win".
	Reveals map[encounter.FactID]bool
}

// FactionFacts is what a scenario may ask about one faction.
type FactionFacts struct {
	// CanLearn reports whether the faction has a mind to come to know a
	// fact through: a declared one, or a faction of one, whose sole member
	// is its mind by rule. `party` never can.
	CanLearn bool

	// UntilFact is, per other faction, the fact whose knowledge ends this
	// faction's hostility to it — every disposition declared hostile with an
	// `until: { fact }` that names this faction.
	UntilFact map[encounter.FactionID]encounter.FactID
}

// FactsFrom builds the facts a scenario may ask about, from a compiled
// dungeon's field. The ONE place a Compiled is narrowed to what a scenario
// sees, so no scenario can reach past it.
//
// memberFactions is the faction of each MONSTER placement, in any order —
// the authored word, or "" for one the author put nowhere, which is
// `monsters` (rpg-project#375). The field carries no placements, and the
// faction-of-one rule needs to count them; a caller with none to report
// leaves the list out and every faction is judged by its declared mind
// alone.
func FactsFrom(field encounter.FieldInput, memberFactions ...encounter.FactionID) *DungeonFacts {
	facts := &DungeonFacts{
		Props:    make(map[encounter.PropID]bool, len(field.Props)),
		Exits:    make(map[encounter.ExitID]bool, len(field.Exits)),
		Factions: make(map[encounter.FactionID]FactionFacts, 2+len(field.Factions)),
		Reveals:  make(map[encounter.FactID]bool),
	}

	members := make(map[encounter.FactionID]int, len(memberFactions))
	for _, f := range memberFactions {
		if f == "" {
			f = encounter.FactionMonsters
		}
		members[f]++
	}
	declaredMind := make(map[encounter.FactionID]bool, len(field.Factions))
	for _, fa := range field.Factions {
		declaredMind[fa.ID] = fa.Mind != ""
	}
	consider := func(id encounter.FactionID) {
		if _, seen := facts.Factions[id]; seen {
			return
		}
		facts.Factions[id] = FactionFacts{
			CanLearn:  id != encounter.FactionParty && (declaredMind[id] || members[id] == 1),
			UntilFact: make(map[encounter.FactionID]encounter.FactID),
		}
	}
	consider(encounter.FactionParty)
	consider(encounter.FactionMonsters)
	for _, fa := range field.Factions {
		consider(fa.ID)
	}
	for _, d := range field.Dispositions {
		t, ok := d.Until.(encounter.TriggerFact)
		if !ok || d.Stance != encounter.StanceHostile {
			continue
		}
		for i, id := range d.Between {
			other := d.Between[1-i]
			if ff, has := facts.Factions[id]; has {
				ff.UntilFact[other] = t.Fact
			}
		}
	}
	for _, rec := range field.Intel {
		if rec.Reveals.Fact != "" {
			facts.Reveals[rec.Reveals.Fact] = true
		}
	}
	for _, p := range field.Props {
		if p.ID != "" {
			facts.Props[p.ID] = p.Holdable
		}
	}
	for _, ex := range field.Exits {
		// An unnamed exit is not bindable, for the reason an unnamed prop is
		// not: a scenario binds by id, and "" is not an id. compileExits
		// already refuses one, so this is the narrowing staying consistent
		// with the props branch above rather than a second guard against a
		// state the composition can reach (Copilot, PR #1499 review).
		if ex.ID != "" {
			facts.Exits[ex.ID] = true
		}
	}
	return facts
}

// registry is every scenario this build knows, by id.
var registry = map[string]Scenario{}

// register adds a scenario to the registry. Called from each scenario's own
// init, so adding a scenario is adding a file.
//
// PANICS ON AN EMPTY OR DUPLICATE ID, at package init, before anything can
// run (Copilot, PR #1499 review). A map assignment would have silently
// overwritten the first scenario with the second, and — the part that makes
// it worth a panic — NO TEST COULD HAVE CAUGHT IT: [All] walks the map, so a
// key collision means the loser never appears, and a uniqueness check over
// All() is a check over the survivors. The scenario would simply not exist
// at runtime, with nothing anywhere saying so.
//
// A panic rather than an error because there is no caller to hand one to: an
// init function has nowhere to return, and a scenario package that cannot
// register is a build that must not start.
func register(s Scenario) {
	if err := registerInto(registry, s); err != nil {
		panic(fmt.Sprintf("scenarios: %v", err))
	}
}

// registerInto is register's decision, separated from its panic so the rules
// can be exercised by a test rather than asserted about a process that has
// already died.
func registerInto(into map[string]Scenario, s Scenario) error {
	if s == nil {
		return errors.New("a nil scenario cannot be registered")
	}
	id := s.ID()
	if id == "" {
		return errors.New("a scenario with no id cannot be registered")
	}
	if prev, dup := into[id]; dup {
		return fmt.Errorf("scenario %q is already registered by %q — one id, one scenario", id, prev.Name())
	}
	into[id] = s
	return nil
}

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
