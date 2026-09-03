// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

func TestUnconsciousConditionRemainsLoadableButDeathSaveHandlersAreInert(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	roller := &countingLegacyRoller{value: 1}
	condition := &UnconsciousCondition{
		CharacterID: "char-1",
		Roller:      roller,
		deathSaveState: &saves.DeathSaveState{
			Successes: 2,
			Failures:  1,
		},
	}
	require.NoError(t, condition.Apply(ctx, bus))
	require.True(t, condition.IsApplied())
	require.Len(t, condition.subscriptionIDs, 4,
		"legacy topic subscriptions remain attach-compatible but their handlers are inert")

	var rolled int
	_, err := dnd5eEvents.DeathSaveRolledTopic.On(bus).Subscribe(ctx,
		func(context.Context, dnd5eEvents.DeathSaveRolledEvent) error {
			rolled++
			return nil
		})
	require.NoError(t, err)

	require.NoError(t, dnd5eEvents.TurnStartTopic.On(bus).Publish(ctx, dnd5eEvents.TurnStartEvent{
		SubjectID: "char-1", Round: 1,
	}))
	require.NoError(t, dnd5eEvents.DamageReceivedTopic.On(bus).Publish(ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID: "char-1", Amount: 5, IsCritical: true,
	}))
	require.NoError(t, dnd5eEvents.HealingReceivedTopic.On(bus).Publish(ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: "char-1", Amount: 5, Source: "cure_wounds",
	}))

	require.Zero(t, roller.calls, "legacy blobs never auto-roll")
	require.Zero(t, rolled, "legacy blobs never publish authoritative outcomes")
	require.Equal(t, &saves.DeathSaveState{Successes: 2, Failures: 1}, condition.deathSaveState,
		"damage and healing cannot mutate the legacy progress ledger")
	require.True(t, condition.IsApplied(), "healing does not let the legacy blob author condition lifetime")
}

func TestUnconsciousConditionPersistenceCompatibility(t *testing.T) {
	original := &UnconsciousCondition{
		CharacterID: "char-1",
		deathSaveState: &saves.DeathSaveState{
			Successes: 3, Failures: 2, Stabilized: true,
		},
	}

	raw, err := original.ToJSON()
	require.NoError(t, err)

	loadedBehavior, err := LoadJSON(raw)
	require.NoError(t, err)
	loaded, ok := loadedBehavior.(*UnconsciousCondition)
	require.True(t, ok)
	require.Equal(t, original.CharacterID, loaded.CharacterID)
	require.Equal(t, original.deathSaveState, loaded.deathSaveState)
	require.Nil(t, loaded.Roller)

	var data UnconsciousData
	require.NoError(t, json.Unmarshal(raw, &data))
	require.Equal(t, original.CharacterID, data.MemberID)
	require.Equal(t, 3, data.Successes)
	require.Equal(t, 2, data.Failures)
	require.True(t, data.Stabilized)
}

func TestUnconsciousConditionApplyAndRemove(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	condition := NewUnconsciousCondition("char-1", nil)

	require.NoError(t, condition.Apply(ctx, bus))
	require.Error(t, condition.Apply(ctx, bus))
	require.NoError(t, condition.Remove(ctx, bus))
	require.False(t, condition.IsApplied())
	require.Nil(t, condition.subscriptionIDs)
}

type countingLegacyRoller struct {
	value int
	calls int
}

func (r *countingLegacyRoller) Roll(context.Context, int) (int, error) {
	r.calls++
	return r.value, nil
}

func (r *countingLegacyRoller) RollN(context.Context, int, int) ([]int, error) {
	return nil, nil
}
