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
// It subscribes to the spend topic only for a sheet that HAS an economy,
// because that is literally how the two real keepers differ — character.Load
// wires the row and monster.Load does not. Reproducing the asymmetry as a
// missing subscription rather than as an if inside the handler is what makes
// this fake a stand-in rather than a second implementation: a monster pays
// nothing here for the same reason it pays nothing in production, and if that
// reason ever stopped being true the fake would stop reproducing it.
//
// It is not the real keeper and does not pretend to be. `conditions` cannot
// import `character` (character imports conditions to load them), so the proof
// that the REAL keepers hold these rows lives in their own packages, and the
// proof that a real fold drives them end to end lives in monstertraits and in
// session.
type fakeSheetKeeper struct {
	sheet *fakeConditionOwner

	// spent records every debit asked of this sheet, in order, so a test can
	// tell "the right slot once" from "something, sometime".
	spent []coreCombat.ActionType

	// dirtied counts the state changes reported about this sheet.
	dirtied int
}

// keeperFor attaches a keeper for sheet to bus and returns it.
func keeperFor(ctx context.Context, bus events.EventBus, sheet *fakeConditionOwner) (*fakeSheetKeeper, error) {
	k := &fakeSheetKeeper{sheet: sheet}

	if sheet.hasEconomy {
		if _, err := dnd5eEvents.SpendRequestedTopic.On(bus).Subscribe(ctx, k.onSpendRequested); err != nil {
			return nil, err
		}
	}

	if _, err := dnd5eEvents.ConditionStateChangedTopic.On(bus).Subscribe(ctx, k.onStateChanged); err != nil {
		return nil, err
	}

	return k, nil
}

func (k *fakeSheetKeeper) onSpendRequested(_ context.Context, event dnd5eEvents.SpendRequestedEvent) error {
	if event.MemberID != k.sheet.id {
		return nil
	}

	k.spent = append(k.spent, event.ActionType)
	if event.ActionType == coreCombat.ActionReaction {
		k.sheet.reactions -= event.Amount
	}

	return nil
}

func (k *fakeSheetKeeper) onStateChanged(_ context.Context, event dnd5eEvents.ConditionStateChangedEvent) error {
	if event.MemberID != k.sheet.id {
		return nil
	}

	k.dirtied++

	return nil
}
