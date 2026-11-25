// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// wait.go is the Wait verb at the seam (rpg-project#1079, slice #1088): a
// seated player outside a fight waits, and one round passes on the world
// clock. It is built as Loot is: open the act, refuse what the session rules,
// ask the encounter, commit.

// WaitInput declares one seated player waiting a round.
type WaitInput struct {
	// Session is the session to act in.
	Session string
	// Member is who waits. THE HOST MUST BIND Member TO THE AUTHENTICATED
	// CALLER — the acting-as gate every verb carries.
	Member string
}

// WaitOutput acknowledges the wait: the two host-facing reports and nothing
// else. The wait and what the round brought arrive on the act stream.
type WaitOutput struct {
	Saved    SaveReport     `json:"saved"`
	Delivery DeliveryReport `json:"delivery"`
}

// Wait is one seated player waiting a round outside a fight.
//
// THE SESSION HOLDS NO RULE HERE except the two it already owns: a downed
// member is refused (ErrDowned, where the sheets are), and opening the act
// refuses while a table-holding window is open or the waiter's own roll waits
// (ErrWindowOpen). The price, every other refusal and who is told are the
// encounter's: a held member or one in custody may wait. The roster is read
// first so a stranger is told so, a downed member is refused before a round is
// spent, and the encounter rules the rest.
//
// Returns ErrNilInput, ErrNoMemberID, ErrNoSessionID, ErrNoSession,
// ErrNoEncounter, ErrClosed, ErrNoMember, ErrInBubble (a fight), ErrDowned,
// ErrWindowOpen, or ErrSaveFailed with a populated report.
func (m *Manager) Wait(ctx context.Context, in *WaitInput) (*WaitOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("wait: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	if in.Member == "" {
		return nil, fmt.Errorf("wait: %w", ErrNoMemberID)
	}

	scope, err := m.openForChangeBy(ctx, "wait", in.Session, in.Member)
	if err != nil {
		return nil, fmt.Errorf("wait: %w", err)
	}

	if _, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)}); err != nil {
		return nil, fmt.Errorf("wait: %w", translate(err))
	}

	if err := refuseIfDown(scope, "member", in.Member); err != nil {
		return nil, fmt.Errorf("wait: %w", err)
	}

	if _, err := scope.enc.Wait(&encounter.WaitInput{Member: encounter.MemberID(in.Member)}); err != nil {
		return nil, fmt.Errorf("wait: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("wait: %w", err)
	}

	return &WaitOutput{Saved: report, Delivery: delivery}, nil
}
