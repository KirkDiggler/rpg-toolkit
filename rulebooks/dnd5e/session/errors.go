// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "errors"

var (
	// ErrNilInput is returned when a verb is called with a nil input struct.
	ErrNilInput = errors.New("nil input")

	// ErrNilConfig is returned by NewManager when the config itself is nil.
	// Distinct from ErrIncompleteConfig: a nil config is a caller mistake at the
	// call site, while an incomplete one is an unfinished wiring decision.
	ErrNilConfig = errors.New("nil config")

	// ErrIncompleteConfig is returned by NewManager when a required repository
	// is absent. The wrapped message names which one.
	//
	// Named for the condition rather than the category of the missing thing: a
	// name like "missing repository" would be accurate only for as long as
	// every required dependency happens to be one, and renaming an exported
	// error later is a breaking change.
	//
	// Construction is total (S8): the manager refuses to exist rather than
	// discovering a nil dependency three verbs later, at which point the
	// failure surfaces as a panic in the middle of a player's turn instead of
	// at process start where a deployment can catch it.
	ErrIncompleteConfig = errors.New("incomplete config")

	// ErrNotFound is what a repository returns when the requested ID does not
	// exist. Implementations must return an error satisfying errors.Is against
	// this sentinel — the manager distinguishes "no such session" (a clean
	// rejection the host can turn into a 404) from "the store is broken" (a
	// retryable failure), and it cannot make that distinction from an opaque
	// error string.
	ErrNotFound = errors.New("not found")

	// ErrBadRepository is returned when a repository violates its contract —
	// most concretely, reporting success while returning no data.
	//
	// Named separately rather than folded into a "not found" because the two
	// send whoever debugs it to different places: one is a caller asking for
	// something absent, the other is an implementation bug in the host's own
	// storage layer. Guessing which one it is, in either direction, is worse
	// than saying plainly that the contract was broken.
	ErrBadRepository = errors.New("repository violated its contract")

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

	// ErrNoCharacter is returned when a player joins naming a character the
	// character repository does not hold.
	//
	// Distinct from ErrNoMember, which is about the encounter's roster: this
	// one fires before anyone is placed, and it is the whole point of loading
	// at join time. A session that accepted a player with no character would
	// look fine until the first verb that needed a sheet, and would fail then
	// in a place with no obvious connection to the join that caused it.
	ErrNoCharacter = errors.New("no such character")

	// ErrBadCharacter is returned when a character's stored data exists but
	// cannot be reconstituted into a usable character.
	//
	// Separate from ErrNoCharacter for the same reason ErrBadRepository is
	// separate from ErrNotFound: absent and corrupt send whoever debugs it to
	// different places. This is also the boundary at which hostile or stale
	// stored bytes are refused rather than carried into a resolution.
	ErrBadCharacter = errors.New("character data could not be loaded")

	// ErrNoRef is returned when Spawn is given an empty ref.
	//
	// Spawn instantiates content that lives in code, and the ref is how that
	// content is named. There is no default worth guessing at.
	ErrNoRef = errors.New("empty ref")

	// ErrBadRef is returned when a ref is not a well-formed module:type:id.
	ErrBadRef = errors.New("malformed ref")

	// ErrNoLoader is returned when a ref is well-formed but names a module or
	// type this build cannot load — "homebrew:monsters:mind-flayer" in a build
	// with no homebrew content registered.
	//
	// Distinct from ErrUnknownContent because the remedies are different: this
	// one says the caller needs a build that knows that content, while the
	// other says the content itself is missing from a catalog we do own. A
	// single error would send whoever debugs it to the wrong place half the
	// time.
	ErrNoLoader = errors.New("no loader for that ref")

	// ErrUnknownContent is returned when a ref routes to a catalog we own but
	// names nothing in it.
	//
	// This is a live case rather than a theoretical one: several canonical
	// monster refs have no constructor yet, so they parse, route correctly,
	// and still cannot be built. Saying so plainly beats reporting them as
	// malformed.
	ErrUnknownContent = errors.New("no such content")

	// ErrMemberExists is returned when a verb would place a member the
	// encounter already holds.
	ErrMemberExists = errors.New("member already present")

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

	// ErrClosed is returned when a verb would change an encounter that has
	// already ended. A closed encounter is a record, not a game.
	ErrClosed = errors.New("encounter closed")

	// ErrNoEnding is returned by End when the key names no declared external
	// ending. Endings are declared when a world is authored, so this is a
	// caller naming something that was never on the menu.
	ErrNoEnding = errors.New("no such ending")

	// ErrEmptyPath is returned by Move when no cells were given. A walk to
	// nowhere is a caller mistake rather than a no-op: silently succeeding
	// would hide a route computation that produced nothing.
	ErrEmptyPath = errors.New("empty path")

	// ErrBrokenPath is returned by Move when consecutive cells are not
	// adjacent, or the first cell is not adjacent to where the member stands.
	//
	// Rejected whole rather than walked up to the gap: a caller who
	// mis-computed a route wants none of it, not an arbitrary prefix that
	// leaves the party somewhere nobody chose.
	ErrBrokenPath = errors.New("path is not a walk")

	// ErrNoConnection is returned when a verb names a connection the encounter
	// does not have.
	ErrNoConnection = errors.New("no such connection")

	// ErrBadPosition is returned when a target cell is out of bounds, or is
	// not a legal cell of its grid family.
	ErrBadPosition = errors.New("bad position")

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

	// ErrFrozen is returned by every verb that would change the world while an
	// interrupt window is open. Read verbs are unaffected.
	//
	// Errors wrapping this are *FrozenError, which names the window and who
	// owes it. Match the sentinel with errors.Is to detect the condition; use
	// errors.As to recover the window and tell the caller what to do about it.
	ErrFrozen = errors.New("world frozen")

	// ErrNoWindow is returned when a verb names a window that is not open —
	// unknown, never opened, or already answered. One answer per window.
	ErrNoWindow = errors.New("no such open window")

	// ErrNotAudience is returned when a member answers a window they do not
	// owe. Answering for someone else is a rejection, not a courtesy.
	ErrNotAudience = errors.New("not this window's audience")

	// ErrNotOffered is returned when an answer names a choice the window did
	// not offer.
	ErrNotOffered = errors.New("choice not offered")

	// ErrNoWindowID is returned when a verb is given an empty or unparseable
	// window identifier.
	ErrNoWindowID = errors.New("bad window id")

	// ErrInvalidSession is returned when stored session state is not a state
	// this module could have written — a hand-edited or corrupted blob.
	//
	// It is deliberately distinct from ErrInvalidWorld. A caller who sees this
	// knows the encounter is fine and only the session record is suspect, which
	// is the difference between "the tomb is unreadable" and "an open window
	// refers to a resolution that cannot be real".
	ErrInvalidSession = errors.New("invalid session data")

	// ErrSaveFailed is returned when one or more aggregates could not be
	// persisted. The accompanying SaveReport names which landed and which did
	// not (S6) — a partial write is never reported as success, and never as an
	// unqualified failure either, because the difference determines whether a
	// retry is safe.
	ErrSaveFailed = errors.New("save failed")
)
