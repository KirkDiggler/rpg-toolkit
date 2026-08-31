// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// Reveal declares that a fact of this kind ends concealment on named
// structure for good.
//
// This folds on the truth grain, not the audience grain: it is checked
// against every fact ever recorded, not only the ones a particular observer
// witnessed. Perceiving present state is not witnessing past events, so an
// observer who arrives after the reveal — with no witnessed facts of their
// own about it at all — sees the revealed structure exactly as anyone who
// was there does. This is the mechanical difference between a Reveal and a
// [Pierce]: a pierce only ever answers for the observer whose own fold
// contains the piercing fact.
type Reveal struct {
	// On is the fact kind that reveals the structure.
	On journal.Kind

	// When narrows which outcomes count.
	When Condition

	// Entities are the concealed entities this fact reveals.
	Entities []journal.EntityID

	// Edges are the concealed edges this fact reveals. Only From, Rel, and
	// To are read — Concealed is not part of a reference's identity.
	Edges []Edge
}

// Pierce declares that a fact of this kind lets its own audience see
// concealed structure, for themselves alone.
//
// This folds on the audience grain, exactly like every reducer in this
// package: a pierce only ever changes what the fold produces for an
// observer whose own witnessed facts include the piercing fact. A knower's
// party-mate who was not there is unaffected — until a [Reveal] catches up
// with them, or they pierce it themselves.
type Pierce struct {
	// On is the fact kind that pierces the structure.
	On journal.Kind

	// When narrows which outcomes count.
	When Condition

	// Entities are the concealed entities this fact's audience learns of.
	Entities []journal.EntityID

	// Edges are the concealed edges this fact's audience learns of. Only
	// From, Rel, and To are read — Concealed is not part of a reference's
	// identity.
	Edges []Edge
}

// bareEdge strips everything but identity from an edge. A reference to an
// edge — in a [Reveal], a [Pierce], or a live [State] — does not restate
// whether the edge it names is concealed; concealment is a property of the
// declaration, not of a mention.
func bareEdge(e Edge) Edge {
	return Edge{From: e.From, Rel: e.Rel, To: e.To}
}

// pierce applies one Pierce declaration to one witnessed fact, un-hiding
// whatever it names for this fold alone.
func (p Pierce) pierce(f journal.Fact, s *State) {
	if f.Kind != p.On || !p.When.holds(f.Outcome) {
		return
	}
	for _, id := range p.Entities {
		delete(s.hiddenEntities, id)
	}
	for _, e := range p.Edges {
		s.addEdge(bareEdge(e))
	}
}

// revealed scans every fact ever recorded — audience ignored, the truth
// grain a [Reveal] folds on — and returns the concealed structure that has
// ever been revealed by one.
func (w *World) revealed(allFacts []journal.Fact) (map[journal.EntityID]bool, map[Edge]bool) {
	entities := make(map[journal.EntityID]bool)
	edges := make(map[Edge]bool)

	for _, f := range allFacts {
		for _, rv := range w.reveals {
			if f.Kind != rv.On || !rv.When.holds(f.Outcome) {
				continue
			}
			for _, id := range rv.Entities {
				entities[id] = true
			}
			for _, e := range rv.Edges {
				edges[bareEdge(e)] = true
			}
		}
	}

	return entities, edges
}

// ErrUnknownConcealedEntity reports a [Reveal] or [Pierce] naming an entity
// that was never declared, or was declared but never marked concealed.
var ErrUnknownConcealedEntity = errors.New(
	"graph: this reveal or pierce names an entity that is not declared concealed — check the id, or mark " +
		"the entity concealed if it should be")

// ErrUnknownConcealedEdge reports a [Reveal] or [Pierce] naming an edge that
// was never declared, or was declared but never marked concealed.
var ErrUnknownConcealedEdge = errors.New(
	"graph: this reveal or pierce names an edge that is not declared concealed — check it, or mark the " +
		"edge concealed if it should be")

// adoptConceal validates and stores the declared reveals and pierces.
//
// Validation is eager, the same as entities and edges: a reveal naming
// structure nobody declared concealed is a form the author has not finished,
// not a runtime surprise waiting in some fold nobody triggered yet.
func (w *World) adoptConceal(reveals []Reveal, pierces []Pierce) error {
	for _, rv := range reveals {
		if err := w.knownConcealed(rv.Entities, rv.Edges); err != nil {
			return fmt.Errorf("reveal %q: %w", rv.On, err)
		}
	}
	for _, p := range pierces {
		if err := w.knownConcealed(p.Entities, p.Edges); err != nil {
			return fmt.Errorf("pierce %q: %w", p.On, err)
		}
	}

	w.reveals = append([]Reveal(nil), reveals...)
	w.pierces = append([]Pierce(nil), pierces...)

	return nil
}

func (w *World) knownConcealed(entities []journal.EntityID, edges []Edge) error {
	for _, id := range entities {
		e, ok := w.entities[id]
		if !ok || !e.Concealed {
			return fmt.Errorf("%w: %q", ErrUnknownConcealedEntity, id)
		}
	}
	for _, ref := range edges {
		if !w.hasConcealedEdge(ref) {
			return fmt.Errorf("%w: %q %s %q", ErrUnknownConcealedEdge, ref.From, ref.Rel, ref.To)
		}
	}

	return nil
}

func (w *World) hasConcealedEdge(ref Edge) bool {
	want := bareEdge(ref)
	for _, e := range w.edges {
		if bareEdge(e) == want && e.Concealed {
			return true
		}
	}

	return false
}
