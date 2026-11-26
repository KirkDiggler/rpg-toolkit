// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// internalTurnHost is the fake of what a session does for real once the
// encounter reads a creature's turn from its sheet (rpg-project#1131), for the
// tests in this package that cannot import the external one: it wraps a
// scene's own Sheets, Striker and Announcer, says in every sheet what is left
// of each member's turn (one action and one bonus action a turn), spends the
// action on a Strike that returned nil or [ErrStrikePaused] and on a Dash, and
// gives a member a full turn again when its turn starts. The external twin is
// testturns_test.go.
type internalTurnHost struct {
	sheets    Sheets
	striker   Striker
	announcer Announcer

	left map[MemberID]TurnFacts
}

// withInternalTurnHost puts a scene's host behind a turn host.
func withInternalTurnHost(c Capabilities) Capabilities {
	h := &internalTurnHost{
		sheets: c.Sheets, striker: c.Striker, announcer: c.Announcer,
		left: map[MemberID]TurnFacts{},
	}
	c.Sheets, c.Striker, c.Announcer, c.Dasher = h, h, h, h

	return c
}

func (h *internalTurnHost) leftOf(member MemberID) TurnFacts {
	if t, ok := h.left[member]; ok {
		return t
	}

	return TurnFacts{ActionsLeft: 1, BonusActionsLeft: 1}
}

func (h *internalTurnHost) spendAction(member MemberID) {
	t := h.leftOf(member)
	t.ActionsLeft = max(0, t.ActionsLeft-1)
	h.left[member] = t
}

func (h *internalTurnHost) Sheets(members []MemberID) (map[MemberID]SheetFacts, error) {
	inner, err := h.sheets.Sheets(members)
	if err != nil {
		return nil, err
	}
	out := make(map[MemberID]SheetFacts, len(inner))
	for id, facts := range inner {
		turn := h.leftOf(id)
		facts.Turn = &turn
		out[id] = facts
	}

	return out, nil
}

func (h *internalTurnHost) Strike(
	ctx context.Context, enc *Encounter, attacker, target MemberID, action core.Ref,
) error {
	err := h.striker.Strike(ctx, enc, attacker, target, action)
	if err == nil || errors.Is(err, ErrStrikePaused) {
		h.spendAction(attacker)
	}

	return err
}

func (h *internalTurnHost) Dash(_ context.Context, _ *Encounter, member MemberID) error {
	h.spendAction(member)

	return nil
}

func (h *internalTurnHost) Announce(ctx context.Context, enc *Encounter, crossed []Boundary) error {
	for _, b := range crossed {
		if b.Kind == TurnStarted {
			delete(h.left, b.Subject)
		}
	}

	return h.announcer.Announce(ctx, enc, crossed)
}
