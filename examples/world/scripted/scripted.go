// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package scripted holds the deterministic stand-ins that make a whole run of a
// world reproducible: dice that hand back a written-down sequence, a clock
// that only moves when you move it, and a witness that answers a written-down
// cast of onlookers.
//
// It exists here rather than in the dice module by ruling: a scripted roller is
// test scaffolding, not a game mechanic, and dice should not grow one. It sits
// beside the scenarios rather than inside one because more than one of them
// needs it and none should have to import another.
//
// The clock is here for the same reason and one more: a goal's clock is an
// injected capability, so the only way to test a deadline is to hold time
// still and move it by hand.
//
// The witness joined for the same reason again (rpg-project
// ideas/living-world/brainstorm.md §22): world.Witness is an injected
// capability, never defaulted, so the only way to test who saw a
// bystander-inclusive act is to hand the world a truth to consult.
package scripted

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// ErrOutOfRolls reports a roller asked for more dice than it was given.
var ErrOutOfRolls = errors.New("scripted: out of rolls")

// Roller is a dice.Roller that yields a written-down sequence.
//
// The same script produces the same facts, the same folds, and the same ending,
// every time. Running out is an error rather than a wrap-around, so a test that
// quietly started rolling more dice than it meant to finds out.
type Roller struct {
	mu    sync.Mutex
	rolls []int
	next  int
}

// NewRoller returns a roller that yields the given results in order.
func NewRoller(rolls ...int) *Roller {
	return &Roller{rolls: append([]int(nil), rolls...)}
}

// Roll returns the next scripted result, ignoring the die size.
//
// Returns [ErrOutOfRolls] once the script is spent.
func (r *Roller) Roll(_ context.Context, size int) (int, error) {
	if size <= 0 {
		return 0, fmt.Errorf("scripted: invalid die size %d", size)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.next >= len(r.rolls) {
		return 0, ErrOutOfRolls
	}
	roll := r.rolls[r.next]
	r.next++

	return roll, nil
}

// RollN returns the next count scripted results.
func (r *Roller) RollN(ctx context.Context, count, size int) ([]int, error) {
	if count < 0 {
		return nil, fmt.Errorf("scripted: invalid die count %d", count)
	}

	out := make([]int, 0, count)
	for range count {
		roll, err := r.Roll(ctx, size)
		if err != nil {
			return nil, err
		}
		out = append(out, roll)
	}

	return out, nil
}

// Remaining reports how much script is left.
func (r *Roller) Remaining() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.rolls) - r.next
}

// Clock is a goal.Clock that reports whatever moment it was last told to.
//
// Time does not pass here; it is set. A deadline test that waited for a real
// second to elapse would be testing the machine it runs on.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// NewClock returns a clock stopped at the given moment.
func NewClock(now time.Time) *Clock {
	return &Clock{now: now}
}

// Now reports the moment the clock is stopped at.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

// Set moves the clock to a moment.
func (c *Clock) Set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = now
}

// Advance moves the clock forward by a duration. Negative durations move it
// back, which is a thing tests are allowed to do and worlds are not.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(d)
}

// Witness is a world.Witness that answers the same declared cast of
// onlookers to every question it is asked, regardless of actor, target, or
// mode.
//
// Unlike Roller there is nothing here to consume: who is standing nearby
// does not change moment to moment the way a die roll does, so one script
// answers every bystander-inclusive act for as long as the world it was
// built for is in play.
type Witness struct {
	bystanders []journal.EntityID
}

// NewWitness returns a witness that always answers with the given ids.
func NewWitness(bystanders ...journal.EntityID) *Witness {
	return &Witness{bystanders: append([]journal.EntityID(nil), bystanders...)}
}

// Bystanders returns the declared cast, unconditionally. The actor, target,
// and mode it is asked about are the kernel's business, not this script's.
func (w *Witness) Bystanders(
	_ context.Context, _, _ journal.EntityID, _ world.Witnessing,
) ([]journal.EntityID, error) {
	return append([]journal.EntityID(nil), w.bystanders...), nil
}
