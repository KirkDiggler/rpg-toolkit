// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Behavior returns a TurnDriver backed by behavior.Basic (import path
// github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/behavior) — the
// reference driver for any member nobody is playing: attack the closest
// standing player if one is in reach, otherwise close the distance one step
// at a time, otherwise pass.
//
// behavior.Basic itself satisfies encounter.TurnDriver, not this package's
// own — it is written one layer down, against the composition's own
// MonsterView, because that is also the layer resolution's and encounter's
// own construction-only fixtures need it at. A host must never import
// encounter to use it (S2), so this wraps it behind the boundary: wire
// `TurnDriver: session.Behavior()` and that import never has to happen.
func Behavior() TurnDriver {
	return basicSeam{}
}

// basicSeam adapts behavior.Basic to this package's own TurnDriver — the
// mirror image of turnDriverSeam, one file over. Where turnDriverSeam
// projects the composition's MonsterView onto this package's own on the way
// IN (a host's driver never sees an encounter type), this seam projects
// this package's own MonsterView back onto the composition's, because
// behavior.Basic is written against that one instead.
//
// THE ROUND TRIP IS THE COST OF THE BOUNDARY, paid here rather than
// skipped: turnDriverSeam.Act already flattened the composition's own view
// into this package's MonsterView before basicSeam ever sees it (every
// TurnDriver reaches through that same seam, reference driver included), so
// unprojectMonsterView below reconstructs it. Slightly wasteful, and
// deliberately so — special-casing "this one driver happens to live in the
// same monorepo" would be exactly the kind of exception S2 exists to
// refuse.
type basicSeam struct{}

// compile-time proof the adapter satisfies what it is handed to.
var _ TurnDriver = basicSeam{}

// Act translates one member's view and intent across the boundary, in the
// direction opposite turnDriverSeam.Act.
func (basicSeam) Act(view MonsterView) (TurnIntent, error) {
	encView, err := unprojectMonsterView(view)
	if err != nil {
		return nil, fmt.Errorf("behavior driver %q: %w: %v", view.Self, ErrBadTurnOutcome, err)
	}

	intent, err := (behavior.Basic{}).Act(encView)
	if err != nil {
		return nil, err
	}

	return intentFromEncounter(view.Self, intent)
}

// Minded returns a TurnDriver backed by behavior.Minded: each member gets the
// mind its sheet names (rule A5), a member naming none is driven as Behavior()
// drives it, and an unknown name fails loudly.
//
// The value is STATEFUL, where a Behavior() is not: it keeps per-member names
// across turns, and it is not safe for concurrent use. ONE OF THESE SERVES ONE
// SESSION — a host hands them out through [Config.TurnDrivers], which is asked
// once per verb for the session that verb is about (rpg-toolkit#1734). Wiring
// one to [Config.TurnDriver] instead would put a single stateful driver in
// front of every session in the process, which is what this constructor's doc
// used to describe and what rpg-api#980's mutex and cross-session member-id
// caveat existed to contain.
//
// What that seam does NOT change is the boundary inside one session: two write
// verbs on the same session at the same instant already race that session's
// own scope, and a driver of its own inherits exactly that boundary rather
// than widening or narrowing it.
func Minded(_ *MindedInput) (TurnDriver, error) {
	driver, err := behavior.NewMinded(nil)
	if err != nil {
		return nil, fmt.Errorf("minded driver: %w", err)
	}

	return mindedSeam{driver: driver}, nil
}

// MindedInput configures the minded driver, and configures nothing today:
// nil is the whole contract, and every host passes it.
//
// It used to carry Patience — one grudge length for every mind in the
// process. Patience is now the MIND's, named by the word a monster's sheet
// says and held by the driver's own preset for that word
// (rpg-toolkit#1745), because a host that set one number could not have a
// berserker and a bow skeleton on the same board.
//
// The type stays because the door does. A knob that belongs to the host
// rather than to a mind — which is not the same thing as a knob that
// belongs to a monster — arrives here, and arrives without changing the
// signature every host already calls.
type MindedInput struct{}

// mindedSeam adapts behavior.Minded to this package's own TurnDriver,
// exactly as basicSeam adapts behavior.Basic — same round trip, same
// reasons (see basicSeam's own doc).
//
// STATEFUL, which basicSeam is not: behavior.Minded remembers which mind each
// member was given, so this value outlives any one turn and is not safe for
// concurrent use. One of these serves ONE SESSION, handed over per verb by the
// host's [TurnDriverSource] — so what the host serializes is one session's own
// turns, which is the boundary that session already had.
type mindedSeam struct {
	driver *behavior.Minded
}

// compile-time proof the adapter satisfies what it is handed to.
var _ TurnDriver = mindedSeam{}

// Act translates one member's view and intent across the boundary.
func (s mindedSeam) Act(view MonsterView) (TurnIntent, error) {
	encView, err := unprojectMonsterView(view)
	if err != nil {
		return nil, fmt.Errorf("behavior driver %q: %w: %v", view.Self, ErrBadTurnOutcome, err)
	}

	intent, err := s.driver.Act(encView)
	if err != nil {
		return nil, err
	}

	return intentFromEncounter(view.Self, intent)
}

// intentFromEncounter maps the composition's own answer onto this package's,
// shared by every seam that wraps a driver written one layer down.
//
// ONE MAPPING, not one per seam: a driver added here that quietly handled a
// different set of intents than the reference one would be a second opinion
// about what a turn can be, and the first anybody would hear of it is a
// monster doing nothing on a board where another monster acts.
func intentFromEncounter(self string, intent encounter.TurnIntent) (TurnIntent, error) {
	switch it := intent.(type) {
	case encounter.Pass:
		return Pass{}, nil
	case encounter.Attack:
		return Attack{Target: string(it.Target), Action: it.Action.String()}, nil
	case encounter.Move:
		return Move{Path: it.Path}, nil
	default:
		return nil, fmt.Errorf("behavior driver %q: %w: %T", self, ErrBadTurnOutcome, intent)
	}
}

// unprojectMonsterView reconstructs the composition's own MonsterView from
// this package's own twin — the exact reverse of projectMonsterView, field
// for field.
//
// A Ref string this package did not itself just produce (via
// projectMonsterView's own a.Ref.String() a moment earlier in the same
// call) is the only way core.ParseString fails here, which makes a failure
// this seam's own bug rather than a caller's mistake — reported as
// ErrBadTurnOutcome by Act, the same sentinel an unrecognised TurnIntent
// type gets, for the same reason: both are an outcome this seam cannot
// make sense of.
func unprojectMonsterView(view MonsterView) (encounter.MonsterView, error) {
	actions := make([]encounter.ActionView, len(view.Actions))
	for i, a := range view.Actions {
		ref, err := core.ParseString(a.Ref)
		if err != nil {
			return encounter.MonsterView{}, fmt.Errorf("action %d ref %q: %w", i, a.Ref, err)
		}
		actions[i] = encounter.ActionView{Ref: *ref, Name: a.Name, RangeFeet: a.RangeFeet, Kind: a.Kind}
	}

	seen := make([]encounter.SeenMember, len(view.Seen))
	for i, sm := range view.Seen {
		inReach := make(map[core.Ref]bool, len(sm.InReach))
		for refStr, ok := range sm.InReach {
			ref, err := core.ParseString(refStr)
			if err != nil {
				return encounter.MonsterView{}, fmt.Errorf("seen %d InReach ref %q: %w", i, refStr, err)
			}
			inReach[*ref] = ok
		}
		seen[i] = encounter.SeenMember{
			ID:            encounter.MemberID(sm.ID),
			Kind:          encounter.MemberKind(sm.Kind),
			Standing:      sm.Standing,
			Position:      sm.Position,
			DistanceCells: sm.DistanceCells,
			InReach:       inReach,
			Path:          append([]spatial.Position(nil), sm.Path...),
			AwayPath:      append([]spatial.Position(nil), sm.AwayPath...),
		}
	}
	remembered := make([]encounter.RememberedMember, len(view.Remembered))
	for i, rm := range view.Remembered {
		remembered[i] = encounter.RememberedMember{
			ID:            encounter.MemberID(rm.ID),
			Kind:          encounter.MemberKind(rm.Kind),
			Position:      rm.Position,
			DistanceCells: rm.DistanceCells,
			Path:          append([]spatial.Position(nil), rm.Path...),
		}
	}

	var holdings []perception.Holding
	if len(view.Holdings) > 0 {
		holdings = make([]perception.Holding, len(view.Holdings))
		for i, h := range view.Holdings {
			holdings[i] = unprojectHolding(h)
		}
	}

	return encounter.MonsterView{
		Self:       encounter.MemberID(view.Self),
		Position:   view.Position,
		Actions:    actions,
		Targeting:  view.Targeting,
		Mind:       view.Mind,
		Holdings:   holdings,
		At:         view.At,
		Seen:       seen,
		Remembered: remembered,
		Budget: encounter.TurnBudget{
			AttacksLeft: view.Budget.AttacksLeft, MovementFeet: view.Budget.MovementFeet,
		},
		Round: view.Round,
	}, nil
}
