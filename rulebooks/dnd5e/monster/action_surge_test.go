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
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// actionSurgeLets is a stat block's one entry: Action Surge with its own one
// use, written the way an author writes it.
const actionSurgeLets = `[{"use":"action surge","uses":{"max":1,"refills":{"on":["short rest","long rest"]}}}]`

// Test 5: a stat block lets its creature use Action Surge with no code of its
// own, and the gate pays one use for one more action.
func TestAStatBlockLetsItsCreatureActionSurge(t *testing.T) {
	ctx := context.Background()
	in := humanOver(actionSurgeLets)
	made, err := monster.FromTemplate(&in)
	require.NoError(t, err)

	abilities := abilitiesOf(t, made.ToData())
	require.Len(t, abilities, 1)
	ability := abilities[0]
	assert.Equal(t, "action surge", ability.Word)
	assert.Equal(t, "action-surge", ability.Grant)
	assert.Equal(t, "Action Surge", ability.Name)
	assert.True(t, ability.Payable)
	require.NotNil(t, ability.Definition)
	assert.Equal(t, 1, ability.Definition.Cost.Pools[monster.PoolKey("action-surge")])
	assert.Len(t, ability.Definition.Cost.Pools, 1)
	assert.Empty(t, ability.Definition.Cost.Slots, "no slot is charged")
	assert.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}, ability.Definition.Cost.GrantSlots)

	bus := events.NewEventBus()
	creature, err := monster.LoadFromData(ctx, made.ToData(), bus)
	require.NoError(t, err)
	require.Equal(t, 1, creature.SlotsLeft(coreCombat.ActionStandard))

	require.NoError(t, combat.Pay(creature, ability.Definition.Cost))
	assert.Equal(t, 2, creature.SlotsLeft(coreCombat.ActionStandard), "one more action")
	assert.Equal(t, 0, creature.PoolLeft(monster.PoolKey("action-surge")), "and no use left")

	require.Error(t, combat.Pay(creature, ability.Definition.Cost), "a second payment is refused")
	assert.Equal(t, 2, creature.SlotsLeft(coreCombat.ActionStandard), "and nothing moved")
	assert.False(t, combat.CanPay(creature, ability.Definition.Cost))

	spent := abilitiesOf(t, creature.ToData())
	require.Len(t, spent, 1)
	assert.False(t, spent[0].Payable, "the stored sheet says the use is gone")

	require.NoError(t, dnd5eEvents.TurnStartTopic.On(bus).Publish(ctx,
		dnd5eEvents.TurnStartEvent{SubjectID: creature.GetID(), Round: 2}))
	assert.Equal(t, 1, creature.SlotsLeft(coreCombat.ActionStandard), "the gift ends with the turn")
	assert.Equal(t, 0, creature.PoolLeft(monster.PoolKey("action-surge")), "the use does not come back with it")
}
