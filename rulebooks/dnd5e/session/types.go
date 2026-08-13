// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// The types below are this package's own twins of what the composition
// returns, and the functions translating into them are the price of S2.
//
// The translation is boring, and that is the point: it is what lets the
// encounter's own View, Story, Status and Atlas types change shape — or be
// replaced outright by a different implementation — without a single host
// source file changing. Re-exporting them would have been fewer lines and
// would have made every one of those modules permanently load-bearing on the
// wire.
//
// Everything here is also proto-shaped by construction: flat structs, string
// enums, no interfaces, nothing whose meaning varies by another field's value.

// GridKind names a room's coordinate family.
//
// A string rather than a mirror of spatial's iota. The composition already
// persists grid this way for the same reason: an iota is an in-process
// enumeration order, not a wire contract, and reordering it upstream would
// silently reinterpret every stored and transmitted value.
type GridKind string

const (
	// GridSquare is a square grid, where distance is Chebyshev.
	GridSquare GridKind = "square"

	// GridHex is a hex grid addressed in axial coordinates, where distance is
	// measured in cube space.
	GridHex GridKind = "hex"
)

// Atlas is the static world map in dungeon-absolute space.
//
// Construction truth: unchanged by movement, joins, exits, or endings. Cache
// it per encounter rather than fetching it per frame.
type Atlas struct {
	// Rooms is every room's absolute footprint, sorted by room ID.
	Rooms []AtlasRoom `json:"rooms,omitempty"`

	// Doorways is every connection's absolute endpoint pair, sorted by
	// connection ID.
	Doorways []AtlasDoorway `json:"doorways,omitempty"`
}

// AtlasRoom is one room's absolute-space footprint.
type AtlasRoom struct {
	// ID is the room's identifier.
	ID string `json:"id"`

	// Grid is the room's coordinate family.
	Grid GridKind `json:"grid"`

	// Origin is the room's dungeon-absolute anchor.
	Origin spatial.Position `json:"origin"`

	// Width is the room's horizontal dimension.
	Width int `json:"width"`

	// Height is the room's vertical dimension.
	Height int `json:"height"`

	// Cells is every cell of the room in dungeon-absolute space, occluded
	// cells included: occlusion is walkability, not ownership.
	Cells []spatial.Position `json:"cells,omitempty"`

	// Occluders is the subset of Cells that blocks line of sight, reported
	// separately so a host can render them distinctly.
	Occluders []spatial.Position `json:"occluders,omitempty"`

	// Boundaries is the room's walls and barriers, both endpoints absolute.
	Boundaries []AtlasBoundary `json:"boundaries,omitempty"`
}

// AtlasBoundary is one wall or barrier crossing between adjacent cells.
type AtlasBoundary struct {
	// From is one endpoint of the crossing, in dungeon-absolute space.
	From spatial.Position `json:"from"`

	// To is the other endpoint, in dungeon-absolute space.
	To spatial.Position `json:"to"`

	// BlocksMovement reports whether an entity may cross.
	BlocksMovement bool `json:"blocks_movement,omitempty"`

	// BlocksLineOfSight reports whether sight may cross.
	BlocksLineOfSight bool `json:"blocks_line_of_sight,omitempty"`
}

// AtlasDoorway is one connection's absolute endpoint pair. The two cells are
// adjacent in absolute space, so crossing one is an ordinary step.
type AtlasDoorway struct {
	// Connection is the connection's identifier.
	Connection string `json:"connection"`

	// From is the source room ID.
	From string `json:"from"`

	// FromCell is the endpoint in From, in dungeon-absolute space.
	FromCell spatial.Position `json:"from_cell"`

	// To is the destination room ID.
	To string `json:"to"`

	// ToCell is the endpoint in To, in dungeon-absolute space.
	ToCell spatial.Position `json:"to_cell"`
}

// Status reports whether an encounter is still running.
type Status struct {
	// Open reports whether the encounter is active.
	Open bool `json:"open"`

	// Outcome is the ending that fired, present only once closed.
	Outcome *Outcome `json:"outcome,omitempty"`
}

// Outcome describes how an encounter ended.
type Outcome struct {
	// Ending is the key of the ending that fired.
	Ending string `json:"ending"`

	// At is the clock reading when it fired.
	At uint64 `json:"at,omitempty"`

	// Members is where everyone stood when it closed.
	Members []MemberOutcome `json:"members,omitempty"`
}

// MemberOutcome is one member's final placement.
type MemberOutcome struct {
	// ID is the member's identifier.
	ID string `json:"id"`

	// Room is the room they were in.
	Room string `json:"room"`

	// Position is where they stood within it.
	Position spatial.Position `json:"position"`
}

// Sighting is one thing an observer currently perceives.
type Sighting struct {
	// Subject names what is perceived.
	Subject string `json:"subject"`

	// Payload is what the observer knows about it, encoded by the composition.
	Payload []byte `json:"payload,omitempty"`

	// Channel is how it was perceived.
	Channel string `json:"channel,omitempty"`

	// At is the clock reading when this knowledge was last refreshed.
	At uint64 `json:"at,omitempty"`

	// CurrentVia lists the channels currently carrying it. Empty means the
	// observer holds a memory rather than a live sighting.
	CurrentVia []string `json:"current_via,omitempty"`

	// Status distinguishes a live sighting from a stale memory.
	Status string `json:"status,omitempty"`
}

// StoryEntry is one beat of what an observer has witnessed.
type StoryEntry struct {
	// Seq is the beat's sequence: monotonic, gapless, never renumbered. A
	// client that notices a gap has missed a beat and should re-query.
	Seq uint64 `json:"seq"`

	// At is the clock reading when the beat was recorded.
	At uint64 `json:"at,omitempty"`

	// Correlation groups cause and effect across beats. Empty is legal.
	Correlation string `json:"correlation,omitempty"`

	// Tags is queryable metadata describing the beat.
	Tags map[string]string `json:"tags,omitempty"`

	// Payload is the beat itself, encoded by the composition.
	Payload []byte `json:"payload,omitempty"`
}

// EventKind names what an event reports.
//
// A string enum rather than free-form prose, because this is the field clients
// branch on: it must be machine-readable and stable, and it maps directly onto
// a proto enum. Adding a kind is compatible; changing what an existing one
// means is not.
type EventKind string

const (
	// EventMoved reports that a member moved to a new position.
	EventMoved EventKind = "moved"

	// EventDiscovered reports that a member perceived something new.
	EventDiscovered EventKind = "discovered"

	// EventJoined reports that a member entered the encounter.
	EventJoined EventKind = "joined"

	// EventExited reports that a member left the encounter.
	EventExited EventKind = "exited"

	// EventEnded reports that the encounter closed.
	EventEnded EventKind = "ended"

	// EventPending reports that the world is frozen awaiting an answer.
	EventPending EventKind = "pending"
)

// Event is one thing that happened, addressed to one recipient.
//
// Deliberately flat and non-polymorphic: it maps onto a proto message, so no
// interface-valued fields, no type switches on the wire, and no payload shape
// that varies by kind in a way a generated client cannot express.
//
// An Event is per-recipient, not per-occurrence. The same underlying beat
// becomes several Events, one for each viewer who may know about it, and their
// payloads may differ — a player being offered a choice receives the choices,
// while everyone else receives only that the world is waiting. Projection is a
// rule, decided where perception lives; filtering a single shared payload would
// mean the difference between viewers was a delivery detail, and the first
// mistake would leak something unperceived.
type Event struct {
	// Session is the session this event belongs to.
	Session string `json:"session"`

	// Seq is the story sequence this event was derived from: monotonic,
	// gapless, and never renumbered. A recipient that notices a gap in Seq has
	// missed an event and can re-query the story from its last known value.
	Seq uint64 `json:"seq"`

	// At is the clock reading when the underlying beat was recorded.
	At uint64 `json:"at,omitempty"`

	// Correlation groups cause and effect across events. Empty is legal.
	Correlation string `json:"correlation,omitempty"`

	// Recipient is the member this projection is addressed to.
	Recipient string `json:"recipient"`

	// Kind names what happened.
	Kind EventKind `json:"kind"`

	// Payload is the kind-specific body, encoded by this package.
	Payload []byte `json:"payload,omitempty"`
}

// SaveReport names which aggregates were persisted by a verb and which were
// not.
//
// Every verb returns one, and a partial write returns an error carrying a
// populated report rather than a bare failure (S6). The distinction is
// load-bearing for the host: "nothing was written" is safe to retry, while
// "the encounter advanced but the character did not" is a repair, and an
// unqualified error cannot tell them apart.
type SaveReport struct {
	// Written names the aggregates that were persisted successfully.
	Written []string `json:"written,omitempty"`

	// Failed names the aggregates that could not be persisted.
	Failed []string `json:"failed,omitempty"`
}

// Partial reports whether this save landed some aggregates but not all — the
// state that needs repair rather than retry.
func (r SaveReport) Partial() bool {
	return len(r.Written) > 0 && len(r.Failed) > 0
}

// MemberKind categorises a member.
//
// A string enum rather than a mirror of the composition's, for the same reason
// GridKind is: it maps onto a proto enum, and adding a kind must be a
// compatible change rather than a renumbering.
type MemberKind string

const (
	// KindPlayer is a player-controlled member.
	KindPlayer MemberKind = "player"

	// KindMonster is a member driven by the game rather than a person.
	KindMonster MemberKind = "monster"
)

// Member is a participant's placement in the world.
type Member struct {
	// ID is the member's identifier.
	ID string `json:"id"`

	// Kind categorises the member.
	Kind MemberKind `json:"kind"`

	// Room is the room the member currently occupies.
	Room string `json:"room"`
}

// Discovery is what changed in one observer's perception.
//
// Three disjoint lists rather than a single "here is everything you now
// perceive": a client rendering a first sighting wants to announce it, a
// refresh wants to update silently, and a fade wants to grey something out.
// Collapsing them would force the client to diff against its own previous
// state to recover the distinction the composition already knows.
type Discovery struct {
	// FirstContact is what came into view that the observer did not hold at
	// all. These are the moments worth announcing.
	FirstContact []Report `json:"first_contact,omitempty"`

	// Refreshed names subjects the observer already held whose knowledge was
	// renewed by a live channel.
	Refreshed []string `json:"refreshed,omitempty"`

	// Faded names subjects that stopped being sustained. The observer keeps a
	// memory; it is no longer a live sighting.
	Faded []string `json:"faded,omitempty"`
}

// Report is one newly perceived subject and what is known about it.
type Report struct {
	// Subject names what was perceived.
	Subject string `json:"subject"`

	// Payload is what the observer learned, encoded by the composition.
	Payload []byte `json:"payload,omitempty"`
}
