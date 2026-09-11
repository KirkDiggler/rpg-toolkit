// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// fakeSheetKeeper is the OTHER half of a participant: the party that owns the
// sheet and applies what the rules ask of it.
//
// A condition can no longer write to a sheet, so a test that only installs a
// cast has built half a world — the rule publishes into a room with nobody in
// it, and the assertion has nothing to read. This is the keeper that would be
// listening in production, doing exactly what the real ones do: debit the
// economy, mark the sheet.
//
// # Which rows it takes is the point
//
// BOTH kinds of sheet take the spend row now, because both real keepers do
// (Kirk, 2026-09-11). What differs is what the row debits: a character has an
// economy and pays the slot, a monster has one reaction and a bool that says
// whether it still holds it. A monster also takes the turn-start row that
// gives the bool back, which its real keeper takes and a character's does not
// — a character's slot is reseeded by a verb the session calls.
//
// Neither debit goes below empty. That is the real ledgers' floor, and it is
// what replaces the once-per-turn flag the opportunity attack used to keep:
// you cannot spend a reaction you do not have, so a duplicate bill is
// harmless without anything having to remember the first one.
//
// It is not the real keeper and does not pretend to be. `conditions` cannot
// import `character` (character imports conditions to load them), so the proof
// that the REAL keepers hold these rows lives in their own packages, and the
// proof that a real fold drives them end to end lives in monstertraits and in
// session.
type fakeSheetKeeper struct {
	sheet *fakeConditionOwner

	// spent records every debit this sheet actually made, in order, so a test
	// can tell "the right slot once" from "something, sometime". A bill the
	// sheet had nothing left to pay is not a debit and is not recorded.
	spent []coreCombat.ActionType

	// dirtied counts the writes reported about this sheet: a condition's own
	// state change, and the meter this keeper keeps.
	dirtied int
}

// keeperFor attaches a keeper for sheet to bus and returns it.
func keeperFor(ctx context.Context, bus events.EventBus, sheet *fakeConditionOwner) (*fakeSheetKeeper, error) {
	k := &fakeSheetKeeper{sheet: sheet}

	if _, err := dnd5eEvents.SpendRequestedTopic.On(bus).Subscribe(ctx, k.onSpendRequested); err != nil {
		return nil, err
	}

	if !sheet.hasEconomy {
		if _, err := dnd5eEvents.TurnStartTopic.On(bus).Subscribe(ctx, k.onTurnStart); err != nil {
			return nil, err
		}
	}

	if _, err := dnd5eEvents.ConditionStateChangedTopic.On(bus).Subscribe(ctx, k.onStateChanged); err != nil {
		return nil, err
	}

	return k, nil
}

func (k *fakeSheetKeeper) onSpendRequested(_ context.Context, event dnd5eEvents.SpendRequestedEvent) error {
	if event.MemberID != k.sheet.id || event.ActionType != coreCombat.ActionReaction {
		return nil
	}

	if !k.sheet.hasEconomy {
		if k.sheet.reactionSpent {
			return nil
		}
		k.sheet.reactionSpent = true
		k.spent = append(k.spent, event.ActionType)
		k.dirtied++

		return nil
	}

	if k.sheet.reactions <= 0 {
		return nil
	}

	k.sheet.reactions -= event.Amount
	if k.sheet.reactions < 0 {
		k.sheet.reactions = 0
	}
	k.spent = append(k.spent, event.ActionType)

	return nil
}

// onTurnStart gives a monster its reaction back at the start of its own turn,
// the row the real monster keeper holds.
func (k *fakeSheetKeeper) onTurnStart(_ context.Context, event dnd5eEvents.TurnStartEvent) error {
	if event.SubjectID != k.sheet.id || !k.sheet.reactionSpent {
		return nil
	}

	k.sheet.reactionSpent = false
	k.dirtied++

	return nil
}

func (k *fakeSheetKeeper) onStateChanged(_ context.Context, event dnd5eEvents.ConditionStateChangedEvent) error {
	if event.MemberID != k.sheet.id {
		return nil
	}

	k.dirtied++

	return nil
}
