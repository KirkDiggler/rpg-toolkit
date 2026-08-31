// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// Kind classifies an entity — "faction", "person". Projections that apply to a
// whole class of thing name a kind rather than listing ids.
type Kind string

// Relation names a typed edge — "hostile-to", "belongs-to", "allied-with".
type Relation string

// Role names a position one entity occupies for another — "leads". A role is a
// slot: at most one occupant at a time, and facts are the only thing that move
// who is in it.
type Role string

// Counter names a tallied quantity derived from facts — "regard". A counter is
// held by an entity and pointed at another, so "the camp's regard for the
// party" is one counter and "the party's regard for the camp" is a different
// one.
type Counter string

// Flag names a derived boolean — "alerted", "defeated". A flag is raised by
// facts and never cleared, because unwitnessing something is not a thing that
// happens.
type Flag string

// LabelName names a derived string — "posture". Labels exist so behaviour can
// read a word rather than recompute a rule.
type LabelName string

// Grain records the audience grain an entity is addressed at by default.
type Grain int

const (
	// GrainGroup means facts are normally audienced to the entity as a whole,
	// and every member folds them. This is the default: the camp witnesses as
	// a unit.
	GrainGroup Grain = iota

	// GrainIndividual means the entity is named in audiences on its own, so a
	// fact can reach it and no one else. Individual grain is what makes the
	// lieutenant who sees through a disguise possible.
	GrainIndividual
)

// Entity is a declared thing in the world.
type Entity struct {
	ID    journal.EntityID
	Kind  Kind
	Grain Grain

	// Concealed marks the entity hidden from every observer's structural
	// view until a [Reveal] ends that for good or a [Pierce] ends it for one
	// observer. False by default: nothing is concealed unless a scenario
	// says so, and a world with no concealment behaves exactly as it always
	// has.
	Concealed bool
}

// Edge is a declared typed relationship. Edges are structure, but they are not
// fixed: projections replace them as the fold demands.
type Edge struct {
	From journal.EntityID
	Rel  Relation
	To   journal.EntityID

	// Concealed marks the edge hidden the same way [Entity.Concealed] does.
	// The physical connection is real from the moment the world exists —
	// concealment hides knowledge of it, not the edge's existence in
	// [World.Truth].
	Concealed bool
}

// Slot is a declared role together with whoever starts out in it.
//
// A slot with an empty Occupant is a real declaration: the position exists and
// is vacant. Facts fill and empty it from there.
type Slot struct {
	Role     Role
	Of       journal.EntityID
	Occupant journal.EntityID
}

// Config is the whole declaration a world is built from.
type Config struct {
	// Entities are every thing the world knows about. Edges and slots may only
	// name ids declared here.
	Entities []Entity

	// Edges are the starting relationships.
	Edges []Edge

	// Slots are the starting role occupancies.
	Slots []Slot

	// Membership is the relation that makes one entity part of another, and it
	// is required. It answers two questions the fold cannot proceed without:
	// which groups an observer folds the facts of, and whose faction an actor
	// is acting for. There is no default — a world that does not say how
	// belonging works is a world whose audiences and allegiances are guesses.
	Membership Relation

	// Reducers fold witnessed facts into derived cells, in this order, once per
	// fact.
	Reducers []Reducer

	// Projections rewrite the folded state, in this order, once per fold.
	Projections []Projection

	// Reveals declare which facts end concealment on named structure for
	// good, on the truth grain. See [Reveal].
	Reveals []Reveal

	// Pierces declare which facts let their own audience see concealed
	// structure, on the audience grain. See [Pierce].
	Pierces []Pierce
}

// ErrNoMembership reports a Config with no Membership relation.
var ErrNoMembership = errors.New("graph: Config.Membership is required")

// ErrDuplicateEntity reports the same entity id declared twice.
var ErrDuplicateEntity = errors.New("graph: entity declared twice")

// ErrDuplicateSlot reports the same role declared twice for one entity.
var ErrDuplicateSlot = errors.New("graph: slot declared twice")

// ErrUnknownEntity reports an edge or slot naming an entity that was never
// declared.
var ErrUnknownEntity = errors.New("graph: undeclared entity")

// World is the declaration. It holds structure and derivation rules, and no
// present state at all — see [World.StateFor].
type World struct {
	entities   map[journal.EntityID]Entity
	order      []journal.EntityID
	edges      []Edge
	slots      []Slot
	membership Relation

	reducers    []Reducer
	projections []Projection
	reveals     []Reveal
	pierces     []Pierce
}

// New validates a declaration and returns the world it describes.
//
// Returns [ErrNoMembership], [ErrDuplicateEntity], [ErrDuplicateSlot],
// [ErrUnknownEntity], [ErrUnknownConcealedEntity], or [ErrUnknownConcealedEdge],
// each wrapped with what was wrong. Validation is strict on purpose: an edge
// pointing at an entity nobody declared would fold into derived state that
// silently mentions a thing that does not exist.
func New(cfg Config) (*World, error) {
	if cfg.Membership == "" {
		return nil, ErrNoMembership
	}

	w := &World{
		entities:    make(map[journal.EntityID]Entity, len(cfg.Entities)),
		membership:  cfg.Membership,
		reducers:    append([]Reducer(nil), cfg.Reducers...),
		projections: append([]Projection(nil), cfg.Projections...),
	}

	for _, e := range cfg.Entities {
		if _, seen := w.entities[e.ID]; seen {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateEntity, e.ID)
		}
		w.entities[e.ID] = e
		w.order = append(w.order, e.ID)
	}

	if err := w.adoptEdges(cfg.Edges); err != nil {
		return nil, err
	}
	if err := w.adoptSlots(cfg.Slots); err != nil {
		return nil, err
	}
	if err := w.adoptConceal(cfg.Reveals, cfg.Pierces); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *World) adoptEdges(edges []Edge) error {
	for _, e := range edges {
		if err := w.known(e.From, e.To); err != nil {
			return fmt.Errorf("edge %q: %w", e.Rel, err)
		}
		w.edges = append(w.edges, e)
	}

	return nil
}

func (w *World) adoptSlots(slots []Slot) error {
	seen := make(map[Slot]bool, len(slots))
	for _, s := range slots {
		key := Slot{Role: s.Role, Of: s.Of}
		if seen[key] {
			return fmt.Errorf("%w: %q of %q", ErrDuplicateSlot, s.Role, s.Of)
		}
		seen[key] = true

		if err := w.known(s.Of); err != nil {
			return fmt.Errorf("slot %q: %w", s.Role, err)
		}
		if s.Occupant != "" {
			if err := w.known(s.Occupant); err != nil {
				return fmt.Errorf("slot %q occupant: %w", s.Role, err)
			}
		}
		w.slots = append(w.slots, s)
	}

	return nil
}

func (w *World) known(ids ...journal.EntityID) error {
	for _, id := range ids {
		if _, ok := w.entities[id]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownEntity, id)
		}
	}

	return nil
}

// Entity returns a declared entity and whether it was declared.
func (w *World) Entity(id journal.EntityID) (Entity, bool) {
	e, ok := w.entities[id]

	return e, ok
}

// Membership returns the relation this world means by belonging.
func (w *World) Membership() Relation {
	return w.membership
}

// StateFor derives the present as one observer holds it.
//
// The observer folds every fact audienced to it or to any group it belongs to,
// in Seq order, and nothing else. Two observers looking at the same journal get
// two different presents whenever the audiences differ — which is the point.
//
// Concealed structure is the one exception to "audience only": a [Reveal]
// folds on the truth grain, so this observer sees revealed structure even
// with zero witnessed facts of their own about it. See [State.Visible].
//
// Nothing is cached. Call it twice and the same state is computed twice; append
// a fact and the next call reflects it; truncate the journal and the state goes
// back to what it was.
func (w *World) StateFor(observer journal.EntityID, log *journal.Journal) *State {
	return w.fold(observer, log.WitnessedBy(w.AudienceOf(observer)...), log.All())
}

// Truth derives the present from every fact, audience ignored.
//
// This is the game master's view and the test's view. It is never how anyone in
// the world behaves — behaviour reads [World.StateFor], or the quiet kill would
// not work. Concealment is bypassed entirely here, revealed or not: the GM
// sees everything.
func (w *World) Truth(log *journal.Journal) *State {
	all := log.All()

	return w.fold("", all, all)
}

// AudienceOf returns the observer together with every group it belongs to,
// transitively, following the declared membership relation.
//
// This is the set to test a fact's audience against: a fact witnessed by the
// camp is witnessed by every bandit in it.
func (w *World) AudienceOf(observer journal.EntityID) []journal.EntityID {
	if observer == "" {
		return nil
	}

	out := []journal.EntityID{observer}
	seen := map[journal.EntityID]bool{observer: true}

	for i := 0; i < len(out); i++ {
		for _, e := range w.edges {
			if e.Rel != w.membership || e.From != out[i] || seen[e.To] {
				continue
			}
			seen[e.To] = true
			out = append(out, e.To)
		}
	}

	return out
}

// fold builds the present. facts is what the reducers and pierces see —
// the observer's own witnessed set for [World.StateFor], everything for
// [World.Truth]. allFacts is always everything: reveals fold on the truth
// grain regardless of who is asking, which is the one place these two lists
// have to differ.
func (w *World) fold(observer journal.EntityID, facts, allFacts []journal.Fact) *State {
	s := w.initialState(observer, allFacts)

	for _, f := range facts {
		for _, r := range w.reducers {
			r.reduce(f, s)
		}
		for _, p := range w.pierces {
			p.pierce(f, s)
		}
	}
	for _, p := range w.projections {
		p.project(s)
	}

	return s
}
