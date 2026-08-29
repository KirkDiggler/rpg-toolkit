// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// This file is the WRITE half of an effect's relationship with a sheet, the
// way member.go is the read half.
//
// An effect reads the world through the cast and writes to it through the bus,
// and neither channel hands it a sheet to mutate. What it used to hold instead
// was an owner handle passed in at attach time — wired bespoke in two loaders,
// silently absent whenever either forgot, and wide enough to reach anything on
// the sheet rather than only the thing the rule meant to change. Both halves
// replace one handle each: member.go replaced the reads, this replaced the
// writes, and there is nothing left for the handle to carry.
//
// Neither function decides anything. They state a fact or make a request and
// let the keeper that owns the sheet act on it — that is the whole content of
// D3/D4, and it is why there is no logic in either body worth hiding.

// publishStateChanged states that a condition's OWN persisted fields moved.
//
// One function for every condition that keeps turn-scoped memory, because the
// statement is the same one every time: whose sheet I hang on, and which
// condition I am. See [dnd5eEvents.ConditionStateChangedEvent] for why this is
// a fact rather than an instruction to mark something dirty.
//
// A FAILED PUBLISH IS RETURNED, not swallowed. This event exists because a
// change nobody hears about is a change discarded at save time; reporting that
// all is well while exactly that happens would rebuild the failure it was
// written to end. It replaced a markDirty() that could not fail, so this is the
// one behavioural difference in the swap — in a path that has no way to be
// reached today, since the only subscribers are the two sheet keepers and
// neither of them returns an error.
func publishStateChanged(ctx context.Context, bus events.EventBus, memberID string, ref *core.Ref) error {
	return dnd5eEvents.ConditionStateChangedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionStateChangedEvent{
		MemberID:     memberID,
		ConditionRef: ref,
	})
}

// publishSpendRequested asks a member's keeper to debit that member's action
// economy.
//
// The caller has already established that the cost is owed and affordable —
// [combat.Pay]'s contract is that a debit past a passed check cannot fail — so
// this is the payment, not a question about it. A keeper with no economy to
// debit simply has no row for the topic, which is how a monster pays nothing
// without anything here knowing what a monster is.
func publishSpendRequested(
	ctx context.Context, bus events.EventBus, memberID string, slot coreCombat.ActionType, amount int, source *core.Ref,
) error {
	return dnd5eEvents.SpendRequestedTopic.On(bus).Publish(ctx, dnd5eEvents.SpendRequestedEvent{
		MemberID:   memberID,
		ActionType: slot,
		Amount:     amount,
		SourceRef:  source,
	})
}
