// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	dnd5eCombat "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func surgeGiver(t *testing.T) dnd5eEvents.RollSource {
	t.Helper()
	ref, err := core.ParseString("example:items:surge_ring")
	require.NoError(t, err)
	return dnd5eEvents.RollSource{Ref: ref, Name: "Surge ring", Label: "Surge ring", SourceID: "ring-1"}
}

// Test 1: Action Surge is found by all five.
func TestActionSurgeIsFoundByAllFive(t *testing.T) {
	ref, ok := features.NamedAbility("action surge")
	require.True(t, ok)
	assert.Equal(t, *refs.Features.ActionSurge(), ref)

	assert.Equal(t, []string{"action surge", "second wind"}, features.NamedAbilities())

	for _, raw := range []json.RawMessage{nil, {}, json.RawMessage(`{}`)} {
		assert.NoError(t, features.ValidateParameters(&features.ValidateParametersInput{Ref: ref, Parameters: raw}), "%q", raw)
	}
	err := features.ValidateParameters(&features.ValidateParametersInput{Ref: ref, Parameters: json.RawMessage(`{"level": 1}`)})
	require.Error(t, err)
	assert.Equal(t, "action surge takes no values", err.Error())

	words := features.AbilityWords(&features.AbilityWordsInput{}).Words
	require.Len(t, words, 2)
	assert.Equal(t, features.AbilityWord{Name: "action surge", Label: "Action Surge"}, words[0], "no values")
	assert.Equal(t, "second wind", words[1].Name)

	giver := surgeGiver(t)
	got, err := features.DescribeFromRef(&features.DescribeFromRefInput{
		Ref:   ref,
		Giver: giver,
		Pools: map[coreResources.ResourceKey]int{"action-surge": 1},
	})
	require.NoError(t, err)
	assert.Equal(t, &actions.Definition{
		Ref:         core.Ref{Module: "dnd5e", Type: "features", ID: "action_surge"},
		Name:        "Action Surge",
		Description: "Take one more action this turn",
		Cost: &dnd5eCombat.SpendProfile{
			Pools:      map[coreResources.ResourceKey]int{"action-surge": 1},
			GrantSlots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		},
	}, got)
	assert.Empty(t, got.Cost.Slots, "no slot is charged")
}

// Test 2: it must be paid from a count.
func TestActionSurgeMustBePaidFromACount(t *testing.T) {
	ref := *refs.Features.ActionSurge()
	_, err := features.DescribeFromRef(&features.DescribeFromRefInput{Ref: ref, Giver: surgeGiver(t)})
	require.Error(t, err)
	assert.Equal(t, "action surge must be paid from a count: its giver gives it uses or a cost", err.Error())

	assert.True(t, features.MustBeCounted(ref))
	assert.False(t, features.MustBeCounted(*refs.Features.SecondWind()))
	assert.False(t, features.MustBeCounted(core.Ref{Module: "dnd5e", Type: "features", ID: "no_such"}))

	// The shared checks come first: a bad giver is judged before the count.
	_, err = features.DescribeFromRef(&features.DescribeFromRefInput{Ref: ref})
	require.Error(t, err)
	assert.Equal(t, "action surge giver requires a Ref", err.Error())

	// Second Wind charges a bonus action, so it is described with no pools.
	_, err = features.DescribeFromRef(&features.DescribeFromRefInput{
		Ref: *refs.Features.SecondWind(), Parameters: json.RawMessage(`{"level":1}`), Giver: surgeGiver(t),
	})
	assert.NoError(t, err)
}
