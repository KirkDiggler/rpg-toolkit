// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package quest

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

// Predicate is a question asked of derived state.
//
// Unlike graph's reducers and projections, this interface is open. The
// asymmetry is deliberate: a derivation writes the world and must not be
// extensible by content, while a predicate only reads it and cannot lie about
// anything. A host with a question this package does not have may ask its own.
type Predicate interface {
	// Holds answers the question against one observer's derived present.
	Holds(s *graph.State) bool

	// Describe renders the question for a transcript or a quest log. It is
	// prose for humans, never parsed.
	Describe() string
}

// NoEdge holds when a relationship is absent.
//
// This is the shape a method-indifferent objective usually takes. "No longer
// hostile" says nothing about how the hostility ended, so every way of ending
// it counts.
type NoEdge struct {
	From journal.EntityID
	Rel  graph.Relation
	To   journal.EntityID
}

// Holds reports whether the relationship is absent from the observer's present.
func (p NoEdge) Holds(s *graph.State) bool {
	return !s.HasEdge(p.From, p.Rel, p.To)
}

// Describe renders the question.
func (p NoEdge) Describe() string {
	return fmt.Sprintf("%s is not %s %s", p.From, p.Rel, p.To)
}

// HasEdge holds when a relationship is present.
type HasEdge struct {
	From journal.EntityID
	Rel  graph.Relation
	To   journal.EntityID
}

// Holds reports whether the relationship is present in the observer's present.
func (p HasEdge) Holds(s *graph.State) bool {
	return s.HasEdge(p.From, p.Rel, p.To)
}

// Describe renders the question.
func (p HasEdge) Describe() string {
	return fmt.Sprintf("%s is %s %s", p.From, p.Rel, p.To)
}

// Flagged holds when a derived flag is raised on an entity.
type Flagged struct {
	Flag graph.Flag
	Of   journal.EntityID
}

// Holds reports whether the flag is raised in the observer's present.
func (p Flagged) Holds(s *graph.State) bool {
	return s.Flagged(p.Flag, p.Of)
}

// Describe renders the question.
func (p Flagged) Describe() string {
	return fmt.Sprintf("%s is %s", p.Of, p.Flag)
}

// Occupies holds when a named entity holds a role.
//
// Leave Who empty to ask only that the slot is filled by somebody.
type Occupies struct {
	Who  journal.EntityID
	Role graph.Role
	Of   journal.EntityID
}

// Holds reports whether the role is held as asked in the observer's present.
func (p Occupies) Holds(s *graph.State) bool {
	occupant := s.Occupant(p.Role, p.Of)
	if p.Who == "" {
		return occupant != ""
	}

	return occupant == p.Who
}

// Describe renders the question.
func (p Occupies) Describe() string {
	if p.Who == "" {
		return fmt.Sprintf("somebody %s %s", p.Role, p.Of)
	}

	return fmt.Sprintf("%s %s %s", p.Who, p.Role, p.Of)
}

// All holds when every one of its parts holds. An empty All holds.
type All []Predicate

// Holds reports whether every part holds.
func (p All) Holds(s *graph.State) bool {
	for _, part := range p {
		if !part.Holds(s) {
			return false
		}
	}

	return true
}

// Describe renders the question.
func (p All) Describe() string {
	out := ""
	for i, part := range p {
		if i > 0 {
			out += " and "
		}
		out += part.Describe()
	}
	if out == "" {
		return "nothing in particular"
	}

	return out
}
