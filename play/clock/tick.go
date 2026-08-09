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
	highWater      int
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

// AdvanceInput reports a driver's displacement since their last report.
// Units are the caller's; the clock never interprets them.
type AdvanceInput struct {
	Driver       core.EntityID
	Displacement int
}

// AdvanceOutput reports the tick and which members now have budget.
type AdvanceOutput struct {
	Milestones []Milestone
	// Ready is a snapshot — members with budget > 0 at this instant — not a
	// became-ready event; a member with unspent budget reappears on every Advance.
	Ready []core.EntityID
}

// Advance records the driver's cumulative displacement; when it raises the
// high-water mark, the delta is granted to every member's budget
// (max-not-sum fairness — design: Tick). Drivers need not be members.
// Errors: ErrBadAmount (negative displacement).
func (k *Tick) Advance(in *AdvanceInput) (*AdvanceOutput, error) {
	if in.Displacement < 0 {
		return nil, fmt.Errorf("advance %q by %d: %w", in.Driver, in.Displacement, ErrBadAmount)
	}
	k.driverProgress[in.Driver] += in.Displacement
	if p := k.driverProgress[in.Driver]; p > k.highWater {
		delta := p - k.highWater
		k.highWater = p
		for id := range k.budgets {
			k.budgets[id] += delta
		}
	}
	ready := make([]core.EntityID, 0, len(k.budgets))
	for id, b := range k.budgets {
		if b > 0 {
			ready = append(ready, id)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i] < ready[j] })
	return &AdvanceOutput{
		Milestones: []Milestone{{Kind: Ticked, Subject: in.Driver}},
		Ready:      ready,
	}, nil
}

// SpendInput deducts from a member's budget.
type SpendInput struct {
	ID     core.EntityID
	Amount int
}

// SpendOutput reports the milestones Spend caused (none in the v1 kind set).
type SpendOutput struct {
	Milestones []Milestone
}

// Spend deducts Amount from the member's budget. Errors: ErrNotMember,
// ErrBadAmount (non-positive), ErrInsufficientBudget.
func (k *Tick) Spend(in *SpendInput) (*SpendOutput, error) {
	b, ok := k.budgets[in.ID]
	if !ok {
		return nil, fmt.Errorf("spend %q: %w", in.ID, ErrNotMember)
	}
	if in.Amount <= 0 {
		return nil, fmt.Errorf("spend %q amount %d: %w", in.ID, in.Amount, ErrBadAmount)
	}
	if in.Amount > b {
		return nil, fmt.Errorf("spend %q amount %d exceeds budget %d: %w", in.ID, in.Amount, b, ErrInsufficientBudget)
	}
	k.budgets[in.ID] = b - in.Amount
	return &SpendOutput{Milestones: nil}, nil
}

// TickData is Tick's persisted shape (design R8). Plain data, no behavior.
type TickData struct {
	Budgets        map[core.EntityID]int `json:"budgets,omitempty"`
	DriverProgress map[core.EntityID]int `json:"driver_progress,omitempty"`
	HighWater      int                   `json:"high_water,omitempty"`
}

// ToData snapshots the clock. Family-convention exemption from R3 (design).
func (k *Tick) ToData() TickData {
	return TickData{
		Budgets:        copyIntMap(k.budgets),
		DriverProgress: copyIntMap(k.driverProgress),
		HighWater:      k.highWater,
	}
}

// LoadTick reconstructs a Tick from persisted state. A constructor, not a
// verb — no milestones. Errors: ErrInvalidData for every R9 rejection.
func LoadTick(data TickData) (*Tick, error) {
	maxProgress := 0
	for id, p := range data.DriverProgress {
		if p < 0 {
			return nil, fmt.Errorf("load tick: driver %q progress %d: %w", id, p, ErrInvalidData)
		}
		if p > maxProgress {
			maxProgress = p
		}
	}
	if data.HighWater < maxProgress {
		return nil, fmt.Errorf("load tick: high water %d below max driver progress %d: %w",
			data.HighWater, maxProgress, ErrInvalidData)
	}
	for id, b := range data.Budgets {
		if b < 0 {
			return nil, fmt.Errorf("load tick: member %q budget %d: %w", id, b, ErrInvalidData)
		}
	}
	k := &Tick{
		budgets:        copyIntMap(data.Budgets),
		driverProgress: copyIntMap(data.DriverProgress),
		highWater:      data.HighWater,
	}
	if k.budgets == nil {
		k.budgets = make(map[core.EntityID]int)
	}
	if k.driverProgress == nil {
		k.driverProgress = make(map[core.EntityID]int)
	}
	return k, nil
}

func copyIntMap(m map[core.EntityID]int) map[core.EntityID]int {
	if m == nil {
		return nil
	}
	out := make(map[core.EntityID]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
