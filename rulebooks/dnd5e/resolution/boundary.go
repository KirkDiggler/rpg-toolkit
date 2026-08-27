// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// BoundaryInput names the temporal boundaries one clock advance crossed.
//
// Data at the seam (R2), and the composition's own [encounter.Boundary] rather
// than a copy of it: this package already takes that module's EncounterData
// whole, and a second shape for the same three fields would be one more place
// for the two to disagree.
type BoundaryInput struct {
	// Crossed is the boundaries, in the causal order the composition crossed
	// them. Required and non-empty — see [NewBoundary].
	Crossed []encounter.Boundary
}

// BoundaryOutcome is what a boundary interaction produced.
//
// Deliberately thin. A boundary's real output is not this value at all: it is
// the DIRTY SHEETS that come back on [Output], because everything a boundary
// does happens inside a condition deciding its own turn is over. Announced is
// here so a caller can tell "the interaction ran and published four things"
// from "the interaction ran and published nothing", which is otherwise
// invisible on a verb that changes no hit points and spends no economy.
type BoundaryOutcome struct {
	// Announced is how many boundaries were published on the bus.
	Announced int
}

func (BoundaryOutcome) isOutcome() {}

// NewBoundary returns the machine that publishes one clock advance's
// boundaries.
//
// Every attached effect hears them on the interaction's own bus, so a
// condition scoped to a turn — dodging, disengaging, raging, reckless attack,
// sneak-attack-used, a death save waiting on its owner's turn — gets its
// chance to expire, tick, or fire, and comes back dirty.
//
// # A boundary is an interaction, and that is the whole shape
//
// The composition NOTICES the boundary; it cannot publish one, because it
// holds no bus (ADR-0038) and the leaf below it *"returns its results as
// values, and never publishes to a bus"*. This package is the only place a bus
// exists. So the composition hands the boundary out through its Announcer
// capability, its host turns that into a call here, and everyone attached hears
// it — which is the same journey a swing already makes, with time as the
// declaring actor instead of a player.
//
// # No new step kind
//
// A crossing is "do this on the bus and hand me the next step", which is what
// [Gather] already does for the contest's imposition. Adding a Step case
// against one more example is the mistake [ADR-0007] exists to remember.
//
// It does move the tally this package's own doc keeps, and that is recorded
// rather than left for someone to rediscover — see "The bus-effect tally".
//
// # Refuses an empty set
//
// An advance that crossed nothing is not an interaction, and running one would
// load every sheet, attach every effect, publish nothing and hand back a clean
// world — an expensive way to do nothing, and a resolution in the registration
// list that no boundary explains. The composition already declines to announce
// an empty set; this refuses one for the same reason from the other side.
func NewBoundary(in *BoundaryInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if len(in.Crossed) == 0 {
		return nil, fmt.Errorf("%w: no boundaries to announce", ErrBadBoundary)
	}
	for i, b := range in.Crossed {
		if b.Subject == "" {
			return nil, fmt.Errorf("%w: boundary %d has no subject", ErrBadBoundary, i)
		}
		if _, ok := boundaryTopics[b.Kind]; !ok {
			return nil, fmt.Errorf("%w: boundary %d has unknown kind %q", ErrBadBoundary, i, b.Kind)
		}
	}
	crossed := make([]encounter.Boundary, len(in.Crossed))
	copy(crossed, in.Crossed)
	return &boundaryMachine{crossed: crossed}, nil
}

// boundaryTopics is the whole map from a crossing to what it publishes.
//
// A SEALED LOOKUP rather than a switch with a default, so a boundary kind this
// build does not know is refused by [NewBoundary] at the door instead of
// silently publishing nothing. The composition's kind set and this one are the
// same set by construction: if one grows, this refuses until the other does.
var boundaryTopics = map[encounter.BoundaryKind]func(
	ctx context.Context, bus events.EventBus, b encounter.Boundary,
) error{
	encounter.TurnStarted: func(ctx context.Context, bus events.EventBus, b encounter.Boundary) error {
		return dnd5eEvents.TurnStartTopic.On(bus).Publish(ctx, dnd5eEvents.TurnStartEvent{
			SubjectID: string(b.Subject),
			Round:     b.Round,
		})
	},
	encounter.TurnEnded: func(ctx context.Context, bus events.EventBus, b encounter.Boundary) error {
		return dnd5eEvents.TurnEndTopic.On(bus).Publish(ctx, dnd5eEvents.TurnEndEvent{
			SubjectID: string(b.Subject),
			Round:     b.Round,
		})
	},
	// No Round on the way out, and that is not an omission. A fight ending is
	// not a coordinate in a clock that no longer exists — play/clock's
	// Turn.Dissolve sets the round back to zero because round numbers are
	// per-fight. Boundary.Round still says WHICH round it ended on, for the
	// record; there is simply nothing on the event for it to become, and
	// inventing a field so the mapping looks symmetrical would be inventing a
	// number subscribers could compare against a later fight's.
	encounter.CombatEnded: func(ctx context.Context, bus events.EventBus, b encounter.Boundary) error {
		return dnd5eEvents.CombatEndTopic.On(bus).Publish(ctx, dnd5eEvents.CombatEndEvent{
			SubjectID: string(b.Subject),
		})
	},
}

type boundaryMachine struct {
	crossed []encounter.Boundary
}

// Start is pure preflight: it validates nothing further (NewBoundary already
// refused what it could) and yields the first crossing without publishing.
func (m *boundaryMachine) Start(_ context.Context, _ *Participants) (Step, error) {
	return m.at(0), nil
}

// at yields the step that publishes crossing i, or Done when they are exhausted.
//
// One step per crossing rather than one step publishing all of them: each is a
// separate thing that happened, and a step log that reads "end alice's turn"
// then "start bob's turn" is the record the composition's causal ordering was
// preserved for.
func (m *boundaryMachine) at(i int) Step {
	if i >= len(m.crossed) {
		return Done{Outcome: BoundaryOutcome{Announced: len(m.crossed)}}
	}
	b := m.crossed[i]
	publish := boundaryTopics[b.Kind]
	return Gather{
		name: fmt.Sprintf("announce %s for %s", b.Kind, b.Subject),
		run: func(ctx context.Context, bus events.EventBus) (Step, error) {
			if err := publish(ctx, bus, b); err != nil {
				return nil, fmt.Errorf("announce %s for %q: %w", b.Kind, b.Subject, err)
			}
			return m.at(i + 1), nil
		},
	}
}
