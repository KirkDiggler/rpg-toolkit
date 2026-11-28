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
)

func surgeCloak(t *testing.T, authored string) magicitems.Data {
	t.Helper()
	out, err := magicitems.Compile(&magicitems.CompileInput{ID: "surge-cloak", Authored: json.RawMessage(authored)})
	require.NoError(t, err)
	return newItem(t, "surge-cloak", out.Description)
}

// Test 6: a worn item that lets its wearer use Action Surge, for a charge and
// from the entry's own uses.
func TestAWornItemGivesActionSurgeForACharge(t *testing.T) {
	cloak := surgeCloak(t, `{"name":"Surge cloak","slot":"cloak","charges":{"max":1,"refills":{"on":["long rest"]}},`+
		`"lets":[{"use":"action surge","cost":{"charges":1}}]}`)
	c := wearer(t, classes.Wizard, map[InventorySlot]string{SlotCloak: "surge-cloak"}, cloak)

	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "surge-cloak", Grant: "action-surge"})
	require.NoError(t, err)
	assert.Equal(t, "Action Surge", out.Definition.Name)
	assert.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}, out.Definition.Cost.GrantSlots)
	assert.Equal(t, 1, out.Definition.Cost.Pools[ItemPoolKey("surge-cloak", "charges")])

	require.Equal(t, 1, c.actionEconomy.ActionsRemaining)
	require.NoError(t, combat.Pay(c, out.Definition.Cost))
	assert.Equal(t, 2, c.actionEconomy.ActionsRemaining, "one more action")
	assert.Equal(t, 0, cloakOf(c, "surge-cloak").Left["charges"], "for the item's charge")
	require.Error(t, combat.Pay(c, out.Definition.Cost), "and no second")
}

func TestAWornItemGivesActionSurgeFromTheEntrysOwnUses(t *testing.T) {
	cloak := surgeCloak(t, `{"name":"Surge cloak","slot":"cloak","lets":[`+
		`{"use":"action surge","uses":{"max":1,"refills":{"on":["short rest"]}}}]}`)
	c := wearer(t, classes.Wizard, map[InventorySlot]string{SlotCloak: "surge-cloak"}, cloak)

	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "surge-cloak", Grant: "action-surge"})
	require.NoError(t, err)
	assert.Equal(t, 1, out.Definition.Cost.Pools[ItemPoolKey("surge-cloak", "action-surge")])
	assert.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}, out.Definition.Cost.GrantSlots)

	require.NoError(t, combat.Pay(c, out.Definition.Cost))
	assert.Equal(t, 2, c.actionEconomy.ActionsRemaining)
	assert.Equal(t, 0, cloakOf(c, "surge-cloak").Left["action-surge"])
	require.Error(t, combat.Pay(c, out.Definition.Cost))
}
