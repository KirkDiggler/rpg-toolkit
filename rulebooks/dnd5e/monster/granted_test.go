// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// Test 3.
func TestTheRulesDescribeItWithTheCreatureAsItsGiver(t *testing.T) {
	m := creatureWithAKit(t)

	out, err := m.GrantedAbility(&monster.GrantedAbilityInput{Grant: "second-wind"})
	require.NoError(t, err)
	definition := out.Definition

	assert.Equal(t, "Second Wind", definition.Name)
	t.Logf("description: %s", definition.Description)

	require.NotNil(t, definition.Cost)
	assert.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, definition.Cost.Slots)
	assert.Len(t, definition.Cost.Pools, 1)
	assert.Equal(t, 1, definition.Cost.Pools[monster.PoolKey("second-wind")])

	require.NotNil(t, definition.Effect)
	healing := definition.Effect.Healing
	require.NotNil(t, healing)
	assert.Equal(t, "1d10", healing.Dice)
	require.Len(t, healing.Modifiers, 1)
	modifier := healing.Modifiers[0]
	assert.Equal(t, 1, modifier.Amount, "plus the level")
	assert.Equal(t, "Human", modifier.Source.Label, "the creature's name is the label")
	assert.Equal(t, "Human", modifier.Source.Name)
	assert.Equal(t, "g-1", modifier.Source.SourceID, "and its id is the source")
	require.NotNil(t, modifier.Source.Ref)
	assert.Equal(t, testRef.String(), modifier.Source.Ref.String())

	// The description shares nothing with the sheet's own ref.
	modifier.Source.Ref.ID = "changed"
	assert.Equal(t, "test-template", m.Ref().ID)
}

func TestAGrantTheKitDoesNotHaveIsRefusedByName(t *testing.T) {
	m := creatureWithAKit(t)

	_, err := m.GrantedAbility(&monster.GrantedAbilityInput{Grant: "rage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `stat block "Human" has no grant "rage"`)

	_, err = m.GrantedAbility(nil)
	require.Error(t, err)
	assert.Equal(t, []string{"second-wind"}, m.Grants())
}
