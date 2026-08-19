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

	// Occluders are room-local positions that block line of sight, compiled
	// onto the canvas at construction: each one is registered at
	// Occluders[i] + Origin, in the dungeon's own absolute frame.
	Occluders []spatial.Position

	// Boundaries are the walls this room draws, as edges between adjacent
	// cells — room-local at authoring, compiled through Origin onto the
	// canvas at construction (rpg-toolkit#1106).
	//
	// AN ENDPOINT MAY LIE OUTSIDE THIS ROOM, and that is the whole point.
	// Until the field became one canvas, a boundary was registered on the
	// room's OWN spatial room, whose grid refuses any endpoint that is not a
	// cell of that room (tools/spatial's validateAndNormalizeBoundaryUnsafe:
	// "boundary endpoints ... must be valid positions in this room"). So the
	// seam between two authored chambers — the one place a dungeon most needs
	// a wall — was the one place a wall could not be drawn, and the
	// room-membership test in sight was standing in for every one of them.
	//
	// A room therefore walls its own edge by naming the cell beyond it: for a
	// 6-wide room, {From: (5,y), To: (6,y)} is the wall along its east side,
	// and (6,y) belongs to whatever sits next door. Compiled absolute, both
	// endpoints must be adjacent cells the canvas holds; the canvas is the
	// union of the field's footprints, so a wall on the field's outer rim has
	// no cell to point at and is refused (ErrBadPlacement) rather than
	// silently dropped.
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
	// overlap in absolute space), W3 (every connection's endpoints are
	// adjacent absolute cells), W5 (anchors are construction data), and W6
	// (the field's absolute footprint fits in one grid of its family — see
	// canvasSpan in encounter.go, rpg-toolkit#1106).
	//
	// ORIGIN IS A COMPILER INPUT, NOT A RUNTIME BRIDGE (rpg-toolkit#1106).
	// It used to be read back on every query: W4 called projection "a read",
	// and Atlas, Absolute and Locate each ran the arithmetic on demand. Now
	// it is spent ONCE, at construction, when the authored rooms are compiled
	// into the single canvas the encounter actually runs on — every cell,
	// occluder and wall lands absolute and stays there, and no verb projects
	// anything. Origin still round-trips through persistence
	// (RoomData.Origin, #929 T2) because it is construction truth, and Atlas
	// still reports each authored room's absolute footprint from it.
	Origin spatial.Position
}

// ConnectionInput describes a connection between two rooms: a bidirectional
// open doorway. FromPosition and ToPosition are the endpoint cells — the
// position a member must stand on in each room to be at the doorway.
// Crossing is not a mechanism of its own (rpg-toolkit#1106): a doorway's two
// endpoints are adjacent absolute cells, so going through one is an ordinary
// step. This declares where the opening sits, and [StepOutput.Crossing] names
// it when a step goes straight across.
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

// FieldInput describes the layout of rooms and connections, and what the
// canvas they compile onto does to a sightline.
type FieldInput struct {
	// Canvas is what this field DECLARES about the map its rooms compile
	// onto — today, what the space between them does to a sightline.
	// REQUIRED: see
	// [Void] for why this module is not allowed to pick (rpg-toolkit#1116).
	Canvas CanvasInput

	// Rooms is the list of rooms in this field.
	Rooms []RoomInput

	// Connections is the list of connections between rooms.
	Connections []ConnectionInput

	// Doors are the doors standing in this field's walls — each a set of
	// edges sharing one state (rpg-toolkit#1123). Optional: a field with no
	// doors is an ordinary field, and every opening in it is simply a gap
	// nobody can shut.
	Doors []DoorInput
}

// MemberInput describes a member being placed into the encounter AT
// CONSTRUCTION — Setup's roster, and Load's restored one.
//
// Room-shaped on purpose, and it stays that way (rpg-toolkit#1106, Kirk's
// ruling: "we author rooms; that they become one thing is after render").
// Authoring says "the skeleton stands in the hall, five cells in", which is
// how the reference tomb is written and how a designer thinks. The canvas is
// what those rooms compile INTO, so this position is projected through the
// room's Origin exactly once, at construction.
//
// ENTRY MID-SCENE IS A DIFFERENT SHAPE. A member walking in during play is not
// being authored; they arrive at a cell on the map, and [JoinInput] takes that
// cell directly (rpg-toolkit#1101). Construction data and play data have
// different shapes for a reason, and this is the construction one.
type MemberInput struct {
	// ID is the member's unique identifier.
	ID MemberID

	// Kind is the member's category (player or monster).
	Kind MemberKind

	// Room is the ID of the room where the member is placed.
	Room string

	// Position is the member's location within the room — ROOM-LOCAL, and
	// compiled to Position + that room's Origin on the canvas.
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
//
// Room-shaped because it is authored alongside the rooms (see [MemberInput]);
// the pair is compiled to a single absolute cell at construction, and that is
// the cell a member's arrival is compared against.
type TriggerReachedPosition struct {
	// Room is the target room ID.
	Room string

	// Position is the target position within the room — ROOM-LOCAL, compiled
	// to Position + that room's Origin at construction.
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

	// Sight reports how far each member can see, in cells (rpg-toolkit#1111).
	// REQUIRED, for the same reason Standing is: the consult runs at every
	// sight refresh including first light, so an encounter that cannot ask
	// cannot build a percept. Refused at construction (ErrNoSight). There is no
	// default — a number meaning "everyone sees this far" would be this module
	// inventing a rule 5e does not have, since sight is per-creature and
	// per-light-source.
	Sight Sight

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

	// Region is the named cell set that holds Position — DERIVED, not stored.
	// A member's cell is the canvas's to know and the authored footprints
	// decide which region that cell falls in, so keeping a region label on the
	// record would be a second truth every verb had to move in step with it
	// (rpg-toolkit#1106; the dual state [memberRecord] already warns about, one
	// field over).
	//
	// A member standing in a doorway is in the region whose cell is under their
	// feet, and the member facing them one cell away is in the other — see
	// [Encounter.RegionAt] for why this composition has no unnamed doorway cell
	// to report instead (rpg-toolkit#1108).
	Region RegionID

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
	// There is no room-local counterpart any more, and no bridge to one: the
	// field IS this frame (rpg-toolkit#1106). Room above names the authored
	// chamber whose footprint holds this cell.
	Position spatial.Position
}

// memberRecord is what the composition stores about a member: identity and
// kind, and nothing spatial at all.
//
// Deliberately NOT their cell — the canvas holds that, and duplicating it here
// would create a second truth that the verbs would have to keep in step. Not
// their room either, as of rpg-toolkit#1106: with one canvas, which authored
// chamber a member stands in is a question the field answers from their cell,
// and the copy this record used to carry had to be mutated by hand on every
// crossing.
type memberRecord struct {
	ID   MemberID
	Kind MemberKind
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

	// Region is the region they finished in — DERIVED from Position at the
	// moment the encounter closed, exactly as [Member.Region] is, and for the
	// same reason (rpg-toolkit#1108).
	Region RegionID

	// Position is the DUNGEON-ABSOLUTE cell they finished on
	// (rpg-toolkit#1068) — the same frame Member.Position and every beat
	// speak, and since rpg-toolkit#1106 the only frame there is.
	//
	// This was the last room-local report on the surface, and the worst place
	// for one to survive: an outcome is read AFTER the encounter is over, when
	// a host has no roster call and no further beats left to cross-check it
	// against. A party that finished in a room anchored anywhere but the
	// origin was reported at cells belonging to whatever room happens to sit
	// there.
	Position spatial.Position
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

// CrossedDoor is one door a step went through, and the state it was in.
type CrossedDoor struct {
	// ID is the door's identifier.
	ID DoorID

	// State is the state it was in when the step passed through it.
	State DoorStateKind
}

// StepOutput reports what the step actually did.
type StepOutput struct {
	// Stepped is the movement, in dungeon-absolute cells at both ends —
	// read straight off the canvas, with no anchor involved at either end.
	Stepped struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}

	// Doors are the doors this step went through, IN TRAVEL ORDER, or empty
	// when it went through none (rpg-toolkit#1123).
	//
	// A LIST, because a step is not necessarily one cell. [Encounter.Step]
	// deliberately does not check adjacency — that is a rule about walking and
	// it lives with the walk — so a move can legitimately cross several
	// crossings, and a singular field would have picked one of them by
	// accident. The seam that walks a path visits each cell in turn, so in
	// practice this is empty or one long. (Found by Copilot on PR #1125.)
	//
	// EACH CARRIES ITS STATE rather than leaving it implied. Every door here is
	// one the step got through, so today the state reads "open" and could read
	// nothing else — but "the step succeeded, therefore the door was open" is
	// an inference that stops being true the moment a third state arrives
	// (ajar, broken, one-way), and a contract a caller has to re-derive from an
	// implication is one nobody can rely on. #1123 asks for the door's identity
	// AND state; this is both.
	//
	// Separate from Crossing rather than folded into it because they are
	// different facts. A crossing is an authored OPENING somebody narrates; a
	// door is a thing with state that may or may not stand in one. A step can
	// go through both, one, or neither.
	Doors []CrossedDoor

	// Crossing names the doorway this step went through, or is empty when it
	// did not go through one.
	//
	// A NAME, NOT A MECHANISM (rpg-toolkit#1106). It used to be the answer to
	// "which of the two movement mechanisms carried this", and before that the
	// thing that granted permission to leave a room at all. Neither survives:
	// a step is a step, and what stops one is a wall. What is left is
	// narration — a host writing "she slips through the gate" should not have
	// to rediscover which gate from the Atlas — and it decides nothing.
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

	// MonsterMoves contains the steps monsters actually took this pump — all
	// of them, whether or not one went through a doorway. There is no second
	// list: a crossing is an ordinary step (rpg-toolkit#1106), and the
	// separate MonsterTraverses that used to sit beside this one described a
	// mechanism the composition no longer has.
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
	// the seam reshape exists to remove (rpg-toolkit#1062).
	MonsterMoves []struct {
		Member MemberID
		From   spatial.Position
		To     spatial.Position
	}

	// IntelDeltas maps member IDs to their updated percepts after all monster actions
	// (SurveilOutput deltas from the single refreshSight cycle).
	IntelDeltas map[MemberID]*intel.SurveilOutput

	// Seqs contains the sequence numbers of the recorded beats (tick beat
	// first, then movement beats in decision order).
	Seqs []uint64

	// Outcome is the encounter outcome if an ending fired; nil otherwise.
	Outcome *Outcome

	// Formed is set when a monster's own movement started a fight — first
	// contact with nobody walking, the case a walk-only trigger seam misses.
	Formed *FormedBubble
}

// JoinInput names who is arriving and the cell on the map they arrive at.
//
// A CELL, NOT A ROOM AND A CELL (rpg-toolkit#1101). This input used to be a
// [MemberInput] — Setup's own type — so a seam holding an absolute cell had to
// resolve it to a room and a local coordinate before it could hand somebody
// through the door, which was the last room translation left in the session's
// reasoning after rpg-toolkit#1059 made the walk absolute.
//
// The two are separate types now rather than one shared one, because they are
// two different things: Setup AUTHORS a roster into authored rooms, and Join
// takes somebody who is already on the map. A second entry VERB was the other
// way to close that gap, and was refused for the reason rpg-toolkit#1059 spent
// two PRs on: two ways in is two places for a rule to land, and eventually one
// of them misses.
type JoinInput struct {
	// Member is the joining member's unique identifier.
	Member MemberID

	// Kind is the member's category (player or monster).
	Kind MemberKind

	// Cell is where they arrive, DUNGEON-ABSOLUTE — the same frame the Atlas
	// draws, [Encounter.Step] takes, and a [Member]'s Position reports.
	//
	// A cell no authored room owns is refused with ErrBadPlacement: void is
	// not floor, and arriving in it is not a placement this composition can
	// make sense of.
	Cell spatial.Position

	// Decider is the monster's decision-making engine (monsters only).
	// Players must not have a Decider; passing one for a player will fail
	// validation. Deciders are NOT persisted; they are re-registered at load.
	Decider Decider
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
