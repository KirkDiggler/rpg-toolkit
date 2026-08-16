// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MoveInput walks a member along a path of cells on the map.
//
// The path stays inside the room the walker is standing in — see Path — until
// the slice that makes a doorway crossing an ordinary step.
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
	//
	// Cells are DUNGEON-ABSOLUTE — the same coordinates the Atlas draws and
	// every other verb speaks (rpg-project#227).
	//
	// A WALK STILL DOES NOT CROSS A DOORWAY. Absolute coordinates make a
	// crossing expressible for the first time — the far side of a doorway is
	// simply the next cell along — but expressible is not permitted: a path
	// that leaves the walker's room is refused with ErrBadPosition, exactly
	// as it was refused before, when it could not even be written down. That
	// changes in its own slice, deliberately, so that a real behavior change
	// is not smuggled in inside a change of dialect.
	Path []spatial.Position
}

// Step is one cell actually entered.
type Step struct {
	// Position is the cell entered, in dungeon-absolute space.
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

	// Formed is present if the walk put the walker in sight of the other side
	// and a fight started, which is also why the walk stopped. The remaining
	// cells were not attempted: a fight member does not free-roam.
	Formed *Formed `json:"formed,omitempty"`

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

	// Formed is present if walking through the doorway put the crosser in
	// sight of the other side and a fight started.
	Formed *Formed `json:"formed,omitempty"`

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
// ErrBrokenPath for a path with a gap in it, ErrNoSession, ErrNoEncounter,
// ErrNoMember, ErrClosed, ErrBadPosition for a cell no room owns OR a cell in
// a room other than the walker's, or ErrSaveFailed with a populated report.
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

	walk, err := resolveWalk(scope.enc, encounter.MemberID(in.Member), in.Path)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	res, err := m.runWalk(scope, in.Member, walk)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}

	return &MoveOutput{
		Steps:      res.steps,
		Discovered: nilIfEmpty(res.discovered),
		Outcome:    res.outcome,
		Formed:     res.formed,
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
		Outcome:    projectOutcome(scope.enc, crossed.Outcome),
		Formed:     projectFormed(crossed.Formed),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// walkResult is what one run of the walk produced.
type walkResult struct {
	steps      []Step
	discovered map[string]Discovery
	outcome    *Outcome
	formed     *Formed
}

// runWalk steps a member along a path, stopping at the first fight, the first
// ending, or the last cell.
//
// IT DECIDES NONE OF THAT. Until rpg-toolkit#964 this loop held a game rule: it
// read each step's perception delta, and a subject seen for the first time
// stopped the walk and opened a window for the walker to answer. That was the
// SDK deciding when an encounter begins — a rule, in the one package whose
// charter is to hold none — and it was wrong twice over besides, because sight
// is not the only way a fight starts and a walker is not the only one who can
// see.
//
// The composition owns it now, at the one place all sight changes pass through,
// and reports a formed bubble on the Move that caused it. This loop reads that
// report exactly the way it already read Outcome: as news. The walk stops
// because the walker is IN a fight and a fight member does not free-roam — the
// composition would refuse the next step with ErrInBubble — so stopping is a
// fact about the world rather than a policy about perception.
func (m *Manager) runWalk(scope *writeScope, member string, walk resolvedWalk) (*walkResult, error) {
	res := &walkResult{discovered: map[string]Discovery{}}

	for i, local := range walk.steps {
		moved, err := scope.enc.Move(&encounter.MoveInput{
			Member: encounter.MemberID(member),
			To:     local,
		})
		if err != nil {
			// Nothing is saved on a mid-walk rejection. The member has really
			// moved in memory for the steps already taken, but that encounter is
			// discarded unsaved, so the persisted world is untouched.
			return nil, fmt.Errorf("step %d of %d: %w", i+1, len(walk.steps), translate(err))
		}

		// Reported from what the composition says happened, projected back —
		// not echoed from the input. A step that landed somewhere other than
		// where it was aimed is a thing worth being able to see.
		res.steps = append(res.steps, Step{
			Position: onMap(scope.enc, walk.room, moved.Moved.To),
			Seq:      moved.Seq,
		})
		mergeDiscoveries(res.discovered, projectDiscoveries(moved.IntelDeltas))

		if moved.Outcome != nil {
			// The encounter ended underfoot. Every remaining step is abandoned:
			// a closed encounter refuses movement anyway, and attempting them
			// would turn a clean stop into a rejection the caller must interpret.
			res.outcome = projectOutcome(scope.enc, moved.Outcome)
			return res, nil
		}

		if moved.Formed != nil {
			res.formed = projectFormed(moved.Formed)
			return res, nil
		}
	}

	return res, nil
}

// projectFormed turns the composition's report of a started fight into the
// SDK's own shape.
func projectFormed(f *encounter.FormedBubble) *Formed {
	if f == nil {
		return nil
	}
	out := &Formed{Seq: f.Seq}
	for _, id := range f.Order {
		out.Order = append(out.Order, string(id))
	}
	for _, id := range f.Surprised {
		out.Surprised = append(out.Surprised, string(id))
	}
	return out
}

// resolvedWalk is a path as the composition takes it: the room being walked,
// and the room-local cell for each step, in order.
type resolvedWalk struct {
	room  string
	steps []spatial.Position
}

// resolveWalk validates a path whole and resolves it ONCE.
//
// Two jobs in one pass, and they belong together. Validation must finish
// before the first cell is entered (R5): a caller who mis-computed a route
// wants none of it rather than an arbitrary prefix. Resolution turns each
// dungeon-absolute cell into the room-local one the composition's Move takes.
// Doing them separately located every cell twice and read the roster twice per
// walk — and, worse, left two places free to disagree about which room a cell
// belongs to.
//
// Adjacency is checked with the room's own grid rather than a hand-rolled
// distance, because the two families disagree about what "one step" means and
// substituting one for the other is a real, previously-shipped defect class:
// Chebyshev distance on axial hex coordinates agrees with cube distance
// everywhere except the diagonals, so a wrong formula passes almost every
// fixture.
//
// The restriction to one room is imposed now rather than later on purpose.
// Loosening a rule is a compatible change; tightening one breaks every host
// that was relying on the looser behaviour. If a walk should ever cross a
// doorway, that can be granted; it could not be taken away.
func resolveWalk(
	enc *encounter.Encounter, member encounter.MemberID, path []spatial.Position,
) (resolvedWalk, error) {
	room, from, err := whereIs(enc, member)
	if err != nil {
		return resolvedWalk{}, err
	}

	grid, err := gridFor(enc, room)
	if err != nil {
		return resolvedWalk{}, err
	}

	// Both frames are tracked: the local one to ask the grid about adjacency,
	// the absolute one to describe a refusal in the coordinates the caller
	// actually wrote. A message that mixed them would send whoever reads it
	// looking for a cell nobody named.
	walk := resolvedWalk{room: room, steps: make([]spatial.Position, 0, len(path))}
	previous, previousCell := from, onMap(enc, room, from)

	for i, cell := range path {
		local, lerr := localTo(enc, room, cell)
		if lerr != nil {
			return resolvedWalk{}, fmt.Errorf("step %d of %d: %w", i+1, len(path), lerr)
		}
		if !grid.IsAdjacent(previous, local) {
			return resolvedWalk{}, fmt.Errorf("step %d from (%v,%v) to (%v,%v): %w",
				i+1, previousCell.X, previousCell.Y, cell.X, cell.Y, ErrBrokenPath)
		}
		walk.steps = append(walk.steps, local)
		previous, previousCell = local, cell
	}
	return walk, nil
}

// whereIs locates a member: which room owns them, and which cell within it.
//
// It used to serialize the whole aggregate — clock, intel, log, field and
// endings — to read two floats, because the composition's roster reported a
// room and no position. That was toolkit#933, and it has landed: Members now
// carries each member's cell on the map, so this is a roster read and a
// projection back down into the room-local terms the composition's own verbs
// take.
func whereIs(enc *encounter.Encounter, member encounter.MemberID) (string, spatial.Position, error) {
	members, err := enc.Members()
	if err != nil {
		return "", spatial.Position{}, err
	}

	for _, m := range members {
		if m.ID != member {
			continue
		}
		located, lerr := enc.Locate(&encounter.LocateInput{Position: m.Position})
		if lerr != nil {
			return "", spatial.Position{}, fmt.Errorf("%q stands at %v, which no room owns: %w",
				member, m.Position, ErrBadPosition)
		}
		return located.Room, located.Position, nil
	}
	return "", spatial.Position{}, fmt.Errorf("%q: %w", member, ErrNoMember)
}

// localTo resolves a map cell into the room-local cell the composition's verbs
// take, refusing anything that is not in the given room.
//
// The refusal is the whole reason this is a named function rather than a call
// to Locate. A walk is within one room, and after the reshape a caller CAN
// write a path that leaves it — the coordinates no longer stop them. So the
// rule that used to be enforced by the shape of the input has to be enforced
// here instead, until the slice that makes a crossing an ordinary step.
func localTo(enc *encounter.Encounter, room string, cell spatial.Position) (spatial.Position, error) {
	located, err := enc.Locate(&encounter.LocateInput{Position: cell})
	if err != nil {
		// Both wrapped: the sentinel is what a caller matches on, and the
		// composition's own error says WHICH way the cell was unusable —
		// owned by nothing, off the grid, or not an integral cell.
		return spatial.Position{}, fmt.Errorf(
			"no room owns (%v,%v): %w: %w", cell.X, cell.Y, ErrBadPosition, err)
	}
	if located.Room != room {
		return spatial.Position{}, fmt.Errorf(
			"(%v,%v) is not in the room being walked: %w", cell.X, cell.Y, ErrBadPosition)
	}
	return located.Position, nil
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
