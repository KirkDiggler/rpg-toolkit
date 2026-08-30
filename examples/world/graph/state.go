// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph

import (
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
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
}

func (w *World) initialState(observer journal.EntityID) *State {
	s := &State{
		observer: observer,
		world:    w,
		slots:    make(map[slotKey]journal.EntityID, len(w.slots)),
		counters: make(map[counterKey]int),
		flags:    make(map[flagKey]bool),
		labels:   make(map[labelKey]string),
		edges:    slices.Clone(w.edges),
	}

	for _, slot := range w.slots {
		s.slots[slotKey{role: slot.Role, of: slot.Of}] = slot.Occupant
	}

	return s
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
