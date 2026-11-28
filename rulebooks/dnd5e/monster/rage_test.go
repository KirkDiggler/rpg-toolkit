// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

const rageLets = `[{"use":"rage","with":{"damage_bonus":2},"uses":{"max":1,"refills":{"on":["long rest"]}}}]`

// Test 9: a stat block lets its creature rage, with no code of the monster
// package's own for Rage.
func TestAStatBlockLetsItsCreatureRage(t *testing.T) {
	in := humanOver(rageLets)
	made, err := monster.FromTemplate(&in)
	require.NoError(t, err)

	abilities := abilitiesOf(t, made.ToData())
	require.Len(t, abilities, 1)
	ability := abilities[0]
	assert.Equal(t, "rage", ability.Word)
	assert.Equal(t, "Rage", ability.Name)
	assert.True(t, ability.Payable)
	assert.Equal(t, 1, ability.Definition.Cost.Pools[monster.PoolKey("rage")])
	assert.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, ability.Definition.Cost.Slots)
	assert.JSONEq(t, `{"damage_bonus":{"fixed":2}}`, string(ability.Definition.Effect.Condition.Parameters))

	granted, err := made.GrantedAbility(&monster.GrantedAbilityInput{Grant: "rage"})
	require.NoError(t, err)
	assert.Equal(t, ability.Definition, granted.Definition)

	t.Run("its bonus action spent", func(t *testing.T) {
		creature, err := monster.LoadFromData(context.Background(), made.ToData(), events.NewEventBus())
		require.NoError(t, err)
		require.NoError(t, combat.Pay(creature, &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}}))
		spent := abilitiesOf(t, creature.ToData())
		require.Len(t, spent, 1)
		assert.False(t, spent[0].Payable)
	})

	t.Run("its use spent", func(t *testing.T) {
		creature, err := monster.LoadFromData(context.Background(), made.ToData(), events.NewEventBus())
		require.NoError(t, err)
		require.NoError(t, combat.Pay(creature, ability.Definition.Cost))
		spent := abilitiesOf(t, creature.ToData())
		require.Len(t, spent, 1)
		assert.False(t, spent[0].Payable)
	})
}
