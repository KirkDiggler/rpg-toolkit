// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
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

	switch it := intent.(type) {
	case encounter.Pass:
		return Pass{}, nil
	case encounter.Attack:
		return Attack{Target: string(it.Target), Action: it.Action.String()}, nil
	case encounter.Move:
		return Move{Path: it.Path}, nil
	default:
		return nil, fmt.Errorf("behavior driver %q: %w: %T", view.Self, ErrBadTurnOutcome, intent)
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
		actions[i] = encounter.ActionView{Ref: *ref, Name: a.Name, ReachFeet: a.ReachFeet, Kind: a.Kind}
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
		}
	}

	return encounter.MonsterView{
		Self:      encounter.MemberID(view.Self),
		Position:  view.Position,
		Actions:   actions,
		Targeting: view.Targeting,
		Seen:      seen,
		Budget: encounter.TurnBudget{
			AttacksLeft: view.Budget.AttacksLeft, MovementFeet: view.Budget.MovementFeet,
		},
		Round: view.Round,
	}, nil
}
