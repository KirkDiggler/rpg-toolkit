// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// wait.go is WAIT (rpg-project#1079): a player who will not or cannot move
// can wait, and a round passes. Ruled by the operator 2026-11-22: one act; it
// costs that character one round on the world clock and does nothing else.
//
// It is made of three pieces that already exist: the act the session opens for
// every verb, the [BeatStayed] entry, and [Encounter.spendWorldAction]. No new
// beat, act body or rule: the stayed entry already carries "a spent round that
// moved nobody", and the world's price for a verb is already one round.

// WaitInput declares one player waiting a round on the world clock.
type WaitInput struct {
	// Member is who waits: a player seated here, outside every fight.
	Member MemberID
}

// WaitOutput acknowledges the wait. What the round brought reaches everyone
// as beats: the waiter's stayed entry, the tick, and whatever the creatures
// that came due did.
type WaitOutput struct{}

// intentCauseWait is the cause a player's wait carries on its [BeatStayed].
var intentCauseWait = core.Ref{Module: "encounter", Type: "intent", ID: "wait"}

// Wait spends one round of the member's on the world clock, on purpose. It
// writes one [BeatStayed] entry naming the wait (cause `encounter:intent:wait`,
// no reason), stamped with the reading the verb started at, then pays the
// round through spendWorldAction, which wakes the creatures that come due.
//
// Errors, in order: ErrNilInput; ErrNoMember for an empty member; ErrClosed
// once the run has ended; ErrNotMember for a member not seated here, or one
// who is not a player (a creature's time is its machine's); ErrInBubble for a
// member in a fight (waiting is done in free roam).
//
// A held member is NOT refused: neither refuseCannotMove nor refuseInCustody is
// consulted, because being held or in custody is no reason to refuse standing
// still. Sneaking is kept: standing still in hiding is hiding. Two players
// waiting together pass one round, because the clock accrues by driver as the
// maximum: the second only reaches the high-water the first set. A downed
// member is refused by the session, where the sheets are, not here.
func (e *Encounter) Wait(in *WaitInput) (*WaitOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("wait: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("wait: %w", ErrNoMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("wait: %w", ErrClosed)
	}
	member, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("wait %q: %w", in.Member, ErrNotMember)
	}
	if member.Kind != KindPlayer {
		return nil, fmt.Errorf("wait %q: only a player waits: %w", in.Member, ErrNotMember)
	}
	bubble, err := e.bubbleFor(in.Member)
	if err != nil && !errors.Is(err, errNotInBubble) {
		return nil, fmt.Errorf("wait %q: %w", in.Member, err)
	}
	if bubble != nil {
		return nil, fmt.Errorf("wait %q: waiting is done in free roam: %w", in.Member, ErrInBubble)
	}

	// The entry goes first: the verb's own beats carry the round it started in.
	at := uint64(e.clock.ToData().HighWater)
	if err := e.appendStayedBeat(in.Member, intentCauseWait, "", at); err != nil {
		return nil, fmt.Errorf("wait %q: %w", in.Member, err)
	}
	if err := e.spendWorldAction(in.Member); err != nil {
		return nil, fmt.Errorf("wait %q: %w", in.Member, err)
	}

	return &WaitOutput{}, nil
}
