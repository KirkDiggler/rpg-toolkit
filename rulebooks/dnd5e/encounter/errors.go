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
	// declared room is itself defective (empty or duplicate ID, unrecognized
	// or no-longer-supported grid shape, non-integral hex origin) or the
	// room list as a whole is incoherent (W1: more than one grid family in
	// one field; W2: two rooms' absolute footprints overlap) — a malformed
	// room list is as unusable as an empty one. Returned at both Setup and
	// Load.
	ErrNoField = errors.New("no field")

	// ErrBadPlacement is returned when a placement fails spatial validation.
	// Wraps the underlying spatial error.
	ErrBadPlacement = errors.New("bad placement")

	// ErrBadConnection is returned when a connection's ID is empty or
	// duplicated, its From/To names an unknown room or itself, an endpoint
	// lies outside its room's bounds or on an occluder position, or (W3)
	// its two endpoints, once anchored to their rooms' Origin, are not
	// adjacent absolute cells. Returned at both Setup and Load — a
	// declaration-time defect.
	ErrBadConnection = errors.New("bad connection")

	// ErrNoConnection is returned by Traverse when the given connection ID
	// does not name any connection in this encounter — a runtime lookup
	// miss, the ErrNotMember analogue for connections. Distinct from
	// ErrBadConnection, which is a declaration-time defect at Setup/Load.
	ErrNoConnection = errors.New("no such connection")

	// ErrInvalidData is returned when LoadEncounter rejects the input data.
	ErrInvalidData = errors.New("invalid encounter data")
)
