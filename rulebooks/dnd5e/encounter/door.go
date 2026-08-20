// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// door.go is A DOOR IS STATE ON THE WALL (rpg-toolkit#1123, world-model S4).
//
// Since #1106 a wall is a boundary edge and a doorway is the ABSENCE of one.
// That is why Kirk's wide open gate — "a large gate that is open and could be 4
// hexes or so where 2 regions touch" — already worked at any width, with no
// doorway concept at all: a step is a step, and what stops one is a wall. What
// did not exist is a doorway that can be SHUT.
//
// So a door is not a new kind of geometry. It is a NAME AND A STATE over edges
// that were already expressible, and its state decides what those edges do.
// Everything mechanical falls out of tools/spatial unchanged: a
// movement-blocking boundary already stops spatial's own MoveEntity on the
// canonical ray, a sight-blocking one is already a hard block on that same ray,
// and registering an edge that already exists already replaces its flags. This
// file adds the noun and the verbs; it adds no geometry.
//
// # One boundary primitive, which is rpg-toolkit#880's framing
//
// #880 asked that movement, pathfinding and line of sight all consult the SAME
// dungeon-owned boundary, that edges be undirected and normalized, that both
// endpoints be in the floor footprint, and that an inner edge never be encoded
// as a blocked CELL. All four hold, and three of them hold because nothing was
// built for them: a door's edges are [spatial.Boundary] registrations on the
// one canvas, so movement and sight already read the same thing and there is no
// cell to block. The two this file enforces are normalization
// (normalizeDoorEdge, so a crossing named backwards is the same crossing) and
// floor (validateDoorInputs — a door hanging in the void is a wall drawn across
// nothing, which is only askable at all because rpg-toolkit#1116 made the
// canvas say what void is).
//
// # One state, N edges — which is the whole point
//
// Kirk's ruling, 2026-08-19: A DOOR IS A SET OF EDGES SHARING ONE STATE. The
// old stack models a door per connection, one per adjacent cell pair, and its
// authored-door identity is DERIVED from that pair — AuthoredDoorID(dungeonKey,
// from, to) in encounter/authored_edges.go, with validation refusing any door
// edge that carries a different id. So a four-hex gate over there is four
// independent doors, and nothing in the model stops two of them disagreeing.
//
// Here the state is the DOOR's. An edge is two cells and nothing else — see
// [DoorEdge], which has no blocking flags of its own precisely so that there is
// nothing for two edges of one gate to disagree ABOUT. That makes "a gate opens
// as one thing" structural rather than something the verbs have to remember to
// do, and it is pinned that way (TestNoEdgeCarriesAStateOfItsOwn).

// DoorID names one door.
//
// An alias rather than a defined type, following [MemberID] and [RegionID]: it
// exists to say which of two strings a signature means, not to make callers
// convert.
type DoorID = string

// DoorStateKind names what state a door is in, in the form the story and the
// blob carry it. See [DoorState].
type DoorStateKind string

const (
	// DoorOpen blocks neither movement nor sight: the gap that was there
	// before doors existed.
	DoorOpen DoorStateKind = "open"

	// DoorClosed blocks both. A shut door is a wall.
	DoorClosed DoorStateKind = "closed"

	// DoorLocked is closed, and carries what it takes to beat it.
	DoorLocked DoorStateKind = "locked"
)

// Lock is what it takes to beat a locked door: three facts this module carries
// from authoring to the caller, and never acts on.
//
// DC IS CARRIED, NOT INTERPRETED, AND NOT COMPARED. This module cannot import
// the rulebook (law C1), so it does not know what a difficulty class IS — and it
// deliberately does not find out by measuring anything against one.
// [Encounter.Unlock] is TOLD whether the lock was beaten; deciding that is 5e's
// job, and "a total that meets the DC succeeds" is a 5e rule that briefly lived
// here and was taken out (Kirk: "I agree on rules leaking in we need to be
// diligent"). What the DC is FOR is content: a client shows "DC 12", and the
// rulebook rolling against it knows the number because this reports it. The
// precedent is the save gate's — it is data; content carries it, nothing here
// executes it.
//
// Ability and Tool are opaque host/rulebook-owned refs (the old stack's
// canonical values are lowercase "dex" and a toolkit item ref like
// "dnd5e:item:thieves-tools"); this module carries them and does nothing else
// with them.
type Lock struct {
	// DC is the lock's authored difficulty, reported to whoever does the
	// deciding. Must be at least 1 — a lock with nothing to beat is not a
	// lock, and zero is what an undeclared one would look like. That check is
	// a well-formedness rule about the DATA, not a judgement about a roll.
	DC int

	// Ability is the opaque ability identifier the check is made with, or
	// empty. Never inspected here.
	Ability string

	// Tool is the opaque tool proficiency ref that applies, or empty. Never
	// inspected here.
	Tool string
}

// DoorState is what state a door is in: a closed set, sealed the way
// [DissolveCause] and [Void] are and for the same reason.
//
// Three cases, and the third carries data, which is why this is an interface
// rather than a string. Open blocks nothing. Closed blocks movement and sight.
// Locked is closed AND carries the [Lock] that has to be beaten — not a fourth
// blocking behaviour but a second fact about a closed door, which is exactly
// the distinction a bare enum would have flattened.
//
// The unexported method seals the set: a fourth state cannot be declared
// outside this package, so adding one means editing this file, and editing this
// file means having the caller that forces it in hand.
//
// REQUIRED on every authored door, never defaulted (rpg-toolkit#1033's law,
// applied to world data exactly as [Void] applies it): a door with no declared
// state would be this module deciding whether a dungeon's gates start open.
type DoorState interface {
	// Kind names which state this is.
	Kind() DoorStateKind

	// Lock reports the lock this state carries, and whether it carries one.
	// Only [DoorIsLocked] does.
	Lock() (Lock, bool)

	// blocks reports whether this state's edges stop movement and sight.
	//
	// Unexported, which SEALS the set. It is the only question the geometry
	// asks a state, so the sealing method is a real one rather than a marker:
	// a fourth state cannot be added without answering it.
	blocks() bool
}

// DoorIsOpen declares a door standing open: its edges block neither movement
// nor sight, which is exactly the doorway this composition already had.
//
// A function rather than a package-level variable so nothing can reassign what
// it means at runtime — [ByDecision]'s reasoning, and [VoidIsOpaque]'s.
func DoorIsOpen() DoorState { return doorOpen{} }

type doorOpen struct{}

func (doorOpen) Kind() DoorStateKind { return DoorOpen }
func (doorOpen) Lock() (Lock, bool)  { return Lock{}, false }
func (doorOpen) blocks() bool        { return false }

// DoorIsClosed declares a door shut but not locked: a wall until somebody opens
// it, and [Encounter.OpenDoor] is all it takes.
func DoorIsClosed() DoorState { return doorClosed{} }

type doorClosed struct{}

func (doorClosed) Kind() DoorStateKind { return DoorClosed }
func (doorClosed) Lock() (Lock, bool)  { return Lock{}, false }
func (doorClosed) blocks() bool        { return true }

// DoorIsLocked declares a door shut and locked: the reference tomb's DC-12
// connector, which blocks sight into the boss chamber until it is beaten.
//
// Blocks exactly as [DoorIsClosed] does — a lock is not a stronger wall, it is
// a fact about who may open one. [Encounter.OpenDoor] refuses it by name and
// [Encounter.Unlock] is the way through, taking the caller's verdict on whether
// the lock was beaten rather than reaching a verdict of its own.
func DoorIsLocked(lock Lock) DoorState { return doorLocked{lock: lock} }

type doorLocked struct{ lock Lock }

func (doorLocked) Kind() DoorStateKind  { return DoorLocked }
func (d doorLocked) Lock() (Lock, bool) { return d.lock, true }
func (doorLocked) blocks() bool         { return true }

// DoorEdge is one crossing a door stands in: two adjacent DUNGEON-ABSOLUTE
// cells, and nothing else.
//
// NO BLOCKING FLAGS, deliberately, and that absence is load-bearing. What an
// edge does is decided by the door's state, so a flag here would be a second
// truth the verbs would have to move in step with — and it is the exact truth
// two edges of one gate could then disagree about. [spatial.Boundary] carries
// its own flags because a wall IS its flags; a door's edge is a pointer at the
// thing that decides.
//
// ABSOLUTE rather than room-local, unlike [RoomInput.Boundaries]. A room's wall
// is the room's to declare, but a door stands at the SEAM and belongs to
// neither chamber it joins, so there is no room whose frame it is naturally in.
// It is field-level for the same reason [ConnectionInput] is — and this is the
// shape the content compiler (S5) emits, which is the caller this exists for.
type DoorEdge struct {
	// From is one endpoint cell, dungeon-absolute.
	From spatial.Position

	// To is the other, adjacent to From.
	To spatial.Position
}

// DoorInput authors one door: a name, the edges it stands in, and the state
// they are all in.
type DoorInput struct {
	// ID is the door's unique identifier.
	ID DoorID

	// Edges are the crossings this door stands in — at least one, and as many
	// as the gate is wide. They share one state, which is the whole design;
	// see this file's own doc comment.
	Edges []DoorEdge

	// State is what state the door starts in. REQUIRED — see [DoorState].
	State DoorState
}

// Door is a door's public read shape: what it is called, where it stands, and
// what state it is in right now.
type Door struct {
	// ID is the door's identifier.
	ID DoorID

	// Edges are the crossings it stands in, dungeon-absolute. Freshly
	// allocated per call — this is a copy-out read, like [Encounter.Atlas].
	Edges []DoorEdge

	// State is its state RIGHT NOW, not the one it was authored in.
	State DoorState
}

// doorRecord is what the composition stores about a door.
//
// The edges are construction truth and never change; the state is the only
// mutable thing, and it is held ONCE for however many edges the door has.
type doorRecord struct {
	id    DoorID
	edges []DoorEdge
	state DoorState
}

// Doors reports every door, in stable ID order, with the state each is in now.
//
// Copy-out: the returned edge slices are freshly allocated per call, so a
// caller cannot move a door by editing what it was handed.
//
// # Why this is not on the Atlas
//
// An [Encounter.Atlas] is a CONSTRUCTION-TIME snapshot and says so in its own
// doc, and a door's state is the one thing here a verb changes mid-scene.
// Putting mutable state inside a snapshot that promises to be construction data
// would make the promise false for every other field on it.
//
// Kirk ruled it, and his reasoning is sharper than that, because it says how
// long the two reads can disagree: "atlas keeps what it had when it loads, the
// door opening changes that and los is given back. next load will have the door
// opened so this is only for the turn where the door was opened."
//
// So this is NOT a permanent duality between two views of the field. The Atlas
// is the field as it was authored and loaded; opening a door changes what the
// map does, and the line of sight that comes back is the live answer. The
// divergence lasts exactly as long as the session that caused it — the state
// persists (see doorDataFrom), so the next load builds an Atlas with the door
// already open and the two agree again. Reading door state here rather than
// there is what keeps that window honest instead of hiding it inside a snapshot
// that would then be lying for one turn.
func (e *Encounter) Doors() []Door {
	out := make([]Door, 0, len(e.doors))
	for _, d := range e.doors {
		out = append(out, Door{
			ID:    d.id,
			Edges: append([]DoorEdge(nil), d.edges...),
			State: d.state,
		})
	}

	return out
}

// doorOf finds a door by ID, or reports that there is no such door.
func (e *Encounter) doorOf(id DoorID) (*doorRecord, error) {
	if id == "" {
		return nil, fmt.Errorf("door: %w", ErrNoDoor)
	}
	d, ok := e.doorsByID[id]
	if !ok {
		return nil, fmt.Errorf("door %q: %w", id, ErrNoDoor)
	}

	return d, nil
}

// normalizeDoorEdge orders an edge's endpoints so the same crossing named
// either way round compares equal.
//
// Doors are undirected, exactly as [spatial.Boundary] is ("spatial normalizes
// an undirected pair on registration", compileCanvas's own note), so every
// duplicate and collision check in this file has to see the same crossing as
// the same crossing however it was written down.
func normalizeDoorEdge(e DoorEdge) DoorEdge {
	if e.To.X < e.From.X || (e.To.X == e.From.X && e.To.Y < e.From.Y) {
		return DoorEdge{From: e.To, To: e.From}
	}

	return e
}

// validateDoorInputs rejects door defects before construction (R5), at both
// seams, and carries no verb prefix in its errors for buildValidRoomGrids'
// reason: each caller wraps its own.
//
// Every check here is about the door as DATA — a name, some edges, a state.
// What the edges DO is spatial's, and this deliberately does not re-decide it.
//
// Requires at least one room and a grid per room, which both seams guarantee:
// each rejects an empty room list at its door (NewEncounter's own first checks,
// LoadEncounter's "no rooms") and each runs buildValidRoomGrids before this.
//
// The floor check is the one worth naming: a door's endpoints must both be
// cells some chamber owns. A door hanging in the void is not a door, it is a
// wall drawn across nothing — #880's rule ("both endpoints must be in the floor
// footprint"), and the reason rpg-toolkit#1116's declaration has to land first
// for this to even be askable.
func validateDoorInputs(
	rooms []RoomInput, grids map[string]spatial.Grid,
	orientation Orientation, doors []DoorInput,
) error {
	if len(doors) == 0 {
		return nil
	}

	// Any room's grid answers adjacency for absolute cells: W1 gives every
	// room in a field the same family, and both families' adjacency is
	// translation-invariant. validateConnectionInputs' W3 check makes the same
	// call, for the same reason.
	grid := grids[rooms[0].ID]

	seenIDs := make(map[DoorID]bool, len(doors))
	seenEdges := make(map[DoorEdge]DoorID)

	// Authored room walls, normalized absolute, so a door drawn on top of one
	// is caught rather than silently winning or losing the registration race.
	walls := make(map[DoorEdge]string)
	for _, r := range rooms {
		for _, b := range r.Boundaries {
			walls[normalizeDoorEdge(DoorEdge{
				From: absoluteOf(r, orientation, b.From), To: absoluteOf(r, orientation, b.To),
			})] = r.ID
		}
	}

	for _, d := range doors {
		if d.ID == "" {
			return fmt.Errorf("door with no id: %w", ErrBadDoor)
		}
		if seenIDs[d.ID] {
			return fmt.Errorf("duplicate door %q: %w", d.ID, ErrBadDoor)
		}
		seenIDs[d.ID] = true

		if d.State == nil {
			return fmt.Errorf("door %q does not say what state it is in (DoorInput.State): %w", d.ID, ErrBadDoor)
		}
		if lock, locked := d.State.Lock(); locked && lock.DC < 1 {
			return fmt.Errorf("door %q is locked at DC %d, which nothing has to beat: %w", d.ID, lock.DC, ErrBadDoor)
		}

		if len(d.Edges) == 0 {
			return fmt.Errorf("door %q stands in no edges: %w", d.ID, ErrBadDoor)
		}

		for _, raw := range d.Edges {
			edge := normalizeDoorEdge(raw)

			if !isIntegralHexCell(grid, edge.From) || !isIntegralHexCell(grid, edge.To) {
				return fmt.Errorf("door %q edge (%g,%g)-(%g,%g) is not an integral axial crossing: %w",
					d.ID, raw.From.X, raw.From.Y, raw.To.X, raw.To.Y, ErrBadDoor)
			}
			if edge.From == edge.To {
				return fmt.Errorf("door %q edge (%g,%g) has the same cell at both ends: %w",
					d.ID, raw.From.X, raw.From.Y, ErrBadDoor)
			}
			if grid.Distance(edge.From, edge.To) != 1 {
				return fmt.Errorf("door %q edge (%g,%g)-(%g,%g) joins cells that are not adjacent: %w",
					d.ID, raw.From.X, raw.From.Y, raw.To.X, raw.To.Y, ErrBadDoor)
			}
			for _, end := range []spatial.Position{edge.From, edge.To} {
				if _, floor := regionAt(rooms, grids, orientation, end); !floor {
					return fmt.Errorf("door %q edge endpoint (%g,%g) is not floor: %w",
						d.ID, end.X, end.Y, ErrBadDoor)
				}
			}
			if owner, taken := seenEdges[edge]; taken {
				if owner == d.ID {
					return fmt.Errorf("door %q names the crossing (%g,%g)-(%g,%g) twice: %w",
						d.ID, raw.From.X, raw.From.Y, raw.To.X, raw.To.Y, ErrBadDoor)
				}
				return fmt.Errorf("doors %q and %q both stand in the crossing (%g,%g)-(%g,%g), which could not then have one state: %w",
					owner, d.ID, raw.From.X, raw.From.Y, raw.To.X, raw.To.Y, ErrBadDoor)
			}
			seenEdges[edge] = d.ID

			if room, walled := walls[edge]; walled {
				return fmt.Errorf("door %q stands in the crossing (%g,%g)-(%g,%g), where room %q already drew a wall: %w",
					d.ID, raw.From.X, raw.From.Y, raw.To.X, raw.To.Y, room, ErrBadDoor)
			}
		}
	}

	return nil
}

// doorRecordsFrom turns validated inputs into the records the encounter keeps,
// sorted by ID (C8 determinism — order is observable in Doors and ToData), with
// every edge normalized and deep-copied so a caller cannot move a door after
// construction.
func doorRecordsFrom(doors []DoorInput) ([]*doorRecord, map[DoorID]*doorRecord) {
	records := make([]*doorRecord, 0, len(doors))
	byID := make(map[DoorID]*doorRecord, len(doors))

	for _, d := range doors {
		edges := make([]DoorEdge, 0, len(d.Edges))
		for _, e := range d.Edges {
			edges = append(edges, normalizeDoorEdge(e))
		}
		rec := &doorRecord{id: d.ID, edges: edges, state: d.State}
		records = append(records, rec)
		byID[d.ID] = rec
	}
	sort.Slice(records, func(i, j int) bool { return records[i].id < records[j].id })

	// byID holds the SAME pointers, so it is an index rather than a second
	// copy — a door's state has one home however it is reached.
	return records, byID
}

// registerDoor puts a door's state onto its edges: one call per edge, all with
// the same flags, because the state is the door's.
//
// Registering an edge that already exists REPLACES its flags (tools/spatial's
// RegisterBoundary says so in its own doc), which is what makes opening and
// closing symmetric without this having to remember what it did last time. An
// open door's edges are registered as blocking nothing rather than removed,
// so a door always owns its crossings and a later close cannot miss one.
func registerDoor(canvas *spatial.BasicRoom, d *doorRecord) error {
	blocks := d.state.blocks()
	for _, e := range d.edges {
		if err := canvas.RegisterBoundary(spatial.Boundary{
			From:              e.From,
			To:                e.To,
			BlocksMovement:    blocks,
			BlocksLineOfSight: blocks,
		}); err != nil {
			return fmt.Errorf("door %q edge (%g,%g)-(%g,%g): %w: %w", d.id, e.From.X, e.From.Y, e.To.X, e.To.Y, ErrBadPlacement, err)
		}
	}

	return nil
}

// doorOnEdge reports the door standing in the crossing between two ADJACENT
// cells, or nil. One edge, one door — validateDoorInputs refuses a crossing
// claimed by two, which is what makes the singular answer sound.
func (e *Encounter) doorOnEdge(from, to spatial.Position) *doorRecord {
	want := normalizeDoorEdge(DoorEdge{From: from, To: to})
	for _, d := range e.doors {
		for _, edge := range d.edges {
			if edge == want {
				return d
			}
		}
	}

	return nil
}

// doorsAlong reports every door a move from one cell to another passes
// through, IN TRAVEL ORDER.
//
// A STEP IS NOT NECESSARILY ONE CELL, which is the whole reason this walks a
// ray instead of looking at the two ends. [Encounter.Step] deliberately does
// not check adjacency — that is a rule about WALKING and it lives with the walk
// (Step's own doc, and rpg-toolkit#1059: a decider's IntentMoveTo never carried
// an adjacency contract) — so a caller can legitimately name a destination
// several cells away. tools/spatial refuses such a move on ANY
// movement-blocking crossing along the way, so a door in the middle of the path
// is as real as one at the end. Looking only at the endpoints was the first
// version of this and it was wrong twice over: a long step through an open door
// reported no door at all, and a long step into a shut one got spatial's
// generic "cannot cross movement-blocking boundary" instead of the door's name
// and state — which is the answer this slice exists to give. Found by Copilot
// on PR #1125.
//
// THE SAME RAY SPATIAL USES, deliberately: [spatial.CanonicalBoundaryRay] is
// what MoveEntity walks to decide the refusal
// (isDirectMovementBoundaryBlockedUnsafe), so asking a different one would be a
// second answer to "what is crossable" — the defect the BlocksMovement mutant
// already made this file delete once. The canonical ray is also ORDERED from
// `from` to `to` whichever way round the endpoints sort, so "travel order" here
// is the mover's order rather than an artifact of the rasterization.
//
// Returns them ALL rather than the first, so a multi-door move has a defined
// answer instead of one a singular field would have picked by accident. In
// practice the seam that walks a path visits each cell in turn, and this is
// empty or one long.
func (e *Encounter) doorsAlong(from, to spatial.Position) []*doorRecord {
	ray := spatial.CanonicalBoundaryRay(e.canvas.GetGrid(), from, to)

	var out []*doorRecord
	for i := 1; i < len(ray); i++ {
		if d := e.doorOnEdge(ray[i-1], ray[i]); d != nil {
			out = append(out, d)
		}
	}

	return out
}
