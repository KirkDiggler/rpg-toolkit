// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"

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

// There were TraverseInput and TraverseOutput here, and a Traverse verb that
// took a connection id and crossed it. All three retired with
// rpg-toolkit#1048: a doorway's two cells are adjacent on the map, so crossing
// one is a step, and Move takes steps. What the verb reported that a step does
// not — which rooms were left and entered — was the room dialect this seam
// stopped speaking two slices ago.

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

	for i, step := range walk.steps {
		moved, err := m.takeStep(scope, member, step)
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
			Position: onMap(scope.enc, step.room, moved.to),
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

// stepResult is what one step produced, whichever mechanism carried it.
//
// The two composition verbs report the same news in different shapes, and this
// is the small amount of translation that lets the walk loop treat a crossing
// exactly like a step: same stop-on-ending, same stop-on-fight, same discovery
// merge, same beat sequence.
type stepResult struct {
	to          spatial.Position
	Seq         uint64
	IntelDeltas map[encounter.MemberID]*intel.SurveilOutput
	Outcome     *encounter.Outcome
	Formed      *encounter.FormedBubble
}

// takeStep executes one resolved step: a move within the room, or a crossing
// through the doorway the step named.
//
// Which of the two it is was decided during resolution, where the map was
// already in hand; this only carries it out. Both are ONE step of a walk and
// neither is time — the composition advances no clock for either.
func (m *Manager) takeStep(scope *writeScope, member string, step walkStep) (stepResult, error) {
	if step.connection == "" {
		moved, err := scope.enc.Move(&encounter.MoveInput{
			Member: encounter.MemberID(member),
			To:     step.local,
		})
		if err != nil {
			return stepResult{}, err
		}
		return stepResult{
			to: moved.Moved.To, Seq: moved.Seq, IntelDeltas: moved.IntelDeltas,
			Outcome: moved.Outcome, Formed: moved.Formed,
		}, nil
	}

	crossed, err := scope.enc.Traverse(&encounter.TraverseInput{
		Member:     encounter.MemberID(member),
		Connection: step.connection,
	})
	if err != nil {
		return stepResult{}, err
	}
	return stepResult{
		to: crossed.Traversed.To, Seq: crossed.Seq, IntelDeltas: crossed.IntelDeltas,
		Outcome: crossed.Outcome, Formed: crossed.Formed,
	}, nil
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

// walkStep is one resolved step of a path: where it lands in the composition's
// own terms, and — when the step crosses a doorway — which doorway carries it.
type walkStep struct {
	room       string
	local      spatial.Position
	connection string // empty for an ordinary step within a room
}

// resolvedWalk is a path as the composition takes it: each step resolved to
// the room it lands in and the cell within it, in order.
type resolvedWalk struct {
	steps []walkStep
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
// Adjacency is checked with a grid of the field's own family rather than a
// hand-rolled distance, because the two families disagree about what "one
// step" means and substituting one for the other is a real, previously-shipped
// defect class: Chebyshev distance on axial hex coordinates agrees with cube
// distance everywhere except the diagonals, so a wrong formula passes almost
// every fixture. One grid answers for the whole path, including the step that
// changes rooms — a field has a single family by law (W1), and adjacency in
// both families survives translation, so an absolute pair is adjacent or not
// regardless of which room's grid is asked.
//
// A step MAY leave the room the walker is in, and that is the only way it may
// leave: through a doorway joining the cell they stand on to the one they are
// stepping to (rpg-toolkit#1048). Adjacency alone is not permission — W2 lets
// two rooms touch without a door — so a cross-room step with no doorway is
// refused with ErrNoCrossing, distinctly from a path that is not a walk at all.
func resolveWalk(
	enc *encounter.Encounter, member encounter.MemberID, path []spatial.Position,
) (resolvedWalk, error) {
	room, from, err := whereIs(enc, member)
	if err != nil {
		return resolvedWalk{}, err
	}

	// The map, fetched ONCE for the whole walk and used for both things this
	// function needs it for: the grid to test adjacency with, and the doorway
	// list to recognise a crossing. Building it twice is what the first
	// version of this did, by asking gridFor to fetch its own.
	atlas, err := enc.Atlas()
	if err != nil {
		return resolvedWalk{}, err
	}

	grid, err := gridFrom(atlas, room)
	if err != nil {
		return resolvedWalk{}, err
	}

	walk := resolvedWalk{steps: make([]walkStep, 0, len(path))}
	// Both frames are tracked: absolute to test adjacency and to describe a
	// refusal in the coordinates the caller actually wrote, room-local because
	// that is what the composition's verbs take.
	here, hereRoom := onMap(enc, room, from), room

	for i, cell := range path {
		if !grid.IsAdjacent(here, cell) {
			return resolvedWalk{}, fmt.Errorf("step %d from (%v,%v) to (%v,%v): %w",
				i+1, here.X, here.Y, cell.X, cell.Y, ErrBrokenPath)
		}

		located, lerr := enc.Locate(&encounter.LocateInput{Position: cell})
		if lerr != nil {
			return resolvedWalk{}, fmt.Errorf("step %d of %d: no room owns (%v,%v): %w: %w",
				i+1, len(path), cell.X, cell.Y, ErrBadPosition, lerr)
		}

		step := walkStep{room: located.Room, local: located.Position}
		if located.Room != hereRoom {
			// Another room: legal only through a doorway joining the cell the
			// walker stands on to the one they are stepping to. Adjacency is
			// not permission — W2 lets two rooms touch without a door between
			// them, and that pair of cells is adjacent and unwalkable.
			connection, ok := doorwayBetween(atlas, here, cell)
			if !ok {
				return resolvedWalk{}, fmt.Errorf(
					"step %d from (%v,%v) to (%v,%v): %w",
					i+1, here.X, here.Y, cell.X, cell.Y, ErrNoCrossing)
			}
			step.connection = connection
		}

		walk.steps = append(walk.steps, step)
		here, hereRoom = cell, located.Room
	}
	return walk, nil
}

// doorwayBetween finds the doorway joining two cells, in either direction — a
// doorway is crossable both ways.
//
// Reads the Atlas, which is where doorways live in absolute space: a pair of
// adjacent cells and the connection that joins them. The composition holds the
// same fact in its own room-shaped terms, and this is the projection of it.
func doorwayBetween(atlas encounter.Atlas, from, to spatial.Position) (string, bool) {
	for _, doorway := range atlas.Doorways {
		if (doorway.FromCell == from && doorway.ToCell == to) || (doorway.ToCell == from && doorway.FromCell == to) {
			return doorway.Connection, true
		}
	}
	return "", false
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

// gridFrom builds a grid matching a room's family and span, for adjacency
// tests, from a map the caller already has.
//
// Takes the Atlas rather than fetching one: it is the caller that knows
// whether it needs the map for anything else, and in resolveWalk's case it
// does — the doorway list comes off the same fetch.
func gridFrom(atlas encounter.Atlas, roomID string) (spatial.Grid, error) {
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
