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

// Lock is what it takes to beat a locked door: a number this module compares
// against, and two identifiers it never looks inside.
//
// DC IS CARRIED, NOT INTERPRETED. This module cannot import the rulebook (law
// C1), so it does not know what a difficulty class IS — [Encounter.Unlock] is
// TOLD a total and compares it, exactly as [InitiativeRoller] is told an order
// rather than rolling one. Ability and Tool are opaque host/rulebook-owned
// refs (the old stack's canonical values are lowercase "dex" and a toolkit item
// ref like "dnd5e:item:thieves-tools"); this module carries them from authoring
// to the caller that knows what they mean and does nothing else with them.
type Lock struct {
	// DC is the total a check must reach to beat this lock. Must be at least
	// 1 — a lock nothing has to beat is not a lock, and zero is what an
	// undeclared one would look like.
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
// it means at runtime — [ByDecision]'s reasoning, and [VoidIsRock]'s.
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
// [Encounter.Unlock] is the way through.
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
// caller cannot move a door by editing what it was handed. Not on
// [Encounter.Atlas] deliberately — an Atlas is a CONSTRUCTION-TIME snapshot and
// says so in its own doc, and a door's state is the one thing here that a verb
// changes mid-scene. Putting mutable state inside a snapshot that promises to
// be construction data would make the promise false for every other field on
// it.
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
func validateDoorInputs(rooms []RoomInput, grids map[string]spatial.Grid, doors []DoorInput) error {
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
			walls[normalizeDoorEdge(DoorEdge{From: b.From.Add(r.Origin), To: b.To.Add(r.Origin)})] = r.ID
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

			if !isIntegralAxialPosition(grid, edge.From) || !isIntegralAxialPosition(grid, edge.To) {
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
				if _, floor := regionAt(rooms, grids, end); !floor {
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

// doorAcross reports the door standing in the crossing between two adjacent
// cells, or nil.
//
// Adjacent cells only, which is what a crossing IS. A multi-cell step's ray can
// pass several crossings and spatial refuses it on any of them; naming WHICH
// door stopped a long step is a question this does not try to answer, because
// [Encounter.Step] is one step by contract and the seam that walks a path
// visits each cell in turn.
func (e *Encounter) doorAcross(from, to spatial.Position) *doorRecord {
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
