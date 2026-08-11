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
	// grid shape) — a malformed room list is as unusable as an empty one.
	// Returned at both Setup and Load.
	ErrNoField = errors.New("no field")

	// ErrBadPlacement is returned when a placement fails spatial validation.
	// Wraps the underlying spatial error.
	ErrBadPlacement = errors.New("bad placement")

	// ErrBadConnection is returned when a connection's ID is empty or
	// duplicated, its From/To names an unknown room or itself, or an
	// endpoint lies outside its room's bounds or on an occluder position.
	// Returned at both Setup and Load.
	ErrBadConnection = errors.New("bad connection")

	// ErrInvalidData is returned when LoadEncounter rejects the input data.
	ErrInvalidData = errors.New("invalid encounter data")
)
