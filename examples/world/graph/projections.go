// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

// Projection rewrites folded state once, after every witnessed fact has been
// reduced.
//
// Projections run in declared order and each sees what the ones before it left.
// The order is the author's statement about precedence, not an implementation
// detail: put [FollowSlot] before [Threshold] and a puppet leader's allegiance
// settles before goodwill is weighed against it.
//
// Sealed for the same reason [Reducer] is.
type Projection interface {
	project(s *State)
}

// FollowSlot declares that a group's stance is whoever holds its slot.
//
// After the fold, the group's edges in Relations are replaced by those of the
// faction the occupant acts for. A leader who belongs to the group they lead
// changes nothing, which is why a camp under its own chief simply keeps its
// declared hostilities. Replace the occupant with somebody from the party and
// the camp inherits the party's stances instead — allegiance follows the
// leader, and no code anywhere knows what a coup is.
//
// A vacant slot leaves the group's declared edges alone. That is what an
// observer who has seen through a disguise ends up looking at.
type FollowSlot struct {
	// Role is the slot whose occupant lends their stance.
	Role Role

	// Relations are the stance edges that follow. Membership must not be one of
	// them: a camp does not join the party's faction because it changed hands.
	Relations []Relation
}

func (p FollowSlot) project(s *State) {
	for _, slot := range s.world.slots {
		if slot.Role != p.Role {
			continue
		}

		occupant := s.Occupant(p.Role, slot.Of)
		if occupant == "" {
			continue
		}

		source := s.FactionOf(occupant)
		if source == slot.Of {
			continue
		}

		inherited := s.edgesFrom(source, p.Relations)
		s.dropEdgesFrom(slot.Of, p.Relations)
		for _, e := range inherited {
			s.addEdge(Edge{From: slot.Of, Rel: e.Rel, To: e.To})
		}
	}
}

// Threshold declares the point at which accumulated regard becomes a different
// relationship.
//
// When an entity's counter toward another reaches At, an edge of relation From
// between them becomes one of relation To. Below the threshold nothing happens
// and no partial state is recorded anywhere — the counter is the only thing
// that moved, and it is itself derived.
//
// The From edge must be present. A threshold does not invent a relationship
// that was never there; it converts one.
type Threshold struct {
	// Counter is the tally being weighed.
	Counter Counter

	// At is the value the counter must reach.
	At int

	// From is the relation that gets replaced.
	From Relation

	// To is what it becomes.
	To Relation
}

func (p Threshold) project(s *State) {
	for _, key := range s.sortedCounters() {
		if key.name != p.Counter || s.counters[key] < p.At {
			continue
		}
		old := Edge{From: key.of, Rel: p.From, To: key.toward}
		if !s.HasEdge(old.From, old.Rel, old.To) {
			continue
		}
		s.removeEdge(old)
		s.addEdge(Edge{From: key.of, Rel: p.To, To: key.toward})
	}
}

// Retire declares that an entity carrying a flag stops holding certain
// relations.
//
// It is how a defeated camp stops being hostile without anyone writing down
// that defeat means peace. The objective asks whether the camp is still
// hostile; assault, conversion, and a change of leadership all answer it, and
// this is the answer defeat gives.
type Retire struct {
	// OnFlag is the flag that retires the edges.
	OnFlag Flag

	// Relations are the edges the flagged entity loses.
	Relations []Relation
}

func (p Retire) project(s *State) {
	var flagged []journal.EntityID
	for key, set := range s.flags {
		if set && key.name == p.OnFlag {
			flagged = append(flagged, key.of)
		}
	}
	slices.Sort(flagged)

	for _, id := range flagged {
		s.dropEdgesFrom(id, p.Relations)
	}
}

// Label declares a derived word for every entity of a kind, chosen by a flag.
//
// Behaviour reads the word. Whether a camp meets an intruder formed up or
// scrambling is not stored anywhere and was never decided by a verb: it is what
// the camp's own fold says about whether it ever heard anything.
type Label struct {
	// Name is the label being derived.
	Name LabelName

	// Of restricts the label to entities of this kind.
	Of Kind

	// WhenFlag chooses between Then and Else.
	WhenFlag Flag

	// Then is the label for entities carrying the flag.
	Then string

	// Else is the label for entities without it.
	Else string
}

func (p Label) project(s *State) {
	for _, id := range s.world.order {
		if s.world.entities[id].Kind != p.Of {
			continue
		}
		if s.Flagged(p.WhenFlag, id) {
			s.setLabel(p.Name, id, p.Then)

			continue
		}
		s.setLabel(p.Name, id, p.Else)
	}
}
