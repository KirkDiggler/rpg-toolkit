// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// useActionSurgeFromItsDescription uses Action Surge the one way left: it asks
// the sheet for the description and pays the description's cost at the gate,
// with the character as the ledger. It returns the payment's error. The pool
// is spent and the turn gains its action together, or neither happens.
func useActionSurgeFromItsDescription(t *testing.T, c *Character) error {
	t.Helper()
	definition, ok, err := c.AbilityDefinition(*refs.Features.ActionSurge())
	require.NoError(t, err)
	require.True(t, ok, "the sheet carries Action Surge's description")
	return combat.Pay(c, definition.Cost)
}

// newLevelTwoFighter builds a level-1 fighter through the draft and takes it to
// fighter level 2 the way a player does, so Action Surge arrives with the
// level. The caller starts a turn if it wants one.
func newLevelTwoFighter(ctx context.Context, t *testing.T, id string, bus events.EventBus) *Character {
	t.Helper()
	char, err := newFighterDraft(t).ToCharacter(ctx, id, bus)
	require.NoError(t, err)
	char.experience = ExperienceThresholdForLevel(2)
	_, err = char.Advance(ctx, &AdvanceInput{ClassID: classes.Fighter, HitPointMethod: HitPointMethodAverage})
	require.NoError(t, err)
	require.Equal(t, 2, char.GetLevel())
	return char
}
