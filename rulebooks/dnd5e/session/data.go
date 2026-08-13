// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// SessionData is the persistent representation of a session.
//
// It holds only session state. The encounter is referenced by ID rather than
// embedded: clock, intel and record ride inside an encounter because they are
// parts of it with no independent lifetime, but an encounter is something a
// session points at. Wrapping it would weld the encounter's storage strategy
// to the session's and would rewrite every room and member on a save that only
// opened a window.
type SessionData struct {
	// ID identifies this session.
	ID string `json:"id"`

	// Encounter is the ID of the encounter this session plays in.
	Encounter string `json:"encounter"`
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
