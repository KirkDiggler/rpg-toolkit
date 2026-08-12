// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "errors"

var (
	// ErrNilInput is returned when a nil *XxxInput is passed to an operation.
	// Indicates a caller defect.
	ErrNilInput = errors.New("nil input")

	// ErrNoMember is returned when an input contains an empty member ID.
	ErrNoMember = errors.New("empty member id")

	// ErrNotMember is returned when an entity is not a member of this encounter.
	ErrNotMember = errors.New("not a member")

	// ErrNoEnding is returned when Setup is called with zero endings, or End
	// is called with an undeclared key. An encounter that cannot end is a
	// liveness hole.
	ErrNoEnding = errors.New("no such ending")

	// ErrClosed is returned when a mutating verb (action, event, exit, etc.)
	// is called on a closed encounter. A closed encounter has an Outcome.
	ErrClosed = errors.New("encounter closed")

	// ErrNoField is returned when Setup is called without rooms, or when a
	// declared room is itself defective — empty or duplicate ID, an
	// unrecognized grid shape value (checked at both Setup and Load), a
	// no-longer-supported grid shape (gridless; Setup-only as of this
	// wave — a stored "gridless" grid string still LOADS today, per
	// RoomData's own doc comment in data.go; Load-side rejection of it is
	// T2's job), non-positive Width/Height, or a non-integral Origin (EVERY
	// grid family, not just hex) — or the room list as a whole is
	// incoherent (W1: more than one grid family in one field; W2: two
	// rooms' absolute footprints overlap) — a malformed room list is as
	// unusable as an empty one.
	//
	// W1, room-dimension legality, origin legality, and W2 are SETUP-ONLY
	// as of this wave: LoadEncounter (data.go) predates them and does not
	// yet enforce any of the four. Load-side enforcement of the W-laws
	// lands with persistence in T2 (RoomInput.Origin's doc comment) —
	// until then, a hand-authored or previously-persisted blob can carry a
	// mixed-family field, a non-positive dimension, or an overlapping pair
	// through Load unrejected.
	ErrNoField = errors.New("no field")

	// ErrBadPlacement is returned when a placement fails spatial validation.
	// Wraps the underlying spatial error.
	ErrBadPlacement = errors.New("bad placement")

	// ErrBadConnection is returned when a connection's ID is empty or
	// duplicated, its From/To names an unknown room or itself, or an
	// endpoint lies outside its room's bounds or on an occluder position —
	// all checked at both Setup and Load — or (W3) its two endpoints, once
	// anchored to their rooms' Origin, are not adjacent absolute cells.
	// W3 is SETUP-ONLY as of this wave (see ErrNoField's doc comment);
	// Load-side enforcement lands with persistence in T2.
	ErrBadConnection = errors.New("bad connection")

	// ErrNoConnection is returned by Traverse when the given connection ID
	// does not name any connection in this encounter — a runtime lookup
	// miss, the ErrNotMember analogue for connections. Distinct from
	// ErrBadConnection, which is a declaration-time defect at Setup/Load.
	ErrNoConnection = errors.New("no such connection")

	// ErrInvalidData is returned when LoadEncounter rejects the input data.
	ErrInvalidData = errors.New("invalid encounter data")
)
