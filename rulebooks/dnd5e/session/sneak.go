// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// sneak.go carries the composition's SetSneaking verb across the seam
// (rpg-project#606, sneaking design R6). It is DELIBERATELY THIN: the
// encounter owns the mode — who may sneak (a player, in free roam), the lost
// verdicts a declaration in plain view writes, the sight refresh that judges
// the declaration at once, and the `snuck` beats any roll leaves — and this
// file only carries that verb across the boundary in
// [Manager.SetDiscoverySharing]'s guarded load/act/save/deliver shape.
//
// WHAT THE SESSION ADDS ELSEWHERE, NOT HERE (design R8, R10):
//   - the half pace, answered by the sheet seam's speed while
//     [encounter.Encounter.Sneaking] says so (sheets.go);
//   - the re-arm distance seed, [encounter.SheetFacts.WatchResetCells]
//     (sheets.go);
//   - the watch arc seed, [encounter.SheetFacts.WatchArc] (sheets.go, R14);
//   - the roll itself, which arrives through [checkSeam.ResolveDiscoveryCheck]
//     against the sneaker's staged record — every write verb stages every
//     player (exploration.go), so a verb by one player that walks a watcher
//     into another sneaking player rolls for the sneaker.

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// SetSneakingInput sets or clears one seated player's sneaking mode.
type SetSneakingInput struct {
	// Session is the session to act in.
	Session string

	// Member is the player who declares. THE HOST MUST BIND Member TO THE
	// AUTHENTICATED CALLER: the mode is the player's own declaration.
	Member string

	// Sneaking is the declared mode: true to sneak, false to stop.
	Sneaking bool
}

// SetSneakingOutput is the mode the encounter now holds, with the ordinary
// persistence reports. It mirrors [SetDiscoverySharingOutput] and the wire's
// SetSneakingResponse: any roll the declaration's refresh caused arrives as an
// `snuck` beat on the mover's own stream, and a fight it formed arrives as the
// formation's own beats — neither is restated here.
type SetSneakingOutput struct {
	Sneaking bool
	Saved    SaveReport
	Delivery DeliveryReport
}

// SetSneaking declares or clears sneaking for Member on the world clock. It
// spends no time; the composition refreshes sight at once, so stopping inside
// a hostile watcher's zone is seen in this call and may form a fight, which
// is delivered on the streams like any other formation.
//
// DECLARING THE MODE ALREADY HELD IS A NO-OP in the composition: no roll, no
// re-arm, no refresh, no beat; the current mode is returned. The verb still
// commits, as [Manager.SetDiscoverySharing] does for a repeated choice, so the
// save report names the records rewritten unchanged.
//
// Errors include ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoMember (a stranger, or a member who is not a player: a
// monster never sneaks), ErrWindowOpen, ErrInBubble (a member in a fight:
// sneaking is declared in free roam), ErrClosed, and the ordinary
// lock/read/save translations. Refusals name the verb.
func (m *Manager) SetSneaking(ctx context.Context, in *SetSneakingInput) (*SetSneakingOutput, error) {
	const verb = "set sneaking"
	if in == nil {
		return nil, fmt.Errorf("%s: %w", verb, ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, fmt.Errorf("%s: %w", verb, lockErr)
	}
	defer release()
	if in.Member == "" {
		return nil, fmt.Errorf("%s: %w", verb, ErrNoMemberID)
	}
	scope, err := m.openForChange(ctx, "set_sneaking", in.Session)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, err)
	}
	out, err := scope.enc.SetSneaking(&encounter.SetSneakingInput{
		Member:   encounter.MemberID(in.Member),
		Sneaking: in.Sneaking,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, translate(err))
	}
	saved, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", verb, err)
	}
	return &SetSneakingOutput{Sneaking: out.Sneaking, Saved: saved, Delivery: delivery}, nil
}
