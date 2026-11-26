// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// turnHost is the fake of what a session does for real once the encounter
// reads a creature's turn from its sheet (rpg-project#1131): it keeps each
// member's turn, says what is left of it in every sheet it answers, and is
// the host that spends it.
//
// IT WRAPS A TEST'S OWN HOST. The Sheets, Striker, optional Disengager and
// Announcer a scene already has stay what they were; the turn host answers
// the same facts with [encounter.SheetFacts.Turn] filled and passes every
// call through to the host it wraps. It is itself a [encounter.Dasher].
//
// A member has one action and one bonus action a turn unless a test says
// otherwise ([turnHost.AllowPerTurn]). A Strike that returned nil or
// [encounter.ErrStrikePaused] spends an action, a Dash spends an action, a
// Disengage that returned nil spends a bonus action, and an Announce that says
// a member's turn started gives that member a full turn again. A test can
// also set what is left by hand ([turnHost.Leave]).
type turnHost struct {
	sheets     encounter.Sheets
	striker    encounter.Striker
	disengager encounter.Disengager
	announcer  encounter.Announcer

	// failDash, when set, is what every Dash answers with.
	failDash error

	perTurn map[encounter.MemberID]encounter.TurnFacts
	left    map[encounter.MemberID]encounter.TurnFacts

	// What it was asked, in order.
	Strikes    []encounter.MemberID
	Dashes     []encounter.MemberID
	Disengages []encounter.MemberID
	Announced  []encounter.Boundary
}

// newTurnHost wraps a scene's own host. disengager and announcer may be nil:
// a nil disengager is not asked and the host does not claim to be one.
func newTurnHost(
	sheets encounter.Sheets, striker encounter.Striker, announcer encounter.Announcer,
) *turnHost {
	if announcer == nil {
		announcer = quietAnnouncer{}
	}

	return &turnHost{
		sheets: sheets, striker: striker, announcer: announcer,
		perTurn: map[encounter.MemberID]encounter.TurnFacts{},
		left:    map[encounter.MemberID]encounter.TurnFacts{},
	}
}

// AllowPerTurn says how many actions and bonus actions member has on a full
// turn, and gives it that now.
func (h *turnHost) AllowPerTurn(member encounter.MemberID, actions, bonus int) {
	full := encounter.TurnFacts{ActionsLeft: actions, BonusActionsLeft: bonus}
	h.perTurn[member] = full
	h.left[member] = full
}

// Leave sets by hand what member has left of this turn.
func (h *turnHost) Leave(member encounter.MemberID, actions, bonus int) {
	h.left[member] = encounter.TurnFacts{ActionsLeft: actions, BonusActionsLeft: bonus}
}

// Left is what member has left of this turn.
func (h *turnHost) Left(member encounter.MemberID) encounter.TurnFacts {
	if t, ok := h.left[member]; ok {
		return t
	}
	if t, ok := h.perTurn[member]; ok {
		return t
	}

	return encounter.TurnFacts{ActionsLeft: 1, BonusActionsLeft: 1}
}

func (h *turnHost) spendAction(member encounter.MemberID) {
	t := h.Left(member)
	t.ActionsLeft = max(0, t.ActionsLeft-1)
	h.left[member] = t
}

func (h *turnHost) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	inner, err := h.sheets.Sheets(members)
	if err != nil {
		return nil, err
	}
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(inner))
	for id, facts := range inner {
		turn := h.Left(id)
		facts.Turn = &turn
		out[id] = facts
	}

	return out, nil
}

func (h *turnHost) Strike(
	ctx context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	h.Strikes = append(h.Strikes, attacker)
	err := h.striker.Strike(ctx, enc, attacker, target, action)
	if err == nil || errors.Is(err, encounter.ErrStrikePaused) {
		h.spendAction(attacker)
	}

	return err
}

// Disengage is the host's [encounter.Disengager]. Wire it only in a scene that
// names a Disengager (wrap one with [turnHost.WithDisengager]).
func (h *turnHost) Disengage(ctx context.Context, enc *encounter.Encounter, member encounter.MemberID) error {
	h.Disengages = append(h.Disengages, member)
	if h.disengager != nil {
		if err := h.disengager.Disengage(ctx, enc, member); err != nil {
			return err
		}
	}
	t := h.Left(member)
	t.BonusActionsLeft = max(0, t.BonusActionsLeft-1)
	h.left[member] = t

	return nil
}

// WithDisengager has Disengage pass through to d as well.
func (h *turnHost) WithDisengager(d encounter.Disengager) *turnHost {
	h.disengager = d

	return h
}

// Dash is the host's [encounter.Dasher]: a run spends an action.
func (h *turnHost) Dash(_ context.Context, _ *encounter.Encounter, member encounter.MemberID) error {
	h.Dashes = append(h.Dashes, member)
	if h.failDash != nil {
		return h.failDash
	}
	h.spendAction(member)

	return nil
}

func (h *turnHost) Announce(ctx context.Context, enc *encounter.Encounter, crossed []encounter.Boundary) error {
	h.Announced = append(h.Announced, crossed...)
	for _, b := range crossed {
		if b.Kind == encounter.TurnStarted {
			delete(h.left, b.Subject)
			if full, ok := h.perTurn[b.Subject]; ok {
				h.left[b.Subject] = full
			}
		}
	}

	return h.announcer.Announce(ctx, enc, crossed)
}

// actors is the host wired as every actor it can be. Disengager is the host
// only when asked for, because a scene with none must not be asked.
func (h *turnHost) actors(mover encounter.Mover, withDisengage bool) encounter.Actors {
	a := encounter.Actors{Striker: h, Mover: mover, Announcer: h, Dasher: h}
	if withDisengage {
		a.Disengager = h
	}

	return a
}
