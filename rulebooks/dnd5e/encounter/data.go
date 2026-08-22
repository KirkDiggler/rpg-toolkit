// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EncounterData is the persistent representation of an Encounter.
// Outcome is omitted when the encounter is open; present when closed.
// All leaves (Clock, Intel, Log) embed their Data types verbatim.
// Deciders are NOT persisted; they are re-registered at load.
type EncounterData struct {
	Outcome *OutcomeData   `json:"outcome,omitempty"`
	Clock   clock.TickData `json:"clock"`
	// Bubbles holds the localized initiative bubbles running in this
	// encounter — zero or more, and zero for any encounter not currently in a
	// fight. Absent in blobs written before this field existed, which load as
	// an empty slice: no bubbles, everyone on the world clock, which is what
	// those encounters meant.
	//
	// A list rather than a single optional bubble because a list grows to N
	// additively. There is no identifier per bubble on purpose — a bubble is
	// reached through a member (R6), never addressed by name.
	Bubbles []clock.TurnData `json:"bubbles,omitempty"`
	Intel   intel.Data       `json:"intel"`
	Log     record.LogData   `json:"log"`
	Field   FieldData        `json:"field"`
	Members []MemberData     `json:"members"`
	// Doors are the field's doors and the state each is in RIGHT NOW
	// (rpg-toolkit#1123). Top level rather than inside Field, beside Members
	// and for the same reason: a door's edges are construction truth but its
	// STATE is world state, and Field is the authored half. Absent in blobs
	// from before doors existed, which load as a field with none — an
	// ordinary field where every opening is a gap nobody can shut, which is
	// exactly what those encounters meant.
	Doors       []DoorData   `json:"doors,omitempty"`
	Endings     []EndingData `json:"endings"`
	EverMembers []MemberID   `json:"ever_members"`
	// Retention is the story-beat window this encounter was built with (see
	// SetupInput.Retention). Persisted so a reloaded encounter keeps the policy
	// it was constructed with rather than silently adopting the package default.
	// Absent in blobs written before #937, which load as zero and therefore take
	// DefaultRetention.
	Retention int `json:"retention,omitempty"`
}

// OutcomeData is the persistent representation of an Outcome.
type OutcomeData struct {
	Ending  string              `json:"ending"`
	At      uint64              `json:"at,omitempty"`
	Members []MemberOutcomeData `json:"members,omitempty"`
}

// MemberOutcomeData is where a member stood when the encounter closed.
//
// Cell is DUNGEON-ABSOLUTE (rpg-toolkit#1068), mirroring the MemberOutcome it
// persists. MemberData followed it in rpg-toolkit#1106, by the same detectable
// rename and for the same reason.
//
// It is a pointer under a NEW key on purpose. This value changed frame without
// changing type, and a bare pair of numbers cannot be told apart by
// inspection, so a blob written before the flip would have loaded clean and
// reported room-local cells as absolute ones — a party drawn in the wrong room
// by a load that reported success. Renaming "position" to "cell" makes the old
// dialect land nowhere, and its absence is then the signal: REQUIRED at load,
// rejected by name, never defaulted to (0,0) — a legal cell that would invent
// a placement. That is the same call RoomData.Origin makes for a missing
// anchor, and this is Kirk's fail-loudly ruling (2026-08-17) for the one
// persisted shape in this family that could be given a detectable one.
//
// THE REGION IS NOT STORED (rpg-toolkit#1108). It used to be, as "room", and it
// was the last derived spatial fact this module persisted: which region holds a
// member is a function of their cell and the authored field, both of which are
// in the blob already. A stored copy could only ever agree with them or lie, so
// load validation had to check it — a whole branch existing to police a second
// truth. It is re-derived at load through the same [regionAt] every live read
// uses, which is why a reloaded outcome cannot disagree with the one the host
// already saw. Blobs written with the old key still load: the key is ignored
// and the value recomputed, which is the same answer unless it was wrong.
type MemberOutcomeData struct {
	ID   MemberID      `json:"id"`
	Cell *PositionData `json:"cell"`
}

// FieldData is the persistent representation of the encounter's field — the
// AUTHORED rooms and their doorways, which is construction truth and the only
// thing worth storing: the canvas they compile into is derived from them, and a
// derived thing that is also stored is a second truth waiting to disagree.
// Rooms hold the composition's own descriptions (mirroring RoomInput exactly).
type FieldData struct {
	// Canvas is what the field declared about its map — today, what the space
	// between the authored chambers does to a sightline (rpg-toolkit#1116).
	// REQUIRED
	// at load, and written by ToData without omitempty, for RoomData.Origin's
	// reason: a declaration that persisted as absence could not be told apart
	// from a blob written before there was one.
	Canvas      CanvasData       `json:"canvas"`
	Rooms       []RoomData       `json:"rooms"`
	Connections []ConnectionData `json:"connections,omitempty"`
}

// CanvasData is the persistent representation of [CanvasInput].
//
// Void carries the declaration's own word ([VoidKind]), not an index into a
// set: a wire form that meant "the second kind" would silently reinterpret
// every old blob the day a third kind is added, which is RoomData.Grid's
// reasoning applied to a younger sealed set. An absent or unknown word is
// refused by name at load rather than defaulted — see voidFromData.
//
// Ambient light will land HERE when rpg-toolkit#1113 arrives, beside the void
// it is a fact of the same species as.
type CanvasData struct {
	Void string `json:"void"`

	// Orientation is which way this field's hexes point, or empty for a
	// square field (rpg-toolkit#1127). Carries the declaration's own word for
	// Void's reason — a wire form meaning "the second layout" would
	// reinterpret every old blob the day a third arrives.
	//
	// Omitted when empty so a square field's blob is byte-identical to what
	// it was before orientations existed, and REQUIRED for a hex one:
	// reloading a hex field without it would read every stored cell in the
	// wrong frame, which is a dungeon drawn correctly and played wrong.
	Orientation string `json:"orientation,omitempty"`
}

// RoomData mirrors RoomInput exactly to persist construction inputs — true
// again as of #929 T2 (a T1-era comment here briefly said otherwise, while
// Origin was Setup-only). Origin is REQUIRED at load (W5): a nil pointer
// means the field was absent from the blob (distinct from a declared
// zero) and is rejected with ErrInvalidData naming the room — a missing
// anchor must never silently default to (0,0), a legal position that
// would invent placement, mirroring ConnectionData's FromPosition/ToPosition
// precedent. ToData ALWAYS writes Origin, even the zero value (no
// omitempty) — a declared (0,0) persists as an explicit "origin":{"x":0,"y":0},
// never as absence, so presence itself is meaningful at load.
// Grid is persisted as spatial's own GridType string ("hex" — tools/spatial's
// GridTypeHex; "gridless" is a v0.2-era value no longer accepted at load,
// #929 T2), not the GridShape iota: the iota is an in-process enumeration
// order, not a wire contract, and would silently reinterpret old blobs if
// spatial ever reordered it. Grid omits when empty (the square zero value)
// so pre-v0.2 blobs and square-only encounters keep byte-identical output.
type RoomData struct {
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Grid   string `json:"grid,omitempty"`

	// Props are the room's authored things (rpg-toolkit#1128). Written
	// under a NEW key, and Occluders below is why.
	Props []PropData `json:"props,omitempty"`

	// Occluders is a TOMBSTONE: the key this room's contents used to be
	// written under, kept solely so a blob carrying it is refused by name.
	//
	// Renaming was not enough on its own, and that is the whole reason this
	// field exists. The precedent (rpg-toolkit#1053/#1068, Kirk's
	// fail-loudly ruling) is that a changed shape gets a detectable name so
	// the old dialect lands nowhere — but "lands nowhere" only SIGNALS
	// anything for a REQUIRED field, whose absence is then the defect.
	// A room's contents are optional: a chamber with nothing in it is
	// ordinary. So an old blob would have loaded clean as a room with no
	// props, silently dropping every blocker somebody authored — a party
	// walking through pillars in a dungeon whose load reported success.
	// That is the exact failure mode MemberOutcomeData.Cell's rename exists
	// to prevent, and here it needs a positive check rather than an absent
	// one.
	//
	// json.RawMessage rather than the old type because nothing decodes it:
	// the only question asked is whether the key was present at all.
	Occluders json.RawMessage `json:"occluders,omitempty"`

	Boundaries []BoundaryData `json:"boundaries,omitempty"`
	Origin     *PositionData  `json:"origin"`
}

// PropData is the persistent representation of a [PropInput].
//
// BOTH FLAGS ARE POINTERS HERE TOO, and for the input's reason rather than a
// mechanical mirror of it: a persisted false and a persisted nothing are
// different facts, and a blob that lost the difference would reload a coffin
// as decoration. Required at load, refused by name, never defaulted — the
// same call RoomData.Origin makes for a missing anchor.
type PropData struct {
	Ref               string       `json:"ref"`
	At                PositionData `json:"at"`
	BlocksMovement    *bool        `json:"blocks_movement"`
	BlocksLineOfSight *bool        `json:"blocks_line_of_sight"`
}

// PositionData is the persistent representation of spatial.Position.
type PositionData struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// BoundaryData is the persistent representation of spatial.Boundary.
type BoundaryData struct {
	From              PositionData `json:"from"`
	To                PositionData `json:"to"`
	BlocksMovement    bool         `json:"blocks_movement,omitempty"`
	BlocksLineOfSight bool         `json:"blocks_line_of_sight,omitempty"`
}

// ConnectionData is the persistent representation of a connection.
// FromPosition and ToPosition are required — ToData always populates
// both; a nil pointer at Load means the field was absent from the
// blob and is rejected (a connection without both endpoints has no
// meaning, and a missing endpoint must never silently default to
// (0,0), a legal cell that would invent topology).
type ConnectionData struct {
	ID           string        `json:"id"`
	From         string        `json:"from"`
	To           string        `json:"to"`
	FromPosition *PositionData `json:"from_position"`
	ToPosition   *PositionData `json:"to_position"`
}

// DoorData is the persistent representation of a door: what it is called,
// where it stands, and the state it is in right now.
//
// State carries the declaration's own word ([DoorStateKind]) rather than an
// index, for CanvasData.Void's reason: a wire form meaning "the third kind"
// would silently reinterpret every old blob the day a fourth is added. Absent
// or unknown is refused by name at load, never defaulted — see doorFromData.
//
// Lock is present exactly when State is "locked", and its absence there is a
// defect rather than a lock with no DC. Both halves are checked: a lock on an
// unlocked door is as wrong as a locked door with none.
type DoorData struct {
	ID    string     `json:"id"`
	Edges []EdgeData `json:"edges"`
	State string     `json:"state"`
	Lock  *LockData  `json:"lock,omitempty"`
}

// EdgeData is the persistent representation of a [DoorEdge]: one crossing,
// both cells dungeon-absolute.
type EdgeData struct {
	From PositionData `json:"from"`
	To   PositionData `json:"to"`
}

// LockData is the persistent representation of a [Lock]. Ability and Tool are
// opaque host/rulebook refs this module never looks inside, so they persist
// verbatim and omit when empty.
type LockData struct {
	DC      int    `json:"dc"`
	Ability string `json:"ability,omitempty"`
	Tool    string `json:"tool,omitempty"`
}

// doorDataFrom renders a door record for the blob.
func doorDataFrom(d *doorRecord) DoorData {
	out := DoorData{
		ID:    d.id,
		Edges: make([]EdgeData, 0, len(d.edges)),
		State: string(d.state.Kind()),
	}
	for _, e := range d.edges {
		out.Edges = append(out.Edges, EdgeData{
			From: PositionData{X: e.From.X, Y: e.From.Y},
			To:   PositionData{X: e.To.X, Y: e.To.Y},
		})
	}
	if lock, locked := d.state.Lock(); locked {
		out.Lock = &LockData{DC: lock.DC, Ability: lock.Ability, Tool: lock.Tool}
	}

	return out
}

// doorStateFromData resolves the persisted word and lock back to the state they
// name.
//
// Loud on every mismatch, per the standing precedent (rpg-toolkit#1053/#1068:
// fail loudly, no migration). An absent word is a door that never said; an
// unknown one is a blob from a dialect this build does not speak; a locked door
// with no lock, or an unlocked one carrying a lock, is a blob whose two halves
// disagree — and guessing which half is right is exactly what this module is
// not allowed to do.
func doorStateFromData(id string, name string, lock *LockData) (DoorState, error) {
	switch DoorStateKind(name) {
	case DoorOpen, DoorClosed:
		if lock != nil {
			return nil, fmt.Errorf("door %q is %s and carries a lock: %w", id, name, ErrBadDoor)
		}
		if DoorStateKind(name) == DoorOpen {
			return DoorIsOpen(), nil
		}
		return DoorIsClosed(), nil
	case DoorLocked:
		if lock == nil {
			return nil, fmt.Errorf("door %q is locked and says nothing about the lock: %w", id, ErrBadDoor)
		}
		return DoorIsLocked(Lock{DC: lock.DC, Ability: lock.Ability, Tool: lock.Tool}), nil
	case "":
		return nil, fmt.Errorf("door %q does not say what state it is in (doors[].state): %w", id, ErrBadDoor)
	default:
		return nil, fmt.Errorf("door %q is in state %q, which this build does not know (doors[].state): %w", id, name, ErrBadDoor)
	}
}

// convertDoorDataToDoorInput converts persisted doors back to construction
// inputs, so Load hands validateDoorInputs exactly what Setup does — the same
// shared-validator discipline #929 T2 established for rooms and connections,
// rather than a mirrored second implementation.
func convertDoorDataToDoorInput(doors []DoorData) ([]DoorInput, error) {
	out := make([]DoorInput, 0, len(doors))
	for _, dd := range doors {
		state, err := doorStateFromData(dd.ID, dd.State, dd.Lock)
		if err != nil {
			return nil, err
		}
		edges := make([]DoorEdge, 0, len(dd.Edges))
		for _, e := range dd.Edges {
			edges = append(edges, DoorEdge{
				From: spatial.Position{X: e.From.X, Y: e.From.Y},
				To:   spatial.Position{X: e.To.X, Y: e.To.Y},
			})
		}
		out = append(out, DoorInput{ID: dd.ID, Edges: edges, State: state})
	}

	return out, nil
}

// MemberData is the persistent representation of a member's current placement.
//
// Cell is DUNGEON-ABSOLUTE (rpg-toolkit#1106), like the outcome cell beside it
// and like every position this composition reports. There is no room field any
// more: which chamber a member stands in is decided by their cell and the
// authored footprints, so persisting a label alongside it would store an answer
// that can disagree with the question.
//
// It is a pointer under a NEW key on purpose, and for exactly the reason
// MemberOutcomeData.Cell is: this value changed frame without changing type,
// and a bare pair of numbers cannot be told apart by inspection. A blob written
// before the flip carries "room" and "position", and both land NOWHERE on this
// shape — so Cell arrives nil, and its absence is the signal. REQUIRED at load,
// rejected by name citing the issue that moved it, never defaulted to (0,0),
// which is a legal cell that would invent a placement. Kirk's fail-loudly
// ruling (2026-08-17), applied to the placement half of the pair #1068 already
// did the outcome half of.
//
// There is nothing to migrate: no consumer of this module persists this shape
// yet (rpg-api runs the old top-level encounter module), so the old dialect has
// no installed base — only a hand-kept fixture, which is exactly what should be
// recreated rather than reinterpreted.
type MemberData struct {
	ID        MemberID         `json:"id"`
	Kind      MemberKind       `json:"kind"`
	Name      string           `json:"name,omitempty"`
	Cell      *PositionData    `json:"cell"`
	SpeedFeet int              `json:"speed_feet,omitempty"`
	SightFeet int              `json:"sight_feet,omitempty"`
	Actions   []ActionViewData `json:"actions,omitempty"`
	Targeting string           `json:"targeting,omitempty"`
}

// ActionViewData is the persistent representation of an [ActionView] — a
// member's static fact about one action, round-tripped verbatim (Kirk,
// rpg-project#254 review: "round-tripped through ToData/LoadEncounter, as
// encounter-owned primitives"). core.Ref already carries its own JSON tags
// and needs no persisted twin of its own.
type ActionViewData struct {
	Ref       core.Ref `json:"ref"`
	Name      string   `json:"name,omitempty"`
	ReachFeet int      `json:"reach_feet,omitempty"`
	Kind      string   `json:"kind,omitempty"`
}

// EndingData is the persistent representation of a declared ending.
// Kind is one of "reached_position" or "external".
// Room, Position, and Member are only populated for reached_position triggers.
type EndingData struct {
	Key      string        `json:"key"`
	Kind     string        `json:"kind"`
	Room     string        `json:"room,omitempty"`
	Position *PositionData `json:"position,omitempty"`
	Member   MemberID      `json:"member,omitempty"`
}

// ToData returns a persistent snapshot of this Encounter.
// All slices and embedded data are deep-copied; mutating the returned
// EncounterData will not affect this Encounter (snapshot immunity).
func (e *Encounter) ToData() EncounterData {
	// Members in sorted-ID order: an unchanged encounter must produce
	// byte-identical snapshots (T6 review M1 — map iteration here made
	// ToData nondeterministic and the round-trip suite ~16% flaky).
	memberIDs := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		memberIDs = append(memberIDs, id)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })
	membersData := make([]MemberData, 0, len(memberIDs))
	for _, id := range memberIDs {
		m := e.members[id]
		cell, ok := e.canvas.GetEntityPosition(string(m.ID))
		if !ok {
			continue // Not placed (shouldn't happen in valid encounter)
		}
		membersData = append(membersData, MemberData{
			ID:        m.ID,
			Kind:      m.Kind,
			Name:      m.Name,
			Cell:      &PositionData{X: cell.X, Y: cell.Y},
			SpeedFeet: m.SpeedFeet,
			SightFeet: m.SightFeet,
			Actions:   actionViewDataFrom(m.Actions),
			Targeting: m.Targeting,
		})
	}

	// Deep-copy endings in declaration order
	endingsData := make([]EndingData, len(e.endings))
	for i, de := range e.endings {
		ed := EndingData{
			Key: de.key,
		}

		switch t := de.trigger.(type) {
		case TriggerReachedPosition:
			ed.Kind = "reached_position"
			ed.Room = t.Room
			ed.Position = &PositionData{X: t.Position.X, Y: t.Position.Y}
			ed.Member = t.Member
		case TriggerExternal:
			ed.Kind = "external"
		}

		endingsData[i] = ed
	}

	// Deep-copy everMembers in sorted order for determinism
	everMembersSlice := make([]MemberID, 0, len(e.everMembers))
	for id := range e.everMembers {
		everMembersSlice = append(everMembersSlice, id)
	}
	slices.SortFunc(everMembersSlice, func(a, b MemberID) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})

	// Deep-copy field from stored inputs
	fieldData := FieldData{
		Canvas: CanvasData{
			Void:        string(e.void.Kind()),
			Orientation: orientationName(e.orientation),
		},
		Rooms:       make([]RoomData, len(e.fieldInput)),
		Connections: make([]ConnectionData, len(e.connectionsInput)),
	}

	for i, ri := range e.fieldInput {
		rd := RoomData{
			ID:         ri.ID,
			Width:      ri.Width,
			Height:     ri.Height,
			Grid:       gridShapeToData(ri.Grid),
			Props:      make([]PropData, len(ri.Props)),
			Boundaries: make([]BoundaryData, len(ri.Boundaries)),
			// Always a fresh pointer, always written — even the zero value
			// (no omitempty, RoomData's own doc comment) — so a declared
			// origin (0,0) round-trips as explicit presence, not absence,
			// and two ToData calls never alias the same PositionData.
			Origin: &PositionData{X: ri.Origin.X, Y: ri.Origin.Y},
		}

		for j, p := range ri.Props {
			// Fresh pointers, never the input's own: two ToData calls must
			// not alias one bool, for RoomData.Origin's reason.
			blocksMovement, blocksSight := *p.BlocksMovement, *p.BlocksLineOfSight
			rd.Props[j] = PropData{
				Ref:               p.Ref,
				At:                PositionData{X: p.At.X, Y: p.At.Y},
				BlocksMovement:    &blocksMovement,
				BlocksLineOfSight: &blocksSight,
			}
		}

		for j, b := range ri.Boundaries {
			rd.Boundaries[j] = BoundaryData{
				From:              PositionData{X: b.From.X, Y: b.From.Y},
				To:                PositionData{X: b.To.X, Y: b.To.Y},
				BlocksMovement:    b.BlocksMovement,
				BlocksLineOfSight: b.BlocksLineOfSight,
			}
		}

		fieldData.Rooms[i] = rd
	}

	for i, ci := range e.connectionsInput {
		fieldData.Connections[i] = ConnectionData{
			ID:           ci.ID,
			From:         ci.From,
			To:           ci.To,
			FromPosition: &PositionData{X: ci.FromPosition.X, Y: ci.FromPosition.Y},
			ToPosition:   &PositionData{X: ci.ToPosition.X, Y: ci.ToPosition.Y},
		}
	}

	// Doors, in the records' own stable ID order (C8) — the state each is in
	// right now, not the one it was authored in.
	var doorData []DoorData
	if len(e.doors) > 0 {
		doorData = make([]DoorData, 0, len(e.doors))
		for _, d := range e.doors {
			doorData = append(doorData, doorDataFrom(d))
		}
	}

	// Build outcome if present
	var outcomeData *OutcomeData
	if e.outcome != nil {
		outcomeData = &OutcomeData{
			Ending:  e.outcome.Ending,
			At:      e.outcome.At,
			Members: make([]MemberOutcomeData, len(e.outcome.Members)),
		}
		for i, mo := range e.outcome.Members {
			outcomeData.Members[i] = MemberOutcomeData{
				ID: mo.ID,
				// Always a fresh pointer, always written — RoomData.Origin's
				// precedent — so presence itself is meaningful at load and two
				// ToData calls never alias the same PositionData.
				Cell: &PositionData{X: mo.Position.X, Y: mo.Position.Y},
			}
		}
	}

	var bubblesData []clock.TurnData
	if len(e.bubbles) > 0 {
		bubblesData = make([]clock.TurnData, 0, len(e.bubbles))
		for _, b := range e.bubbles {
			bubblesData = append(bubblesData, b.ToData())
		}
	}

	return EncounterData{
		Outcome:     outcomeData,
		Clock:       e.clock.ToData(),
		Bubbles:     bubblesData,
		Intel:       e.intelLog.ToData(),
		Log:         e.story.ToData(),
		Field:       fieldData,
		Members:     membersData,
		Doors:       doorData,
		Endings:     endingsData,
		EverMembers: everMembersSlice,
		Retention:   e.retention,
	}
}

// gridShapeToData maps a room's constructed grid shape to its persisted
// string form, reusing spatial's own GridType* constants
// (tools/spatial/data.go) so the wire vocabulary matches spatial's own
// persistence. Square (the zero value) maps to "" so byte-compat
// goldens keep omitting the field entirely. No GridShapeGridless case
// (#929 T2 second review round: deleted — write-only wire value at this
// point, since no RoomInput this function is ever called with can hold
// GridShapeGridless. At Setup, buildValidRoomGrids' shape-legality switch
// rejects it explicitly (encounter.go's doc comment on that switch); at
// Load it never even reaches that switch — gridDataToShape below rejects
// a stored "gridless" string at the wire layer, before a RoomInput ever
// exists, closing the load-side hole T1 left open — see gridDataToShape's
// own doc comment. Either way, a room is never stored in fieldInput and
// later handed to ToData while holding GridShapeGridless). The default
// case already returns "" for it, same as any other unreachable value.
// actionViewDataFrom converts a member's runtime [ActionView] facts to their
// persisted twin. A nil slice stays nil rather than becoming an allocated
// empty one (Copilot, PR #1187 review: `omitempty` already treats the two
// identically on the wire — len()==0 either way — so this is not a wire
// distinction; it is a plain no-op-for-the-common-case allocation avoidance,
// and keeps a round-tripped nil equal to its original by reflect.DeepEqual
// rather than becoming a spurious non-nil empty slice).
func actionViewDataFrom(actions []ActionView) []ActionViewData {
	if actions == nil {
		return nil
	}
	out := make([]ActionViewData, len(actions))
	for i, a := range actions {
		out[i] = ActionViewData(a)
	}
	return out
}

// actionViewsFrom is actionViewDataFrom's inverse, restoring a member's
// runtime [ActionView] facts from their persisted twin.
func actionViewsFrom(data []ActionViewData) []ActionView {
	if data == nil {
		return nil
	}
	out := make([]ActionView, len(data))
	for i, a := range data {
		out[i] = ActionView(a)
	}
	return out
}

func gridShapeToData(shape spatial.GridShape) string {
	switch shape {
	case spatial.GridShapeHex:
		return spatial.GridTypeHex
	default:
		return ""
	}
}

// gridDataToShape is gridShapeToData's inverse, used at Load. Empty
// string and the literal "square" both mean the zero-value shape —
// ToData never emits "square" (it omits the field), but Load accepts
// it defensively for hand-authored fixtures. An unrecognized string
// returns ok=false so the caller can reject with a fragment naming
// the bad value, rather than silently defaulting to square.
//
// "gridless" is deliberately NOT recognized (#929 T2): it was a v0.2
// value, dropped from the shape-legality-valid set in T1 (RoomInput.Grid's
// doc comment) — Setup has rejected it since T1, and a stored "gridless"
// string now falls through to the same ok=false rejection any other
// unrecognized string gets, closing the load-side hole T1 left open
// (a stored "gridless" blob loaded successfully until this change).
func gridDataToShape(s string) (shape spatial.GridShape, ok bool) {
	switch s {
	case "", spatial.GridTypeSquare:
		return spatial.GridShapeSquare, true
	case spatial.GridTypeHex:
		return spatial.GridShapeHex, true
	default:
		return spatial.GridShapeSquare, false
	}
}

// LoadEncounterInput carries everything LoadEncounter needs: what persisted, and
// what is alive for this call.
//
// The two are deliberately separate fields rather than one. Data is the
// persistence shape — the host round-trips it as bytes it never constructs, which
// is the whole basis of this module's replaceability promise. A Decider is a live
// behaviour object and could not ride on Data even if we wanted it to: it does not
// serialize. Keeping them apart in an Input states that distinction where a caller
// reads it, instead of leaving it implied by two positional parameters.
type LoadEncounterInput struct {
	// Data is the persisted encounter, as produced by Encounter.ToData.
	Data EncounterData

	// Deciders re-attaches behaviour to non-player members, keyed by member.
	// Nil is legal and means no member acts on its own. A player member naming a
	// Decider here is rejected (design law C2).
	Deciders map[MemberID]Decider

	// Initiative rolls the order a bubble forms in. REQUIRED, exactly as it is
	// on SetupInput: a loaded encounter runs trigger detection from its first
	// sight refresh, so it can start a fight before its caller does anything,
	// and one it cannot order is a misconfiguration. Refused here rather than
	// guarded where it is used — a nil roller is an error returned at the
	// door, not a branch taken deep inside a verb.
	Initiative InitiativeRoller

	// Standing reports which members are down. REQUIRED, exactly as it is on
	// SetupInput: a loaded encounter consults it on its first sight refresh,
	// so a blob that comes back without one is as unusable as a Setup without
	// one. Refused at the door, never guarded at the use site.
	Standing Standing

	// Sight reports how far each member can see, in cells. REQUIRED, exactly as
	// it is on SetupInput: a loaded encounter consults it on its first sight
	// refresh, so a blob that comes back without one is as unusable as a Setup
	// without one. Refused at the door, never guarded at the use site.
	Sight Sight

	// TurnDriver decides what a member with no player does when the fight's
	// clock lands on their turn. REQUIRED, exactly as it is on SetupInput: a
	// loaded encounter's bubble can land on an unplayed member the moment it
	// is reconstituted, so a blob that comes back without one is as unusable
	// as a Setup without one (rpg-toolkit#1162, ADR-0043). Refused at the
	// door, never guarded at the use site, and never defaulted.
	TurnDriver TurnDriver

	// Striker resolves and records a member's attack when a TurnDriver
	// returns an Attack intent. REQUIRED, exactly as it is on SetupInput and
	// for the same reason (rpg-project#254): a loaded encounter's bubble can
	// land on an unplayed member ready to swing the moment it is
	// reconstituted. Refused at the door, never guarded at the use site, and
	// never defaulted.
	Striker Striker
}

// Validate reports whether the input is usable. It checks only the input's own
// shape; everything about the encounter itself is validated by LoadEncounter.
//
// Returns ErrNilInput — not ErrInvalidData — for a nil input, matching
// NewEncounter and every other *XxxInput seam in this module. The distinction is
// the sentinel's own documented one: ErrNilInput "indicates a caller defect",
// while ErrInvalidData means the persisted blob does not describe a valid
// encounter. A nil input supplied no blob at all, so it cannot be invalid data.
func (in *LoadEncounterInput) Validate() error {
	if in == nil {
		return fmt.Errorf("load encounter: %w", ErrNilInput)
	}
	if in.Initiative == nil {
		return fmt.Errorf("load encounter: Initiative is required: %w", ErrNoInitiative)
	}
	if in.Standing == nil {
		return fmt.Errorf("load encounter: Standing is required: %w", ErrNoStanding)
	}
	if in.Sight == nil {
		return fmt.Errorf("load encounter: Sight is required: %w", ErrNoSight)
	}
	if in.TurnDriver == nil {
		return fmt.Errorf("load encounter: TurnDriver is required: %w", ErrNoTurnDriver)
	}
	if in.Striker == nil {
		return fmt.Errorf("load encounter: Striker is required: %w", ErrNoStriker)
	}

	return nil
}

// LoadEncounter reconstructs an Encounter from persistent data and re-attached deciders.
//
// Returns ErrNilInput for a nil input — a caller defect, distinct from the
// ErrInvalidData every rejection below carries, which means the persisted blob
// does not describe a valid encounter.
//
// Validation order (R5 — validate all before constructing): no rooms, no endings,
// empty/reserved ending keys, duplicate ending keys (#929 hardening round E, mirroring
// NewEncounter's identical check), kind/reached_position checks, undeclared outcome
// ending; THEN, per room list, a wire-only ID pre-pass (empty/duplicate room ID — #929 T2
// second review round, so a later presence error can never misname an empty/ambiguous
// ID), grid-shape resolution (unrecognized-or-no-longer-supported string) and origin
// presence (W5) per room; THEN room-list defects via the SAME buildValidRoomGrids Setup
// uses: shape legality, non-integral or duplicate occluder position in any family (#929
// T3 Opus round F2; duplicate — #929 hardening round D), W1 (one grid family per field),
// room legality (non-positive/oversized/over-cell-
// budget dimensions, out-of-bounds origin — maxRoomSpan/maxAnchorCoord/maxRoomCells/
// maxFieldCells), origin legality (non-representable origin, every family), W2 (rooms
// never overlap); THEN, per connection list, the SAME wire-only ID pre-pass and endpoint
// presence, then connection defects via validateConnectionInputs: unknown or
// self-referencing room, endpoint out of bounds, non-integral (hex), or on an occluder,
// W3 (non-kissing doorway); THEN empty or duplicate member IDs, member's room not in
// field, member cell presence (a missing cell is the pre-#1106 room-local dialect
// announcing itself — MemberData's doc comment) then integrality (hex) then that
// cell being owned by some authored room, ending trigger validity
// (unknown room or unreachable position on a TriggerReachedPosition — #929 T3 Opus round
// F5, the SAME validateEndingTriggers Setup uses), an abandoned outcome with members
// still present, outcome member cell PRESENCE (a missing cell is the pre-#1068
// room-local dialect announcing itself — MemberOutcomeData's doc comment) then that
// member's room and bounds, everMembers missing a current member.
//
// One check runs OUTSIDE this up-front pass, later, during member re-placement and
// decider re-attachment (construction has already begun by then): a player member
// naming a Decider in the reattachment map (design law C2, enforced identically at
// Setup, Join, and here). This is not an R5 violation — nothing external is mutated,
// and the partially-built Encounter is discarded on this error like any other — but it
// is a real ordering asymmetry against the up-front list above; a future cleanup could
// hoist it into the member validation loop instead.
//
// #929 T2: the room-list and connection validation is deliberately the SAME Setup
// runs, not a parallel reimplementation — Setup and Load diverging on the W-laws was
// flagged explicitly in T1 review as a drift risk. RoomData/ConnectionData convert to
// RoomInput/ConnectionInput FIRST (the ID pre-pass, grid-string resolution, and
// Origin/endpoint presence checks above — concerns that only exist at the wire layer,
// since RoomInput.Grid is already typed and RoomInput.Origin is already a value, not a
// pointer), then the converted slices are handed to
// buildValidRoomGrids/validateConnectionInputs verbatim; every error they return is
// wrapped once more with ErrInvalidData (multi-%w, this module's established load-error
// style — the underlying error already carries ErrNoField/ErrBadConnection). Neither
// shared validator's own error messages carry a verb prefix (#929 T2 second review
// round — buildValidRoomGrids' doc comment), so this wrap is the ONLY place "load
// encounter:" enters those messages — NewEncounter wraps the identical unprefixed
// errors with "newencounter:" instead, at its own call sites.
//
// Leaf loaders (clock, intel, record) are called and their rejections are wrapped.
// Intel gets one check of its own first, because it cannot make it itself: a stored
// sight payload naming a room is a pre-#1044 room-local frame, and intel holds
// payloads as opaque bytes by contract — see refuseRoomLocalSightings.
// On success, the authored rooms are compiled into the canvas via the same
// compileCanvas Setup uses (no re-surveil), and members are re-placed at their
// persisted absolute cells.
func LoadEncounter(input *LoadEncounterInput) (*Encounter, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	data, deciders := input.Data, input.Deciders

	// R5: Validate everything before constructing
	// No rooms
	if len(data.Field.Rooms) == 0 {
		return nil, fmt.Errorf("load encounter: no rooms: %w: %w", ErrInvalidData, ErrNoField)
	}

	// No endings
	if len(data.Endings) == 0 {
		return nil, fmt.Errorf("load encounter: bad endings: %w: %w", ErrInvalidData, ErrNoEnding)
	}

	// Validate ending keys and kinds: empty/reserved, duplicate (#929
	// hardening round E — the SAME liveness hole NewEncounter's
	// identical check closes, mirrored at Load), and kind.
	seenEndingKeys := make(map[string]bool, len(data.Endings))
	for _, ed := range data.Endings {
		if ed.Key == "" || ed.Key == "abandoned" {
			return nil, fmt.Errorf("load encounter: bad endings: %w: %w", ErrInvalidData, ErrNoEnding)
		}
		if seenEndingKeys[ed.Key] {
			return nil, fmt.Errorf("load encounter: duplicate ending %q: %w: %w", ed.Key, ErrInvalidData, ErrNoEnding)
		}
		seenEndingKeys[ed.Key] = true

		if ed.Kind != "reached_position" && ed.Kind != "external" {
			return nil, fmt.Errorf("load encounter: unknown ending kind %q: %w: %w", ed.Kind, ErrInvalidData, ErrNoEnding)
		}

		// A reached_position ending without a position would panic at
		// construction — LoadEncounter is the trust boundary for
		// persisted bytes and must reject, never crash (T6 review M2).
		if ed.Kind == "reached_position" && (ed.Position == nil || ed.Room == "") {
			return nil, fmt.Errorf("load encounter: ending %q reached_position without room/position: %w: %w", ed.Key, ErrInvalidData, ErrNoEnding)
		}
	}

	// Validate outcome (if present) references a declared ending
	if data.Outcome != nil {
		found := false
		if data.Outcome.Ending == "abandoned" {
			found = true // Reserved ending is valid
		} else {
			for _, ed := range data.Endings {
				if ed.Key == data.Outcome.Ending {
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("load encounter: outcome ending %q not declared: %w: %w", data.Outcome.Ending, ErrInvalidData, ErrNoEnding)
		}
	}

	// Convert the wire representation to RoomInput/ConnectionInput — the
	// ONLY load-specific pre-validation left (grid-string resolution, W5
	// origin presence, connection endpoint presence: concerns that exist
	// only because the wire form uses strings and pointers where the
	// construction form uses typed values) — then hand off to the SAME
	// room-list and connection validation Setup uses (see this function's
	// doc comment). Every remaining room-list/connection defect class
	// (shape legality, W1, room legality, origin legality, W2, W3, and the
	// existing bounds/occluder/self-connection checks) is enforced by
	// buildValidRoomGrids/validateConnectionInputs, not duplicated here.

	// What the space between the chambers does to a sightline, resolved from
	// the word the blob carries. Refused by name when absent or unknown — a guess here
	// would load a party into a dungeon whose walls the host never authored
	// (rpg-toolkit#1116; the standing no-migration precedent, #1053/#1068).
	void, err := voidFromData(data.Field.Canvas.Void)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}

	roomInputs, err := convertRoomDataToRoomInput(data.Field.Rooms)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}
	orientation, err := orientationFromData(fieldGridShape(roomInputs), data.Field.Canvas.Orientation)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}
	roomGrids, err := buildValidRoomGrids(roomInputs, orientation)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}

	connectionInputs, err := convertConnectionDataToConnectionInput(data.Field.Connections)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}
	if err = validateConnectionInputs(roomInputs, roomGrids, orientation, connectionInputs); err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}

	// Doors, through the SAME validator Setup runs (rpg-toolkit#1123). The
	// load-only part is resolving the persisted word and lock back into a
	// state; everything else a door can get wrong is checked once, in one
	// place, for both seams.
	doorInputs, err := convertDoorDataToDoorInput(data.Doors)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}
	if err = validateDoorInputs(roomInputs, roomGrids, orientation, doorInputs); err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}

	// Validate members: no duplicates, rooms exist, positions in bounds
	seenIDs := make(map[MemberID]bool)
	for _, m := range data.Members {
		// Empty member IDs are unreachable (Setup and Join both reject).
		if m.ID == "" {
			return nil, fmt.Errorf("load encounter: empty member id: %w: %w", ErrInvalidData, ErrNoMember)
		}
		if seenIDs[m.ID] {
			return nil, fmt.Errorf("load encounter: duplicate member %q: %w: %w", m.ID, ErrInvalidData, ErrNoMember)
		}
		seenIDs[m.ID] = true

		// A missing cell is how the pre-#1106 dialect announces itself: that
		// blob's "room" and "position" keys land nowhere on today's shape, so
		// the field arrives absent rather than wrong (MemberData's doc
		// comment). Checked FIRST, so the older mistake is named as itself
		// instead of surfacing as an out-of-bounds (0,0).
		if m.Cell == nil {
			return nil, fmt.Errorf(
				"load encounter: member %q has no cell — a room-local placement from before rpg-toolkit#1106, recreate the save: %w: %w",
				m.ID, ErrInvalidData, ErrBadPlacement)
		}

		cell := spatial.Position{X: m.Cell.X, Y: m.Cell.Y}

		// Hex fields require integral axial cells, asked FIRST and named as
		// itself — the same order and the same words Step and Join use
		// (isIntegralHexCell). Ownership refuses a fractional hex cell
		// too, but it would report it as "owned by no room", which sends
		// whoever reads it to the map instead of to their arithmetic; three
		// seams answering one defect three ways is exactly the drift #929 T2
		// made Setup and Load share validators to avoid. W1 gives the whole
		// field one grid family, so any room's grid answers for all of them.
		if !isIntegralHexCell(roomGrids[roomInputs[0].ID], cell) {
			return nil, fmt.Errorf("load encounter: member %q cell is not an integral axial cell: %w: %w", m.ID, ErrInvalidData, ErrBadPlacement)
		}

		// The cell is absolute and every room's grid speaks its own local
		// frame, so the bounds check runs the compile backwards: some region
		// must hold it. One check, two defects — a cell outside the field
		// entirely, and a cell in the space BETWEEN regions, which the canvas
		// spans but which is not floor. The SAME lookup the live verbs use
		// (rpg-toolkit#1108); it used to be a twin that had to be kept in step
		// by hand.
		if _, owned := regionAt(roomInputs, roomGrids, orientation, cell); !owned {
			return nil, fmt.Errorf("load encounter: member %q cell is owned by no region: %w: %w", m.ID, ErrInvalidData, ErrBadPlacement)
		}
	}

	// A TriggerReachedPosition ending must name a real room and reachable
	// position — the SAME shared validator Setup uses (#929 T3 Opus round
	// F5; validateEndingTriggers' doc comment), fed the wire endings
	// resolved to their runtime Trigger via endingTriggerFromData — the
	// SAME conversion the "Restore declared endings" construction below
	// reuses, so the switch on ed.Kind exists exactly once, not twice.
	endingInputsForValidation := make([]EndingInput, len(data.Endings))
	for i, ed := range data.Endings {
		endingInputsForValidation[i] = EndingInput{Key: ed.Key, Trigger: endingTriggerFromData(ed)}
	}
	if err := validateEndingTriggers(roomInputs, endingInputsForValidation, roomGrids); err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}

	// Outcome members must reference rooms that exist with in-bounds
	// positions (design R9 — the outcome list was wholly unvalidated,
	// T6 review M5), and an abandoned outcome with members still
	// present is unreachable (abandonment means the membership emptied).
	if data.Outcome != nil {
		if data.Outcome.Ending == "abandoned" && len(data.Members) > 0 {
			return nil, fmt.Errorf("load encounter: abandoned outcome with members present: %w: %w", ErrInvalidData, ErrNoMember)
		}
		for _, om := range data.Outcome.Members {
			// A missing cell is how the pre-#1068 dialect announces itself:
			// that blob's "position" key lands nowhere on today's shape, so
			// the field arrives absent rather than wrong (MemberOutcomeData's
			// doc comment). Checked FIRST, so the older mistake is named as
			// itself instead of surfacing as an out-of-bounds (0,0).
			if om.Cell == nil {
				return nil, fmt.Errorf("load encounter: outcome member %q has no cell — a room-local outcome from before rpg-toolkit#1068, recreate the save: %w: %w", om.ID, ErrInvalidData, ErrBadPlacement)
			}
			// Where they finished has to be floor — the SAME question, asked
			// the SAME way, as for a live member above. This used to be an
			// inline third implementation of region ownership, checking the
			// cell against the region NAMED BESIDE IT in the blob; with the
			// region derived rather than stored there is nothing to
			// cross-check, only somewhere to be (rpg-toolkit#1108).
			if _, owned := regionAt(roomInputs, roomGrids, orientation, spatial.Position{X: om.Cell.X, Y: om.Cell.Y}); !owned {
				return nil, fmt.Errorf("load encounter: outcome member %q cell is owned by no region: %w: %w", om.ID, ErrInvalidData, ErrBadPlacement)
			}
		}
	}

	// Validate everMembers contains all current members
	for _, m := range data.Members {
		found := false
		for _, em := range data.EverMembers {
			if em == m.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("load encounter: member %q missing from ever_members: %w: %w", m.ID, ErrInvalidData, ErrNoMember)
		}
	}

	// Leaf validation via loaders (delegated — they own their rules).
	// Load ONCE and keep the results: a second call with a discarded
	// error was a needless crash path (T6 review, item 8).
	loadedClock, err := clock.LoadTick(data.Clock)
	if err != nil {
		return nil, fmt.Errorf("load encounter clock: %w: %w", ErrInvalidData, err)
	}

	var loadedBubbles []*clock.Turn
	if len(data.Bubbles) > 0 {
		loadedBubbles = make([]*clock.Turn, 0, len(data.Bubbles))
		for i := range data.Bubbles {
			// An idle bubble has no meaning in a blob: a bubble exists only
			// while a fight does, and every verb that can empty one prunes it
			// in the same call (dropBubbleIfIdle) — this module never writes
			// this shape, so reading it means the blob was edited.
			if len(data.Bubbles[i].Order) == 0 {
				return nil, fmt.Errorf("load encounter bubble %d: idle bubble persisted: %w", i, ErrInvalidData)
			}
			b, berr := clock.LoadTurn(data.Bubbles[i])
			if berr != nil {
				return nil, fmt.Errorf("load encounter bubble %d: %w: %w", i, ErrInvalidData, berr)
			}
			loadedBubbles = append(loadedBubbles, b)
		}
	}

	// R6 — an entity belongs to at most one clock. Validated HERE rather than
	// trusted, because this is the trust boundary for persisted bytes: a blob
	// placing someone in two clocks at once would make ClockOf's answer depend
	// on iteration order, and the whole point of reaching a bubble through a
	// member is that the lookup is a function.
	// Only members may be on a clock, and no member may be on two.
	//
	// The membership half is not decoration. A non-member on the world clock
	// accrues budget on every Advance forever; a non-member in a bubble order
	// can be reported as Active, so ClockOf would answer a real member's
	// question by naming somebody who is not in the encounter. Neither
	// announces itself — the encounter simply runs with a passenger. LoadTick
	// rejects some of these incidentally (a budget above the high-water mark),
	// which is exactly the kind of accidental coverage that reads as a
	// guarantee: a ghost with budget 0 sails through, and a bubble made
	// entirely of non-members loaded clean before this check existed.
	isMember := make(map[core.EntityID]struct{}, len(data.Members))
	for _, m := range data.Members {
		isMember[m.ID] = struct{}{}
	}

	onAClock := make(map[core.EntityID]struct{})
	tickMembers, err := loadedClock.Members()
	if err != nil {
		return nil, fmt.Errorf("load encounter clock members: %w: %w", ErrInvalidData, err)
	}
	for _, id := range tickMembers {
		if _, ok := isMember[id]; !ok {
			return nil, fmt.Errorf(
				"load encounter clock: %q is on the world clock but is not a member: %w",
				id, ErrInvalidData)
		}
		onAClock[id] = struct{}{}
	}
	for i, b := range loadedBubbles {
		order, oerr := b.Order()
		if oerr != nil {
			return nil, fmt.Errorf("load encounter bubble %d order: %w: %w", i, ErrInvalidData, oerr)
		}
		for _, id := range order {
			if _, ok := isMember[id]; !ok {
				return nil, fmt.Errorf(
					"load encounter bubble %d: %q is in the order but is not a member: %w",
					i, id, ErrInvalidData)
			}
			if _, dup := onAClock[id]; dup {
				return nil, fmt.Errorf(
					"load encounter bubble %d: %q is on more than one clock: %w",
					i, id, ErrInvalidData)
			}
			onAClock[id] = struct{}{}
		}
	}

	if err = refuseRoomLocalSightings(data.Intel); err != nil {
		return nil, err
	}

	loadedIntel, err := intel.LoadIntel(data.Intel)
	if err != nil {
		return nil, fmt.Errorf("load encounter intel: %w: %w", ErrInvalidData, err)
	}

	loadedLog, err := record.LoadLog(data.Log)
	if err != nil {
		return nil, fmt.Errorf("load encounter log: %w: %w", ErrInvalidData, err)
	}

	// All validated; now reconstruct via the same path Setup uses
	e := &Encounter{
		members:     make(map[MemberID]*memberRecord),
		everMembers: make(map[MemberID]bool),
		deciders:    make(map[MemberID]Decider),
		initiative:  input.Initiative,
		standing:    input.Standing,
		sight:       input.Sight,
		turnDriver:  input.TurnDriver,
		striker:     input.Striker,
		endings:     nil,
		retention:   normalizeRetention(data.Retention),
		logFloor:    logFloorOf(data.Log),
		void:        void,
		orientation: orientation,
	}

	// Compile the authored rooms into the canvas — the SAME compileCanvas
	// NewEncounter runs, fed the SAME already-validated roomInputs, so a
	// reloaded encounter's map is built by one implementation rather than a
	// mirrored second one (#929 T2's shared-validator lesson, applied to
	// construction).
	e.doors, e.doorsByID = doorRecordsFrom(doorInputs)

	e.canvas, err = compileCanvas(roomInputs, roomGrids, void, orientation, e.doors)
	if err != nil {
		return nil, fmt.Errorf("load encounter: %w: %w", ErrInvalidData, err)
	}
	e.roomGrids = roomGrids

	// Load leaf state (constructors always succeed after validation)
	e.clock = loadedClock
	e.bubbles = loadedBubbles
	e.intelLog = loadedIntel
	e.story = loadedLog

	// Re-place members at persisted positions (no surveil here — outcomes already in intel)
	for _, m := range data.Members {
		entity := &memberEntity{
			id:   string(m.ID),
			kind: m.Kind,
		}

		if err = e.canvas.PlaceEntity(entity, spatial.Position{X: m.Cell.X, Y: m.Cell.Y}); err != nil {
			return nil, fmt.Errorf("load encounter member placement: %w: %w: %w", ErrInvalidData, ErrBadPlacement, err)
		}

		member := &memberRecord{
			ID:        m.ID,
			Kind:      m.Kind,
			Name:      m.Name,
			SpeedFeet: m.SpeedFeet,
			SightFeet: m.SightFeet,
			Actions:   actionViewsFrom(m.Actions),
			Targeting: m.Targeting,
		}
		e.members[m.ID] = member
		e.everMembers[m.ID] = true

		// Re-attach decider if present and non-nil — a literal nil entry
		// in the reattachment map is equivalent to an ABSENT one: a
		// monster without a decider is legal and simply holds (Setup and
		// Join already treat a nil MemberInput.Decider this way). Storing
		// a nil Decider interface here would panic Pump's first Decide
		// call on that monster (reject-never-crash: LoadEncounter is the
		// trust boundary for the caller-supplied reattachment map too,
		// not just the persisted bytes). Players cannot carry deciders
		// regardless (C2, enforced at all three seams: Setup, Join, load).
		if d, ok := deciders[m.ID]; ok && d != nil {
			if m.Kind == KindPlayer {
				return nil, fmt.Errorf("load encounter: player %s cannot carry a decider: %w: %w", m.ID, ErrInvalidData, ErrNoMember)
			}
			e.deciders[m.ID] = d
		}
	}

	// Put every member on a clock that is not already on one.
	//
	// This is what makes the field retrofittable with no migration. A blob
	// written before members were tracked on the world clock has an empty
	// budget map and no bubbles, so every member lands here and goes to the
	// world clock — which is exactly what such an encounter meant. A blob
	// written after carries its own membership and this loop finds nothing to
	// do. Deriving the default rather than storing a flag is deliberate: a
	// migration nobody runs is a migration that silently did not happen.
	for _, m := range data.Members {
		if _, ok := onAClock[core.EntityID(m.ID)]; ok {
			continue
		}
		if _, cerr := e.clock.Join(&clock.JoinInput{ID: core.EntityID(m.ID)}); cerr != nil {
			return nil, fmt.Errorf("load encounter member %q world clock: %w: %w", m.ID, ErrInvalidData, cerr)
		}
	}

	// Restore the full ever-members set (exited members keep Story access)
	for _, em := range data.EverMembers {
		e.everMembers[em] = true
	}

	// Restore declared endings — endingTriggerFromData is the SAME conversion
	// the ending-trigger validation above already ran, and compileEndings is
	// the SAME projection Setup runs, so a reloaded encounter's endings fire on
	// the cells the original's did.
	e.endings = compileEndings(endingInputsForValidation, roomInputs, e.orientation)

	// Restore outcome if present
	if data.Outcome != nil {
		outcome := &Outcome{
			Ending:  data.Outcome.Ending,
			At:      data.Outcome.At,
			Members: make([]MemberOutcome, len(data.Outcome.Members)),
		}
		for i, m := range data.Outcome.Members {
			cell := spatial.Position{X: m.Cell.X, Y: m.Cell.Y}
			region, _ := regionAt(roomInputs, roomGrids, orientation, cell)
			outcome.Members[i] = MemberOutcome{
				ID: m.ID,
				// The region is DERIVED from the cell, through the same
				// lookup every live read uses (rpg-toolkit#1108). Ownership
				// was checked for every outcome cell before construction
				// began (R5), so this cannot come back empty.
				Region: region,
				// The cell is stored and returned in the frame it was
				// reported in — no re-derivation, so a reloaded outcome and
				// the one the host already saw cannot disagree. Non-nil by
				// R5: every cell was checked before construction began.
				Position: cell,
			}
		}
		e.outcome = outcome
	}

	// Store field and connections inputs for future ToData calls — the
	// SAME roomInputs/connectionInputs already built (and validated)
	// above, not re-converted: both were freshly allocated from data by
	// convertRoomDataToRoomInput/convertConnectionDataToConnectionInput,
	// never aliasing the caller's data (T6 review M4's alias-immunity
	// requirement, pinned at this seam by TestAliasImmunityLoadEncounter).
	// Connections are kept sorted by ID (C8 determinism — order is
	// observable in ToData).
	e.fieldInput = roomInputs
	e.connectionsInput = connectionInputs
	sort.Slice(e.connectionsInput, func(i, j int) bool { return e.connectionsInput[i].ID < e.connectionsInput[j].ID })

	return e, nil
}

// endingTriggerFromData converts one persisted ending's Kind/Room/Position/
// Member into its runtime Trigger. Shared by two call sites in
// LoadEncounter above — the ending-trigger validation (which needs it
// early, right after roomGrids exists, so validateEndingTriggers has
// something to check) and the "Restore declared endings" construction
// (which needs it again, once construction is safe to begin, R5) — ONE
// conversion, not two copies of the same switch (#929 T3 Opus round F5).
// ed.Kind is already guaranteed to be "reached_position" or "external" by
// the key/kind checks earlier in LoadEncounter, and a "reached_position"
// ed.Position is already guaranteed non-nil there too — both preconditions
// checked before this is ever called, so no error return is needed here.
func endingTriggerFromData(ed EndingData) Trigger {
	switch ed.Kind {
	case "reached_position":
		return TriggerReachedPosition{
			Room:     ed.Room,
			Position: spatial.Position{X: ed.Position.X, Y: ed.Position.Y},
			Member:   ed.Member,
		}
	case "external":
		return TriggerExternal{}
	}
	return nil
}

// refuseRoomLocalSightings rejects a persisted sight payload written in the
// dialect rpg-toolkit#1044 replaced.
//
// Intel round-trips payloads as opaque bytes and carries no version — that is
// the leaf's whole contract, and it means nothing beneath this composition can
// notice that stored bytes now mean something different. A sighting written
// before sight payloads went dungeon-absolute carries a "room" key beside a
// room-LOCAL cell; decoded through today's SightPayload it becomes an absolute
// cell in some other room entirely, or in no room at all, and the load reports
// success. Load never re-derives sight (see the reconstruction below — the
// outcomes are already in intel), so nothing downstream corrects it either.
//
// Kirk's ruling, 2026-08-17: fail loudly, no migration. The only blobs in
// existence are dev and workbench saves, so a stale one is refused by name and
// recreated rather than silently reinterpreted.
func refuseRoomLocalSightings(data intel.Data) error {
	// Sorted, so a blob holding several stale sightings names the same one on
	// every run — a rejection that moves under map iteration is a rejection
	// nobody can write a test against.
	observers := make([]core.EntityID, 0, len(data.Holdings))
	for observer := range data.Holdings {
		observers = append(observers, observer)
	}
	slices.Sort(observers)

	for _, observer := range observers {
		subjects := make([]intel.Subject, 0, len(data.Holdings[observer]))
		for subject := range data.Holdings[observer] {
			subjects = append(subjects, subject)
		}
		slices.Sort(subjects)

		for _, subject := range subjects {
			holding := data.Holdings[observer][subject]
			if holding.Channel != intel.Sight || len(holding.Payload) == 0 {
				continue
			}
			// A payload this module cannot read as an object at all is
			// somebody else's: intel carries testimony for any channel a
			// composition invents, and only the room key THIS one used to
			// write is ours to recognize. Unreadable bytes are left to
			// whoever wrote them.
			var peek map[string]json.RawMessage
			if err := json.Unmarshal(holding.Payload, &peek); err != nil {
				continue
			}
			// PRESENCE of the key, not its value. Decoding "room" into a
			// typed field let a null through as absent and a non-string
			// through as unparseable (raised by Copilot on #1072, and both
			// loaded clean before this) — and since this composition is the
			// only writer of sight payloads, a payload naming a room AT ALL
			// is not one it wrote today, whatever the name decodes to.
			if room, named := peek["room"]; named {
				return fmt.Errorf(
					"load encounter intel: %q's sighting of %q names a room (%s) — a room-local sight payload from before rpg-toolkit#1044, recreate the save: %w",
					observer, subject, room, ErrInvalidData)
			}
		}
	}
	return nil
}

// convertRoomDataToRoomInput converts RoomData to RoomInput, both for the
// SAME room-list validation Setup uses (buildValidRoomGrids — see
// LoadEncounter's doc comment) and for later storage. This is the ONLY
// place LoadEncounter resolves the wire-only concerns that don't exist on
// RoomInput itself: ID presence/uniqueness (a cheap pre-pass, #929 T2
// second review round — see below), the Grid string (rejecting an
// unrecognized value, including the no-longer-supported "gridless" —
// gridDataToShape's doc comment), and Origin's W5 presence requirement (a
// nil pointer means the field was absent from the blob, distinct from a
// declared zero — RoomData's doc comment). All reject with ErrNoField,
// matching the room-list defect vocabulary buildValidRoomGrids itself uses
// for every OTHER room-list defect, so a caller inspecting the error chain
// sees one consistent sentinel regardless of which check fired.
//
// The ID pre-pass runs as its OWN first pass over the raw wire IDs, before
// any other conversion: without it, an empty or duplicate room ID could
// reach the Grid/Origin checks below and produce a message naming that
// same empty or ambiguous ID — e.g. `room "" missing origin` — instead of
// the actual defect (the ID itself). buildValidRoomGrids ALSO checks
// ID empty/duplicate, but only after every room in the list has already
// survived conversion — too late to prevent THIS symptom. The pre-pass is
// intentionally redundant with that later check for a room list that
// passes it: its only job is to guarantee an ID-defective room never
// reaches a presence error that would misname it.
func convertRoomDataToRoomInput(rooms []RoomData) ([]RoomInput, error) {
	seenIDs := make(map[string]bool, len(rooms))
	for _, rd := range rooms {
		if rd.ID == "" {
			return nil, fmt.Errorf("room has empty id: %w", ErrNoField)
		}
		if seenIDs[rd.ID] {
			return nil, fmt.Errorf("duplicate room %q: %w", rd.ID, ErrNoField)
		}
		seenIDs[rd.ID] = true
	}

	result := make([]RoomInput, len(rooms))
	for i, rd := range rooms {
		shape, ok := gridDataToShape(rd.Grid)
		if !ok {
			return nil, fmt.Errorf("room %q has unknown grid shape %q: %w", rd.ID, rd.Grid, ErrNoField)
		}
		if rd.Origin == nil {
			return nil, fmt.Errorf("room %q missing origin: %w", rd.ID, ErrNoField)
		}
		// The tombstone, checked before anything is built from this room —
		// see RoomData.Occluders for why a rename alone would have loaded
		// this blob clean and thrown its blockers away.
		if len(rd.Occluders) > 0 {
			return nil, fmt.Errorf(
				"room %q carries occluders, a dialect this build does not speak: "+
					"a room's contents are props now, and each says its ref and what it "+
					"blocks (rpg-toolkit#1128): %w", rd.ID, ErrNoField)
		}

		ri := RoomInput{
			ID:         rd.ID,
			Width:      rd.Width,
			Height:     rd.Height,
			Grid:       shape,
			Props:      make([]PropInput, len(rd.Props)),
			Boundaries: make([]spatial.Boundary, len(rd.Boundaries)),
			Origin:     spatial.Position{X: rd.Origin.X, Y: rd.Origin.Y},
		}

		for j, pd := range rd.Props {
			// REQUIRED at load, both of them, by name. A persisted prop that
			// does not say what it blocks is a blob from before this module
			// asked — loading it under a guess would put a party in a room
			// whose blockers the host never authored (PropData).
			if pd.BlocksMovement == nil {
				return nil, fmt.Errorf("room %q prop %q does not say whether it blocks_movement: %w",
					rd.ID, pd.Ref, ErrNoField)
			}
			if pd.BlocksLineOfSight == nil {
				return nil, fmt.Errorf("room %q prop %q does not say whether it blocks_line_of_sight: %w",
					rd.ID, pd.Ref, ErrNoField)
			}
			// Fresh pointers again: a loaded RoomInput must not alias the
			// caller's EncounterData (snapshot immunity, both directions).
			blocksMovement, blocksSight := *pd.BlocksMovement, *pd.BlocksLineOfSight
			ri.Props[j] = PropInput{
				Ref:               pd.Ref,
				At:                spatial.Position{X: pd.At.X, Y: pd.At.Y},
				BlocksMovement:    &blocksMovement,
				BlocksLineOfSight: &blocksSight,
			}
		}

		for j, bd := range rd.Boundaries {
			ri.Boundaries[j] = spatial.Boundary{
				From:              spatial.Position{X: bd.From.X, Y: bd.From.Y},
				To:                spatial.Position{X: bd.To.X, Y: bd.To.Y},
				BlocksMovement:    bd.BlocksMovement,
				BlocksLineOfSight: bd.BlocksLineOfSight,
			}
		}

		result[i] = ri
	}
	return result, nil
}

// convertConnectionDataToConnectionInput converts ConnectionData to
// ConnectionInput, both for the SAME connection validation Setup uses
// (validateConnectionInputs — see LoadEncounter's doc comment) and for
// later storage. This is the ONLY place LoadEncounter resolves the
// wire-only concerns that don't exist on ConnectionInput itself: ID
// presence/uniqueness (a cheap pre-pass, #929 T2 second review round — same
// reasoning as convertRoomDataToRoomInput's own pre-pass, so an endpoint-
// presence error can never misname an empty/ambiguous connection ID) and a
// nil FromPosition/ToPosition pointer, meaning the field was absent from
// the blob, not merely zero-valued (ConnectionData's doc comment). Rejects
// with ErrBadConnection, matching validateConnectionInputs' own vocabulary
// for every other connection defect.
func convertConnectionDataToConnectionInput(conns []ConnectionData) ([]ConnectionInput, error) {
	seenIDs := make(map[string]bool, len(conns))
	for _, cd := range conns {
		if cd.ID == "" {
			return nil, fmt.Errorf("connection has empty id: %w", ErrBadConnection)
		}
		if seenIDs[cd.ID] {
			return nil, fmt.Errorf("duplicate connection %q: %w", cd.ID, ErrBadConnection)
		}
		seenIDs[cd.ID] = true
	}

	result := make([]ConnectionInput, len(conns))
	for i, cd := range conns {
		if cd.FromPosition == nil {
			return nil, fmt.Errorf("connection %q missing from_position: %w", cd.ID, ErrBadConnection)
		}
		if cd.ToPosition == nil {
			return nil, fmt.Errorf("connection %q missing to_position: %w", cd.ID, ErrBadConnection)
		}
		result[i] = ConnectionInput{
			ID:           cd.ID,
			From:         cd.From,
			To:           cd.To,
			FromPosition: spatial.Position{X: cd.FromPosition.X, Y: cd.FromPosition.Y},
			ToPosition:   spatial.Position{X: cd.ToPosition.X, Y: cd.ToPosition.Y},
		}
	}
	return result, nil
}
