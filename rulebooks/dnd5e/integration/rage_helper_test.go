// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// freshTurnLedger is the account a barbarian that is not in a turn pays Rage's
// cost from: a fresh turn's one bonus action, and the character's own pool.
type freshTurnLedger struct {
	*character.Character
	bonus int
}

func (l *freshTurnLedger) InCombat() bool { return true }

func (l *freshTurnLedger) SlotsLeft(slot coreCombat.ActionType) int {
	if slot == coreCombat.ActionBonus {
		return l.bonus
	}
	return 0
}

func (l *freshTurnLedger) SpendSlots(slot coreCombat.ActionType, n int) {
	if slot == coreCombat.ActionBonus {
		l.bonus -= n
	}
}

var _ dnd5eCombat.Ledger = (*freshTurnLedger)(nil)

// useRageFromItsDescription stands in for the resolver's effect machine
// (rulebooks/dnd5e/resolution/effect.go), which this module cannot import. It
// does what the door does, in the door's order, and reads everything from the
// DESCRIPTION: it asks the sheet for Rage's description, builds the status
// first from definition.Effect.Condition with the ability and its user as its
// origin, pays definition.Cost at the gate, and only then publishes the status
// to its user as a feature's. It returns the first refusal.
func useRageFromItsDescription(
	ctx context.Context, t *testing.T, c *character.Character, bus events.EventBus,
) error {
	t.Helper()
	definition, ok, err := c.AbilityDefinition(*refs.Features.Rage())
	if err != nil {
		return err
	}
	require.True(t, ok, "the sheet carries Rage's description")
	require.NotNil(t, definition.Effect, "the description says what Rage puts on its user")
	require.NotNil(t, definition.Effect.Condition)
	status := definition.Effect.Condition
	built, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref:      status.Ref.String(),
		Config:   status.Parameters,
		MemberID: c.GetID(),
		Origin:   &dnd5eEvents.ConditionOrigin{Ref: definition.Ref, EntityID: c.GetID()},
	})
	require.NoError(t, err)
	if err := dnd5eCombat.Pay(&freshTurnLedger{Character: c, bonus: 1}, definition.Cost); err != nil {
		return err
	}
	return dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: c, Source: dnd5eEvents.ConditionSourceFeature, Condition: built.Condition,
	})
}
