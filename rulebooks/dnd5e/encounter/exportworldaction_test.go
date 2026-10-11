// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

// ExportedSpendWorldAction pays the world clock's price for one action by
// actor, for the external test package: one round for the actor, and the world
// thinks on it.
//
// It exists because the scenes that need "and then time passed" used to get it
// from the explicit Search verb, which was universally attemptable, needed no
// target and no die. That verb is gone; the price it charged is not, and every
// remaining action verb that charges it also writes beats or needs a target a
// time-passing fixture should not have to invent.
//
// A TEST-ONLY DOOR, in a _test.go file, for the reason
// [ExportedPlacedCentre] gives. It refuses what an action verb refuses before
// it spends: a closed encounter and a stranger.
func ExportedSpendWorldAction(e *Encounter, actor MemberID) error {
	if e.outcome != nil {
		return fmt.Errorf("spend world action: %w", ErrClosed)
	}
	if _, ok := e.members[actor]; !ok {
		return fmt.Errorf("spend world action: %w", ErrNotMember)
	}
	return e.spendWorldAction(actor)
}

// ExportedFind makes member learn the named concealments the way a beaten
// check does, for the external test package: one reveal per concealment not
// already known, in sorted-ID order, then the sweep every sight refresh runs,
// then the world clock's price for an action.
//
// It exists for the scenes that are about what FINDING a secret changes — the
// reveal beat, the atlas, the door lists, who is told — and not about how the
// find was earned. The explicit Search verb used to give them a find on
// demand; automatic discovery earns it by walking within a hex, which would
// bind every one of those scenes to a member's distance and the attempts
// allowance. The reveal itself still goes through the one production writer
// ([Encounter.revealConcealmentTo]), so nothing here asserts on a copy of it.
//
// A TEST-ONLY DOOR, in a _test.go file, for the reason [ExportedPlacedCentre]
// gives. An unknown concealment id is refused, so a typo in a scene fails
// instead of finding nothing.
func ExportedFind(e *Encounter, member MemberID, ids ...ConcealmentID) error {
	if e.outcome != nil {
		return fmt.Errorf("find: %w", ErrClosed)
	}
	if _, ok := e.members[member]; !ok {
		return fmt.Errorf("find: %w", ErrNotMember)
	}
	wanted := make(map[ConcealmentID]bool, len(ids))
	for _, id := range ids {
		if e.field.concealmentOf(id) == nil {
			return fmt.Errorf("find: %w: %q", ErrNoConcealment, id)
		}
		wanted[id] = true
	}
	at := uint64(e.clock.ToData().HighWater)
	for _, id := range e.world.concealments {
		if !wanted[id] || e.world.knowsConcealment(member, id) {
			continue
		}
		if err := e.revealConcealmentTo(member, e.field.concealmentOf(id), "automatic discovery", at); err != nil {
			return err
		}
	}
	if err := e.sweepConcealment(); err != nil {
		return err
	}
	return e.spendWorldAction(member)
}
