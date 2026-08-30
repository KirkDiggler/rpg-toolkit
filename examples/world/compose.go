// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package world

import (
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

// ErrNothingToCompose reports a composition with no parts.
var ErrNothingToCompose = errors.New("give this at least one piece of content to compose")

// ErrMembershipDisagrees reports pieces of content that mean different things by
// belonging.
var ErrMembershipDisagrees = errors.New(
	"these pieces of content do not agree on what belonging means — they all have to use the same " +
		"membership relation, because it is how news reaches a group's members and how a faction's " +
		"allegiance reaches the people in it")

// ErrCollision reports two pieces of content laying claim to the same name.
var ErrCollision = errors.New(
	"two pieces of content declare this same name — a region can only have one of each, so rename it in " +
		"one of them")

// Compose merges declared content into one region.
//
// A region is not a bigger scenario; it is several, side by side, sharing one
// journal. That sharing is the point: a goal over the whole region is a fold
// over one log, so a company solving the camp and a company freeing hostages
// are pushing the same needle without anything having to add their
// contributions together.
//
// Everything concatenates in the order given, which matters for reducers and
// projections, where order is precedence. Names may not collide: entities,
// verbs and jobs are refused if two pieces of content claim the same one.
//
// Returns [ErrNothingToCompose], [ErrMembershipDisagrees], or [ErrCollision].
func Compose(parts ...Scenario) (Scenario, error) {
	if len(parts) == 0 {
		return Scenario{}, ErrNothingToCompose
	}

	out := Scenario{Graph: graph.Config{Membership: parts[0].Graph.Membership}}

	entities := make(map[journal.EntityID]bool)
	verbs := make(map[VerbName]bool)
	jobs := make(map[string]bool)

	for i, part := range parts {
		if part.Graph.Membership != out.Graph.Membership {
			return Scenario{}, fmt.Errorf("%w: %q and %q",
				ErrMembershipDisagrees, out.Graph.Membership, part.Graph.Membership)
		}
		if err := adopt(&out, part, entities, verbs, jobs); err != nil {
			return Scenario{}, fmt.Errorf("piece %d: %w", i+1, err)
		}
	}

	return out, nil
}

func adopt(
	out *Scenario, part Scenario,
	entities map[journal.EntityID]bool, verbs map[VerbName]bool, jobs map[string]bool,
) error {
	for _, e := range part.Graph.Entities {
		if entities[e.ID] {
			return fmt.Errorf("%w: entity %q", ErrCollision, e.ID)
		}
		entities[e.ID] = true
	}
	for _, v := range part.Verbs {
		if verbs[v.Name] {
			return fmt.Errorf("%w: action %q", ErrCollision, v.Name)
		}
		verbs[v.Name] = true
	}
	for _, q := range part.Quests {
		if jobs[q.ID] {
			return fmt.Errorf("%w: job %q", ErrCollision, q.ID)
		}
		jobs[q.ID] = true
	}

	out.Graph.Entities = append(out.Graph.Entities, part.Graph.Entities...)
	out.Graph.Edges = append(out.Graph.Edges, part.Graph.Edges...)
	out.Graph.Slots = append(out.Graph.Slots, part.Graph.Slots...)
	out.Graph.Reducers = append(out.Graph.Reducers, part.Graph.Reducers...)
	out.Graph.Projections = append(out.Graph.Projections, part.Graph.Projections...)
	out.Verbs = append(out.Verbs, part.Verbs...)
	out.Quests = append(out.Quests, part.Quests...)

	return nil
}

// Ties is content that adds nothing of its own but connects what is already
// there — the edges that make three companies parts of one guild, say.
//
// It is a [Scenario] because that is what the composer takes, and stating the
// membership relation is not ceremony: ties are edges, and an edge whose
// relation nobody agreed on is how a region quietly stops sharing audiences.
func Ties(membership graph.Relation, edges ...graph.Edge) Scenario {
	return Scenario{
		Graph: graph.Config{Membership: membership, Edges: slices.Clone(edges)},
	}
}
