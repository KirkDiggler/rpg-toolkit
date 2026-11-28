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

const surgeCloakAuthored = `{"name":"Surge cloak","slot":"cloak","charges":{"max":1,"refills":{"on":["long rest"]}},` +
	`"lets":[{"use":"action surge","cost":{"charges":1}}]}`

func surgeEntry(t *testing.T, c *Character) AvailableAbility {
	t.Helper()
	var found []AvailableAbility
	for _, a := range c.AvailableAbilities() {
		if a.Giver != nil && a.Giver.ItemID == "surge-cloak" {
			found = append(found, a)
		}
	}
	require.Len(t, found, 1)
	return found[0]
}

func surgeWearer(t *testing.T) *Character {
	t.Helper()
	return wearer(t, classes.Wizard, map[InventorySlot]string{SlotCloak: "surge-cloak"}, surgeCloak(t, surgeCloakAuthored))
}

// Test 1: she is offered it, free.
func TestAWornItemsFreeAbilityIsOfferedAsAFreeAction(t *testing.T) {
	e := surgeEntry(t, surgeWearer(t))
	assert.Equal(t, "Action Surge", e.Name)
	assert.Equal(t, coreCombat.ActionFree, e.ActionType)
	assert.Equal(t, EconomySlotFree, e.EconomySlot)
	assert.True(t, e.CanUse)
	assert.Equal(t, 1, e.ResourceCurrent)
	assert.Equal(t, 1, e.ResourceMax)
	assert.Empty(t, e.Reason)
}

// Test 2: it does not need her action.
func TestAWornItemsFreeAbilityNeedsNeitherActionNorBonusAction(t *testing.T) {
	c := surgeWearer(t)
	c.actionEconomy.ActionsRemaining = 0
	c.actionEconomy.BonusActionsRemaining = 0
	e := surgeEntry(t, c)
	assert.True(t, e.CanUse)
	assert.Empty(t, e.Reason)
}

// Test 3: it needs the count.
func TestAWornItemsFreeAbilityNeedsTheItemsCount(t *testing.T) {
	c := surgeWearer(t)
	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "surge-cloak", Grant: "action-surge"})
	require.NoError(t, err)
	require.NoError(t, combat.Pay(c, out.Definition.Cost))

	e := surgeEntry(t, c)
	assert.False(t, e.CanUse)
	assert.Equal(t, "the Surge cloak has no charges left", e.Reason)
	assert.Equal(t, 0, e.ResourceCurrent)
}

// Test 4: paying for it at the gate, with the character alone as the ledger.
func TestPayingForAWornItemsFreeAbilitySpendsTheChargeAndGivesAnAction(t *testing.T) {
	c := surgeWearer(t)
	require.True(t, surgeEntry(t, c).CanUse)
	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "surge-cloak", Grant: "action-surge"})
	require.NoError(t, err)
	require.NoError(t, combat.Pay(c, out.Definition.Cost))
	assert.Equal(t, 0, cloakOf(c, "surge-cloak").Left["charges"])
	assert.Equal(t, 2, c.actionEconomy.ActionsRemaining)
}

// Test 5: a one-slot ability is as it was.
func TestAWornItemsOneSlotAbilityIsStillABonusAction(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Wizard, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)
	entries := secondWindEntries(c)
	require.Len(t, entries, 1)
	assert.Equal(t, coreCombat.ActionBonus, entries[0].ActionType)
	assert.Equal(t, EconomySlotBonusAction, entries[0].EconomySlot)
}

// Test 7: out of combat no row is listed, a free one no more than a slot one.
func TestAWornItemsFreeAbilityIsNotListedOutOfCombat(t *testing.T) {
	c := surgeWearer(t)
	require.NotEmpty(t, c.AvailableAbilities())
	c.actionEconomy = nil // not in combat
	assert.Empty(t, c.AvailableAbilities())
}
