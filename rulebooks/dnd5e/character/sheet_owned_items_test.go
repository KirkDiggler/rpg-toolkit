// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

// Bytes captured from origin/main at ce7eab72 for the draft-made fighter, before
// either view was changed. The only difference the change may make is the new
// Custom field, which is false on every catalogue row.
const baselineEquipmentView = `
{"Items":[{"ItemID":"case-map","Name":"Case, Map or Scroll","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"blanket","Name":"Blanket","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"clothes-common","Name":"Clothes, Common","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"herbalism-kit","Name":"Herbalism Kit","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"chain-mail","Name":"Chain Mail","Kind":"armor","SlotKeys":["armor"],"StatLine":"AC 16 · heavy","Quantity":1},{"ItemID":"shield","Name":"Shield","Kind":"shield","SlotKeys":["off_hand"],"StatLine":"+2 AC","Quantity":1},{"ItemID":"longsword","Name":"Longsword","Kind":"weapon","SlotKeys":["main_hand","off_hand"],"StatLine":"1d8 slashing damage · versatile","Quantity":1},{"ItemID":"light-crossbow","Name":"Light Crossbow","Kind":"weapon","SlotKeys":["main_hand"],"StatLine":"1d8 piercing damage · ammunition, loading, two-handed","Quantity":1},{"ItemID":"bolts-20","Name":"Crossbow bolts (20)","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"backpack","Name":"Backpack","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"crowbar","Name":"Crowbar","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"hammer","Name":"Hammer","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"piton","Name":"Piton","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":10},{"ItemID":"torch","Name":"Torch","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":10},{"ItemID":"tinderbox","Name":"Tinderbox","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"rations","Name":"Rations (1 Day)","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":10},{"ItemID":"waterskin","Name":"Waterskin","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1},{"ItemID":"hempen-rope","Name":"Hempen Rope (50 feet)","Kind":"gear","SlotKeys":null,"StatLine":"","Quantity":1}],"Equipped":null,"Slots":[{"Key":"main_hand","DisplayLabel":"Main hand","Accepts":["weapon"]},{"Key":"off_hand","DisplayLabel":"Off hand","Accepts":["weapon","shield"]},{"Key":"armor","DisplayLabel":"Armor","Accepts":["armor"]}],"ACTotal":12,"ACNote":"10 + 2 DEX","MainHandDamage":""}`

const baselineStatusView = `
{"Level":1,"HitPoints":{"Current":12,"Maximum":12},"LifeState":"conscious","DeathSaves":null,"BaseSpeedFeet":30,"Features":[{"Ref":"dnd5e:features:second_wind","Name":"Second Wind","Detail":"","ResourceKey":"second_wind"}],"Conditions":[{"Ref":"dnd5e:conditions:fighting_style_defense","Name":"Defense","Detail":"","SourceMember":null},{"Ref":"dnd5e:conditions:opportunity_attack","Name":"Opportunity Attack","Detail":"Use your reaction to make one melee attack against a creature that leaves your reach.","SourceMember":null}],"Resources":[{"Key":"hit_dice","Name":"Hit Dice","Current":1,"Maximum":1},{"Key":"second_wind","Name":"Second Wind","Current":1,"Maximum":1}]}`

// Test 1.
func TestACharacterWithNoCustomItemsSeesWhatTheySawBefore(t *testing.T) {
	c := wearer(t, classes.Fighter, nil)

	ev, err := c.EquipmentView(context.Background())
	require.NoError(t, err)
	raw, err := json.Marshal(ev)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(baselineEquipmentView), strings.ReplaceAll(string(raw), `,"Custom":false`, ""))
	for _, row := range ev.Items {
		assert.False(t, row.Custom, row.ItemID)
	}

	sv, err := c.StatusView(nil)
	require.NoError(t, err)
	raw, err = json.Marshal(sv.View)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(baselineStatusView), string(raw))
}

func viewsOf(t *testing.T, c *Character) (*EquipmentView, []ResourceView) {
	t.Helper()
	ev, err := c.EquipmentView(context.Background())
	require.NoError(t, err)
	sv, err := c.StatusView(nil)
	require.NoError(t, err)
	return ev, sv.View.Resources
}

func slotKeysOf(ev *EquipmentView) []string {
	keys := make([]string, 0, len(ev.Slots))
	for _, s := range ev.Slots {
		keys = append(keys, s.Key)
	}
	return keys
}

func customRows(ev *EquipmentView) []EquippedItemView {
	var out []EquippedItemView
	for _, row := range ev.Items {
		if row.Custom {
			out = append(out, row)
		}
	}
	return out
}

const cloakPool = "item:cloak-1/charges"

// Test 2.
func TestAnOwnedCloakIsAnInventoryRowWithASlotToPutItIn(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, nil, cloak)
	before := wearer(t, classes.Fighter, nil)

	ev, resources := viewsOf(t, c)
	base, baseResources := viewsOf(t, before)

	require.Len(t, ev.Items, len(base.Items)+1)
	assert.Equal(t, base.Items, ev.Items[:len(base.Items)], "the catalogue rows are as before")
	assert.Equal(t, EquippedItemView{
		ItemID: "cloak-1", Name: "Lieutenant's cloak", Kind: "cloak", SlotKeys: []string{"cloak"},
		StatLine: "Second Wind · 1 of 1 charges", Quantity: 1, Custom: true,
	}, ev.Items[len(ev.Items)-1])

	require.Len(t, ev.Slots, 4)
	assert.Equal(t, base.Slots, ev.Slots[:3])
	assert.Equal(t, SlotDefView{Key: "cloak", DisplayLabel: "Cloak", Accepts: []string{"cloak"}}, ev.Slots[3])
	assert.NotContains(t, ev.Equipped, SlotCloak)
	assert.Equal(t, baseResources, resources, "an item in the pack adds no resource row")
}

// Test 3.
func TestAWornCloaksCountIsOnTheSheetNamedWithItsAbilityAndTheItem(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)

	ev, resources := viewsOf(t, c)
	assert.Equal(t, "cloak-1", ev.Equipped[SlotCloak])
	require.Len(t, resources, 3)
	assert.Equal(t, ResourceView{Key: "second_wind", Name: "Second Wind", Current: 1, Maximum: 1}, resources[1], "their own is unchanged")
	assert.Equal(t, ResourceView{Key: ItemPoolKey("cloak-1", "charges"), Name: "Second Wind · Lieutenant's cloak", Current: 1, Maximum: 1}, resources[2])
	assert.Equal(t, cloakPool, string(resources[2].Key))
}

// Test 4.
func TestAPaidUseShowsOnTheStatLineAndTheResourceRow(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)
	require.NoError(t, payCloak(t, c, "cloak-1"))

	ev, resources := viewsOf(t, c)
	rows := customRows(ev)
	require.Len(t, rows, 1)
	assert.Equal(t, "Second Wind · 0 of 1 charges", rows[0].StatLine)
	assert.Equal(t, 0, resources[len(resources)-1].Current)
	assert.Equal(t, 1, resources[len(resources)-1].Maximum)
}

// Test 5.
func TestARingFitsBothRingSlots(t *testing.T) {
	ring := newItem(t, "ring-1", ringDescription("ring-1"))
	c := wearer(t, classes.Fighter, nil, ring)

	ev, _ := viewsOf(t, c)
	rows := customRows(ev)
	require.Len(t, rows, 1)
	assert.Equal(t, "ring", rows[0].Kind)
	assert.Equal(t, []string{"ring_left", "ring_right"}, rows[0].SlotKeys)
	require.Len(t, ev.Slots, 5)
	assert.Equal(t, SlotDefView{Key: "ring_left", DisplayLabel: "Ring (left)", Accepts: []string{"ring"}}, ev.Slots[3])
	assert.Equal(t, SlotDefView{Key: "ring_right", DisplayLabel: "Ring (right)", Accepts: []string{"ring"}}, ev.Slots[4])
}

// Test 6.
func TestAWizardWearingTheCloakGetsBothViews(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Wizard, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)

	ev, resources := viewsOf(t, c)
	assert.Len(t, customRows(ev), 1)
	require.NotEmpty(t, resources)
	assert.Equal(t, ResourceView{Key: ItemPoolKey("cloak-1", "charges"), Name: "Second Wind · Lieutenant's cloak", Current: 1, Maximum: 1}, resources[len(resources)-1])
}

// Test 7.
func TestAnItemWithNoPoolShowsTheAbilityAloneAndAddsNoRow(t *testing.T) {
	d := cloakDescription()
	d.Pools = nil
	d.Grants = map[string]magicitems.Grant{"second-wind": {Ability: d.Grants["second-wind"].Ability, Parameters: d.Grants["second-wind"].Parameters}}
	cloak := newItem(t, "cloak-1", d)
	c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)
	base, baseResources := viewsOf(t, wearer(t, classes.Fighter, nil))
	_ = base

	ev, resources := viewsOf(t, c)
	rows := customRows(ev)
	require.Len(t, rows, 1)
	assert.Equal(t, "Second Wind", rows[0].StatLine)
	assert.Equal(t, baseResources, resources)
}

// Test 8.
func TestOwningTheItemIsWhatListsItsSlot(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)
	require.NoError(t, c.UnequipItem(SlotCloak))

	ev, _ := viewsOf(t, c)
	assert.Contains(t, slotKeysOf(ev), "cloak")
	assert.Len(t, customRows(ev), 1)

	d, err := c.ToData()
	require.NoError(t, err)
	_, err = RemoveItem(d, "cloak-1")
	require.NoError(t, err)
	reloaded, err := LoadFromData(context.Background(), d, events.NewEventBus())
	require.NoError(t, err)
	ev, _ = viewsOf(t, reloaded)
	assert.Equal(t, []string{"main_hand", "off_hand", "armor"}, slotKeysOf(ev))
	assert.Empty(t, customRows(ev))
}
