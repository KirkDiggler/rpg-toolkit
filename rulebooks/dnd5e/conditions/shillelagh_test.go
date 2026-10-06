// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/require"
)

func TestShillelaghClockSurvivesReloadAndIgnoresOtherTurns(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	effect, err := NewShillelaghCondition("caster", ShillelaghConfig{Weapons: []HeldWeapon{{Slot: "main_hand", ItemID: "club"}}, WeaponSlot: "main_hand", Ability: abilities.WIS})
	require.NoError(t, err)
	require.NoError(t, effect.Apply(ctx, bus))
	turns := dnd5eEvents.TurnEndTopic.On(bus)
	require.NoError(t, turns.Publish(ctx, dnd5eEvents.TurnEndEvent{SubjectID: "other"}))
	require.Equal(t, 10, effect.TurnEndsLeft)
	require.True(t, effect.SkipFirstTurnEnd)
	require.NoError(t, turns.Publish(ctx, dnd5eEvents.TurnEndEvent{SubjectID: "caster"}))
	require.Equal(t, 10, effect.TurnEndsLeft)
	require.False(t, effect.SkipFirstTurnEnd)
	raw, err := effect.ToJSON()
	require.NoError(t, err)
	require.NoError(t, effect.Remove(ctx, bus))
	reloaded := &ShillelaghCondition{}
	require.NoError(t, reloaded.loadJSON(raw))
	require.NoError(t, reloaded.Apply(ctx, bus))
	for i := 0; i < 9; i++ {
		require.NoError(t, turns.Publish(ctx, dnd5eEvents.TurnEndEvent{SubjectID: "caster"}))
	}
	require.True(t, reloaded.IsApplied())
	require.Equal(t, 1, reloaded.TurnEndsLeft)
	require.NoError(t, turns.Publish(ctx, dnd5eEvents.TurnEndEvent{SubjectID: "caster"}))
	require.False(t, reloaded.IsApplied())
	require.Nil(t, reloaded.WeaponAttackOverride("main_hand", "club"))
}
