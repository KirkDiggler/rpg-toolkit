// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TurnDriver decides what a member with no player does when it is given time:
// its turn in a fight, or a round of the world. Required.
//
// This package's own twin of encounter.Driver (S2): a host implementing the
// composition's interface directly would name a module this SDK intends to
// keep replaceable underneath it.
//
// WIRE [Driver] — the creature's own authored table, which is what every
// monster in a real run is driven by (rpg-project#465). session.Pass{} remains
// for v1's original whole answer: every unplayed member's turn ends the moment
// the clock reaches it, which is what a workbench and most fixtures want.
//
// A HOST'S OWN DRIVER ROLLS NOTHING, and the view it is handed says so: this
// twin carries no table, no temperament and no deeds, because the one driver
// that reads those never crosses this boundary (see [tableDriver]). What a
// driver written against this interface gets is what it always got — position,
// actions, sight, memory, budget — and what it answers with is an intent.
type TurnDriver interface {
	// Act decides what member does on their turn, given everything this SDK
	// is willing to tell it about the member's own situation (view), or
	// returns an error that aborts the verb that discovered the unplayed
	// turn. Nothing is persisted on that error: this package's load-mutate-
	// save shape means the in-memory world is simply discarded.
	Act(view MonsterView) (TurnIntent, error)
}

// TurnDriverSource hands over the [TurnDriver] that serves ONE session. Wire
// it as Config.TurnDrivers when the driver is stateful; wire the driver
// itself as Config.TurnDriver when it is not. Exactly one of the two.
//
// # Why a per-session seam exists at all
//
// A stateful driver — [Minded], and every authored mind that comes after it —
// is a game with per-member names and memory, and it is not safe for
// concurrent use. One driver per Manager meant one driver for every session
// the process served, so two parties running the same authored dungeon shared
// a skeleton's assigned mind and names, because member ids are authored per
// dungeon rather than minted per run (rpg-api#980's caveat, rpg-toolkit#1734).
// This is the seam that lets a host give each session its own.
//
// # The cache is the HOST's, deliberately
//
// A session's lifetime is the host's — a Redis TTL, a run ending — and this
// Manager is stateless per verb (S1) with no session-end signal to evict on.
// A cache here would have no owner, so the host keeps one and this package
// asks it. See rule A6 in docs/ideas/mind/behavior/adoption.md.
//
// # An interface rather than a func, and it takes the verb's context
//
// An interface because every other capability on Config is one (Roller,
// PresentationIDGenerator, the three repositories): a host implements them
// side by side and this reads like its neighbours. The context because the
// payer named for this seam is minds as authored data — a driver built from
// per-session material a host may have to go and fetch — and adding the
// parameter afterwards would break every host that had implemented the
// interface, which is the asymmetry Config's own doc warns about.
type TurnDriverSource interface {
	// DriverFor returns the driver that serves sessionID, or an error that
	// fails the verb asking. It is called ONCE per verb, for the session that
	// verb is about, and the driver it hands over serves that whole verb.
	//
	// An error here is never recovered from: a host that cannot say which
	// driver serves a session has a wiring fault, and a fallback to the
	// reference driver would answer a monster's turn with somebody else's
	// brain and look like a design choice.
	DriverFor(ctx context.Context, sessionID string) (TurnDriver, error)
}

// staticTurnDrivers is the source a host that wired Config.TurnDriver gets for
// free: one stateless driver, handed to every session.
//
// IT EXISTS SO THERE IS ONE RESOLUTION PATH rather than two. Every verb asks a
// source for its session's driver; the Manager never holds a driver of its
// own, so "which driver is this verb using" has exactly one answer and no
// branch to get wrong.
type staticTurnDrivers struct {
	driver TurnDriver
}

// compile-time proof the stand-in satisfies what it is handed to.
var _ TurnDriverSource = staticTurnDrivers{}

// DriverFor returns the one driver, for any session.
func (s staticTurnDrivers) DriverFor(context.Context, string) (TurnDriver, error) {
	return s.driver, nil
}

// refusingTurnDriver is the stand-in for a world no session holds —
// [encounter.RefusingStriker]'s pattern one capability over, and the fourth
// refusing stand-in at the same call site ([Manager.loadAuthored]).
//
// An authored world is loaded to be inspected and re-serialized: no clock
// advances and no turn is ever driven there, so a driver asked to act on one
// is this package's own bug. There is also no session to name, which is the
// structural reason this is a stand-in rather than a resolution — a host's
// source is asked about a session, and StartSession's has not been created yet
// while AtlasOf has none at all.
type refusingTurnDriver struct{}

// compile-time proof the stand-in satisfies what it is handed to.
var _ TurnDriver = refusingTurnDriver{}

// Act always refuses: no authored world drives a turn.
func (refusingTurnDriver) Act(MonsterView) (TurnIntent, error) {
	return nil, fmt.Errorf("a turn was driven on a construction-only world: %w", ErrInvalidWorld)
}

// MonsterView is this package's own twin of encounter.MonsterView — what a
// TurnDriver is told about its own turn. Plain data throughout (rpg-project#254
// review): loggable, replayable, fixture-buildable, and never a live
// *encounter.Encounter reference.
type MonsterView struct {
	// Self is who this view is for.
	Self string

	// Position is where this member stands, dungeon-absolute.
	Position spatial.Position

	// Actions are this member's own static facts about what it can do.
	Actions []ActionView

	// Targeting is this member's target-selection strategy, in the
	// rulebook's own words. Opaque here (S2): a driver that cares what
	// "closest" means already knows.
	Targeting string

	// Holdings is everything this member holds, on every channel, as
	// values — the raw testimony Seen and Remembered are decoded from,
	// plus what they drop (a deeds-channel holding, for one). A driver
	// that reads testimony itself reads it here; the store stays the
	// encounter's (rule A1), and nothing on this slice reaches it.
	Holdings []Holding

	// At is the clock's high-water when this view was built: the same
	// stamp every holding above was landed at, so a driver can age them.
	At uint64

	// Seen are the other members this member currently, actively holds
	// sight intel on.
	Seen []SeenMember

	// Remembered are stale positions held from prior sight testimony. They
	// are plain knowledge only: a driver may move toward one, but it is not
	// an attack target and carries no concealed current-state facts.
	Remembered []RememberedMember

	// Budget is what remains of this member's turn.
	Budget TurnBudget

	// Round is the fight's own round counter.
	Round int
}

// RememberedMember is one other member's last-known position. It carries no
// attack reach, standing, or other concealed current-state fact.
type RememberedMember struct {
	// ID identifies the remembered member.
	ID string

	// Kind is whether the remembered member is a player or monster.
	Kind MemberKind

	// Opposed is whether the stance graph puts this remembered member on the
	// other side right now — [SeenMember.Opposed]'s twin.
	//
	// ASKED NOW, NOT REMEMBERED. Where they stand is stale by definition;
	// whose side they are on is the graph's answer at this instant, because a
	// truce made while the creature was not looking is still a truce.
	Opposed bool

	// Position is the member's last-known dungeon-absolute cell and may be stale.
	Position spatial.Position

	// DistanceCells is the grid distance from this member to Position, in cells.
	DistanceCells float64

	// Path is the exact-cell route toward Position; it is empty or nil when the
	// remembered cell is unreachable.
	Path []spatial.Position
}

// Holding is this package's own twin of perception.Holding (S2): one piece
// of testimony a member holds, on one channel, as a value.
//
// A SECOND TWIN of the same inner type, beside [Sighting], and deliberately
// so. Sighting answers a client's question — "what does this member know
// about that one, and is it fresh" — and folds the channel list into a
// status word. This one answers a driver's: the record as the store holds
// it, every field, so a mind that reads a channel this SDK has not typed
// can. Folding the two would make one of the two questions answer the
// other's shape.
type Holding struct {
	// Subject is what this testimony is about — a member id on the sight
	// channel, a qualified id on another (mind/perception's rule 11).
	Subject string

	// Payload is the testimony itself, in its channel's own encoding.
	Payload []byte

	// Channel is what delivered it: "sight", "deeds", and whatever else
	// the rulebook lands. Opaque here for Targeting's reason.
	Channel string

	// Observed is the clock reading this payload was FIRST perceived at.
	Observed uint64

	// Confirmed is the clock reading it was last landed at, whether or not
	// it changed — what a driver ages a memory against.
	Confirmed uint64

	// CurrentVia is every channel delivering this subject right now. Empty
	// means nothing sustains it: a ghost, or discrete testimony like a
	// deed, which is always in the past the moment it exists.
	CurrentVia []string
}

// ActionView is this package's own twin of encounter.ActionView: a static
// fact about one action a member can take.
type ActionView struct {
	// Ref identifies this authored action definition, as "module:type:id"
	// (core.Ref.String()) rather than core.Ref itself (S2 — core.Ref is not
	// on this package's contract-type allow-list, and a driver never
	// constructs one field by field; it only ever echoes this string back
	// verbatim as Attack.Action). Round-tripped by core.ParseString on the
	// way back across the boundary (see turnDriverSeam.Act).
	Ref string

	// Name is this action's authored display name — "Longsword", "bite".
	Name string

	// RangeFeet is how far this action reaches, in feet.
	RangeFeet int

	// Kind is the action's delivery projection: "melee" or "ranged". It is
	// derived from the shared definition and remains opaque to this seam.
	Kind string
}

// SeenMember is this package's own twin of encounter.SeenMember: one other
// member this member currently holds active sight intel on.
type SeenMember struct {
	// ID is the seen member's identifier.
	ID string

	// Kind is whether they are a player or a monster.
	Kind MemberKind

	// Opposed is whether the stance graph puts this sighting on the other side
	// RIGHT NOW (rpg-project#465).
	//
	// PROJECTED, NEVER DERIVED BY A DRIVER. A driver has no stance graph to
	// ask and must not get one: who is against whom is a fact about factions
	// and dispositions that only the composition holds, and a driver reading
	// [Kind] instead would make every monster hostile to every player forever
	// — which is exactly what kept a neutral goblin from being possible.
	//
	// False for a member in no faction, a world NPC included: "nobody is
	// against them" is the honest answer, and it is what lets a creature stand
	// quietly in a room full of vendors.
	Opposed bool

	// Standing is false when this member is known to be down.
	Standing bool

	// Position is where they were last actively sighted, dungeon-absolute.
	Position spatial.Position

	// DistanceCells is the grid distance from this member's own position to
	// this sighting, in cells.
	DistanceCells float64

	// InReach maps each of this member's own action refs — the same
	// "module:type:id" string ActionView.Ref reports (S2, see its own doc)
	// — to whether this sighting is within that action's reach right now.
	InReach map[string]bool

	// Path is the shortest walkable route from this member's own position
	// toward this sighting, ending on the nearest cell from which the
	// sighting is within this member's own longest reach. Empty when
	// unreachable, or when this member is already within reach without
	// moving at all.
	Path []spatial.Position

	// AwayPath is one step that puts more of the board between this member
	// and the seen one — the composition's own answer, budget one cell —
	// or empty when every step leads closer or nowhere. A driver that
	// keeps its distance reads it rather than reaching for the canvas
	// (rule A2): fleeing into a corner is not fleeing, and the board is
	// what knows where the corners are.
	AwayPath []spatial.Position
}

// TurnBudget is this package's own twin of encounter.TurnBudget: what
// remains of a member's turn.
type TurnBudget struct {
	// AttacksLeft is how many more attacks this member may declare this
	// turn.
	AttacksLeft int

	// MovementFeet is how much further this member may move this turn, in
	// feet.
	MovementFeet int
}

// TurnIntent is this package's own twin of encounter.TurnIntent — a sealed
// vocabulary (unexported marker method), named to avoid colliding with the
// composition's own Intent (Decider's free-roam vocabulary) one layer down.
type TurnIntent interface {
	isTurnIntent()
}

// Pass ends this member's turn immediately, with no other effect. Also this
// SDK's own ready-made TurnDriver: wire `TurnDriver: session.Pass{}` and
// every unplayed member's turn ends the moment the clock reaches it.
type Pass struct{}

// isTurnIntent marks Pass as a TurnIntent.
func (Pass) isTurnIntent() {}

// Act always passes, regardless of who is asked. Pass satisfies TurnDriver
// as well as being one of its outcomes, so a host that wants v1's whole
// behavior wires the same value to both jobs.
func (Pass) Act(MonsterView) (TurnIntent, error) {
	return Pass{}, nil
}

// Attack declares a strike against Target using Action — one of this
// member's own ActionView.Ref strings, from MonsterView.Actions, echoed back
// verbatim.
type Attack struct {
	Target string
	Action string
}

// isTurnIntent marks Attack as a TurnIntent.
func (Attack) isTurnIntent() {}

// Move declares a step-by-step path this member intends to walk this turn,
// dungeon-absolute.
type Move struct {
	Path []spatial.Position
}

// isTurnIntent marks Move as a TurnIntent.
func (Move) isTurnIntent() {}

// MovePolicy is how a route is measured when a driver hands over a policy
// instead of a path — this package's own twin of the composition's word.
//
// A STRING ENUM RATHER THAN A MIRROR OF THE COMPOSITION'S, for [MemberKind]'s
// reason: it crosses to a host, and a word added later must be a compatible
// change. The set is the composition's to grow; a word it does not walk is
// refused there rather than filtered here, so this package never becomes a
// second opinion about what can be walked.
type MovePolicy string

const (
	// MoveToward routes to a cell beside the anchor and stops there, as far as
	// the budget reaches.
	MoveToward MovePolicy = "toward"

	// MoveAway routes to the reachable cell farthest from the anchor.
	MoveAway MovePolicy = "away"
)

// Routed asks the composition to find the path and walk it, then end the turn.
//
// WHERE Move HANDS OVER A PATH, THIS HANDS OVER A POLICY AND AN ANCHOR:
// "toward that member, as far as this turn's movement reaches." The
// composition routes it, walks the cells through the same step a Move's walk
// takes — so it provokes, pauses for a player's window, and resumes — and ends
// the turn when the walk stops for any reason.
//
// TERMINAL, which is the difference worth knowing before wiring one: a driver
// that wants to walk and then act hands a path to [Move] as before. The
// customers for a whole turn spent walking are a compelled creature, whose turn
// IS the walk, and a monster that decides to run.
//
// It is the one intent that carries a Cause, because it is the one intent the
// member did not decide. See [Routed.Cause].
type Routed struct {
	// Policy is how the route is measured: "toward" or "away", the
	// composition's own words. An unsupported one is refused rather than
	// walked some other way.
	Policy MovePolicy

	// Anchor is the member the policy is measured from — the thing approached,
	// or the thing fled. A member nobody can find ends the turn, as Pass
	// would; their cell is read at execution rather than carried, because the
	// anchor may have moved since whatever decided this.
	Anchor string

	// Cause is what routed them, as a core.Ref string, and it travels on every
	// beat this walk appends. REQUIRED: a creature whose whole turn was spent
	// walking somewhere it did not choose, narrated with no cause, is an
	// observer being told it walked off of its own accord.
	Cause string
}

// isTurnIntent marks Routed as a TurnIntent.
func (Routed) isTurnIntent() {}

// turnDriverSeam adapts the host's TurnDriver to the composition's, both
// directions: encounter.MonsterView projects onto this package's own
// MonsterView on the way in, and this package's own TurnIntent projects onto
// encounter.TurnIntent on the way out.
//
// Unexported for the reason every seam in this file is: if the host had to
// satisfy encounter.Driver directly, replacing the composition would break
// every host that implemented it.
type turnDriverSeam struct {
	driver TurnDriver
}

// Act translates one member's view and intent across the boundary.
//
// THE PICK IS ALWAYS NIL, and that is a statement rather than an omission. A
// [Decision] carries the roll that chose an intent when one was rolled
// (rpg-project#465); a driver on this side of the boundary was handed a view
// with no table in it and rolled nothing, so the composition writes no answer
// beat for the turn it takes. The one driver that DOES roll never comes
// through here — see [tableDriver].
func (s turnDriverSeam) Act(view encounter.MonsterView) (encounter.Decision, error) {
	intent, err := s.driver.Act(projectMonsterView(view))
	if err != nil {
		return encounter.Decision{}, err
	}

	switch it := intent.(type) {
	case Pass, *Pass:
		return encounter.Decision{Intent: encounter.Pass{}}, nil
	case Attack:
		return decisionOf(attackToEncounter(view.Self, it))
	case *Attack:
		return decisionOf(attackToEncounter(view.Self, *it))
	case Move:
		return encounter.Decision{Intent: encounter.Move{Path: it.Path}}, nil
	case *Move:
		return encounter.Decision{Intent: encounter.Move{Path: it.Path}}, nil
	case Routed:
		return decisionOf(routedToEncounter(view.Self, it))
	case *Routed:
		return decisionOf(routedToEncounter(view.Self, *it))
	default:
		return encounter.Decision{}, fmt.Errorf("turn driver %q: %w: %T", view.Self, ErrBadTurnOutcome, intent)
	}
}

// decisionOf wraps one translated intent as the rolless decision every driver
// on this side of the boundary makes. It exists so the four translating arms
// above read as one line each rather than three, and so "a host's driver
// rolled nothing" is written once.
func decisionOf(intent encounter.TurnIntent, err error) (encounter.Decision, error) {
	if err != nil {
		return encounter.Decision{}, err
	}

	return encounter.Decision{Intent: intent}, nil
}

// attackToEncounter parses an Attack's Action string back into the core.Ref
// the composition speaks, the reverse of ActionView.Ref's own projection.
//
// A string a driver could not have gotten from this package's own
// MonsterView — hand-built or corrupted in transit — reports the same
// ErrBadTurnOutcome an unrecognised TurnIntent type does: it is the same
// fact, an outcome this seam cannot translate, from a different cause.
func attackToEncounter(self encounter.MemberID, it Attack) (encounter.TurnIntent, error) {
	ref, err := core.ParseString(it.Action)
	if err != nil {
		return nil, fmt.Errorf("turn driver %q: %w: action %q: %v", self, ErrBadTurnOutcome, it.Action, err)
	}
	return encounter.Attack{Target: encounter.MemberID(it.Target), Action: *ref}, nil
}

// routedToEncounter parses a Routed's Cause back into the core.Ref the
// composition speaks, the reverse of the string this package publishes.
//
// A cause that will not parse is the same fact [attackToEncounter] reports
// about an action string: an outcome this seam cannot translate. It is refused
// here rather than passed on empty, because the composition's own refusal
// would name a missing cause and the truth is a malformed one — and a driver
// author reading "no cause" while looking at the cause they wrote has been
// told the wrong thing.
//
// The POLICY is crossed as-is rather than parsed. Unlike the cause it is not
// this package's string to re-derive: [MovePolicy] is the composition's own
// vocabulary projected here, and the composition refuses a word it does not
// walk.
func routedToEncounter(self encounter.MemberID, it Routed) (encounter.TurnIntent, error) {
	cause, err := core.ParseString(it.Cause)
	if err != nil {
		return nil, fmt.Errorf("turn driver %q: %w: cause %q: %v", self, ErrBadTurnOutcome, it.Cause, err)
	}
	return encounter.Routed{
		Policy: encounter.MovePolicy(it.Policy),
		Anchor: encounter.MemberID(it.Anchor),
		Cause:  *cause,
	}, nil
}

// compile-time proof the adapter satisfies what it is handed to.
var _ encounter.Driver = turnDriverSeam{}

// projectMonsterView turns the composition's own MonsterView into this
// package's own twin — the boring, load-bearing translation S2 is the price
// of (see the package-level types.go comment on this file's siblings).
func projectMonsterView(view encounter.MonsterView) MonsterView {
	actions := make([]ActionView, len(view.Actions))
	for i, a := range view.Actions {
		actions[i] = ActionView{Ref: a.Ref.String(), Name: a.Name, RangeFeet: a.RangeFeet, Kind: a.Kind}
	}

	seen := make([]SeenMember, len(view.Seen))
	for i, sm := range view.Seen {
		inReach := make(map[string]bool, len(sm.InReach))
		for ref, ok := range sm.InReach {
			inReach[ref.String()] = ok
		}
		seen[i] = SeenMember{
			ID:            string(sm.ID),
			Kind:          MemberKind(sm.Kind),
			Opposed:       sm.Opposed,
			Standing:      sm.Standing,
			Position:      sm.Position,
			DistanceCells: sm.DistanceCells,
			InReach:       inReach,
			Path:          append([]spatial.Position(nil), sm.Path...),
			AwayPath:      append([]spatial.Position(nil), sm.AwayPath...),
		}
	}
	remembered := make([]RememberedMember, len(view.Remembered))
	for i, rm := range view.Remembered {
		remembered[i] = RememberedMember{
			ID:            string(rm.ID),
			Kind:          MemberKind(rm.Kind),
			Opposed:       rm.Opposed,
			Position:      rm.Position,
			DistanceCells: rm.DistanceCells,
			Path:          append([]spatial.Position(nil), rm.Path...),
		}
	}

	var holdings []Holding
	if len(view.Holdings) > 0 {
		holdings = make([]Holding, len(view.Holdings))
		for i, h := range view.Holdings {
			holdings[i] = projectHolding(h)
		}
	}

	return MonsterView{
		Self:       string(view.Self),
		Position:   view.Position,
		Actions:    actions,
		Targeting:  view.Targeting,
		Holdings:   holdings,
		At:         view.At,
		Seen:       seen,
		Remembered: remembered,
		Budget:     TurnBudget{AttacksLeft: view.Budget.AttacksLeft, MovementFeet: view.Budget.MovementFeet},
		Round:      view.Round,
	}
}

// projectHolding turns one piece of the composition's own testimony into
// this package's twin — ids and channels as the strings S2 crosses, the
// payload untouched because only its own channel can read it.
func projectHolding(h perception.Holding) Holding {
	// NOTHING IS INVENTED WHERE THERE WAS NOTHING. An absent channel list
	// stays absent rather than becoming an empty one, because a deed is
	// current on nothing by construction and the two seams have to agree
	// about what its holding looks like on the way back.
	var via []string
	if len(h.CurrentVia) > 0 {
		via = make([]string, len(h.CurrentVia))
		for i, c := range h.CurrentVia {
			via[i] = string(c)
		}
	}

	return Holding{
		Subject:    string(h.Subject),
		Payload:    append([]byte(nil), h.Payload...),
		Channel:    string(h.Channel),
		Observed:   h.Observed,
		Confirmed:  h.Confirmed,
		CurrentVia: via,
	}
}
