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
		return nil, fmt.Errorf("set order: order is empty: %w", ErrBadOrder)
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

// EndInput names the actor ending their turn.
type EndInput struct {
	Actor core.EntityID
}

// EndOutput reports what End caused and who acts next.
type EndOutput struct {
	Milestones   []Milestone
	Next         core.EntityID
	RoundWrapped bool
}

// End advances past Actor's turn. Errors: ErrIdle, ErrNotActive (with no
// state change — R5).
func (t *Turn) End(in *EndInput) (*EndOutput, error) {
	if len(t.order) == 0 {
		return nil, fmt.Errorf("end turn: %w", ErrIdle)
	}
	active := t.order[t.activeIdx]
	if in.Actor != active {
		return nil, fmt.Errorf("end turn: %q is not the active entity (%q is): %w", in.Actor, active, ErrNotActive)
	}
	ms := []Milestone{{Kind: TurnEnded, Subject: active, Round: t.round}}
	t.activeIdx++
	wrapped := false
	if t.activeIdx >= len(t.order) {
		t.activeIdx = 0
		t.round++
		wrapped = true
		ms = append(ms, Milestone{Kind: RoundStarted, Round: t.round})
	}
	next := t.order[t.activeIdx]
	ms = append(ms, Milestone{Kind: TurnStarted, Subject: next, Round: t.round})
	return &EndOutput{Milestones: ms, Next: next, RoundWrapped: wrapped}, nil
}

// InsertInput places a fall-in or reinforcement at a caller-chosen position.
type InsertInput struct {
	ID  core.EntityID
	Pos int
}

// InsertOutput reports the milestones Insert caused.
type InsertOutput struct {
	Milestones []Milestone
}

// Insert adds a member at Pos. Errors: ErrIdle (bubbles start via
// SetOrder), ErrDuplicateMember, ErrBadPosition. Inserting at or before
// the active position keeps the currently active entity active.
func (t *Turn) Insert(in *InsertInput) (*InsertOutput, error) {
	if len(t.order) == 0 {
		return nil, fmt.Errorf("insert %q: %w", in.ID, ErrIdle)
	}
	if t.indexOf(in.ID) >= 0 {
		return nil, fmt.Errorf("insert %q: %w", in.ID, ErrDuplicateMember)
	}
	if in.Pos < 0 || in.Pos > len(t.order) {
		return nil, fmt.Errorf("insert %q at %d (order length %d): %w", in.ID, in.Pos, len(t.order), ErrBadPosition)
	}
	t.order = append(t.order, "")
	copy(t.order[in.Pos+1:], t.order[in.Pos:])
	t.order[in.Pos] = in.ID
	if in.Pos <= t.activeIdx {
		t.activeIdx++
	}
	return &InsertOutput{Milestones: []Milestone{
		{Kind: MemberJoined, Subject: in.ID, Round: t.round},
	}}, nil
}

// RemoveInput names the member leaving (death, flight, transfer).
type RemoveInput struct {
	ID core.EntityID
}

// RemoveOutput reports the milestones Remove caused.
type RemoveOutput struct {
	Milestones []Milestone
}

// Remove drops a member, keeping the active entity correct (design: Turn
// verbs). Errors: ErrNotMember.
func (t *Turn) Remove(in *RemoveInput) (*RemoveOutput, error) {
	idx := t.indexOf(in.ID)
	if idx < 0 {
		return nil, fmt.Errorf("remove %q: %w", in.ID, ErrNotMember)
	}
	wasActive := idx == t.activeIdx
	t.order = append(t.order[:idx], t.order[idx+1:]...)
	ms := []Milestone{{Kind: MemberLeft, Subject: in.ID, Round: t.round}}
	switch {
	case len(t.order) == 0:
		t.activeIdx = 0
	case wasActive:
		if t.activeIdx >= len(t.order) {
			t.activeIdx = 0 // active was last; next is first, round unchanged
		}
		ms = append(ms, Milestone{Kind: TurnStarted, Subject: t.order[t.activeIdx], Round: t.round})
	case idx < t.activeIdx:
		t.activeIdx--
	}
	return &RemoveOutput{Milestones: ms}, nil
}
