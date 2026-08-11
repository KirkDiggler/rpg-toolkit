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
	// value — Width x Height cells, Chebyshev distance), GridShapeHex
	// (Width x Height offset column/row hex cells — see tools/spatial's
	// HexGrid), or GridShapeGridless (continuous positions within Width x
	// Height). The zero value keeps every pre-existing room square, so v0.1
	// persisted blobs without this field unmarshal to square unchanged.
	Grid spatial.GridShape

	// Occluders are positions that block line of sight.
	Occluders []spatial.Position

	// Boundaries define walls or barriers between adjacent cells.
	Boundaries []spatial.Boundary
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

// PumpInput contains no parameters; the pump is parameterless in wave 1.
type PumpInput struct{}

// PumpOutput reports the results of a world tick.
type PumpOutput struct {
	// Tick is the exploration clock's reading after the advance.
	Tick uint64

	// MonsterMoves contains the successful moves executed by monsters during this pump.
	MonsterMoves []struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}

	// IntelDeltas maps member IDs to their updated percepts after all monster actions
	// (SurveilOutput deltas from the single refreshSight cycle).
	IntelDeltas map[MemberID]*intel.SurveilOutput

	// Seqs contains the sequence numbers of the recorded beats (tick beat first, then move beats).
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
