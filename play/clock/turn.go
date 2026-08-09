// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Turn is a localized initiative bubble (design: Turn). The zero value is
// valid and idle. Not safe for concurrent use (design R10).
type Turn struct {
	order     []core.EntityID
	activeIdx int
	round     int
}

// SetOrderInput carries the rulebook-rolled initiative order.
type SetOrderInput struct {
	Order []core.EntityID
}

// SetOrderOutput reports the milestones SetOrder caused.
type SetOrderOutput struct {
	Milestones []Milestone
}

// SetOrder replaces the order, starting round 1 with the first member
// active. Errors: ErrBadOrder (empty), ErrDuplicateMember.
func (t *Turn) SetOrder(in *SetOrderInput) (*SetOrderOutput, error) {
	if len(in.Order) == 0 {
		return nil, fmt.Errorf("set order: empty: %w", ErrBadOrder)
	}
	seen := make(map[core.EntityID]struct{}, len(in.Order))
	for _, id := range in.Order {
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("set order: %q appears twice: %w", id, ErrDuplicateMember)
		}
		seen[id] = struct{}{}
	}
	t.order = append([]core.EntityID(nil), in.Order...)
	t.activeIdx = 0
	t.round = 1
	return &SetOrderOutput{Milestones: []Milestone{
		{Kind: RoundStarted, Round: 1},
		{Kind: TurnStarted, Subject: t.order[0], Round: 1},
	}}, nil
}

// Active returns the entity whose turn it is; ErrIdle when no order is set.
func (t *Turn) Active() (core.EntityID, error) {
	if len(t.order) == 0 {
		return "", fmt.Errorf("active: %w", ErrIdle)
	}
	return t.order[t.activeIdx], nil
}

// Round returns the current round; ErrIdle when no order is set.
func (t *Turn) Round() (int, error) {
	if len(t.order) == 0 {
		return 0, fmt.Errorf("round: %w", ErrIdle)
	}
	return t.round, nil
}

// Order returns a copy of the current order. An idle clock answers with an
// empty slice and nil error — an empty list is an answer.
func (t *Turn) Order() ([]core.EntityID, error) {
	return append([]core.EntityID(nil), t.order...), nil
}

// ContainsInput names the entity being asked about.
type ContainsInput struct {
	ID core.EntityID
}

// Contains reports membership; false is an answer, never an error today.
func (t *Turn) Contains(in *ContainsInput) (bool, error) {
	return t.indexOf(in.ID) >= 0, nil
}

func (t *Turn) indexOf(id core.EntityID) int {
	for i, m := range t.order {
		if m == id {
			return i
		}
	}
	return -1
}
