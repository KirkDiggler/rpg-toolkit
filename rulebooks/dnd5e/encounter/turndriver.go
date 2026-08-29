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
// driver receives ONLY this: its own static facts, its current sight and held
// location knowledge, and the turn's remaining budget — never the full
// encounter, and never another member's concealed live truth.
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
	// sight intel on — Status == [intel.Current] only. Current sight keeps
	// its existing standing, reach, and reach-aware path meaning. Held
	// testimony never appears here: held known positions project separately
	// into Remembered, while held unknown locations are not actionable.
	Seen []SeenMember

	// Remembered are the OTHER members this monster knows only through its own
	// held sight testimony (Status == [intel.Held]). These are plain knowledge
	// data: they are never attackable, contain no standing or reach fact, and
	// never expose the subject's concealed current position. Position and Path
	// are the remembered cell and an exact-cell route to that cell, if one is
	// reachable through this composition's geometry. Held unknown testimony
	// persists in Intel but produces no entry. The view is rebuilt after each
	// driven move, so new current sight is available on the next Act call and a
	// visible-first driver can interrupt remembered pursuit immediately.
	Remembered []RememberedMember

	// Budget is what remains of this member's turn.
	Budget TurnBudget

	// Round is the fight's own round counter (see play/clock's Turn.Round),
	// carried through so a driver that wants to vary behaviour by round — a
	// monster that flees only after round 2, say — has the fact without
	// this composition growing a second capability to answer it.
	Round int
}

// RememberedMember is one other member's last-known position, projected from
// this monster's own held sight testimony. It is stale knowledge only: the
// entry cannot be used as an attack target and carries no hidden standing or
// reach fact. Path is an exact-cell route from the monster's position to the
// remembered cell, excluding the starting cell and including the destination;
// it is empty when that cell is unreachable (or already occupied by the
// monster).
type RememberedMember struct {
	// ID is the remembered member's identifier.
	ID MemberID

	// Kind is whether the remembered member is a player or monster.
	Kind MemberKind

	// Position is the remembered, possibly stale, DUNGEON-ABSOLUTE cell.
	Position spatial.Position

	// DistanceCells is the grid distance from the monster's own current cell to
	// the remembered cell.
	DistanceCells float64

	// Path is the exact-cell shortest route to Position. It contains no live
	// standing or reach information and is nil when Position is unreachable.
	Path []spatial.Position
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

	// DistanceCells is the grid distance from this monster's OWN position
	// to this sighting, in CELLS — the same unit [Encounter.Distance]
	// answers in, computed once so a driver never needs to ask the
	// encounter for it directly.
	DistanceCells float64

	// InReach maps each of THIS MONSTER'S OWN action refs (see
	// [MonsterView.Actions]) to whether this sighting is within that
	// action's reach right now — precomputed so a driver never converts
	// feet to cells itself (see [ActionView.RangeFeet]'s doc for why that
	// conversion belongs here, once, rather than on every driver).
	InReach map[core.Ref]bool

	// Path is the shortest route from this monster's OWN position toward
	// this sighting, computed against this composition's own walls, doors
	// and floor — the same geometry [Encounter.Step] enforces —
	// DUNGEON-ABSOLUTE, first element the first step to take.
	//
	// ENDS ON THE NEAREST CELL (to this member's own position — the fewest
	// steps, never one more than necessary) FROM WHICH THIS SIGHTING IS
	// WITHIN REACH of this member's own longest-reaching action (Kirk,
	// rpg-project#254 review): NEVER the sighting's own occupied cell, and
	// never farther than the shortest route needs. A driver moving along
	// Path therefore never has to separately check InReach before issuing
	// a Move — Path itself already stops exactly where InReach would turn
	// true. Empty in two cases a caller does not need to tell apart: no
	// walkable route exists, or this member is already within reach
	// without moving at all (Distance already <= its own best reach).
	//
	// DATA, NOT A LIVE CAPABILITY (supersedes an earlier closure-based
	// design this PR shipped and walked back): MonsterView must stay
	// loggable, replayable and fixture-buildable (rpg-project#235's Debug
	// Feed journey; the monster-ai brainstorm's "perception is data, the
	// decision layer never reaches live state") — a func field breaks all
	// three. Computed once per Seen member, per Act call — ONE BFS PER
	// SIGHTING, not one for the chosen target alone; noted here as the
	// acknowledged cost of keeping this a plain value rather than a lazy
	// callback.
	Path []spatial.Position
}

// TurnBudget is what remains of a member's turn.
type TurnBudget struct {
	// AttacksLeft is how many more attacks this member may declare this
	// turn — 1 at the start of a v1 turn, 0 once an [Attack] intent has
	// executed.
	AttacksLeft int

	// MovementFeet is how much further this member may move this turn, in
	// FEET (Kirk, rpg-project#254 review — see [ActionView.RangeFeet]'s doc
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

// BoundaryKind names a temporal boundary a clock verb crossed.
//
// TWO KINDS, because two are what anything publishes. play/clock also reports
// RoundStarted, and this composition translates it to nothing — losing nothing,
// because clock.Turn.End increments the round BEFORE stamping the TurnStarted
// that follows a wrap. The round advancing is therefore already visible as a
// changed Round on the next turn boundary, and a second way to say it would be
// a second way to disagree.
//
// Nor is a round boundary merely unpublished-for-now. 5e measures durations in
// rounds and resolves every one of them on a turn — "until the start of your
// next turn", "at the end of its turn" — so the round is a COORDINATE rather
// than a trigger. Kirk ruled it 2026-08-27: "rage is from my turn to my turn.
// If I am last in initiative I keep rage until the end of my next turn. what
// would need round end?" Nothing did. The day something does, the kind arrives
// with its publisher and its first subscriber together.
type BoundaryKind string

const (
	// TurnStarted marks Subject's turn beginning.
	TurnStarted BoundaryKind = "turn_started"
	// TurnEnded marks Subject's turn ending.
	TurnEnded BoundaryKind = "turn_ended"
)

// Boundary is one temporal boundary this composition crossed, in this
// composition's own vocabulary.
//
// A translation of [clock.Milestone] rather than a re-export of it, for the
// reason every other clock report here is translated: the leaf speaks
// core.EntityID and this module speaks MemberID, and a host must never come to
// name an inner type.
type Boundary struct {
	// Kind is which boundary was crossed.
	Kind BoundaryKind

	// Subject is whose turn it is about. Never empty: both kinds are about
	// somebody.
	Subject MemberID

	// Round is which round of the fight the boundary belongs to.
	Round int
}

// Announcer publishes the temporal boundaries a clock advance crossed.
//
// SUPPLIED, NEVER DEFAULTED (rpg-toolkit#1033), like every other
// rulebook-facing capability on this module and for the same reason: this
// module's go.mod cannot import the rulebook (C1), so it cannot know what a
// turn boundary MEANS to a condition. What it can do is notice the boundary —
// which is its job, since "the composition is the first layer allowed to have
// an opinion about the game" — and hand it to something that can.
//
// # Why a capability rather than a return value
//
// This is the whole reason this type exists instead of another output field.
//
// [Encounter.EndTurn] drives every consecutive unplayed member forward inside
// one call (rpg-toolkit#1162), and those members ATTACK during the drive,
// through [Striker]. Boundaries merely returned and published afterwards would
// put every driven member's turn-start AFTER that member had already swung.
//
// Today that is invisible: nothing on a monster's sheet subscribes to a turn
// boundary. Which is exactly the shape of defect worth refusing to build —
// correct by coincidence, silent the moment the coincidence ends. So the
// boundary is announced AT the crossing, before the member the clock landed on
// acts, and an implementation gets the live *Encounter for the same reason
// Striker does.
//
// # Required, never defaulted
//
// A nil Announcer is not "boundaries are switched off". It is "every condition
// scoped to a turn silently never expires" — which is the bug this capability
// was introduced to fix, and it went unnoticed for months precisely because
// nothing said anything. Refused at the door, exactly as [TurnDriver],
// [Striker], Standing and Sight are.
type Announcer interface {
	// Announce publishes the boundaries one clock advance crossed, in the
	// causal order given, before the next member acts.
	//
	// An error here is an ANNOUNCER MALFUNCTION and aborts the caller's
	// whole verb, exactly as [TurnDriver.Act] and [Striker.Strike] errors
	// do. Nothing is persisted until the caller's own commit, so a failure
	// costs the retry and nothing else.
	Announce(ctx context.Context, enc *Encounter, crossed []Boundary) error
}

// RefusingAnnouncer is an Announcer for construction-only worlds — the exact
// twin of [RefusingStriker], and for the identical reason.
//
// A clock cannot advance on a world built only to be inspected: rpg-api's
// placement probes, a template's acceptance test, resolution's own reconstructed
// snapshot (which never calls EndTurn or form, so no boundary can be crossed
// inside one). Reaching this is therefore a HOST BUG.
//
// NOT A NO-OP, and the difference is the whole point. A silently-succeeding
// default would be indistinguishable from a boundary nobody published — which
// is the defect this capability exists to end. The test for an absent value is
// not "is it harmless" but "does it say what the author meant".
type RefusingAnnouncer struct{}

// Announce always fails with ErrRefusingAnnouncer.
func (RefusingAnnouncer) Announce(context.Context, *Encounter, []Boundary) error {
	return ErrRefusingAnnouncer
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
