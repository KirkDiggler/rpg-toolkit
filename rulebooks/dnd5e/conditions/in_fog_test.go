// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestInFogMembershipRoundTripKeepsIndependentAreas(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	var memberships []*InFogCondition
	for _, area := range []string{"cloud-a", "cloud-b"} {
		condition, err := NewInFogCondition(NewInFogConditionInput{MemberID: "cleric", SourceID: area, SourceRef: refs.Spells.FogCloud()})
		require.NoError(t, err)
		raw, err := condition.ToJSON()
		require.NoError(t, err)
		loaded, err := LoadJSON(raw)
		require.NoError(t, err)
		require.Equal(t, condition.ConditionAddress(), ConditionAddressOf("cleric", loaded))
		require.NoError(t, loaded.Apply(ctx, bus))
		memberships = append(memberships, loaded.(*InFogCondition))
	}
	require.NotEqual(t, memberships[0].ConditionAddress(), memberships[1].ConditionAddress())
	require.NoError(t, memberships[0].Remove(ctx, bus))
	require.False(t, memberships[0].IsApplied())
	require.True(t, memberships[1].IsApplied())
	display, ok := DisplayFor(*refs.Conditions.InFog())
	require.True(t, ok)
	require.Equal(t, "In Fog", display.Name)
}

func TestInFogRejectsMalformedMembershipAndSupportsFactory(t *testing.T) {
	for _, data := range []InFogConditionData{
		{Ref: refs.Conditions.InFog(), SourceID: "area", SourceRef: refs.Spells.FogCloud()},
		{Ref: refs.Conditions.InFog(), MemberID: "cleric", SourceRef: refs.Spells.FogCloud()},
		{Ref: refs.Conditions.InFog(), MemberID: "cleric", SourceID: "area", SourceRef: refs.Spells.Bless()},
	} {
		raw, err := json.Marshal(data)
		require.NoError(t, err)
		_, err = LoadJSON(raw)
		require.Error(t, err)
	}
	out, err := CreateFromRef(&CreateFromRefInput{Ref: refs.Conditions.InFog().String(), MemberID: "cleric", SourceRef: refs.Spells.FogCloud().String(), Config: json.RawMessage(`{"source_id":"area"}`)})
	require.NoError(t, err)
	require.Equal(t, "area", ConditionAddressOf("cleric", out.Condition).SourceID)
}
