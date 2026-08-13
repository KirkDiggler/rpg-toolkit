// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MemberKind categorizes whether a member is a player or monster.
type MemberKind string

const (
	// KindPlayer indicates a player-controlled member.
	KindPlayer MemberKind = "player"

	// KindMonster indicates a monster member.
	KindMonster MemberKind = "monster"
)

// MemberID is an alias for core.EntityID used for clarity in member contexts.
type MemberID = core.EntityID

// RoomInput describes a room to be created.
type RoomInput struct {
	// ID is the unique room identifier.
	ID string

	// Width is the room's horizontal dimension.
	Width int

	// Height is the room's vertical dimension.
	Height int

	// Grid selects the room's coordinate system: GridShapeSquare (the zero
	// value — Width x Height cells, origin (0,0), Chebyshev distance) or
	// GridShapeHex — the only two families Setup and Load accept as of
	// #929 T1/T2. GridShapeGridless still exists as a spatial.GridShape
	// value but is rejected outright (shape legality — buildValidRoomGrids'
	// doc comment in encounter.go): gridless left the composition in v0.3,
	// the wire cannot carry a continuous room's absolute projection. The
	// zero value keeps every pre-existing room square, so v0.1 persisted
	// blobs without this field unmarshal to square unchanged.
	//
	// GridShapeHex rooms speak AXIAL cube coordinates (tools/spatial's
	// AxialHexGrid), not offset: Position.X is Q, Position.Y is R, and S
	// = -(Q+R) is derived. Bounds are ORIGIN-CENTERED spans, unlike
	// square — Q is valid in [-Width/2, Width/2) and R in
	// [-Height/2, Height/2), so negative coordinates are legal and
	// expected, not a defect. Distance, adjacency, and line of sight in a
	// hex room run true cube hex math via spatial. This is a deliberate
	// choice, not an implementation detail: the wire (and Platform's
	// pathing) already speaks cube coordinates natively, and axial is
	// cube's 2D projection — an IDENTITY mapping to the wire. A bounded
	// offset column/row grid (spatial's HexGrid) would force a lossy,
	// orientation-dependent offset<->cube conversion at that seam for no
	// benefit, since the composition never renders a grid itself.
	Grid spatial.GridShape

	// Occluders are positions that block line of sight.
	Occluders []spatial.Position

	// Boundaries define walls or barriers between adjacent cells.
	Boundaries []spatial.Boundary

	// Origin is this room's dungeon-absolute anchor: local (0,0) maps to
	// Origin in the field's shared absolute space. Local→absolute is
	// element-wise addition (local cell + Origin) — for hex rooms this is
	// ordinary axial cube arithmetic (axial+axial is valid cube math), not
	// a special case. The zero value anchors a room at the absolute
	// origin, which is legal on its own; in a multi-room field, leaving
	// every Origin at its zero value collides every room at (0,0) and is
	// rejected by W2 (see NewEncounter) — there is no separate
	// "origin required" check.
	//
	// Origin must be an INTEGRAL cell (X and Y both whole numbers) for
	// EVERY grid family, square included — not just hex. W2's "rooms never
	// overlap" promise is only sound over an integer cell lattice: two 5x5
	// SQUARE rooms anchored at (0,0) and (0.5,0.5) have disjoint integer
	// cell sets (a naive per-cell W2 check would accept them) while their
	// continuous footprints interpenetrate roughly 81% of each room's
	// area — a fractional origin defeats the very disjointness this field
	// exists to guarantee, not just a hex-specific edge case.
	//
	// Both construction seams — Setup and Load — validate every field
	// against the W-laws identically (#929 T2: LoadEncounter routes
	// through the SAME shared validators Setup uses, not a parallel
	// reimplementation): W1 (one grid family per field), W2 (rooms never
	// overlap in absolute space), and W3 (every connection's endpoints
	// are adjacent absolute cells). Rules and verbs (Move, View,
	// Traverse, ...) stay room-local — absolute coordinates only ever
	// appear in query OUTPUTS: Atlas, Absolute, and Locate project
	// through Origin (W4 — "projection is a read", #929 T3), never a
	// rule's own logic. Origin also round-trips through persistence
	// (RoomData.Origin, #929 T2).
	Origin spatial.Position
}

// ConnectionInput describes a connection between two rooms: a bidirectional
// open doorway. FromPosition and ToPosition are the endpoint cells — the
// position a member must stand on in each room to traverse the connection.
// Traversal itself is not implemented here; this only declares where the
// doorway sits.
type ConnectionInput struct {
	// ID is the unique connection identifier.
	ID string

	// From is the source room ID.
	From string

	// To is the destination room ID.
	To string

	// FromPosition is the endpoint cell within room From.
	FromPosition spatial.Position

	// ToPosition is the endpoint cell within room To.
	ToPosition spatial.Position
}

// FieldInput describes the layout of rooms and connections.
type FieldInput struct {
	// Rooms is the list of rooms in this field.
	Rooms []RoomInput

	// Connections is the list of connections between rooms.
	Connections []ConnectionInput
}

// MemberInput describes a member being placed into the encounter.
type MemberInput struct {
	// ID is the member's unique identifier.
	ID MemberID

	// Kind is the member's category (player or monster).
	Kind MemberKind

	// Room is the ID of the room where the member is placed.
	Room string

	// Position is the member's location within the room.
	Position spatial.Position

	// Decider is the monster's decision-making engine (monsters only).
	// Players must not have a Decider; passing one for a player will fail validation.
	// Deciders are NOT persisted; they are re-registered at load.
	Decider Decider
}

// Trigger is an interface for ending conditions.
type Trigger interface {
	isTrigger()
}

// TriggerReachedPosition fires when a member reaches a specific position.
type TriggerReachedPosition struct {
	// Room is the target room ID.
	Room string

	// Position is the target position within the room.
	Position spatial.Position

	// Member is the target member ID (empty = any player member).
	// This v1 resolution means: if empty, the ending fires when ANY player
	// member (not monster) reaches the position.
	Member MemberID
}

// isTrigger marks TriggerReachedPosition as a Trigger.
func (t TriggerReachedPosition) isTrigger() {}

// TriggerExternal fires when the external caller requests it.
type TriggerExternal struct{}

// isTrigger marks TriggerExternal as a Trigger.
func (t TriggerExternal) isTrigger() {}

// EndingInput describes a declared encounter ending.
type EndingInput struct {
	// Key is the unique ending identifier.
	Key string

	// Trigger defines when this ending fires.
	Trigger Trigger
}

// SetupInput contains all information needed to construct an encounter.
type SetupInput struct {
	// Field describes the spatial layout.
	Field FieldInput

	// Members are the initial roster.
	Members []MemberInput

	// Endings are the declared ways the encounter can close.
	Endings []EndingInput

	// Retention is how many story beats the encounter keeps. Older beats are
	// trimmed after each append, so an encounter's blob does not grow without
	// bound and a save does not rewrite the whole history.
	//
	// Zero selects DefaultRetention. RetentionUnbounded disables trimming —
	// appropriate for verified-transcript scenes, which are asserting on the
	// story itself rather than on the retention policy.
	//
	// The default is deliberately small, and that is a test strategy rather
	// than a storage economy: a generous window means the full-resync path
	// almost never runs and stays unexercised until a real player's
	// connection drops. A small one makes resync the common path, so the
	// expensive branch is the well-trodden one (#937).
	//
	// Retention persists with the encounter, so a reloaded encounter keeps
	// the policy it was built with.
	Retention int
}

// ViewInput is used to query a member's current percepts.
type ViewInput struct {
	// Member is the observer's ID.
	Member MemberID
}

// StoryInput is used to query a member's story entries after a given sequence number.
type StoryInput struct {
	// Audience is the member requesting their story.
	Audience MemberID

	// AfterSeq is the sequence number after which to return entries.
	AfterSeq uint64
}

// Member represents a member's public read-side data.
type Member struct {
	ID   MemberID
	Kind MemberKind
	Room string
}

// Status represents the encounter's open/closed state.
type Status struct {
	// Open indicates whether the encounter is active.
	Open bool

	// Outcome is the ending, if closed.
	Outcome *Outcome
}

// Outcome is returned when an encounter closes.
type Outcome struct {
	// Ending is the key of the ending that fired.
	Ending string

	// At is the clock reading when the ending fired.
	At uint64

	// Members are the final positions of all members.
	Members []MemberOutcome
}

// MemberOutcome is a member's position when the encounter closed.
type MemberOutcome struct {
	// ID is the member's identifier.
	ID MemberID

	// Room is their room ID when the encounter closed.
	Room string

	// Position is their position within the room.
	Position spatial.Position
}

// MoveInput contains the member and target position for a movement action.
type MoveInput struct {
	// Member is the ID of the member moving.
	Member MemberID

	// To is the target position within the same room (v1 constraint).
	To spatial.Position
}

// MoveOutput reports the results of a movement action.
type MoveOutput struct {
	// Moved contains the member's ID, original position, and new position.
	Moved struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}

	// IntelDeltas maps member IDs to their updated percepts after movement
	// (SurveilOutput deltas from the refreshSight cycle).
	IntelDeltas map[MemberID]*intel.SurveilOutput

	// Seq is the sequence number of the recorded movement beat.
	Seq uint64

	// Outcome is the encounter outcome if an ending fired; nil otherwise.
	Outcome *Outcome
}

// TraverseInput contains the member and connection to traverse. The member
// must be standing exactly on one of the connection's two endpoints; they
// arrive at the other.
type TraverseInput struct {
	// Member is the ID of the member traversing.
	Member MemberID

	// Connection is the ID of the connection to traverse.
	Connection string
}

// TraverseOutput reports the results of a traversal action.
type TraverseOutput struct {
	// Traversed contains the member's ID, departure room/position, and
	// arrival room/position.
	Traversed struct {
		Member   MemberID
		FromRoom string
		From     spatial.Position
		ToRoom   string
		To       spatial.Position
	}

	// IntelDeltas maps member IDs to their updated percepts after traversal
	// (SurveilOutput deltas from the refreshSight cycle, across both rooms).
	IntelDeltas map[MemberID]*intel.SurveilOutput

	// Seq is the sequence number of the recorded traversal beat.
	Seq uint64

	// Outcome is the encounter outcome if an ending fired; nil otherwise.
	Outcome *Outcome
}

// PumpInput contains no parameters; the pump is parameterless in wave 1.
type PumpInput struct{}

// PumpOutput reports the results of a world tick.
type PumpOutput struct {
	// Tick is the exploration clock's reading after the advance.
	Tick uint64

	// MonsterMoves contains the successful same-room moves executed by monsters during this pump.
	MonsterMoves []struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}

	// MonsterTraverses contains the successful cross-room traverses executed by
	// monsters during this pump (IntentTraverse). An illegal traverse intent
	// (unknown connection, or the monster not at its threshold) does not appear
	// here — it is silently skipped, matching MonsterMoves' spatial-rejection
	// contract.
	MonsterTraverses []struct {
		Member   MemberID
		FromRoom string
		From     spatial.Position
		ToRoom   string
		To       spatial.Position
	}

	// IntelDeltas maps member IDs to their updated percepts after all monster actions
	// (SurveilOutput deltas from the single refreshSight cycle).
	IntelDeltas map[MemberID]*intel.SurveilOutput

	// Seqs contains the sequence numbers of the recorded beats (tick beat
	// first, then move/traverse beats in decision order).
	Seqs []uint64

	// Outcome is the encounter outcome if an ending fired; nil otherwise.
	Outcome *Outcome
}

// JoinInput contains the member and placement information for joining the encounter.
type JoinInput struct {
	// Member describes the joining member (ID, kind, room, position, optional decider).
	Member MemberInput
}

// JoinOutput reports the results of a successful join.
type JoinOutput struct {
	// Member is the joined member's read-side data.
	Member Member

	// IntelDeltas maps member IDs to their updated percepts after the join
	// (SurveilOutput deltas from the refreshSight cycle).
	IntelDeltas map[MemberID]*intel.SurveilOutput

	// Seq is the sequence number of the recorded join beat.
	Seq uint64

	// Outcome is the encounter outcome if an ending fired during join; nil otherwise.
	Outcome *Outcome
}

// ExitInput contains the member ID of the member exiting.
type ExitInput struct {
	// Member is the ID of the member exiting.
	Member MemberID
}

// ExitOutput reports the results of a successful exit.
type ExitOutput struct {
	// Outcome is the exiting member's final placement and holdings.
	Outcome MemberOutcome

	// Carry contains the exiting member's holdings at the time of exit (copy-out).
	Carry []intel.Holding

	// Seq is the sequence number of the recorded exit beat.
	Seq uint64

	// Closed is the encounter outcome if the encounter auto-closed due to
	// the last member exiting; nil otherwise.
	Closed *Outcome
}

// EndInput contains the ending key to fire.
type EndInput struct {
	// Ending is the key of the ending to fire (must be External).
	Ending string
}

// EndOutput reports the results of a successful ending.
type EndOutput struct {
	// Outcome is the final state of the encounter when closed.
	Outcome Outcome
}
