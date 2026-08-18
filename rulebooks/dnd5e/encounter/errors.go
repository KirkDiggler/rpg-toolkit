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
	// Also returned by Absolute (#929 T3) when the named room does not
	// exist in this field — a runtime lookup miss reusing the room-list
	// defect vocabulary rather than a dedicated sentinel, unlike
	// ErrNoConnection's split from ErrBadConnection below.
	ErrNoField = errors.New("no field")

	// ErrBadPlacement is returned when a placement or position is bad in
	// a way runtime spatial state can catch (not declaration-time
	// validation — that's ErrNoField/ErrBadConnection): a room or entity
	// lookup miss, a position out of bounds or (hex) non-integral, a
	// member not standing at a connection's threshold, or an actual
	// underlying spatial-package call (AddRoom, PlaceEntity,
	// TransitionEntity, RemoveEntity) failing. Only the LAST class wraps
	// an underlying spatial error — most call sites (Absolute, Locate,
	// moveMember's own bounds/integrality checks, Traverse's threshold
	// check, Pump's snapshot lookups, Join's/Exit's own lookups) reject
	// before ever reaching spatial, with nothing beneath ErrBadPlacement
	// to wrap. Also covers LoadEncounter's member and outcome-member
	// checks — room lookup miss, out-of-bounds position, non-integral
	// (hex) position — which mirror NewEncounter's identical member
	// checks and used to carry only ErrInvalidData (#929 hardening
	// round F) — plus, outcome-only, a MISSING cell, which has no Setup
	// analogue for the same structural reason RoomData.Origin's absence
	// does not: an outcome's position is a plain value in memory and a
	// pointer on the wire, so only Load can tell a blob that omitted it
	// (the pre-#1068 room-local dialect) from one that declared (0,0).
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

	// ErrNoCrossing is returned by Step when the destination cell lies in
	// ANOTHER room and no doorway joins it to the cell the member is standing
	// on. The cell is real and the member simply cannot get there from here.
	//
	// Distinct from ErrBadPlacement on purpose, because the remedies differ:
	// "that is not a cell" sends a caller back to its arithmetic, while "there
	// is no way through" sends it back to the map. W2 lets two rooms share an
	// edge without a door between them, so two absolutely-adjacent cells can be
	// permanently unwalkable — a refusal a caller reading only the Atlas's
	// CELLS cannot predict, since the doorway is in the doorway list or it is
	// nowhere.
	ErrNoCrossing = errors.New("no doorway joins those cells")

	// ErrNoConnection is returned by Traverse when the given connection ID
	// does not name any connection in this encounter — a runtime lookup
	// miss, the ErrNotMember analogue for connections. Distinct from
	// ErrBadConnection, which is a declaration-time defect at Setup/Load.
	ErrNoConnection = errors.New("no such connection")

	// ErrInBubble is returned when a verb requires its member NOT be in a
	// running bubble and they are: Form names a member already in a fight
	// (rejected, never silently merged — merging is a Merge-verb decision
	// that arrives with multiple bubbles), Form is called while this
	// encounter's one allowed bubble is already running (#963 policy: one
	// bubble per encounter, so fights stay linear and the party stays
	// together), Transfer names ClockTurn for a member already in the
	// fight, or Move/Traverse is asked to free-roam a member who is
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
)
