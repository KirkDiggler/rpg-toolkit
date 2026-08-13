// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MoveInput walks a member along a path within their current room.
type MoveInput struct {
	// Session is the session to act in.
	Session string

	// Member is the member walking.
	Member string

	// Path is the cells to walk through, in order, each adjacent to the last
	// and the first adjacent to where the member currently stands.
	//
	// A path, not a destination. The caller says where to walk; what actually
	// happened comes back as Steps, which may be shorter. A single-cell path is
	// the ordinary case and entirely legal.
	Path []spatial.Position
}

// Step is one cell actually entered.
type Step struct {
	// Position is the cell entered.
	Position spatial.Position `json:"position"`

	// Seq is the story sequence of the recorded step.
	Seq uint64 `json:"seq"`
}

// MoveOutput reports how far the member got and what it revealed.
type MoveOutput struct {
	// Steps is what actually happened, in order.
	//
	// Shorter than the requested Path means the walk stopped early, and the
	// reason is in Outcome. This is deliberately not an error: the walk did
	// what it could, and where it got to is the answer.
	Steps []Step `json:"steps,omitempty"`

	// Discovered is what changed in each observer's perception across the whole
	// walk, keyed by observer.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Outcome is present if an ending fired underfoot, which is also why the
	// walk stopped.
	Outcome *Outcome `json:"outcome,omitempty"`

	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// TraverseInput moves a member through a connection into an adjoining room.
type TraverseInput struct {
	// Session is the session to act in.
	Session string

	// Member is the member crossing.
	Member string

	// Connection is the connection to cross.
	Connection string
}

// TraverseOutput reports the crossing.
type TraverseOutput struct {
	// FromRoom is the room departed.
	FromRoom string `json:"from_room"`

	// From is where the member stood before crossing.
	From spatial.Position `json:"from"`

	// ToRoom is the room arrived in.
	ToRoom string `json:"to_room"`

	// To is where the member arrived.
	To spatial.Position `json:"to"`

	// Discovered is what changed in each observer's perception.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Seq is the story sequence of the recorded crossing.
	Seq uint64 `json:"seq"`

	// Outcome is present if an ending fired on arrival.
	Outcome *Outcome `json:"outcome,omitempty"`

	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// Move walks a member along a path, one cell at a time.
//
// This is the first verb that does something the composition cannot do alone:
// the composition's own Move is a single hop, and walking is what makes the
// cells in between real. That matters beyond tidiness — anything that fires
// because a member *entered a particular cell* can only be noticed by something
// that visits each of them.
//
// The walk stops early when an ending fires underfoot. Remaining steps are not
// attempted, and `len(Steps) < len(Path)` is how the caller learns, with the
// reason in Outcome. Reporting that as an error would be wrong: the movement
// that happened, happened, and it is exactly what the caller asked for up to
// the point the world changed.
//
// Validation is complete before the first cell is entered (R5): a path with a
// gap in it is rejected whole rather than walked up to the gap, because a
// caller who mis-computed a route wants none of it rather than an arbitrary
// prefix of it.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrEmptyPath,
// ErrBrokenPath, ErrNoSession, ErrNoEncounter, ErrNoMember, ErrClosed,
// ErrBadPosition, or ErrSaveFailed with a populated report.
func (m *Manager) Move(ctx context.Context, in *MoveInput) (*MoveOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("move: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("move: %w", ErrNoMemberID)
	}
	if len(in.Path) == 0 {
		return nil, fmt.Errorf("move: %w", ErrEmptyPath)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	if err := validatePath(scope.enc, encounter.MemberID(in.Member), in.Path); err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	steps := make([]Step, 0, len(in.Path))
	merged := map[string]Discovery{}
	var outcome *Outcome

	for _, cell := range in.Path {
		moved, moveErr := scope.enc.Move(&encounter.MoveInput{
			Member: encounter.MemberID(in.Member),
			To:     cell,
		})
		if moveErr != nil {
			// Nothing is saved on a mid-walk rejection. The member has really
			// moved in memory for the steps already taken, but that encounter
			// is discarded unsaved, so the persisted world is untouched — the
			// whole walk fails, rather than leaving the party at an arbitrary
			// cell nobody chose.
			return nil, fmt.Errorf("move: step %d of %d: %w",
				len(steps)+1, len(in.Path), translate(moveErr))
		}

		steps = append(steps, Step{Position: moved.Moved.To, Seq: moved.Seq})
		mergeDiscoveries(merged, projectDiscoveries(moved.IntelDeltas))

		if moved.Outcome != nil {
			// The encounter ended underfoot. Every remaining step is abandoned:
			// a closed encounter refuses movement anyway, and attempting them
			// would turn a clean stop into a rejection the caller has to
			// interpret.
			outcome = projectOutcome(moved.Outcome)
			break
		}
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	return &MoveOutput{
		Steps:      steps,
		Discovered: nilIfEmpty(merged),
		Outcome:    outcome,
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// Traverse moves a member through a connection into the adjoining room.
//
// Under world anchoring the two endpoint cells are adjacent in absolute space,
// so a crossing is one ordinary step of the same size as any other — which is
// why a client rendering the result cannot tell a doorway from a corridor.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoMember, ErrNoConnection, ErrClosed, or ErrSaveFailed
// with a populated report.
func (m *Manager) Traverse(ctx context.Context, in *TraverseInput) (*TraverseOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("traverse: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("traverse: %w", ErrNoMemberID)
	}
	if in.Connection == "" {
		return nil, fmt.Errorf("traverse: %w", ErrNoConnection)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("traverse: %w", err)
	}

	crossed, err := scope.enc.Traverse(&encounter.TraverseInput{
		Member:     encounter.MemberID(in.Member),
		Connection: in.Connection,
	})
	if err != nil {
		return nil, fmt.Errorf("traverse: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("traverse: %w", err)
	}

	return &TraverseOutput{
		FromRoom:   crossed.Traversed.FromRoom,
		From:       crossed.Traversed.From,
		ToRoom:     crossed.Traversed.ToRoom,
		To:         crossed.Traversed.To,
		Discovered: projectDiscoveries(crossed.IntelDeltas),
		Seq:        crossed.Seq,
		Outcome:    projectOutcome(crossed.Outcome),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// validatePath rejects a path that is not a walk, before any of it is walked.
//
// Adjacency is checked with the room's own grid rather than a hand-rolled
// distance, because the two families disagree about what "one step" means and
// substituting one for the other is a real, previously-shipped defect class:
// Chebyshev distance on axial hex coordinates agrees with cube distance
// everywhere except the diagonals, so a wrong formula passes almost every
// fixture.
//
// The restriction is imposed now rather than later on purpose. Loosening a rule
// is a compatible change; tightening one breaks every host that was relying on
// the looser behaviour. If a path should ever be allowed to teleport, that can
// be granted; it could not be taken away.
func validatePath(enc *encounter.Encounter, member encounter.MemberID, path []spatial.Position) error {
	room, from, err := whereIs(enc, member)
	if err != nil {
		return err
	}

	grid, err := gridFor(enc, room)
	if err != nil {
		return err
	}

	previous := from
	for i, cell := range path {
		if !grid.IsAdjacent(previous, cell) {
			return fmt.Errorf("step %d from (%v,%v) to (%v,%v): %w",
				i+1, previous.X, previous.Y, cell.X, cell.Y, ErrBrokenPath)
		}
		previous = cell
	}
	return nil
}

// whereIs locates a member: which room, and which cell within it.
//
// Read out of a snapshot rather than a query, because the composition's
// Members() reports a member's room but not their position — the ergonomics gap
// already filed as toolkit#933. A snapshot is heavier than the question
// deserves, but it is the only source of truth for a member's current cell, and
// it runs once per walk rather than once per step.
//
// When #933 lands this becomes a direct lookup and the snapshot goes away.
func whereIs(enc *encounter.Encounter, member encounter.MemberID) (string, spatial.Position, error) {
	for _, m := range enc.ToData().Members {
		if m.ID != member {
			continue
		}
		return m.Room, spatial.Position{X: m.Position.X, Y: m.Position.Y}, nil
	}
	return "", spatial.Position{}, fmt.Errorf("%q: %w", member, ErrNoMember)
}

// gridFor builds a grid matching a room's family and span, for adjacency tests.
func gridFor(enc *encounter.Encounter, roomID string) (spatial.Grid, error) {
	atlas, err := enc.Atlas()
	if err != nil {
		return nil, err
	}
	for _, room := range atlas.Rooms {
		if room.ID != roomID {
			continue
		}
		switch room.Grid {
		case spatial.GridShapeHex:
			return spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{
				SpanWidth:  float64(room.Width),
				SpanHeight: float64(room.Height),
			}), nil
		case spatial.GridShapeSquare:
			return spatial.NewSquareGrid(spatial.SquareGridConfig{
				Width:  float64(room.Width),
				Height: float64(room.Height),
			}), nil
		default:
			return nil, fmt.Errorf("room %q: %w", roomID, ErrInvalidWorld)
		}
	}
	return nil, fmt.Errorf("room %q: %w", roomID, ErrInvalidWorld)
}

// mergeDiscoveries folds one step's perception deltas into the walk's running
// total.
//
// A walk produces a delta per step, and a caller wants what changed across the
// whole movement rather than a per-cell replay. First contact is appended;
// refreshed and faded subjects accumulate, deduplicated, because a subject that
// faded and returned should not appear twice in the same list.
func mergeDiscoveries(into map[string]Discovery, from map[string]Discovery) {
	for observer, delta := range from {
		running := into[observer]
		running.FirstContact = append(running.FirstContact, delta.FirstContact...)
		running.Refreshed = appendUnique(running.Refreshed, delta.Refreshed)
		running.Faded = appendUnique(running.Faded, delta.Faded)
		into[observer] = running
	}
}

func appendUnique(dst []string, src []string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, s := range dst {
		seen[s] = struct{}{}
	}
	for _, s := range src {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		dst = append(dst, s)
	}
	return dst
}

func nilIfEmpty(m map[string]Discovery) map[string]Discovery {
	if len(m) == 0 {
		return nil
	}
	return m
}
