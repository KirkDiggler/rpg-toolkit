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

	// ErrBadNPC is returned when stored npc.Data cannot be used as given:
	// PlaceNPC rejects a MovementPolicy that is empty or unrecognized, and
	// Interact rejects a stored Inventory whose bytes are present but
	// malformed.
	//
	// Separate from ErrNoRef for the same reason ErrBadCharacter is separate
	// from ErrNoCharacter: PlaceNPC takes already-built content directly
	// rather than resolving it from a ref, so nothing upstream of this seam
	// validates it the way npc.New does for a caller who went through that
	// constructor — a caller who builds npc.Data by hand can still reach
	// this seam with a malformed value, and it must be named as itself
	// rather than folded into ErrNoRef's "nil NPC" meaning.
	ErrBadNPC = errors.New("npc data could not be used")

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

	// ErrNoDeclarationID is returned when Attack or EndTurn omits the opaque
	// selector echoed from Afford, or when a turn-clock Move omits it. A
	// world-clock Move is the deliberate exception: Afford has no world-clock
	// declarations, so its selector must be empty.
	ErrNoDeclarationID = errors.New("empty declaration id")

	// ErrStaleDeclaration is returned when an echoed selector does not name the
	// verb's one current compiled offer, when that offer is now unavailable, or
	// when Attack's selected target is absent or unavailable in its current
	// candidate set. It is a current-world refusal: callers refresh Turn and
	// Afford and never retry the mutation automatically.
	ErrStaleDeclaration = errors.New("declaration is stale")

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

	// ErrLocked is returned when a door refuses to open because it is locked.
	//
	// A fiction beat, not a defect: the party found the way and the way is
	// shut, which is a thing to tell a player and a reason to go looking for a
	// key or roll against the DC. It earns a sentinel of its own for that
	// reason — a host that could only see "bad position" would have nothing to
	// say and would send somebody hunting a bug in coordinates that are fine.
	//
	// Raised for the door verb's refusal AND for a walk INTO a locked door —
	// the composition names both with its own sentinel now
	// (rpg-toolkit#1135), so a blocked step no longer hides behind
	// ErrBadPosition.
	ErrLocked = errors.New("door is locked")

	// ErrDoorShut is returned when a walk crosses a door that is merely
	// closed — shut but not locked. The other half of the rpg-toolkit#1135
	// split: the cell is real and the way is shut, a fiction beat whose
	// remedy is OpenDoor, not different coordinates.
	ErrDoorShut = errors.New("door is shut")

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

	// ErrOutOfRange is Interact's refusal when the target stands farther than
	// the configured range (default: adjacent, one cell) from the actor —
	// the host-seam twin of encounter.ErrOutOfRange. Distinct from
	// ErrOutOfReach, which is Attack's own reach validation against a
	// compiled delivery's max range; Interact has no delivery to check
	// against, only a plain cell distance.
	ErrOutOfRange = errors.New("target out of range")

	// ErrNotVisible is Interact's refusal when the target is not in the
	// actor's current sight — the host-seam twin of encounter.ErrNotVisible.
	ErrNotVisible = errors.New("target not visible")

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

	// ErrInBubble is returned when a verb requires its member NOT be in a
	// running bubble and they are.
	//
	// NOT MOVE'S OWN REFUSAL ANY MORE (rpg-toolkit#1169): the active member of
	// a fight may walk on the turn clock now, priced and gated by whose turn
	// it is (ErrNotYourTurn) rather than refused outright. What survives here
	// are the composition's own Form/Transfer-shaped refusals translated
	// unchanged — naming a member already in a fight, or attempting to enter
	// one that is already running (#963's one-bubble-per-encounter policy).
	//
	// It became reachable with rpg-toolkit#964: a fight now starts by itself,
	// so a walk could end with the walker in one and (then) the very next Move
	// was refused by it. Before that a member could only enter a fight by a
	// caller deciding to start one, which nothing in this package ever did —
	// the sentinel had no path to a host and so did not exist.
	ErrInBubble = errors.New("member is in a fight")

	// ErrNotInFight is returned when a verb needs the member to be in a fight
	// and they are not — ending a turn while free-roaming, most obviously.
	//
	// The mirror of ErrInBubble, and both exist because a member is always in
	// exactly one kind of time: the two errors are how a caller learns which
	// one they guessed wrong about.
	ErrNotInFight = errors.New("member is not in a fight")

	// ErrNotYourTurn is returned when a verb needs its member to be the
	// CURRENT active member of the fight they are in, and they are not.
	//
	// TWO VERBS SHARE THIS REFUSAL (rpg-toolkit#1169): EndTurn naming someone
	// whose turn it is not, and Move asking a bubble member who is not the one
	// the clock is waiting on to walk. Translated from the composition's own
	// encounter.ErrNotActive rather than left to cross the boundary unnamed
	// (S2) — see translate. Distinct from ErrNotInFight: this member IS in the
	// fight, just not up right now.
	ErrNotYourTurn = errors.New("not your turn")

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

	// ErrElsewhere is returned by Search when the named region is not the
	// one the searcher stands in — INCLUDING a region that does not exist,
	// deliberately indistinguishably: a distinct no-such-region answer
	// would let a guessed ID probe for hidden rooms (the composition's own
	// contract, carried across unweakened).
	ErrElsewhere = errors.New("not standing in that region")

	// ErrNoSheet is returned when a member has no stored sheet to resolve
	// against — an authored monster standing in a world nobody spawned.
	//
	// Distinct from ErrNoCharacter, which is about a player's sheet being
	// missing from the host's repository: this one is about content the
	// session itself never recorded.
	ErrNoSheet = errors.New("member has no stored sheet")

	// ErrDowned is returned when a verb asks a DOWNED member to ACT: one at zero
	// hit points, out of the fight. The opposite state is UP.
	//
	// DOWNED IS NOT PRONE, and the two must not be read as the same word.
	// Prone is a posture condition the rulebook tracks — knocked flat, still in
	// the fight, still acting, with its own effects on attack rolls — and this
	// package never gates on it. Downed is the hit point total reaching zero.
	// A bare "down" reads as either, which is why this seam says downed
	// (Kirk's ruling, rpg-toolkit#1084).
	//
	// A downed member is still a member — on the map, in the roster, recordable
	// against, readable by Where and View (ruled fork (a) on rpg-toolkit#959) —
	// so this is not "no such member" and must never be mistaken for it. It is
	// narrower than that and narrower than ErrClosed: the world is fine, the
	// member is there, and this particular member cannot do this particular
	// thing.
	//
	// It reaches Attack and Move, which are the two verbs where a downed member
	// could still act. Inside a fight the swing already stops because the TURN
	// ORDER stops (the composition splices them out of it), but free roam has
	// no turn order — so a downed character could walk, and could initiate,
	// which is rpg-toolkit#845's shape reproduced on the new stack. The
	// composition deliberately did not invent this refusal
	// (rpg-toolkit#1077); it is ruled here, where the sheets are.
	//
	// NOT returned by the reads, and not by recording a blow ABOUT a downed
	// member. Reading where a downed member fell, and writing down the killing
	// stroke, are both things that must stay legal, and gating them would be a
	// different ruling that nobody made.
	//
	// WHAT A PLAYER EXPERIENCES. Zero hit points has no exit in v1 — there are
	// no death saves yet (ruled fork (b) on rpg-toolkit#959) — so a character
	// who drops is refused these verbs for the rest of the session, with no way
	// back up. That is the honest state of the game rather than an oversight,
	// and it stops being permanent when saves arrive, additively, without this
	// sentinel changing meaning.
	ErrDowned = errors.New("member is downed")

	// ErrCannotActivate is returned when an ability could have run and said
	// no: the charges are gone, the barbarian is already raging, the fighter is
	// at full hit points.
	//
	// THE PLAYER-FACING ONE, and it is separate from ErrBadActivation for the
	// reason ErrCannotAfford is separate from ErrBadCost: an actor who cannot
	// do this right now wants a different verb, and wiring that is wrong wants
	// a developer. A single sentinel would send the first one looking at the
	// wrong sheet. The ability's own words ride along as text, so the message
	// still names what ran out.
	ErrCannotActivate = errors.New("ability cannot be activated")

	// ErrBadActivation is returned when an activation nobody could run reaches
	// this seam — no ability named, a target on an ability that takes none, a
	// member the interaction never received. Content or wiring being wrong.
	ErrBadActivation = errors.New("activation is invalid")

	// ErrBadAttack is returned when an attack cannot be compiled from the
	// attacker's own sheet or shared persisted definition — an empty hand,
	// malformed declared action, or a weapon the strike has no semantics for.
	ErrBadAttack = errors.New("attack cannot be made")

	// ErrOutOfReach is Attack's final defensive resolution validation when a
	// target is further away than the selected definition's delivery permits:
	// one cell for ordinary melee, two for Reach weapons (rpg-toolkit#1010),
	// and beyond long range for a supported ranged weapon.
	//
	// Normal execution refuses reach changes earlier as ErrStaleDeclaration:
	// Afford projects the shared preflight as candidate availability, and Attack
	// regenerates and selects that current candidate before resolution. When no
	// candidate is in reach, Afford still answers once — a single unavailable
	// Attack declaration with Why.Reason ShortfallNoTargetInReach — rather than
	// an empty list a client could mistake for "nothing to ask about yet."
	ErrOutOfReach = errors.New("no target in reach")

	// ErrCannotAfford is returned when the final payment door cannot pay for
	// what an actor declared. Move reaches it when the selected path is longer
	// than the turn's remaining movement (rpg-toolkit#1169). Attack normally
	// exposes exhaustion in Afford and rejects the now-unavailable selector as
	// ErrStaleDeclaration; this remains its defensive resolution translation.
	//
	// THIS IS A FACT ABOUT THE GAME, not about the code, and that is the whole
	// reason it is separate from ErrBadCost. A player who has run out of actions
	// has done nothing wrong and needs to hear a different sentence from the one
	// a developer needs to hear, so the two are never merged — E2 split them at
	// the door for the same reason and this seam keeps the split rather than
	// flattening it on the way out (rpg-toolkit#1097).
	//
	// The message NAMES THE CURRENCY that ran out — "action: 1 needed, 0 left" —
	// because a refusal a client cannot explain is a refusal that reads as a bug.
	// A host may show it or match the sentinel and say it in its own words.
	// Move's own refusal composes its own "ft"-suffixed text rather than
	// [combat.SpendProfile]'s unitless one — see [movementShortfall] — but the
	// YES/NO answer both come from is the same [combat.Pay] call either way.
	//
	// It is a FIGHT's refusal. The action economy exists only in combat, so a
	// member on the world clock is charged nothing and can never see this.
	ErrCannotAfford = errors.New("action cannot be paid for")

	// ErrBadCost is returned when a price could not be charged to anybody: a
	// profile keyed to a currency no ledger holds, or a cost naming a payer this
	// cast cannot charge.
	//
	// The programmer-facing half of the split ErrCannotAfford describes. This one
	// means content or wiring is wrong — the remedy is to go and look at the
	// code, not at a player's sheet — and reporting it as "out of actions" would
	// send whoever debugs it to exactly the wrong place.
	//
	// Not reachable from a well-formed call today: this package compiles the only
	// prices it charges, and it names the attacker, who is always in the cast. It
	// exists so that the day something else compiles one, the failure has a name
	// that is not a lie. Its translation is pinned in translate_internal_test.go
	// rather than in sentinels_test.go for exactly that reason.
	ErrBadCost = errors.New("cost cannot be charged")

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

	// ErrSaveFailed is returned when an operation fails after persistence is
	// involved. Usually an aggregate could not be saved. A first-admission Join
	// can instead have saved its independently valid rest before a later local
	// refusal; then SaveReport.Written is populated while Failed is empty. In
	// both cases the report preserves S6's distinction between a safe retry and
	// durable progress rather than returning an unqualified failure.
	ErrSaveFailed = errors.New("save failed")

	// ErrBadTurnOutcome is returned when a TurnDriver's Act answers with a
	// TurnOutcome the seam that translates it to the composition does not
	// recognise.
	//
	// Reachable, not merely defensive: TurnOutcome is this package's own
	// vocabulary, and the day it grows a second case, an internal adapter
	// that has not been updated to translate it hits this rather than
	// silently treating the new outcome as Pass.
	ErrBadTurnOutcome = errors.New("turn driver returned an unrecognised outcome")
)
