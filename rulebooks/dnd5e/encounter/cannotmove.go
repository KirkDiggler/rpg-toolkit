// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"errors"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// ErrCannotMove is returned when a member whose sheet says it cannot move
// under its own power ([SheetFacts.CannotMove]) tries to: a player's
// [Encounter.Step] (a dash included) or a cast whose landing moves its own
// caster (rpg-project#990). It arrives as a [CannotMoveError], whose text is
// the sentence the player is shown and whose HeldBy names what holds them.
//
// A move made ON the member — a push, a pull, [Encounter.Direct],
// [Encounter.Relocate] — is never refused by it. A driven member's walk is
// not refused with it either: that walk simply does not happen
// ([MonsterView.CannotMove]).
var ErrCannotMove = errors.New("encounter: cannot move under its own power")

// CannotMoveError is the refusal of a held member's own move. It is
// [ErrCannotMove] to errors.Is.
type CannotMoveError struct {
	// Member is who tried to move.
	Member MemberID
	// HeldBy is what holds them, as their sheet named it, in its order.
	HeldBy []string
}

// Error is the sentence the player is shown: "You are Held; you cannot move."
func (e *CannotMoveError) Error() string {
	return "You are " + listNames(e.HeldBy) + "; you cannot move."
}

// Is reports whether target is [ErrCannotMove].
func (e *CannotMoveError) Is(target error) bool { return target == ErrCannotMove }

// listNames joins names as a sentence does: "A", "A and B", "A, B and C".
func listNames(names []string) string {
	switch len(names) {
	case 0:
		return "held"
	case 1:
		return names[0]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}

// refuseCannotMove is the hold door a member's own-will move asks, after the
// custody door ([Encounter.refuseInCustody]): a member both in custody and
// held is refused by custody, always. It asks the sheet now (a pull, never
// remembered) and refuses with a [CannotMoveError] when the sheet names a
// hold.
func (e *Encounter) refuseCannotMove(verb string, id MemberID) error {
	facts, err := e.sheetOf(id)
	if err != nil {
		return fmt.Errorf("%s %q: %w", verb, id, err)
	}
	if len(facts.CannotMove) == 0 {
		return nil
	}
	return fmt.Errorf("%s %q: %w", verb, id, &CannotMoveError{Member: id, HeldBy: facts.CannotMove})
}

// refuseHeldCaster refuses a cast whose landing would move its own caster
// while the caster's sheet says it cannot move under its own power
// (rpg-project#990), before anything of the cast lands. A push on anybody
// else — a held member included — is not the pushed member's own move and
// is not refused. Asked after custody ([Encounter.holdTheHeldInPlace]). A nil
// input passes through for [Encounter.prepareCast] to refuse.
func (e *Encounter) refuseHeldCaster(in *RecordCastInput) error {
	if in == nil {
		return nil
	}
	for _, p := range in.Pushes {
		if p.Mover == in.Actor && len(p.Route) > 0 {
			return e.refuseCannotMove("record cast", in.Actor)
		}
	}
	return nil
}

// intentCauseMove is the cause a held member's refused own [Move] carries on
// its [BeatStayed]: the move word itself, whoever's driver said it — the
// attack refusal's shape ([intentCauseAttack]).
var intentCauseMove = core.Ref{Module: "encounter", Type: "intent", ID: "move"}

// stayHeld is a driven member's walk, willing or compelled, as a hold answers it
// (rpg-project#990): the walk the driver picked does not happen, nothing is
// charged, a [BeatStayed] names the cause and "cannot move: <what holds it>",
// and the member's turn (or its share of the world's round) ends. It is not
// an error: the round goes on for everybody else.
func (e *Encounter) stayHeld(member MemberID, cause core.Ref, heldBy []string) (bool, map[MemberID]*IntelDelta, error) {
	why := "cannot move: " + listNames(heldBy)
	if err := e.appendStayedBeat(member, cause, why, uint64(e.clock.ToData().HighWater)); err != nil {
		return false, nil, err
	}
	return true, nil, nil
}
