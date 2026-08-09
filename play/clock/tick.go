// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Tick is the player-driven world clock (design: Tick): it advances only
// because players act. Accrual is high-water max-displacement. Construct
// via NewTick or LoadTick; the zero value is not usable. Not safe for
// concurrent use (design R10).
type Tick struct {
	budgets        map[core.EntityID]int
	driverProgress map[core.EntityID]int
	highWater      int //nolint:unused // design: reserved for Task 9 (Advance)
}

// NewTick constructs a valid, idle world clock. The error return conforms
// to design R3(a); construction cannot fail today, but a future
// TickConfig can.
func NewTick() (*Tick, error) {
	return &Tick{
		budgets:        make(map[core.EntityID]int),
		driverProgress: make(map[core.EntityID]int),
	}, nil
}

// JoinInput names the entity joining the world clock.
type JoinInput struct {
	ID core.EntityID
}

// JoinOutput reports the milestones Join caused.
type JoinOutput struct {
	Milestones []Milestone
}

// Join adds a member at budget 0. Errors: ErrDuplicateMember.
func (k *Tick) Join(in *JoinInput) (*JoinOutput, error) {
	if _, ok := k.budgets[in.ID]; ok {
		return nil, fmt.Errorf("join %q: %w", in.ID, ErrDuplicateMember)
	}
	k.budgets[in.ID] = 0
	return &JoinOutput{Milestones: []Milestone{{Kind: MemberJoined, Subject: in.ID}}}, nil
}

// LeaveInput names the entity leaving the world clock.
type LeaveInput struct {
	ID core.EntityID
}

// LeaveOutput reports the milestones Leave caused.
type LeaveOutput struct {
	Milestones []Milestone
}

// Leave removes a member and its budget. Errors: ErrNotMember.
func (k *Tick) Leave(in *LeaveInput) (*LeaveOutput, error) {
	if _, ok := k.budgets[in.ID]; !ok {
		return nil, fmt.Errorf("leave %q: %w", in.ID, ErrNotMember)
	}
	delete(k.budgets, in.ID)
	return &LeaveOutput{Milestones: []Milestone{{Kind: MemberLeft, Subject: in.ID}}}, nil
}

// BudgetInput names the member whose budget is being read.
type BudgetInput struct {
	ID core.EntityID
}

// Budget returns the member's current budget. Errors: ErrNotMember —
// never an ambiguous zero.
func (k *Tick) Budget(in *BudgetInput) (int, error) {
	b, ok := k.budgets[in.ID]
	if !ok {
		return 0, fmt.Errorf("budget %q: %w", in.ID, ErrNotMember)
	}
	return b, nil
}

// Members returns the member set in stable (sorted) order. An empty clock
// answers with an empty slice and nil error.
func (k *Tick) Members() ([]core.EntityID, error) {
	out := make([]core.EntityID, 0, len(k.budgets))
	for id := range k.budgets {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// Contains reports membership; false is an answer.
func (k *Tick) Contains(in *ContainsInput) (bool, error) {
	_, ok := k.budgets[in.ID]
	return ok, nil
}
