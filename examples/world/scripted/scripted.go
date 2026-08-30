// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package scripted is a dice roller that hands back a written-down sequence,
// so a whole run of a world is reproducible.
//
// It exists here rather than in the dice module by ruling: a scripted roller is
// test scaffolding, not a game mechanic, and dice should not grow one. It sits
// beside the scenarios rather than inside one because more than one scenario
// needs it and neither should have to import the other.
package scripted

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
