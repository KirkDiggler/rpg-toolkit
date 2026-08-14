// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
)

// ClockKind names which kind of clock a member is on.
//
// It is an enumerated kind rather than a bool because "is this member in
// combat" is the question a caller will be tempted to ask, and it is the wrong
// one: a third clock kind is expressible later, and a bool would have to be
// re-litigated the moment one arrives.
type ClockKind string

const (
	// ClockWorld is the player-driven world clock — every member's home, and
	// where a member is when no fight has pulled them elsewhere. It is not a
	// "free roam mode"; there is no mode.
	ClockWorld ClockKind = "world"

	// ClockTurn is a localized initiative bubble: an ordered clock that
	// advances a turn at a time and holds only the members caught up in one
	// fight. Members elsewhere in the encounter keep running on ClockWorld
	// while it does.
	ClockTurn ClockKind = "turn"
)

// ClockOfInput names the member whose clock is being read.
type ClockOfInput struct {
	// Member is who to look up.
	Member MemberID
}

// ClockOfOutput reports which clock a member is on, and — when that clock is
// ordered — where in it they stand.
//
// There is deliberately no bubble identifier here. A bubble is never addressed
// by name; it is reached through a member, which R6 ("an entity belongs to at
// most one clock") makes a total function.
type ClockOfOutput struct {
	// Kind is which clock the member is on.
	Kind ClockKind

	// Active is whose turn it currently is on that clock. Empty for
	// ClockWorld, which has no turn order — on the world clock everyone acts,
	// and their own movement is what advances it.
	Active MemberID

	// Round is the current round of that clock, or 0 for ClockWorld.
	Round int

	// Order is the full initiative order of that clock, or nil for ClockWorld.
	// Returned as a copy; mutating it does not affect the encounter.
	Order []MemberID
}

// ClockOf reports which clock a member is on.
//
// This is the query the composition is expected to be asked constantly, and it
// is deliberately member-first rather than clock-first: "whose turn is it" is
// not a question an encounter can answer, because several clocks may be running
// and each has its own answer. Asking it of the encounter would force a single
// privileged clock to exist, which is exactly the mode model this stack does
// not have.
//
// Errors: ErrNilInput, ErrNotMember (the member is not in this encounter).
func (e *Encounter) ClockOf(in *ClockOfInput) (*ClockOfOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("clock_of: %w", ErrNilInput)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("clock_of %q: %w", in.Member, ErrNotMember)
	}

	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, fmt.Errorf("clock_of %q: %w", in.Member, err)
	}
	if bubble == nil {
		return &ClockOfOutput{Kind: ClockWorld}, nil
	}

	active, err := bubble.Active()
	if err != nil {
		return nil, fmt.Errorf("clock_of %q active: %w", in.Member, err)
	}
	round, err := bubble.Round()
	if err != nil {
		return nil, fmt.Errorf("clock_of %q round: %w", in.Member, err)
	}
	order, err := bubble.Order()
	if err != nil {
		return nil, fmt.Errorf("clock_of %q order: %w", in.Member, err)
	}

	return &ClockOfOutput{
		Kind:   ClockTurn,
		Active: MemberID(active),
		Round:  round,
		Order:  toMemberIDs(order),
	}, nil
}

// bubbleFor returns the bubble holding this member, or nil if they are on the
// world clock. Never returns two: R6 is validated on load and upheld by every
// verb that moves a member between clocks.
func (e *Encounter) bubbleFor(id MemberID) (*clock.Turn, error) {
	for _, b := range e.bubbles {
		in, err := b.Contains(&clock.ContainsInput{ID: core.EntityID(id)})
		if err != nil {
			return nil, err
		}
		if in {
			return b, nil
		}
	}
	return nil, nil
}

// leaveAnyClock removes a member from whichever clock holds them.
//
// Callers that are removing a member from the encounter entirely (Exit) must
// use this rather than leaving the world clock directly: a member caught in a
// bubble is NOT on the world clock, so a bare tick Leave would fail for exactly
// the members most likely to be leaving — the ones in a fight.
func (e *Encounter) leaveAnyClock(id MemberID) error {
	bubble, err := e.bubbleFor(id)
	if err != nil {
		return err
	}
	if bubble != nil {
		_, rerr := bubble.Remove(&clock.RemoveInput{ID: core.EntityID(id)})
		return rerr
	}
	_, lerr := e.clock.Leave(&clock.LeaveInput{ID: core.EntityID(id)})
	return lerr
}

// toMemberIDs converts a clock's entity order into member IDs, copying so the
// caller cannot reach the clock's own slice.
func toMemberIDs(ids []core.EntityID) []MemberID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]MemberID, 0, len(ids))
	for _, id := range ids {
		out = append(out, MemberID(id))
	}
	return out
}
