// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castPush is one shove a cast imposed, between the moment the contest decided
// it and the moment the board walks it.
//
// # Two questions, and the record sits between them
//
// The composition splits a directed move in half, and this type is that split
// read from up here. [encounter.Encounter.Route] is a pure computation: it says
// which cells the push WOULD cross and why it stops where it does, and it
// writes nothing. [encounter.Encounter.Direct] is the walk, and every cell of
// it is a beat.
//
// So the route is taken BEFORE [encounter.Encounter.RecordCast] — which is what
// lets the cast's own beat say the blast moved somebody one cell instead of two
// — and the walk happens AFTER it, so the movement beats follow the cast beat
// in the order a client animates them. Thunder, then the slide.
//
// # What is not reconciled
//
// The board can change between the two. A push routed through a cell that
// somebody else's push has since vacated, or arrived in, makes the walk fall
// short of its own route ([encounter.DirectOutput.StoppedBy]), and that is
// already narrated on the movement beats. It is deliberately NOT written back
// onto the cast beat: the cast beat is the cast's account of what it bought,
// the movement beats are the account of what happened, and a record that
// quietly revised the first to match the second would leave a reader unable to
// tell the two apart.
//
// The same board-moved-underneath applies to two pushes from ONE cast, because
// every route here is taken before any of them walks. Two creatures shoved
// along the same line are routed with both still standing, so the nearer one
// stops in front of the farther one even though the farther one is about to
// leave. That is the conservative answer — nothing is ever pushed THROUGH
// anybody — and it is the price of the cast beat being able to report the
// distance at all.
type castPush struct {
	// target is who is being moved.
	target encounter.MemberID

	// move is the whole of what the content said about how.
	move resolution.MoveDirective

	// targetAt and resultAt name the beat this push reports itself on:
	// targets[targetAt].Results[resultAt]. Indices rather than a pointer,
	// because the results are still being appended to when this is recorded.
	targetAt, resultAt int

	// route is what Route answered: the cells to walk, filled in before the
	// record is written and walked after it.
	route []spatial.Position
}

// routeCastPushes asks the board how far each push goes and writes the answer
// onto the result beat that will report it.
//
// IT WRITES NOTHING TO THE WORLD. Every call here is a read, which is what
// makes it safe to run before the record: a cast refused at this point has
// moved nobody.
func routeCastPushes(
	enc *encounter.Encounter, pushes []castPush, targets []encounter.CastTargetResult,
) error {
	if len(pushes) == 0 {
		return nil
	}

	// ASKED OF THE BOARD, NOT OF THE ROSTER THE VERB OPENED WITH. The
	// directive names its anchor by id precisely so the position is read at
	// the moment the push runs rather than at the moment the save resolved,
	// and reusing an earlier read would put the two moments back together.
	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("cast push: %w", translate(err))
	}
	positions := rosterPositions(roster)
	speeds := rosterSpeeds(roster)

	for i := range pushes {
		push := &pushes[i]

		policy, ok := routePolicy(push.move.Policy)
		if !ok {
			return fmt.Errorf("%w: unsupported move policy %q", ErrBadCast, push.move.Policy)
		}
		// THE BUDGET, AND THE ONE THE CONTENT COULD NOT WRITE. A push in
		// cells is a distance the spell chose; a flee is the creature's own
		// legs, and the same whisper moves a dwarf and a horse different
		// distances. The roster row is where that fact lives and this is the
		// only module that reads it, so the conversion into cells happens
		// here — a LOOKUP, not a ruling: nothing on this side decides what a
		// creature's speed is, only where to find it and how many cells five
		// feet make.
		budget := push.move.Cells
		if push.move.Speed {
			budget = encounter.CellsFromFeet(speeds[string(push.target)])
		}
		anchor, placed := positions[push.move.AnchorID]
		if !placed {
			return fmt.Errorf("%w: move anchor %q is not on the roster",
				ErrBadCast, push.move.AnchorID)
		}

		if push.targetAt >= len(targets) || push.resultAt >= len(targets[push.targetAt].Results) {
			// The indices are minted beside the results they name, so this is
			// unreachable by construction. Failing closed rather than indexing
			// blind keeps a future edit to that pairing from writing a
			// distance onto somebody else's beat.
			return fmt.Errorf("%w: a push names a result that was not recorded", ErrInvalidWorld)
		}
		result := &targets[push.targetAt].Results[push.resultAt]

		if push.move.Speed && budget <= 0 {
			// THE THIRD ZERO, AND THE ONLY LAYER THAT CAN TELL IT APART. A
			// route that hit a wall names the wall and a price nobody could
			// pay names the price; a budget read off a row that carries no
			// speed would be the one that says nothing. Asking the board is
			// worse than useless here — Route answers a zero budget with an
			// empty path and no sentence, because from down there "you were
			// given nowhere to go" and "you got nowhere" are the same walk.
			//
			// NOT A THRESHOLD, WHICH IS WHY IT IS NOT A RULE. It is the
			// fail-closed question about a looked-up fact: the row did not say
			// how fast this creature is, so nothing on this side may guess.
			// A speed under five feet reads the same way and is the same
			// sentence — less than one cell on a five-foot grid is no run.
			result.Moved = 0
			result.StoppedBy = noSpeedToRunWith
			continue
		}

		route, routeErr := enc.Route(encounter.RouteInput{
			Mover:  push.target,
			Policy: policy,
			Anchor: anchor,
			Budget: budget,
		})
		if routeErr != nil {
			return fmt.Errorf("cast push: %w", translate(routeErr))
		}
		push.route = route.Path

		result.Moved = len(route.Path)
		result.StoppedBy = route.StoppedBy
	}
	return nil
}

// noSpeedToRunWith is what a move budgeted by the mover's own speed reports
// when the roster row carried none.
//
// IT IS A REASON, NOT AN ERROR. The cast happened, the save was failed, the
// damage landed and whatever the move was priced at was paid — the creature
// simply has no legs the record knows about. Phrased the way resolution
// phrases its own untaken move, because both end up in the same field of the
// same beat and a reader should not be able to tell which layer wrote them.
const noSpeedToRunWith = "has no speed to run with"

// walkCastPushes walks each routed push, so the movement beats and their cause
// land after the cast beat that reported them. It reports whether a walk STOPPED
// TO ASK somebody about a cell, which is a checkpoint rather than a failure.
//
// The cells are the ones routeCastPushes priced. They are supplied rather than
// recomputed for [encounter.DirectInput.Route]'s own reason: the caller that
// decided how far the push goes is the caller that decides how much of it
// happens.
//
// # A pause ends this loop, and today that costs nothing
//
// The composition refuses a second directive while one is held, so there is no
// honest way to walk a later push past a pause: the hold has to be answered
// before anything else moves. Every cast that can pause has exactly one target
// — a flee provokes and a shove does not, and only a flee reaches a player —
// so the loop stopping here is the whole of what can happen rather than a case
// being dropped.
//
// The return is also what keeps a later push away from a refusal it would
// deserve: the composition rejects a second directive while anything is held,
// so continuing the loop would turn the pause into a cast push error rather
// than a dropped push.
//
// The day a cast shoves two creatures AND provokes, the remaining pushes need
// somewhere to wait while the first one's question stands. That is a second
// hold, which is the composition's to build; a queue up here would be this seam
// deciding what order a board walks in.
func walkCastPushes(
	ctx context.Context, enc *encounter.Encounter, pushes []castPush, cause core.Ref,
) (bool, error) {
	for _, push := range pushes {
		if len(push.route) == 0 {
			// A creature with a wall at its back is pushed nowhere. That is an
			// ordinary outcome, already on the cast beat as a distance of
			// zero, and there is no walk to take.
			continue
		}
		walked, err := enc.Direct(ctx, encounter.DirectInput{
			Mover:    push.target,
			Cause:    cause,
			Route:    push.route,
			Provokes: push.move.Provokes,
		})
		if err != nil {
			return false, fmt.Errorf("cast push: %w", translate(err))
		}
		if walked.Paused {
			return true, nil
		}
	}
	return false, nil
}

// routePolicy maps the content's word for how a creature is moved onto the
// composition's.
//
// TWO CLOSED SETS WITH TWO MEMBERS EACH, and the crossing is spelled out rather
// than converted by a cast between two string types. They are the same words
// today and they are not the same vocabulary: content says what a spell does
// and the composition says what it can walk, and the day one of them learns a
// word the other has not, this function is where a builder is stopped and asked
// which it meant.
func routePolicy(policy combatActions.MovePolicy) (encounter.MovePolicy, bool) {
	switch policy {
	case combatActions.MoveLine:
		return encounter.MoveLine, true
	case combatActions.MoveAway:
		return encounter.MoveAway, true
	default:
		return "", false
	}
}
