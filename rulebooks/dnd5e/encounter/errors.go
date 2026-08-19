// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "errors"

var (
	// ErrNilInput is returned when a nil *XxxInput is passed to an operation.
	// Indicates a caller defect.
	ErrNilInput = errors.New("nil input")

	// ErrNoMember is returned when an input contains an empty or
	// duplicate member ID (Setup, Join, and Load — Load's identical
	// checks used to carry only ErrInvalidData, #929 hardening round F),
	// a member is declared a player while carrying a Decider — design
	// law C2 — at any of the three seams that accept one (NewEncounter,
	// LoadEncounter, Join), Join names a member ID already in the
	// encounter, Exit is called with an empty member ID, Story's
	// audience never joined, or — Load-only, since these are persisted-
	// state coherence checks with no Setup analogue — a persisted
	// abandoned outcome with non-empty membership (abandonment means the
	// membership emptied) or a current member missing from EverMembers
	// (#929 hardening round F).
	ErrNoMember = errors.New("empty member id")

	// ErrNotMember is returned when an entity is not a member of this encounter.
	ErrNotMember = errors.New("not a member")

	// ErrNoRegion is returned when [Encounter.MembersIn] is asked about a
	// region the field does not have. An EMPTY region is an ordinary answer —
	// "nobody has reached the tomb yet" is a fact worth reporting — so a
	// mistyped region name must not be able to say it (rpg-toolkit#1108).
	ErrNoRegion = errors.New("no such region")

	// ErrNoEnding is returned when Setup or Load is called with zero
	// endings, a declared ending's key is empty or the reserved
	// "abandoned", a declared ending's key duplicates an earlier one in
	// the same input (#929 hardening round E — an unvalidated duplicate
	// lets declaration order silently shadow a later ending, e.g. a
	// reached_position twin permanently hiding an external ending), End
	// is called with an undeclared key or one whose Trigger is not
	// TriggerExternal, or a TriggerReachedPosition ending names an
	// unknown room or an unreachable position (#929 T3 Opus round F5,
	// checked identically at Setup and Load — see
	// validateEndingTriggers), or — Load-only, since these are wire-
	// format/persisted-state concerns with no Setup analogue —
	// EndingData names an unrecognized Kind string, a reached_position
	// EndingData is missing its Room or Position, or a persisted Outcome
	// names an ending key that was never declared (these three used to
	// carry only ErrInvalidData, #929 hardening round F). An encounter
	// that cannot end — whether it declares zero endings or one that can
	// never fire — is a liveness hole.
	ErrNoEnding = errors.New("no such ending")

	// ErrClosed is returned when a mutating verb (action, event, exit, etc.)
	// is called on a closed encounter. A closed encounter has an Outcome.
	ErrClosed = errors.New("encounter closed")

	// ErrNoField is returned when Setup is called without rooms, or when a
	// declared room is itself defective — empty or duplicate ID, an
	// unrecognized-or-no-longer-supported grid shape value (gridless
	// included — RoomData's doc comment in data.go), a non-integral
	// occluder position in ANY family, not just hex (#929 T3 Opus round
	// F2), a duplicate occluder position within one room (#929 hardening
	// round D — previously escaped module validation and rejected only
	// in spatial's own voice, an accident of the pre-index-based
	// occluder entity ID), non-positive OR oversized Width/Height
	// (maxRoomSpan), a
	// per-room or field-total cell count exceeding maxRoomCells/
	// maxFieldCells (allocation safety for Atlas — #929 T3 Opus round F1),
	// an out-of-bounds Origin (maxAnchorCoord) or a non-representable one
	// (non-integral, ±Inf, NaN — EVERY grid family, not just hex), or —
	// Load-only, since RoomInput.Origin is a plain value and can't be
	// absent the way RoomData.Origin's pointer can — a MISSING Origin (W5
	// presence: a nil pointer means the field was absent from the blob,
	// distinct from a declared zero — RoomData's doc comment in data.go)
	// — or the room list as a whole is incoherent (W1: more than one grid
	// family in one field; W2: two rooms' absolute footprints overlap) —
	// a malformed room list is as unusable as an empty one.
	//
	// Checked identically at Setup and Load (#929 T2): LoadEncounter routes
	// room-list validation through the SAME buildValidRoomGrids Setup uses
	// (LoadEncounter's doc comment in data.go), so W1, room-dimension
	// legality, origin legality, and W2 reject a persisted blob exactly as
	// they would a live SetupInput — Setup and Load can no longer drift on
	// these checks by construction, not just by convention. The shared
	// validator's own error messages carry no verb prefix — NewEncounter
	// and LoadEncounter each wrap their own ("newencounter:" / "load
	// encounter:") at their own call sites (#929 T2 second review round;
	// buildValidRoomGrids' doc comment).
	//
	// Also returned when the field's absolute footprint cannot be drawn on a
	// single grid of its family (W6, rpg-toolkit#1106): the canvas the rooms
	// compile into is one grid, and a square field reaching a negative cell
	// has no such grid — see canvasSpan, whose message names the remedy.
	ErrNoField = errors.New("no field")

	// ErrBadPlacement is returned when a placement or position is bad in
	// a way runtime spatial state can catch (not declaration-time
	// validation — that's ErrNoField/ErrBadConnection): a room or entity
	// lookup miss, a position out of bounds or (hex) non-integral, a
	// member not standing at a connection's threshold, or an actual
	// underlying spatial-package call (PlaceEntity, MoveEntity,
	// RegisterBoundary, RemoveEntity) failing. Only the LAST class wraps an
	// underlying spatial error — most call sites (moveMember's own
	// integrality check, Pump's snapshot lookups, Join's and Exit's own
	// lookups) reject before ever reaching spatial, with nothing beneath
	// ErrBadPlacement to wrap.
	//
	// It is also what a WALL answers with. Since rpg-toolkit#1106 the thing
	// that stops a step between two chambers is a movement-blocking boundary
	// on the canvas, refused by spatial's own MoveEntity — where it used to be
	// the absence of a doorway in a connection list, refused with a sentinel
	// of its own (ErrNoCrossing, deleted with the concept). "There is no way
	// through" and "that is not a cell you can stand on" stopped being
	// different kinds of answer when walls became geometry.
	//
	// VOID IS NOT FLOOR carries the same error: the canvas spans the field's
	// bounding box, so a cell can be on the map and still belong to no
	// authored chamber, and stepping or joining onto one is refused here.
	//
	// Also covers LoadEncounter's member and outcome-member checks, which
	// mirror NewEncounter's identical member checks and used to carry only
	// ErrInvalidData (#929 hardening round F) — plus a MISSING cell on either,
	// which has no Setup analogue for the same structural reason
	// RoomData.Origin's absence does not: a cell is a plain value in memory
	// and a pointer on the wire, so only Load can tell a blob that omitted it
	// (the room-local dialects from before #1068 and #1106) from one that
	// declared (0,0).
	ErrBadPlacement = errors.New("bad placement")

	// ErrBadConnection is returned when a connection's ID is empty or
	// duplicated, its From/To names an unknown room or itself, an
	// endpoint lies outside its room's bounds, is non-integral (hex
	// rooms only), or sits on an occluder position — or (W3) its two
	// endpoints, once anchored to their rooms' Origin, are not adjacent
	// absolute cells — or, Load-only, since ConnectionInput's endpoints
	// are plain values and can't be absent the way ConnectionData's
	// pointers can, a MISSING FromPosition or ToPosition (the connection
	// analogue of ErrNoField's missing-Origin case). Checked identically
	// at Setup and Load (#929 T2 — see ErrNoField's doc comment):
	// LoadEncounter routes connection validation through the SAME
	// validateConnectionInputs Setup uses.
	ErrBadConnection = errors.New("bad connection")

	// ErrInBubble is returned when a verb requires its member NOT be in a
	// running bubble and they are: Form names a member already in a fight
	// (rejected, never silently merged — merging is a Merge-verb decision
	// that arrives with multiple bubbles), Form is called while this
	// encounter's one allowed bubble is already running (#963 policy: one
	// bubble per encounter, so fights stay linear and the party stays
	// together), Transfer names ClockTurn for a member already in the
	// fight, or Step is asked to free-roam a member who is
	// mid-fight — a fight member acts through the bubble's own turn
	// structure, never by stepping around it.
	ErrInBubble = errors.New("already in a bubble")

	// ErrNoBubble is returned when a verb requires a running bubble and
	// finds none: Transfer names ClockTurn while no fight is running, or
	// Transfer-to-world, EndTurn, or Dissolve names a member who is on the
	// world clock — there is no bubble to leave, act in, or dissolve.
	ErrNoBubble = errors.New("no bubble")

	// ErrBadClock is returned when TransferInput.To names neither
	// ClockWorld nor ClockTurn. Direction is load-bearing on a transfer —
	// inferring it from the member's current clock would make the verb a
	// toggle, and a toggle applied to stale state silently moves somebody
	// the WRONG way instead of failing.
	ErrBadClock = errors.New("unknown clock kind")

	// ErrInvalidData is returned when LoadEncounter rejects the input data.
	ErrInvalidData = errors.New("invalid encounter data")

	// ErrTrimmed is returned by Story when the requested resume point has
	// already been trimmed out of the retention window — the caller asked to
	// continue from a sequence the log no longer holds.
	//
	// This is a REJECTION, never a short answer. A caller resuming from a
	// known sequence is asserting "I already have everything below this";
	// silently returning only what survives would look identical to a
	// complete answer and would leave a permanent hole in that caller's
	// view of the story. Rejecting tells it to resync from scratch.
	//
	// AfterSeq == 0 never produces this error: zero means "I have nothing,
	// send what you have," which is always answerable. The distinction is
	// between a first load and a reconnect (#937).
	ErrTrimmed = errors.New("story range trimmed")

	// ErrNoInitiative indicates Setup was given no InitiativeRoller. Trigger
	// detection runs from first light, so every encounter needs one — refused
	// at construction rather than at the moment a fight would have started.
	ErrNoInitiative = errors.New("encounter: no initiative roller")

	// ErrNoStanding indicates Setup or Load was given no Standing capability.
	// The consult runs from first light, so every encounter needs one —
	// refused at construction rather than at the moment a body would have
	// started a fight.
	ErrNoStanding = errors.New("encounter: no standing capability")

	// ErrNoSight indicates this module was not told, usably, how far somebody
	// can see. Three ways to earn it: Setup or Load was given no Sight
	// capability; the capability answered without covering a member it was
	// asked about; or it answered with a negative distance. All three are the
	// same defect seen from different sides — a sight range this module would
	// have to invent, which rpg-toolkit#1033 forbids it to do.
	ErrNoSight = errors.New("encounter: no sight capability")
)
