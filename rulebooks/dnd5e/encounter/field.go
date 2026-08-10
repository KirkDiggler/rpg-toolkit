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

	// Occluders are positions that block line of sight.
	Occluders []spatial.Position

	// Boundaries define walls or barriers between adjacent cells.
	Boundaries []spatial.Boundary
}

// ConnectionInput describes a connection between two rooms.
type ConnectionInput struct {
	// ID is the unique connection identifier.
	ID string

	// From is the source room ID.
	From string

	// To is the destination room ID.
	To string
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
