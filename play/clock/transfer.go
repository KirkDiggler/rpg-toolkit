// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Leaver is the leaving half of the Membership seam.
type Leaver interface {
	LeaveMember(in *LeaveMemberInput) (*LeaveMemberOutput, error)
}

// Joiner is the joining half of the Membership seam.
type Joiner interface {
	JoinMember(in *JoinMemberInput) (*JoinMemberOutput, error)
}

// Membership is what Transfer requires of both sides: the ability to
// join AND leave, so a failed transfer can compensate (design: Transfer).
type Membership interface {
	Leaver
	Joiner
}

// TransferInput moves ID from one clock to another, Pos choosing its slot
// when To is ordered. Both From and To must be non-nil.
type TransferInput struct {
	From Membership
	To   Membership
	ID   core.EntityID
	Pos  int
}

// TransferOutput reports the transfer's milestones, leave-then-join.
type TransferOutput struct {
	Milestones []Milestone
}

// Transfer moves ID between clocks upholding one-clock-per-entity (R6):
// on any failure both clocks are unchanged and the underlying sentinel
// propagates. Execution is join-first with compensating leave — the
// transient dual membership is invisible under R10's single-threaded
// contract; milestones are reported in leave-then-join order per the
// design regardless.
func Transfer(in *TransferInput) (*TransferOutput, error) {
	if in.From == in.To {
		return nil, fmt.Errorf("transfer %q: from and to are the same clock: %w", in.ID, ErrSameClock)
	}
	join, err := in.To.JoinMember(&JoinMemberInput{ID: in.ID, Pos: in.Pos})
	if err != nil {
		return nil, fmt.Errorf("transfer %q: join: %w", in.ID, err)
	}
	leave, err := in.From.LeaveMember(&LeaveMemberInput{ID: in.ID})
	if err != nil {
		if _, undoErr := in.To.LeaveMember(&LeaveMemberInput{ID: in.ID}); undoErr != nil {
			return nil, fmt.Errorf(
				"transfer %q: leave failed (%v) and compensation failed — %q may remain on both clocks: %w",
				in.ID, err, in.ID, undoErr)
		}
		return nil, fmt.Errorf("transfer %q: leave: %w", in.ID, err)
	}
	return &TransferOutput{
		Milestones: append(append([]Milestone(nil), leave.Milestones...), join.Milestones...),
	}, nil
}
