// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// useRageFromItsDescription stands in for the resolver's effect machine
// (rulebooks/dnd5e/resolution/effect.go), which this module cannot import. It
// does what the door does, in the door's order, and reads everything from the
// DESCRIPTION: it asks the sheet for Rage's description (a refusal is returned,
// nothing built or paid), builds the status first from
// definition.Effect.Condition with the ability and its user as its origin,
// pays definition.Cost at the gate with the character as the ledger, and only
// then publishes the status to its user as a feature's. It returns the first
// refusal.
func useRageFromItsDescription(ctx context.Context, t *testing.T, c *Character, bus events.EventBus) error {
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
	if err := combat.Pay(c, definition.Cost); err != nil {
		return err
	}
	return dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: c, Source: dnd5eEvents.ConditionSourceFeature, Condition: built.Condition,
	})
}
