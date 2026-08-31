// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package quest

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// InstanceSubject stands in for whichever individual an instance is about.
//
// One template, many instances, each minted about a different person: a
// predicate written against this sentinel reads "my hostage is free", not
// "Deryn is free". Authors write the template once and the mint supplies the
// name — which is the whole of instance isolation, and the reason three
// parties working the same job never collide.
const InstanceSubject journal.EntityID = "quest:instance-subject"

// Bindings are what a predicate is asked against: one observer's derived
// present, and the individual this instance is about.
//
// The state is somebody's state — objectives are read in a named observer's
// view, so a predicate never sees a truth nobody holds.
type Bindings struct {
	// State is the observer's derived present.
	State *graph.State

	// Subject is the individual this instance was minted about, substituted
	// wherever a predicate names [InstanceSubject].
	Subject journal.EntityID
}

// Resolve substitutes the instance's subject for [InstanceSubject], and
// returns any other id unchanged.
//
// Custom predicates must call it on every entity id they name, or they will
// silently ask about an entity that does not exist.
func (b Bindings) Resolve(id journal.EntityID) journal.EntityID {
	if id == InstanceSubject {
		return b.Subject
	}

	return id
}

// Predicate is a question asked of derived state.
//
// Unlike graph's reducers and projections, this interface is open. The
// asymmetry is deliberate: a derivation writes the world and must not be
// extensible by content, while a predicate only reads it and cannot lie about
// anything. A host with a question this package does not have may ask its own.
type Predicate interface {
	// Holds answers the question against one observer's derived present.
	Holds(b Bindings) bool

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
func (p NoEdge) Holds(b Bindings) bool {
	return !b.State.HasEdge(b.Resolve(p.From), p.Rel, b.Resolve(p.To))
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
func (p HasEdge) Holds(b Bindings) bool {
	return b.State.HasEdge(b.Resolve(p.From), p.Rel, b.Resolve(p.To))
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
func (p Flagged) Holds(b Bindings) bool {
	return b.State.Flagged(p.Flag, b.Resolve(p.Of))
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
func (p Occupies) Holds(b Bindings) bool {
	occupant := b.State.Occupant(p.Role, b.Resolve(p.Of))
	if p.Who == "" {
		return occupant != ""
	}

	return occupant == b.Resolve(p.Who)
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
func (p All) Holds(b Bindings) bool {
	for _, part := range p {
		if !part.Holds(b) {
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

// Any holds when at least one of its parts holds. An empty Any does not hold.
//
// It is what a job with two acceptable endings is written with: turn them back,
// or put them down, and the quest does not care which.
type Any []Predicate

// Holds reports whether any part holds.
func (p Any) Holds(b Bindings) bool {
	for _, part := range p {
		if part.Holds(b) {
			return true
		}
	}

	return false
}

// Describe renders the question.
func (p Any) Describe() string {
	out := ""
	for i, part := range p {
		if i > 0 {
			out += " or "
		}
		out += part.Describe()
	}
	if out == "" {
		return "nothing at all"
	}

	return out
}

// Anything holds always.
//
// Its use is as the last [Bucket] in a classification: the state an instance is
// in when none of the interesting things have happened to it yet. Writing that
// down beats leaving it implicit, because "captive" is a real answer and not an
// absence of one.
type Anything struct{}

// Holds always reports true.
func (Anything) Holds(Bindings) bool {
	return true
}

// Describe renders the question.
func (Anything) Describe() string {
	return "anything"
}
