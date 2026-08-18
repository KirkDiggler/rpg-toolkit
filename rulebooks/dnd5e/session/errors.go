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

	// ErrNoCrossing is returned by Move when a step lands in another room and
	// no doorway joins it to the cell the walker is standing on.
	//
	// Distinct from ErrBrokenPath, which means the two cells are not adjacent
	// at all. These are different author mistakes: a broken path is arithmetic
	// a caller can check against the map it already has, while two rooms may
	// TOUCH without a door between them (W2 permits it), so a pair of cells can
	// be perfectly adjacent and still have nothing to walk through. That second
	// case is invisible to a client reading only the Atlas's cells — it is in
	// the doorway list or it is nowhere — which is exactly why it earns a
	// sentinel of its own rather than being folded into "not a walk".
	//
	// Distinct from ErrBadPosition too: "there is no cell there" and "there is
	// no way there from here" send whoever reads them to different places.
	ErrNoCrossing = errors.New("no doorway joins those cells")

	// ErrNoConnection is the translation of a composition-side refusal to cross
	// a doorway.
	//
	// No verb takes a connection id any more (rpg-toolkit#1048): a caller names
	// cells, and this package finds the doorway joining them in the Atlas. So
	// this can no longer be a caller's mistake — if it appears, this package
	// derived a crossing the composition then rejected, which is a defect here
	// rather than in the call. It stays as the translation of record so that an
	// inner sentinel never crosses the boundary (S2).
	ErrNoConnection = errors.New("no such connection")

	// ErrBadPosition is returned when a target cell is out of bounds, is not a
	// legal cell of its grid family, or is the cell the walker is already
	// standing on.
	//
	// That last case shares the sentinel rather than earning its own because
	// the remedy is the same one: the caller named a cell that cannot be
	// stepped to, and must fix the cell. It is emphatically NOT ErrBrokenPath —
	// a broken path has a gap in it, and whoever read "not a walk" about a
	// zero-distance step would go hunting for arithmetic that is perfectly
	// fine (rpg-toolkit#1060).
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
	//
	// It is also the TRANSLATION OF RECORD for a composition-side complaint
	// about the world itself — a blob that cannot be loaded, or a field that
	// does not hold the room somebody is standing in — for the reason
	// ErrNoConnection is the translation of record for a refused crossing: an
	// inner sentinel must never cross the boundary (S2). One sentinel covers
	// both because the host's remedy is the same either way. The stored world
	// is unusable, and the repair is upstream of anything a caller can retry.
	ErrInvalidWorld = errors.New("invalid encounter data")

	// ErrInBubble is returned when a member is asked to free-roam while they
	// are in a fight. Movement inside a fight goes through its turn structure,
	// which this SDK does not surface yet.
	//
	// It became reachable with rpg-toolkit#964: a fight now starts by itself,
	// so a walk can end with the walker in one, and the very next Move is
	// refused. Before that a member could only enter a fight by a caller
	// deciding to start one, which nothing in this package ever did — the
	// sentinel had no path to a host and so did not exist.
	ErrInBubble = errors.New("member is in a fight")

	// ErrNotInFight is returned when a verb needs the member to be in a fight
	// and they are not — ending a turn while free-roaming, most obviously.
	//
	// The mirror of ErrInBubble, and both exist because a member is always in
	// exactly one kind of time: the two errors are how a caller learns which
	// one they guessed wrong about.
	ErrNotInFight = errors.New("member is not in a fight")

	// ErrNoCause is returned when a verb that must say WHY is not told.
	//
	// Ending a fight is the one that has it: a fight ends either because
	// somebody chose to leave or because the world noticed something, and a
	// caller that will not say which is asking for an effect without an
	// account of it.
	ErrNoCause = errors.New("no cause given")

	// ErrNotACharacter is returned when a verb needs a character and was
	// given a member that is not one.
	//
	// Attack has it: v1 compiles character attackers only, because a
	// monster's action can declare a save gate and the rider that gate
	// resolves has no recorded vocabulary yet. Scope, not oversight — the
	// case arrives with the behavior work that calls for it.
	ErrNotACharacter = errors.New("member is not a character")

	// ErrNoSheet is returned when a member has no stored sheet to resolve
	// against — an authored monster standing in a world nobody spawned.
	//
	// Distinct from ErrNoCharacter, which is about a player's sheet being
	// missing from the host's repository: this one is about content the
	// session itself never recorded.
	ErrNoSheet = errors.New("member has no stored sheet")

	// ErrBadAttack is returned when an attack cannot be compiled from the
	// attacker's sheet — an empty hand, or a weapon the strike has no
	// semantics for.
	ErrBadAttack = errors.New("attack cannot be made")

	// ErrFrozen, ErrNoWindow, ErrNotAudience, ErrNotOffered and ErrNoWindowID
	// lived here. Every one of them described an open interrupt window, and
	// nothing in this module opens one (rpg-toolkit#964 slice 2) — a sentinel
	// no code path can return reads as a condition a caller should handle, and
	// is worse than an absence. See doc.go for what wave 5 re-creates.

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
