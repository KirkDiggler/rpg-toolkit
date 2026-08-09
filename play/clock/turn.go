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
	// order matters: sole-member removal is also wasActive; wasActive arm indexes into possibly-empty order
	switch {
	case len(t.order) == 0:
		t.activeIdx = 0
		t.round = 0
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

// MergeInput combines Other into the receiver under a caller-supplied
// interleaved order (the rulebook decides how initiatives mesh).
type MergeInput struct {
	Other *Turn
	Order []core.EntityID
}

// MergeOutput reports the milestones Merge caused.
type MergeOutput struct {
	Milestones []Milestone
}

// Merge absorbs Other's members under Order, which must be a permutation
// of the union of both member sets. The receiver's active entity remains
// active and its round is retained; Other is reset to the zero/idle state.
// Other must be non-nil and distinct from the receiver.
// Errors: ErrIdle (idle receiver), ErrSameClock, ErrBadOrder.
func (t *Turn) Merge(in *MergeInput) (*MergeOutput, error) {
	if len(t.order) == 0 {
		return nil, fmt.Errorf("merge: receiver: %w", ErrIdle)
	}
	if in.Other == t {
		return nil, fmt.Errorf("merge: cannot merge a clock into itself: %w", ErrSameClock)
	}
	union := make(map[core.EntityID]struct{}, len(t.order)+len(in.Other.order))
	for _, id := range t.order {
		union[id] = struct{}{}
	}
	for _, id := range in.Other.order {
		union[id] = struct{}{}
	}
	if len(in.Order) != len(union) {
		return nil, fmt.Errorf("merge: order has %d entries, union has %d: %w", len(in.Order), len(union), ErrBadOrder)
	}
	seen := make(map[core.EntityID]struct{}, len(in.Order))
	for _, id := range in.Order {
		if _, ok := union[id]; !ok {
			return nil, fmt.Errorf("merge: %q is in neither clock: %w", id, ErrBadOrder)
		}
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("merge: %q appears twice: %w", id, ErrBadOrder)
		}
		seen[id] = struct{}{}
	}
	active := t.order[t.activeIdx]
	t.order = append([]core.EntityID(nil), in.Order...)
	t.activeIdx = t.indexOf(active)
	in.Other.order = nil
	in.Other.activeIdx = 0
	in.Other.round = 0
	return &MergeOutput{Milestones: []Milestone{{Kind: Merged, Round: t.round}}}, nil
}

// DissolveInput is empty today; verbs keep their Input struct for
// additive evolution (design R3(b)).
type DissolveInput struct{}

// DissolveOutput returns the members for the composition to re-home.
type DissolveOutput struct {
	// Members transfers the internal slice; the clock drops its reference in the same call —
	// the sanctioned exception to the module's copy-on-read convention (design: Dissolve row).
	Members    []core.EntityID
	Milestones []Milestone
}

// Dissolve empties the clock at fight end. Members transfers the internal slice; the clock
// drops its reference in the same call — the sanctioned exception to the module's copy-on-read
// convention (design: Dissolve row). Errors: ErrIdle (already empty).
func (t *Turn) Dissolve(_ *DissolveInput) (*DissolveOutput, error) {
	if len(t.order) == 0 {
		return nil, fmt.Errorf("dissolve: %w", ErrIdle)
	}
	members := t.order
	round := t.round
	t.order = nil
	t.activeIdx = 0
	t.round = 0
	return &DissolveOutput{
		Members:    members,
		Milestones: []Milestone{{Kind: Dissolved, Round: round}},
	}, nil
}

// TurnData is Turn's persisted shape (design R8). Plain data, no behavior.
// An idle snapshot has nil Order and marshals to {}.
type TurnData struct {
	Order     []core.EntityID `json:"order,omitempty"`
	ActiveIdx int             `json:"active_idx,omitempty"`
	Round     int             `json:"round,omitempty"`
}

// ToData snapshots the clock. Family-convention exemption from R3 (design).
func (t *Turn) ToData() TurnData {
	return TurnData{
		Order:     append([]core.EntityID(nil), t.order...),
		ActiveIdx: t.activeIdx,
		Round:     t.round,
	}
}

// LoadTurn reconstructs a Turn from persisted state. A constructor, not a
// verb — no milestones. Errors: ErrInvalidData for every R9 rejection.
func LoadTurn(data TurnData) (*Turn, error) {
	if len(data.Order) == 0 {
		if data.ActiveIdx != 0 {
			return nil, fmt.Errorf("load turn: idle clock with active idx %d: %w", data.ActiveIdx, ErrInvalidData)
		}
		if data.Round != 0 {
			return nil, fmt.Errorf("load turn: idle clock with round %d: %w", data.Round, ErrInvalidData)
		}
		return &Turn{}, nil
	}
	seen := make(map[core.EntityID]struct{}, len(data.Order))
	for _, id := range data.Order {
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("load turn: duplicate member %q: %w", id, ErrInvalidData)
		}
		seen[id] = struct{}{}
	}
	if data.ActiveIdx < 0 || data.ActiveIdx >= len(data.Order) {
		return nil, fmt.Errorf(
			"load turn: active idx %d out of range [0,%d): %w",
			data.ActiveIdx, len(data.Order), ErrInvalidData,
		)
	}
	if data.Round < 1 {
		return nil, fmt.Errorf("load turn: round %d with non-empty order: %w", data.Round, ErrInvalidData)
	}
	return &Turn{
		order:     append([]core.EntityID(nil), data.Order...),
		activeIdx: data.ActiveIdx,
		round:     data.Round,
	}, nil
}

// JoinMemberInput is the Joiner seam's input shape.
type JoinMemberInput struct {
	ID  core.EntityID
	Pos int
}

// JoinMemberOutput is the Joiner seam's output shape.
type JoinMemberOutput struct {
	Milestones []Milestone
}

// LeaveMemberInput is the Leaver seam's input shape.
type LeaveMemberInput struct {
	ID core.EntityID
}

// LeaveMemberOutput is the Leaver seam's output shape.
type LeaveMemberOutput struct {
	Milestones []Milestone
}

// JoinMember adapts Insert to the Joiner seam.
func (t *Turn) JoinMember(in *JoinMemberInput) (*JoinMemberOutput, error) {
	out, err := t.Insert(&InsertInput{ID: in.ID, Pos: in.Pos})
	if err != nil {
		return nil, err
	}
	return &JoinMemberOutput{Milestones: out.Milestones}, nil
}

// LeaveMember adapts Remove to the Leaver seam.
func (t *Turn) LeaveMember(in *LeaveMemberInput) (*LeaveMemberOutput, error) {
	out, err := t.Remove(&RemoveInput{ID: in.ID})
	if err != nil {
		return nil, err
	}
	return &LeaveMemberOutput{Milestones: out.Milestones}, nil
}
