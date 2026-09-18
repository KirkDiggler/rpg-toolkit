// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// tableCauseTable is the cause every beat of a table-driven walk carries, so
// an observer can tell a creature obeying its own orders from a creature
// being shoved, commanded or routed by a spell.
//
// THE TABLE IS THE CAUSE, and naming it is the point. [Routed]'s own doc says
// "a Move is the member's own decision; a Routed is somebody else's" — which
// was true when the only Routed was Command's. A table's `away` is the
// creature's own decision carried out by the engine's router, so the cause
// names the policy rather than a spell. A walk narrated with no cause is an
// observer being told a creature walked away for no reason, which is exactly
// what [DirectInput.Cause] is required to prevent.
var tableCauseTable = core.Ref{Module: "encounter", Type: "table", ID: "away"}

// TableDriver is THE ONE DRIVER: it rolls the creature's own authored table
// under [AnswerTime] and turns the entry that fired into an intent
// (rpg-project#465, design §7).
//
// # It decides nothing
//
// Every choice this makes was written by an author and loaded by a
// temperament. What this type owns is the translation — a word and a selector
// into a [TurnIntent] — and the selector resolution, which is done against
// the view and nothing else (C2): the nearest opposed sighting, the actor of
// a deed this creature holds, an authored cell. It never reaches for the
// encounter, which is what makes it testable with a fixture view and no world
// at all.
//
// # The pick travels back
//
// [Decision.Pick] carries the whole arithmetic so the encounter can append the
// answer beat. The die is the world's shared dice, and the creature named on
// that beat is whose it is.
//
// # What it is NOT asked
//
// A compelled turn (Command, Dissonant Whispers) does not pass through here:
// those keep their own engine-driven [Routed] path and their own driver. A
// player's turn never reaches a driver at all.
type TableDriver struct {
	// Roller is the world's shared dice. Required: a driver that cannot
	// roll cannot pick, and a local rand would make every table unreplayable
	// (R7).
	Roller dice.Roller
}

// Act rolls this member's `time` table and turns the entry that fired into an
// intent.
//
// A CONTEXT OF ITS OWN, for [Encounter.executeTurnIntent]'s stated reason: no
// verb on this composition accepts one, and plumbing one through every verb
// between a host's request and here is a bigger change than this slice asks
// for.
//
// AN ENTRY THAT SELECTED NOBODY STILL ROLLED. `attack: attacker` on a creature
// nobody has attacked, `toward: enemy` with nothing opposed in sight or
// memory: the entry was on the table, the die chose it, and the answer is
// [Pass] with the pick attached — so the beat shows what the creature tried to
// do and a reader can see why nothing happened. Silently re-rolling would hide
// an authoring mistake behind plausible behaviour.
func (d TableDriver) Act(view MonsterView) (Decision, error) {
	chosen, err := behavior.Pick(context.Background(), &behavior.PickInput{
		Key: AnswerTime, Table: view.Table, Temper: view.Temper, Facts: factsFromView(view), Die: d.Roller,
	})
	if err != nil {
		return Decision{}, fmt.Errorf("table driver %q: %w", view.Self, err)
	}
	// pick never answers nil for AnswerTime — an empty table is a hold — but
	// a nil here would be a nil-pointer panic inside a driver, which is the
	// one failure mode this seam turns into an aborted verb rather than a
	// crash.
	if chosen == nil {
		return Decision{}, fmt.Errorf("table driver %q: %w", view.Self, ErrBadAnswer)
	}

	intent, err := d.intentFor(view, chosen.Answer)
	if err != nil {
		return Decision{}, err
	}

	return Decision{Intent: intent, Pick: chosen}, nil
}

// intentFor translates one fired entry into the intent that carries it out.
func (d TableDriver) intentFor(view MonsterView, entry Answer) (TurnIntent, error) {
	switch {
	case entry.Hold:
		return Pass{}, nil

	case entry.Attack != nil:
		return d.attackIntent(view, entry), nil

	case entry.Toward != nil:
		return d.towardIntent(view, entry), nil

	case entry.Away != nil:
		return d.awayIntent(view, entry), nil

	default:
		// `fact`, `flee` and a bare line are social words, refused under
		// `time` by [validateTable] at every door a table comes in through.
		// Reaching here means a blob got past that, and the honest answer is
		// a named error rather than a creature standing there for reasons
		// nobody could find.
		return nil, fmt.Errorf("table driver %q: %q under `time`: %w",
			view.Self, answerWord(entry), ErrBadAnswer)
	}
}

// attackIntent strikes the selected member with the first action it is in
// reach of, in the author's own order.
//
// THE ORDER IS THE INSTRUCTION ([dungeonspec.MonsterPlacement.Actions]): the
// first action whose target is in reach is the one taken, so sorting the list
// would quietly rewrite what the author said the monster does.
//
// NOBODY IN REACH IS A PASS, not a walk. The table has `toward` for closing
// and this entry said `attack`; turning one word into the other would be this
// driver making a decision the author did not write.
func (d TableDriver) attackIntent(view MonsterView, entry Answer) TurnIntent {
	if view.Budget.AttacksLeft <= 0 {
		return Pass{}
	}
	target, ok := d.selectMember(view, entry, *entry.Attack)
	if !ok {
		return Pass{}
	}
	for _, s := range view.Seen {
		if s.ID != target || !s.Standing {
			continue
		}
		for _, a := range view.Actions {
			if s.InReach[a.Ref] {
				return Attack{Target: target, Action: a.Ref}
			}
		}
	}

	return Pass{}
}

// towardIntent walks toward the selected member's BELIEVED position, or the
// authored cell, for this turn's movement.
//
// TWO SHAPES, AND THE DIFFERENCE IS WHO KNOWS THE PATH. A member the view can
// see or remember arrives with a route already computed against this
// composition's own walls and doors ([SeenMember.Path],
// [RememberedMember.Path]), truncated here to what the budget pays for — a
// [Move], so the creature may still swing when it arrives. An authored cell
// has no such route on the view, so it goes out as a [Routed] the encounter
// paths itself, which is TERMINAL: a creature walking to a cell somebody
// wrote on the map has nothing else it was told to do there.
func (d TableDriver) towardIntent(view MonsterView, entry Answer) TurnIntent {
	cells := CellsFromFeet(view.Budget.MovementFeet)
	if cells <= 0 {
		return Pass{}
	}

	if at := entry.Toward.At; at != nil {
		cell := *at

		return Routed{Policy: MoveToward, AnchorAt: &cell, Cause: tableCauseTable}
	}

	target, ok := d.selectMember(view, entry, *entry.Toward)
	if !ok {
		return Pass{}
	}
	path := d.pathTo(view, target)
	if len(path) == 0 {
		return Pass{}
	}
	if len(path) > cells {
		path = path[:cells]
	}

	return Move{Path: append([]spatial.Position(nil), path...)}
}

// awayIntent routes away from the selected member for this turn's movement —
// the coward's run.
//
// [Routed] RATHER THAN A PATH, because running away is a policy and not a
// route: the board is what knows where the corners are, and a driver picking
// its own cells would flee into one. Terminal, which is what running is.
func (d TableDriver) awayIntent(view MonsterView, entry Answer) TurnIntent {
	if CellsFromFeet(view.Budget.MovementFeet) <= 0 {
		return Pass{}
	}
	target, ok := d.selectMember(view, entry, *entry.Away)
	if !ok {
		return Pass{}
	}

	return Routed{Policy: MoveAway, Anchor: target, Cause: tableCauseTable}
}

// pathTo is the route the view already computed toward one member — current
// sight first, then memory.
//
// SIGHT BEFORE MEMORY, because a creature that can see where somebody IS does
// not walk to where they WERE. The view is rebuilt after every executed
// intent, so a remembered pursuit is interrupted by the first glimpse.
func (d TableDriver) pathTo(view MonsterView, target MemberID) []spatial.Position {
	for _, s := range view.Seen {
		if s.ID == target {
			return s.Path
		}
	}
	for _, r := range view.Remembered {
		if r.ID == target {
			return r.Path
		}
	}

	return nil
}

// selectMember resolves a selector word against the view.
//
//   - `enemy`: the nearest opposed member in sight, else the nearest opposed
//     member remembered. Opposition is the stance graph's answer, projected
//     onto the view — "a mind is never handed a target it is not opposed to",
//     which is what keeps a neutral creature from advancing on the party.
//   - `attacker`: who last landed an attack on this creature, off its own
//     held deeds.
//   - `actor`: the actor of the deed this entry's `when` named, so `fled`
//     pairs with `away: actor`.
//
// ok=false is a selector that named nobody, which the caller turns into a
// [Pass] with the pick still attached.
func (d TableDriver) selectMember(view MonsterView, entry Answer, sel Selector) (MemberID, bool) {
	switch sel.Word {
	case SelectorEnemy:
		return nearestOpposed(view)

	case SelectorAttacker:
		if held, ok := freshestDeed(view.Deeds, DeedAttack); ok && held.Actor != "" {
			return held.Actor, true
		}

		return "", false

	case SelectorActor:
		if entry.When == nil || entry.When.Deed == "" {
			return "", false
		}
		if held, ok := freshestDeed(view.Deeds, DeedVerbFor(entry.When.Deed)); ok && held.Actor != "" {
			return held.Actor, true
		}

		return "", false

	default:
		return "", false
	}
}

// nearestOpposed is the nearest opposed member the view holds: sighted first,
// then remembered. Ties break on the member's id so the same view always
// selects the same target (C8).
func nearestOpposed(view MonsterView) (MemberID, bool) {
	best := MemberID("")
	bestDist := 0.0
	for _, s := range view.Seen {
		if !s.Opposed || !s.Standing {
			continue
		}
		if best == "" || s.DistanceCells < bestDist {
			best, bestDist = s.ID, s.DistanceCells
		}
	}
	if best != "" {
		return best, true
	}
	for _, r := range view.Remembered {
		if !r.Opposed {
			continue
		}
		if best == "" || r.DistanceCells < bestDist {
			best, bestDist = r.ID, r.DistanceCells
		}
	}

	return best, best != ""
}
