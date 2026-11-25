// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// useSecondWindFromItsDescription uses Second Wind the one way left: it asks
// the sheet for the description, pays the description's cost at the gate, and
// resolves the description's healing declaration with the roller. It returns
// the payment's error, and the resolved calculation when the payment went
// through. Applying the heal to hit points is the resolver's, tested there
// (rpg-project#1146), so it is not applied here; a test that needs the hit
// points to rise publishes the event itself with [announceSecondWindHeal].
func useSecondWindFromItsDescription(
	ctx context.Context, t *testing.T, c *Character, roller dice.Roller,
) (*dnd5eEvents.RollCalculation, error) {
	t.Helper()
	definition, ok, err := c.AbilityDefinition(*refs.Features.SecondWind())
	require.NoError(t, err)
	require.True(t, ok, "the sheet carries Second Wind's description")
	if err := combat.Pay(c, definition.Cost); err != nil {
		return nil, err
	}
	resolved, err := healing.Resolve(ctx, *definition.Effect.Healing,
		dnd5eEvents.RollSource{Ref: refs.Features.SecondWind(), Name: "Second Wind", SourceID: c.GetID()}, roller)
	require.NoError(t, err)
	return resolved, nil
}
