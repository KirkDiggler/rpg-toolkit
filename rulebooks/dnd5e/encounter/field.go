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

	// Initiative rolls the order a bubble forms in when trigger detection
	// starts a fight (rpg-toolkit#964). REQUIRED — trigger detection runs from
	// first light, so a fight can start before the caller does anything, and
	// an encounter that cannot order one is a misconfiguration. Setup refuses
	// without it (ErrNoInitiative).
	Initiative InitiativeRoller

	// Standing reports which members are down (rpg-toolkit#1075). REQUIRED,
	// for the same reason Initiative is: the consult runs from first light, so
	// an encounter that cannot ask who is standing would start fights with
	// bodies and walk them around the map. Setup refuses without it
	// (ErrNoStanding). There is no default — a nil meaning "everyone is
	// standing" would be this module deciding a rule it is not allowed to know.
	Standing Standing

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

// StoryInput is used to query a member's story entries from a given sequence
// number onward.
type StoryInput struct {
	// Audience is the member requesting their story.
	Audience MemberID

	// AfterSeq is the INCLUSIVE lower bound: the returned entries begin AT this
	// sequence, not after it. Zero means "from the beginning of what is
	// retained" and is always answerable.
	//
	// The name predates the behaviour and is now misleading — it is passed
	// straight through as record.SliceFor's FromSeq, which is inclusive. A
	// caller who read the old wording and passed its last-seen sequence to
	// resume would receive that entry a second time, and under retention could
	// also draw an unexpected ErrTrimmed at the boundary. Documented rather
	// than renamed because the rename is a breaking change to a shipped field;
	// it belongs in the next deliberate break (Copilot, PR #939).
	//
	// To resume after entry N, pass N+1.
	AfterSeq uint64
}

// Member represents a member's public read-side data: who they are, and
// where they stand on the dungeon map.
//
// A READ SHAPE, not the stored record. What this composition keeps about a
// member is [memberRecord]; a member's cell is the spatial room's to know,
// and this type is built by asking it. The two were one type until #1040,
// which put a position on the record that the record did not own — the dual
// state this module has paid for before.
type Member struct {
	ID   MemberID
	Kind MemberKind
	Room string

	// Position is where the member stands, in DUNGEON-ABSOLUTE space —
	// already projected through their room's origin, so it can be compared
	// with any other absolute coordinate this composition reports (an Atlas
	// cell, a doorway endpoint, another member) without the caller redoing
	// the arithmetic.
	//
	// Absolute rather than room-local because a position without its room is
	// not an answer in a multi-room field, and pairing every coordinate with
	// a room ID is the dialect the seam reshape exists to remove: the
	// composition has rooms, and it projects the absolute geometry so its
	// caller sees one map (rpg-project#227). W4 already put projection on
	// this side of the line — absolute coordinates belong in query outputs,
	// never in a rule's own logic.
	//
	// The room-local cell is still available: pass this position to [Locate],
	// which is the exact inverse.
	Position spatial.Position
}

// memberRecord is what the composition stores about a member: identity, kind,
// and which room owns them. Deliberately NOT their cell — the spatial room
// holds that, and duplicating it here would create a second truth that the
// verbs would have to keep in step.
type memberRecord struct {
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

// MemberOutcome is where a member stood when the encounter closed.
type MemberOutcome struct {
	// ID is the member's identifier.
	ID MemberID

	// Room is their room ID when the encounter closed.
	Room string

	// Position is the DUNGEON-ABSOLUTE cell they finished on
	// (rpg-toolkit#1068) — the same frame Member.Position and every beat
	// speak, projected through the same absoluteOf.
	//
	// This was the last room-local report on the surface, and the worst place
	// for one to survive: an outcome is read AFTER the encounter is over, when
	// a host has no roster call and no further beats left to cross-check it
	// against. A party that finished in a room anchored anywhere but the
	// origin was reported at cells belonging to whatever room happens to sit
	// there.
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

	// Formed is set when this step started a fight. Nil otherwise.
	Formed *FormedBubble
}

// FormedBubble reports a fight that trigger detection started.
type FormedBubble struct {
	// Order is the initiative order the bubble formed with, first to act
	// first.
	Order []MemberID

	// Surprised names who entered unaware, sorted. A subset of Order.
	Surprised []MemberID

	// Seq is the story sequence of the formation beat.
	Seq uint64
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

	// Formed is set when walking through the door started a fight — the case
	// review caught, because traversing into a room refreshes sight exactly
	// like moving within one does.
	Formed *FormedBubble

	// IntelDeltas maps member IDs to their updated percepts after traversal
	// (SurveilOutput deltas from the refreshSight cycle, across both rooms).
	IntelDeltas map[MemberID]*intel.SurveilOutput

	// Seq is the sequence number of the recorded traversal beat.
	Seq uint64

	// Outcome is the encounter outcome if an ending fired; nil otherwise.
	Outcome *Outcome
}

// StepInput names who steps and which cell they step to.
type StepInput struct {
	// Member is the ID of the member stepping.
	Member MemberID

	// To is the destination cell, DUNGEON-ABSOLUTE — the same frame the Atlas
	// draws, a Member's Position reports, and every movement beat speaks.
	//
	// A cell, not a room and a cell. Whether this one is inside the stepper's
	// current room or through a doorway is decided here rather than by the
	// caller, which is the whole point of the verb (rpg-toolkit#1059).
	To spatial.Position
}

// StepOutput reports what the step actually did.
type StepOutput struct {
	// Stepped is the movement, in dungeon-absolute cells at both ends.
	//
	// From and To are projected through their OWN rooms' anchors, which for a
	// crossing are two different ones — on the map that is simply two adjacent
	// cells (W3), and a caller never learns there was an anchor involved.
	Stepped struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}

	// Crossing names the doorway this step went through, or is empty when the
	// step stayed inside one room.
	//
	// A doorway identifier is not a room: it is a thing on the map, and the
	// Atlas carries the same ids. It is reported because "what happened" is
	// genuinely two answers here, and a caller narrating a crossing ("she
	// slips through the gate") should not have to re-derive which one it was
	// from the geometry.
	Crossing string

	// IntelDeltas maps member IDs to their updated percepts after the step
	// (SurveilOutput deltas from the refreshSight cycle).
	IntelDeltas map[MemberID]*intel.SurveilOutput

	// Seq is the sequence number of the recorded movement beat.
	Seq uint64

	// Outcome is the encounter outcome if an ending fired underfoot; nil
	// otherwise.
	Outcome *Outcome

	// Formed is set when this step started a fight. Nil otherwise.
	Formed *FormedBubble
}

// PumpInput contains no parameters; the pump is parameterless in wave 1.
type PumpInput struct{}

// PumpOutput reports the results of a world tick.
type PumpOutput struct {
	// Tick is the exploration clock's reading after the advance.
	Tick uint64

	// MonsterMoves contains the successful same-room moves executed by monsters
	// during this pump.
	//
	// From and To are DUNGEON-ABSOLUTE — already projected through the room's
	// origin, so they can be compared with any other absolute coordinate this
	// composition reports (a [Member]'s Position, an Atlas cell, the position
	// on this move's own "moved" beat) without the caller redoing the
	// arithmetic. They are also the frame the decider named the step in: what
	// the pump reports back is the cell that was asked for.
	//
	// No room field, deliberately: an absolute cell does not need one, and
	// carrying a composition-internal room ID beside a position is the dialect
	// the seam reshape exists to remove (rpg-toolkit#1062). To recover the
	// room-local cell, pass the position to [Encounter.Locate].
	MonsterMoves []struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}

	// MonsterTraverses contains the doorway crossings monsters made during this
	// pump — an intended step whose cell turned out to be on the far side of a
	// doorway. A step that could not be taken does not appear here: a cell in
	// another room with no doorway joining it to where the monster stands is
	// silently skipped, matching MonsterMoves' spatial-rejection contract.
	//
	// From and To are DUNGEON-ABSOLUTE, exactly as MonsterMoves' are — and here
	// the two sides are projected through DIFFERENT anchors, since a crossing's
	// departure cell belongs to FromRoom and its arrival cell to ToRoom. On the
	// map the pair is simply two adjacent cells (W3), which is the whole point:
	// a crossing reads like an ordinary step.
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

	// Formed is set when a monster's own movement started a fight — first
	// contact with nobody walking, the case a walk-only trigger seam misses.
	Formed *FormedBubble
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

	// Formed is set when arriving in sight of the other side started a fight.
	// A joiner walks into a scene like anybody else.
	Formed *FormedBubble

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
