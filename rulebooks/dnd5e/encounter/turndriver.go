// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TurnDriver decides what a member with no player does when a fight's clock
// lands on their turn.
//
// # Why this is not Decider
//
// Decider already answers "what does an unplayed member do" — but only in
// free roam. Pump skips a monster caught in a bubble entirely: "a monster in
// a fight is not consulted... its decider is skipped until the fight
// dissolves." That boundary is load-bearing rather than an oversight —
// Decider's Intent vocabulary (IntentMoveTo, IntentHold) answers WHERE TO BE,
// and a turn asks a different question, ATTACKING, TARGETING, AND THE ACTION
// ECONOMY (rpg-project#254 gives it one: [MonsterView], [TurnIntent]).
// Reusing Decider for a turn would blur the exact clock-kind boundary
// ClockKind exists to keep sharp: the world thinks on the tick, the fight
// thinks in turns, and now each has its own driver, and its own vocabulary —
// TurnIntent rather than Intent, so the two Go types cannot collide.
//
// # Required, never defaulted — and why that differs from Decider
//
// Decider is optional per member; a monster with none simply holds forever in
// free roam, which is locally inert — nothing else in the encounter depends
// on that monster ever moving. TurnDriver has no such safe default: a member
// the clock lands on with no driver stalls the ENTIRE bubble, forever, for
// every member in it — the exact defect rpg-toolkit#1162 exists to close. So
// it is required at construction like Standing, Sight and Initiative, and for
// the identical reason: a nil answer here would be this module guessing a
// rule instead of asking for one. See ADR-0043.
//
// # One capability, not one per member
//
// Unlike Decider, TurnDriver is asked with a member's whole [MonsterView]
// rather than a bare ID: the same driver instance answers for every unplayed
// member the clock lands on, keying its own decision off View.Self and
// whatever View tells it, the same way any Decider already keys off the ID
// it is handed. There is nothing yet that requires a second capability shape
// at this seam.
type TurnDriver interface {
	// Act decides what a member with no player does on its turn, given
	// everything this composition is willing to tell it about its own
	// situation (view) — or returns an error that aborts the caller's whole
	// verb: see EndTurn and form, both of which consult this synchronously
	// and persist nothing until the caller's own commit, so a driver
	// failure here costs nothing but the retry.
	//
	// A Go error here is a DRIVER MALFUNCTION — compare [ErrBadIntent],
	// which a syntactically valid but unexecutable TurnIntent earns
	// instead, and which does NOT abort the caller (see
	// [Encounter.driveMonsterTurns]).
	Act(view MonsterView) (TurnIntent, error)
}

// MonsterView is what a TurnDriver is told about its own situation on its
// turn — the anti-wall-hack contract [Decider]'s Snapshot already keeps
// (C2), extended to a turn's own questions that Snapshot cannot express. A
// driver receives ONLY this: its own static facts, what it currently holds
// sight intel on, and the turn's remaining budget — never the full
// encounter, and never another member's live truth beyond what has actually
// been seen.
//
// A PROJECTION OF THE MEMBER RECORD PLUS THE TURN'S DYNAMIC PARTS (Kirk,
// rpg-project#254 review): Self, Position, Actions and Targeting are read
// straight off this member's own [memberRecord] — the same static facts
// [Member] itself projects — and Seen, Budget and Round are computed fresh
// for this call, the same static/dynamic split [Sight]'s own doc draws.
type MonsterView struct {
	// Self is who this view is for.
	Self MemberID

	// Position is where this member stands, DUNGEON-ABSOLUTE — the same
	// frame every SeenMember.Position and every Move path cell speaks.
	Position spatial.Position

	// Actions are this member's own static facts about what it can do —
	// carried forward from [MemberInput.Actions]/[Member.Actions] verbatim.
	// An [Attack] intent must name one of these.
	Actions []ActionView

	// Targeting is this member's target-selection strategy, in the
	// rulebook's own words — carried forward from
	// [MemberInput.Targeting]/[Member.Targeting]. Opaque here (C1): a
	// driver that cares what "closest" means already knows, because the
	// rulebook that authored the string is the one reading it.
	Targeting string

	// Seen are the OTHER members this monster currently, actively holds
	// sight intel on — Status == [intel.Current] only. A stale "held"
	// memory (intel.Held, a ghost: known but not currently sustained) is
	// not a target this member can act on THIS turn, so it never appears
	// here; a driver that wants to chase a last-known position needs no
	// special case for that, because Seen simply will not contain it once
	// the sighting lapses.
	Seen []SeenMember

	// PathTo is one step "toward there" made whole: the complete shortest
	// path from this member's own Position to the requested cell, computed
	// against this composition's own walls, doors and floor — the same
	// geometry [Encounter.Step] enforces — DUNGEON-ABSOLUTE, first element
	// the first step to take, last element the requested cell itself. ok is
	// false when no path exists (an unreachable target, an unowned cell).
	//
	// ok TRUE WITH AN EMPTY path IS A LEGAL ANSWER (Copilot, PR #1189
	// review): requesting a path to the cell this member is already
	// standing on has nothing to walk, so path is nil rather than
	// containing that one cell redundantly. A caller must check len(path)
	// before indexing it, not rely on ok alone — [behavior.Basic]'s own
	// call does exactly this ("ok && len(path) > 0").
	//
	// A CLOSURE, NOT A LIVE *Encounter (rpg-project#254) — the same
	// anti-wall-hack reason [Decider]'s own Snapshot carries no live
	// reference either: a driver may ask this one narrow question ("how do
	// I get from here to there") and nothing else, never touch the map or
	// another member's raw state directly. Bound once, at this view's own
	// build time, to this member's own Position — a driver wanting an
	// updated path after moving gets one the same way it gets an updated
	// Position: by being asked again next Act call.
	//
	// "expose a path helper from encounter, don't compute hex math in
	// behavior" (the brief) is what this exists to satisfy: rulebooks/dnd5e/
	// behavior's Basic driver calls this rather than reimplementing grid
	// traversal.
	PathTo func(to spatial.Position) (path []spatial.Position, ok bool)

	// Budget is what remains of this member's turn.
	Budget TurnBudget

	// Round is the fight's own round counter (see play/clock's Turn.Round),
	// carried through so a driver that wants to vary behaviour by round — a
	// monster that flees only after round 2, say — has the fact without
	// this composition growing a second capability to answer it.
	Round int
}

// SeenMember is one other member this monster currently holds active sight
// intel on, projected for a turn's own questions.
type SeenMember struct {
	// ID is the seen member's identifier.
	ID MemberID

	// Kind is whether they are a player or a monster.
	Kind MemberKind

	// Standing is false when this composition's last standing consult found
	// them down (see [Standing]) — a driver should not target a body, and
	// [Encounter.driveMonsterTurns] refuses an Attack naming one
	// ([ErrBadIntent]).
	Standing bool

	// Position is where they were last actively sighted, DUNGEON-ABSOLUTE.
	Position spatial.Position

	// Distance is the grid distance from this monster's OWN position to
	// this sighting, in CELLS — the same unit [Encounter.Distance] answers
	// in, computed once so a driver never needs to ask the encounter for it
	// directly.
	Distance float64

	// InReach maps each of THIS MONSTER'S OWN action refs (see
	// [MonsterView.Actions]) to whether this sighting is within that
	// action's reach right now — precomputed so a driver never converts
	// feet to cells itself (see [ActionView.ReachFeet]'s doc for why that
	// conversion belongs here, once, rather than on every driver).
	InReach map[core.Ref]bool
}

// TurnBudget is what remains of a member's turn.
type TurnBudget struct {
	// AttacksLeft is how many more attacks this member may declare this
	// turn — 1 at the start of a v1 turn, 0 once an [Attack] intent has
	// executed.
	AttacksLeft int

	// MovementFeet is how much further this member may move this turn, in
	// FEET (Kirk, rpg-project#254 review — see [ActionView.ReachFeet]'s doc
	// for the feet/cells split) — [Member.SpeedFeet] at the start of a
	// turn, decremented by 5 feet per cell as [Move] intents execute.
	MovementFeet int
}

// TurnIntent is a sealed vocabulary (unexported marker method) — a
// TurnDriver's decision for one Act call.
//
// A DIFFERENT TYPE FROM Intent, DELIBERATELY (rpg-project#254). [Decider]'s
// own sealed vocabulary (IntentMoveTo, IntentHold) answers WHERE TO BE in
// free roam; a turn asks ATTACKING, TARGETING, AND THE ACTION ECONOMY —
// different questions with different answers — and naming this TurnIntent
// keeps the two Go types from colliding rather than overloading one
// vocabulary to mean two things depending which clock a member is on.
//
// THREE CASES, following this repo's practice for sealed vocabularies at
// this seam (ADR-0038's rule for Gather | Pose | Request | Done): Pass,
// Attack, Move. A fourth case is real vocabulary growth and should probably
// earn its own ADR rather than being added quietly, the same note the old
// one-case TurnOutcome carried.
type TurnIntent interface {
	isTurnIntent()
}

// Pass ends this member's turn immediately: the clock advances and a beat
// is recorded, with no other effect. Returned by a driver with nothing left
// to do — an empty [MonsterView.Seen], an exhausted [MonsterView.Budget], or
// simply a decision to do nothing — and the only intent [PassDriver] ever
// returns.
type Pass struct{}

// isTurnIntent marks Pass as a TurnIntent.
func (Pass) isTurnIntent() {}

// Attack declares a strike against Target using Action — one of this
// member's own [ActionView.Ref] values, from [MonsterView.Actions].
//
// Refused with [ErrBadIntent] — the turn simply ends, see
// [Encounter.driveMonsterTurns] — unless: Target names a member currently in
// [MonsterView.Seen] AND [SeenMember.Standing]; Action names one of this
// member's own [MonsterView.Actions]; [SeenMember.InReach] says Target is in
// reach for Action; and [TurnBudget.AttacksLeft] is greater than zero.
type Attack struct {
	// Target is who this member is attacking.
	Target MemberID

	// Action is which of this member's own actions is doing the attacking.
	Action core.Ref
}

// isTurnIntent marks Attack as a TurnIntent.
func (Attack) isTurnIntent() {}

// Move declares a step-by-step path this member intends to walk this turn,
// cell by cell, DUNGEON-ABSOLUTE.
//
// Each cell executes exactly as [Encounter.Step] would for a player — the
// same refusals, the same walls, the same doors — via [Encounter.stepTo],
// and a step the canvas refuses simply stops the walk there, precisely as it
// would for a player mid-corridor: the turn does not end because of it, and
// whatever cells DID succeed are kept. What DOES end the turn with
// [ErrBadIntent] is a Move this member cannot even afford: an empty Path, or
// one whose length in cells exceeds [TurnBudget.MovementFeet] converted via
// [CellsFromFeet] — a driver asking for more than its own budget allows is a
// bad decision, not a wall in the way.
type Move struct {
	// Path is the sequence of cells this member intends to step through, in
	// travel order.
	Path []spatial.Position
}

// isTurnIntent marks Move as a TurnIntent.
func (Move) isTurnIntent() {}

// PassDriver is a TurnDriver that always passes — the same v1 answer every
// unplayed member's turn used to get automatically before this capability
// existed, now available as an explicit, reusable supply (rpg-toolkit#1167)
// rather than every caller hand-rolling one. A hosting seam with nothing
// smarter to offer yet — or a scene with no monsters at all — can supply
// this and get exactly the behaviour a fight had before a real driver
// (rulebooks/dnd5e/behavior's Basic, or a caller's own) existed.
type PassDriver struct{}

// Act always returns Pass, unconditionally.
func (PassDriver) Act(MonsterView) (TurnIntent, error) {
	return Pass{}, nil
}

// RefusingStriker is a Striker for construction-only worlds: an encounter
// built to hold members and geometry but never actually driven through a
// turn — rpg-api's placement probes, a template's own acceptance test, any
// scene a host constructs only to inspect. Calling Strike on one is a HOST
// BUG, not a legal outcome a caller should ever see recover: a driven turn
// that reaches this means some TurnDriver decided to attack in a world with
// nothing that can carry it out, and the honest answer is a named error
// rather than a silently fabricated hit — the same reasoning [PassDriver]'s
// own doc gives for existing at all (rpg-toolkit#1167), one capability over.
type RefusingStriker struct{}

// Strike always fails with ErrRefusingStriker.
func (RefusingStriker) Strike(context.Context, *Encounter, MemberID, MemberID, core.Ref) error {
	return ErrRefusingStriker
}

// Striker resolves and records one member's attack against another.
//
// SUPPLIED, NEVER DEFAULTED (rpg-toolkit#1033), for the same reason every
// other rulebook-facing capability on this module is: this module's go.mod
// cannot import the rulebook (C1), so it cannot roll to hit, apply damage,
// or decide what a "shortsword" is. What it CAN do is hand the capability
// everything it needs to decide for itself and act — including the live
// *Encounter, so a successful strike can call [Encounter.Record] itself,
// the same public verb any other caller uses to put a struck/missed beat in
// the story (and, through it, the same [Encounter.noticeDown] consult every
// other route to a beat already runs).
//
// REQUIRED AT CONSTRUCTION, exactly as [TurnDriver] is and for the same
// reason (ADR-0043): a monster's driver can decide to attack the moment a
// fight forms, so an encounter that cannot resolve one would stall — or
// worse, silently drop the swing — before its caller does anything.
type Striker interface {
	// Strike resolves attacker's attack against target using action — one
	// of attacker's own [ActionView.Ref] values — and records the outcome
	// itself via [Encounter.Record]. Errors here are STRIKER MALFUNCTIONS
	// (a resolution failure, a corrupt sheet) and abort the caller's whole
	// verb exactly as a [TurnDriver.Act] error does; a miss is not an
	// error — it is an ordinary [OutcomeMissed] recorded the same as a hit.
	Strike(ctx context.Context, enc *Encounter, attacker, target MemberID, action core.Ref) error
}
