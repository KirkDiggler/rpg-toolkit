// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

type slotKey struct {
	role Role
	of   journal.EntityID
}

type counterKey struct {
	name   Counter
	of     journal.EntityID
	toward journal.EntityID
}

type flagKey struct {
	name Flag
	of   journal.EntityID
}

type labelKey struct {
	name LabelName
	of   journal.EntityID
}

// State is the present, as one observer holds it.
//
// It is produced by [World.StateFor] and is never stored anywhere: it is a
// value computed from the declaration and the facts the observer witnessed.
// Nothing in this package writes to a state after the fold that built it
// returns.
type State struct {
	observer journal.EntityID
	world    *World

	slots    map[slotKey]journal.EntityID
	counters map[counterKey]int
	flags    map[flagKey]bool
	labels   map[labelKey]string

	edges    []Edge
	refusals []string

	// hiddenEntities are the concealed entities not currently visible to
	// this observer — empty for [World.Truth], which bypasses concealment
	// entirely, and for any world with nothing declared concealed at all.
	hiddenEntities map[journal.EntityID]bool
}

// initialState seeds a fold's starting point: slots as declared, and
// structure filtered by concealment. allFacts is the truth-grain fact list —
// used here to resolve every [Reveal] before a single reducer runs, since a
// reveal has to hold for an observer with zero witnessed facts of their own,
// which the reducer pass below never sees. [World.Truth] passes an empty
// observer and bypasses concealment altogether: nothing is ever hidden from
// it, revealed or not.
func (w *World) initialState(observer journal.EntityID, allFacts []journal.Fact) *State {
	s := &State{
		observer:       observer,
		world:          w,
		slots:          make(map[slotKey]journal.EntityID, len(w.slots)),
		counters:       make(map[counterKey]int),
		flags:          make(map[flagKey]bool),
		labels:         make(map[labelKey]string),
		hiddenEntities: make(map[journal.EntityID]bool),
	}

	for _, slot := range w.slots {
		s.slots[slotKey{role: slot.Role, of: slot.Of}] = slot.Occupant
	}

	truth := observer == ""
	revealedEntities, revealedEdges := w.revealed(allFacts)

	for _, id := range w.order {
		e := w.entities[id]
		if e.Concealed && !truth && !revealedEntities[id] {
			s.hiddenEntities[id] = true
		}
	}

	for _, e := range w.edges {
		if e.Concealed && !truth && !revealedEdges[bareEdge(e)] {
			continue
		}
		s.edges = append(s.edges, bareEdge(e))
	}

	return s
}

// Visible reports whether an entity's own structure is visible in this
// present.
//
// An entity that was never declared concealed is always visible — this is
// the zero-value truth that keeps every world with no concealment behaving
// exactly as it did before this existed. A concealed one is visible once
// [World.Truth] is asking, once a [Reveal] has fired for it on the truth
// grain, or once a [Pierce] has fired for it in this observer's own fold.
func (s *State) Visible(id journal.EntityID) bool {
	return !s.hiddenEntities[id]
}

// Observer names whose present this is. It is empty for [World.Truth].
func (s *State) Observer() journal.EntityID {
	return s.observer
}

// Occupant returns who holds a role for an entity, or the empty id if the slot
// is vacant or was never declared.
func (s *State) Occupant(role Role, of journal.EntityID) journal.EntityID {
	return s.slots[slotKey{role: role, of: of}]
}

// Count returns the value of a counter an entity holds toward another.
//
// A counter nobody has moved is zero, which reads as "no regard either way" —
// the same thing the world starts with.
func (s *State) Count(name Counter, of, toward journal.EntityID) int {
	return s.counters[counterKey{name: name, of: of, toward: toward}]
}

// Flagged reports whether a flag has been raised on an entity.
func (s *State) Flagged(name Flag, of journal.EntityID) bool {
	return s.flags[flagKey{name: name, of: of}]
}

// Label returns a derived label for an entity, or the empty string if no
// declared [Label] covers it.
func (s *State) Label(name LabelName, of journal.EntityID) string {
	return s.labels[labelKey{name: name, of: of}]
}

// HasEdge reports whether a relationship holds right now.
func (s *State) HasEdge(from journal.EntityID, rel Relation, to journal.EntityID) bool {
	return slices.Contains(s.edges, Edge{From: from, Rel: rel, To: to})
}

// Edges returns every current relationship, in a stable order.
func (s *State) Edges() []Edge {
	return slices.Clone(s.edges)
}

// FactionOf returns the group an entity acts for: the far end of the membership
// chain, or the entity itself when it belongs to nobody.
//
// This is what makes a fact about one bandit land on the camp's counters, and
// what makes an impostor from the party lend the party's stance to whatever
// they lead.
func (s *State) FactionOf(id journal.EntityID) journal.EntityID {
	seen := map[journal.EntityID]bool{id: true}
	current := id

	for {
		next, ok := s.oneStepUp(current)
		if !ok || seen[next] {
			return current
		}
		seen[next] = true
		current = next
	}
}

func (s *State) oneStepUp(id journal.EntityID) (journal.EntityID, bool) {
	for _, e := range s.edges {
		if e.Rel == s.world.membership && e.From == id {
			return e.To, true
		}
	}

	return "", false
}

// Refusals returns the declared derivations that declined to fold a fact
// because the structure could not carry it — an [Occupy] naming a role nobody
// declared a slot for, say.
//
// A fold that quietly drops what it cannot apply is the failure mode this
// exists to prevent. In a healthy world the result is empty, and a test may say
// so.
func (s *State) Refusals() []string {
	return slices.Clone(s.refusals)
}

func (s *State) refusef(format string, args ...any) {
	s.refusals = append(s.refusals, fmt.Sprintf(format, args...))
}

func (s *State) hasSlot(role Role, of journal.EntityID) bool {
	_, ok := s.slots[slotKey{role: role, of: of}]

	return ok
}

func (s *State) setOccupant(role Role, of, occupant journal.EntityID) {
	s.slots[slotKey{role: role, of: of}] = occupant
}

// vacateHeldBy empties every slot of the given role currently held by occupant.
func (s *State) vacateHeldBy(role Role, occupant journal.EntityID) bool {
	vacated := false
	for key, held := range s.slots {
		if key.role == role && held == occupant {
			s.slots[key] = ""
			vacated = true
		}
	}

	return vacated
}

// flaggedWith returns every entity carrying a flag, in a stable order.
func (s *State) flaggedWith(name Flag) []journal.EntityID {
	var out []journal.EntityID
	for key, set := range s.flags {
		if set && key.name == name {
			out = append(out, key.of)
		}
	}
	slices.Sort(out)

	return out
}

// adopt replaces target's edges in the given relations with source's.
func (s *State) adopt(target, source journal.EntityID, rels []Relation) {
	inherited := s.edgesFrom(source, rels)
	s.dropEdgesFrom(target, rels)
	for _, e := range inherited {
		s.addEdge(Edge{From: target, Rel: e.Rel, To: e.To})
	}
}

func (s *State) addCount(name Counter, of, toward journal.EntityID, by int) {
	s.counters[counterKey{name: name, of: of, toward: toward}] += by
}

func (s *State) raise(name Flag, of journal.EntityID) {
	s.flags[flagKey{name: name, of: of}] = true
}

func (s *State) setLabel(name LabelName, of journal.EntityID, value string) {
	s.labels[labelKey{name: name, of: of}] = value
}

func (s *State) addEdge(e Edge) {
	if !slices.Contains(s.edges, e) {
		s.edges = append(s.edges, e)
	}
}

func (s *State) removeEdge(e Edge) {
	s.edges = slices.DeleteFunc(s.edges, func(have Edge) bool { return have == e })
}

// dropEdgesFrom removes every edge leaving from in any of the given relations.
func (s *State) dropEdgesFrom(from journal.EntityID, rels []Relation) {
	s.edges = slices.DeleteFunc(s.edges, func(e Edge) bool {
		return e.From == from && slices.Contains(rels, e.Rel)
	})
}

func (s *State) edgesFrom(from journal.EntityID, rels []Relation) []Edge {
	var out []Edge
	for _, e := range s.edges {
		if e.From == from && slices.Contains(rels, e.Rel) {
			out = append(out, e)
		}
	}

	return out
}

// sortedCounters returns counter keys in a stable order so projections apply
// deterministically regardless of map iteration.
func (s *State) sortedCounters() []counterKey {
	keys := make([]counterKey, 0, len(s.counters))
	for k := range s.counters {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b counterKey) int {
		if c := strings.Compare(string(a.name), string(b.name)); c != 0 {
			return c
		}
		if c := strings.Compare(string(a.of), string(b.of)); c != 0 {
			return c
		}

		return strings.Compare(string(a.toward), string(b.toward))
	})

	return keys
}
