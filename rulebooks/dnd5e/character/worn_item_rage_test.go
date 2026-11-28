// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// Test 8: an item lets a wearer who is not a barbarian rage, with no code of
// the character's own for Rage.
func TestAWornItemLetsItsWearerRage(t *testing.T) {
	out, err := magicitems.Compile(&magicitems.CompileInput{ID: "rage-cloak", Authored: json.RawMessage(
		`{"name":"Rage cloak","slot":"cloak","charges":{"max":1,"refills":{"on":["long rest"]}},` +
			`"lets":[{"use":"rage","with":{"damage_bonus":2},"cost":{"charges":1}}]}`)})
	require.NoError(t, err)
	cloak := newItem(t, "rage-cloak", out.Description)
	c := wearer(t, classes.Wizard, map[InventorySlot]string{SlotCloak: "rage-cloak"}, cloak)

	var rows []AvailableAbility
	for _, a := range c.AvailableAbilities() {
		if a.Giver != nil && a.Giver.ItemID == "rage-cloak" {
			rows = append(rows, a)
		}
	}
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "Rage", row.Name)
	assert.Equal(t, *refs.Features.Rage(), *row.Ref)
	assert.Equal(t, coreCombat.ActionBonus, row.ActionType)
	assert.Equal(t, TargetKindSelf, row.TargetKind)
	assert.Equal(t, 1, row.ResourceCurrent)
	assert.Equal(t, 1, row.ResourceMax)
	assert.True(t, row.CanUse)

	granted, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "rage-cloak", Grant: "rage"})
	require.NoError(t, err)
	assert.Equal(t, "Rage", granted.Definition.Name)
	assert.Equal(t, 1, granted.Definition.Cost.Pools[ItemPoolKey("rage-cloak", "charges")])
	assert.JSONEq(t, `{"damage_bonus":{"fixed":2}}`, string(granted.Definition.Effect.Condition.Parameters))

	require.Equal(t, 1, c.actionEconomy.BonusActionsRemaining)
	require.NoError(t, combat.Pay(c, granted.Definition.Cost))
	assert.Equal(t, 0, c.actionEconomy.BonusActionsRemaining, "her bonus action is spent")
	assert.Equal(t, 0, cloakOf(c, "rage-cloak").Left["charges"], "and the charge")
}
