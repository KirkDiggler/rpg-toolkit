// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

// ownUsesCloak is an item written the way a creature's stat block will be: no
// `charges`, and the one entry carries its own count.
func ownUsesCloak(t *testing.T, refills string) magicitems.Data {
	t.Helper()
	out, err := magicitems.Compile(&magicitems.CompileInput{
		ID: "wind-cloak",
		Authored: json.RawMessage(`{"name": "Wind cloak", "slot": "cloak", "lets": [` +
			`{"use": "second wind", "with": {"level": 1}, "uses": {"max": 1, "refills": {"on": ` + refills + `}}}]}`),
	})
	require.NoError(t, err)
	return newItem(t, "wind-cloak", out.Description)
}

func windLeft(c *Character) int { return cloakOf(c, "wind-cloak").Left["second-wind"] }

func wornOwnUses(t *testing.T, refills string) *Character {
	t.Helper()
	return wearer(t, classes.Wizard, map[InventorySlot]string{SlotCloak: "wind-cloak"}, ownUsesCloak(t, refills))
}

// Test 5: an item whose one entry carries its own count, played on a sheet.
func TestAWornItemPaidFromItsEntrysOwnUsesGrantsPaysAndComesBack(t *testing.T) {
	ctx := context.Background()
	c := wornOwnUses(t, `["short rest"]`)
	markSaved(c)

	entries := secondWindEntries(c)
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].Giver)
	assert.Equal(t, AbilityGiver{ItemID: "wind-cloak", ItemName: "Wind cloak", Grant: "second-wind"}, *entries[0].Giver)
	assert.Equal(t, "Second Wind", entries[0].Name)
	assert.True(t, entries[0].CanUse)
	assert.Equal(t, 1, entries[0].ResourceCurrent)
	assert.Equal(t, 1, entries[0].ResourceMax)

	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "wind-cloak", Grant: "second-wind"})
	require.NoError(t, err)
	assert.Equal(t, 1, out.Definition.Cost.Pools[ItemPoolKey("wind-cloak", "second-wind")])
	require.NoError(t, combat.Pay(c, out.Definition.Cost))
	assert.Equal(t, 0, c.actionEconomy.BonusActionsRemaining)
	assert.Equal(t, 0, windLeft(c))
	assert.True(t, c.IsDirty())

	_, err = c.StartTurn(ctx, &StartTurnInput{TurnNumber: 2, Speed: 30})
	require.NoError(t, err)
	err = combat.Pay(c, out.Definition.Cost)
	require.Error(t, err, "a second use the same rest is refused")
	assert.Equal(t, 1, c.actionEconomy.BonusActionsRemaining, "a refused payment spends nothing")
	refused := secondWindEntries(c)[0]
	assert.False(t, refused.CanUse)
	assert.Contains(t, refused.Reason, "Wind cloak")
	assert.Contains(t, refused.Reason, "second-wind")

	d, err := c.ToData()
	require.NoError(t, err)
	require.Len(t, d.Items, 1)
	assert.Equal(t, map[string]int{"second-wind": 0}, d.Items[0].Left)
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"left":{"second-wind":0}`)

	_, err = c.ShortRest(ctx, &ShortRestInput{})
	require.NoError(t, err)
	assert.Equal(t, 1, windLeft(c))

	// A long rest satisfies a short-rest reset too, as it does for `charges`.
	require.NoError(t, combat.Pay(c, out.Definition.Cost))
	_, err = c.LongRest(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, windLeft(c))
}

func TestAnEntrysOwnUsesComeBackOnALongRestWhenItNamesOne(t *testing.T) {
	ctx := context.Background()
	c := wornOwnUses(t, `["long rest"]`)
	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "wind-cloak", Grant: "second-wind"})
	require.NoError(t, err)
	require.NoError(t, combat.Pay(c, out.Definition.Cost))

	_, err = c.ShortRest(ctx, &ShortRestInput{})
	require.NoError(t, err)
	assert.Equal(t, 0, windLeft(c))
	_, err = c.LongRest(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, windLeft(c))
}

// Design 4: what a player is shown for a pool that is not `charges`.
func TestTheSheetNamesAnEntrysOwnPoolByItsAbilityAndItem(t *testing.T) {
	c := wornOwnUses(t, `["short rest"]`)
	item := cloakOf(c, "wind-cloak")

	assert.Equal(t, "Second Wind · Wind cloak", itemPoolName(item, "second-wind"))
	assert.Equal(t, "Second Wind · 1 of 1 second-wind", ownedItemStatLine(item))
	rows := c.wornItemResources()
	require.Len(t, rows, 1)
	assert.Equal(t, ResourceView{Key: ItemPoolKey("wind-cloak", "second-wind"), Name: "Second Wind · Wind cloak", Current: 1, Maximum: 1}, rows[0])
}
