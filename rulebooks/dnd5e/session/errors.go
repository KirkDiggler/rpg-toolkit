// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "errors"

var (
	// ErrNilInput is returned when a verb is called with a nil input struct.
	ErrNilInput = errors.New("nil input")

	// ErrNilConfig is returned by NewManager when the config itself is nil.
	// Distinct from ErrMissingPort: a nil config is a caller mistake at the
	// call site, while a missing port is an incomplete wiring decision.
	ErrNilConfig = errors.New("nil config")

	// ErrMissingPort is returned by NewManager when a required repository is
	// absent. The wrapped message names which one.
	//
	// Construction is total (S8): the manager refuses to exist rather than
	// discovering a nil port three verbs later, at which point the failure
	// would surface as a panic in the middle of a player's turn instead of at
	// process start where a deployment can catch it.
	ErrMissingPort = errors.New("missing required port")

	// ErrNotFound is what a repository returns when the requested ID does not
	// exist. Implementations must return an error satisfying errors.Is against
	// this sentinel — the manager distinguishes "no such session" (a clean
	// rejection the host can turn into a 404) from "the store is broken" (a
	// retryable failure), and it cannot make that distinction from an opaque
	// error string.
	ErrNotFound = errors.New("not found")

	// ErrNoSession is returned when a verb names a session that does not
	// exist. This is the manager's own rejection, translated from a
	// repository's ErrNotFound so hosts match on one vocabulary rather than
	// two.
	ErrNoSession = errors.New("no such session")

	// ErrNoEncounter is returned when a session references an encounter the
	// encounter repository does not hold. Distinct from ErrNoSession: the
	// session exists and is readable, but the world it points at is missing,
	// which is a data-integrity problem on the host's side rather than a bad
	// request.
	ErrNoEncounter = errors.New("no such encounter")

	// ErrNoMemberID is returned when a verb is given an empty member ID.
	ErrNoMemberID = errors.New("empty member id")

	// ErrNoMember is returned when a verb names a member the encounter does
	// not hold.
	ErrNoMember = errors.New("no such member")

	// ErrStoryTrimmed is returned by Story when the requested resume point has
	// aged out of the retention window. The caller must resync from zero
	// rather than resume: a short answer would be indistinguishable from a
	// complete one and would leave a permanent hole in its view of the story.
	//
	// This package's own sentinel rather than the composition's, and the
	// distinction matters more than it looks. The boundary test reads exported
	// signatures, and a sentinel is not a type in a signature — so if hosts
	// matched on the inner package's error value, replacing that package would
	// break their error handling exactly as surely as leaking a struct would,
	// through a channel no test is watching.
	ErrStoryTrimmed = errors.New("story range trimmed")

	// ErrNoSessionID is returned when a verb is given an empty session ID.
	ErrNoSessionID = errors.New("empty session id")

	// ErrNoEncounterID is returned when a verb is given an empty encounter ID.
	ErrNoEncounterID = errors.New("empty encounter id")

	// ErrSessionExists is returned by StartSession when the ID is already in
	// use.
	//
	// Starting over an existing session must never be silent: the ID names a
	// game in progress, and overwriting it would destroy a party's state
	// because someone reused a string. A host that genuinely wants to restart
	// deletes first, deliberately.
	ErrSessionExists = errors.New("session already exists")

	// ErrInvalidWorld is returned when the authored encounter handed to
	// StartSession cannot be loaded.
	//
	// Validated by loading it before anything is written, so a world that
	// cannot be reconstituted is rejected at the door rather than persisted
	// and discovered on the next verb — at which point the session would exist
	// and be permanently unusable.
	ErrInvalidWorld = errors.New("invalid encounter data")

	// ErrSaveFailed is returned when one or more aggregates could not be
	// persisted. The accompanying SaveReport names which landed and which did
	// not (S6) — a partial write is never reported as success, and never as an
	// unqualified failure either, because the difference determines whether a
	// retry is safe.
	ErrSaveFailed = errors.New("save failed")
)
