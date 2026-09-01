// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TurnDriver decides what a member with no player does when a fight's clock
// lands on their turn. Required.
//
// This package's own twin of encounter.TurnDriver (S2): a host implementing
// encounter's interface directly would name a module this SDK intends to
// keep replaceable underneath it. Wire behavior.Basic (import path
// github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/behavior) for a real
// driver, or session.Pass{} for v1's original whole answer — every unplayed
// member's turn ends the moment the clock reaches it.
type TurnDriver interface {
	// Act decides what member does on their turn, given everything this SDK
	// is willing to tell it about the member's own situation (view), or
	// returns an error that aborts the verb that discovered the unplayed
	// turn. Nothing is persisted on that error: this package's load-mutate-
	// save shape means the in-memory world is simply discarded.
	Act(view MonsterView) (TurnIntent, error)
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

	// Position is the member's last-known dungeon-absolute cell and may be stale.
	Position spatial.Position

	// DistanceCells is the grid distance from this member to Position, in cells.
	DistanceCells float64

	// Path is the exact-cell route toward Position; it is empty or nil when the
	// remembered cell is unreachable.
	Path []spatial.Position
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

// turnDriverSeam adapts the host's TurnDriver to the composition's, both
// directions: encounter.MonsterView projects onto this package's own
// MonsterView on the way in, and this package's own TurnIntent projects onto
// encounter.TurnIntent on the way out.
//
// Unexported for the reason every seam in this file is: if the host had to
// satisfy encounter.TurnDriver directly, replacing the composition would
// break every host that implemented it.
type turnDriverSeam struct {
	driver TurnDriver
}

// Act translates one member's view and intent across the boundary.
func (s turnDriverSeam) Act(view encounter.MonsterView) (encounter.TurnIntent, error) {
	intent, err := s.driver.Act(projectMonsterView(view))
	if err != nil {
		return nil, err
	}

	switch it := intent.(type) {
	case Pass, *Pass:
		return encounter.Pass{}, nil
	case Attack:
		return attackToEncounter(view.Self, it)
	case *Attack:
		return attackToEncounter(view.Self, *it)
	case Move:
		return encounter.Move{Path: it.Path}, nil
	case *Move:
		return encounter.Move{Path: it.Path}, nil
	default:
		return nil, fmt.Errorf("turn driver %q: %w: %T", view.Self, ErrBadTurnOutcome, intent)
	}
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

// compile-time proof the adapter satisfies what it is handed to.
var _ encounter.TurnDriver = turnDriverSeam{}

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
			Standing:      sm.Standing,
			Position:      sm.Position,
			DistanceCells: sm.DistanceCells,
			InReach:       inReach,
			Path:          append([]spatial.Position(nil), sm.Path...),
		}
	}
	remembered := make([]RememberedMember, len(view.Remembered))
	for i, rm := range view.Remembered {
		remembered[i] = RememberedMember{
			ID:            string(rm.ID),
			Kind:          MemberKind(rm.Kind),
			Position:      rm.Position,
			DistanceCells: rm.DistanceCells,
			Path:          append([]spatial.Position(nil), rm.Path...),
		}
	}

	return MonsterView{
		Self:       string(view.Self),
		Position:   view.Position,
		Actions:    actions,
		Targeting:  view.Targeting,
		Seen:       seen,
		Remembered: remembered,
		Budget:     TurnBudget{AttacksLeft: view.Budget.AttacksLeft, MovementFeet: view.Budget.MovementFeet},
		Round:      view.Round,
	}
}
