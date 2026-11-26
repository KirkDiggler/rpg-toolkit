// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// fighterData is the fighter the item tests share: a longsword in hand and no
// item of any other kind.
func fighterData() *Data {
	return &Data{
		ID: "char-1", PlayerID: "player-1", Name: "Serialized", Level: 1, ProficiencyBonus: 2,
		RaceID: races.Human, ClassID: classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 14, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints: 12, MaxHitPoints: 12,
		Inventory:      []InventoryItemData{{Type: shared.EquipmentTypeWeapon, ID: "longsword", Quantity: 1}},
		EquipmentSlots: EquipmentSlots{SlotMainHand: "longsword"},
	}
}

func cloakDescription() magicitems.Description {
	return magicitems.Description{
		Ref:   core.Ref{Module: "site", Type: "items", ID: "lieutenants-cloak"},
		Name:  "Lieutenant's cloak",
		Slots: []string{"cloak"},
		Pools: map[string]magicitems.Pool{
			"charges": {Maximum: 1, RestoreOn: []coreResources.ResetType{coreResources.ResetShortRest, coreResources.ResetLongRest}},
		},
		Grants: map[string]magicitems.Grant{
			"second-wind": {
				Ability:    *refs.Features.SecondWind(),
				Parameters: json.RawMessage(`{"level":1}`),
				Pool:       "charges",
				Uses:       1,
			},
		},
	}
}

func ringDescription(id string) magicitems.Description {
	d := cloakDescription()
	d.Ref.ID = core.ID(id)
	d.Name = "Ring " + id
	d.Slots = []string{"ring_left", "ring_right"}
	return d
}

func newItem(t *testing.T, id string, d magicitems.Description) magicitems.Data {
	t.Helper()
	item, err := magicitems.New(id, d)
	require.NoError(t, err)
	return item
}

func loadWith(t *testing.T, items ...magicitems.Data) (*Data, *Character) {
	t.Helper()
	d := fighterData()
	for _, item := range items {
		require.NoError(t, AddItem(d, item))
	}
	c, err := Load(context.Background(), d)
	require.NoError(t, err)
	return d, c
}

// Test 1. The bytes below were captured from this same fighter on
// origin/main (28b42774) before data.go changed; UpdatedAt is zeroed because
// ToData stamps the clock.
const fighterWithoutItemsJSON = `{"id":"char-1","player_id":"player-1","name":"Serialized","level":1,"proficiency_bonus":2,"levels":[{"level":1,"class_id":"fighter","hit_point_gain":12,"hit_point_method":"max"}],"experience":0,"race_id":"human","class_id":"fighter","background_id":"","ability_scores":{"cha":10,"con":14,"dex":12,"int":10,"str":14,"wis":10},"hit_points":12,"max_hit_points":12,"skills":null,"saving_throws":null,"languages":null,"armor_proficiencies":null,"weapon_proficiencies":null,"tool_proficiencies":null,"inventory":[{"type":"weapon","id":"longsword","quantity":1}],"wallet":{"copper":0},"equipment_slots":{"main_hand":"longsword"},"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}`

func TestACharacterWithNoItemsSavesTheBytesItSavedBefore(t *testing.T) {
	c, err := Load(context.Background(), fighterData())
	require.NoError(t, err)
	d, err := c.ToData()
	require.NoError(t, err)
	d.UpdatedAt = d.CreatedAt

	raw, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Equal(t, fighterWithoutItemsJSON, string(raw))
}

// Test 4.
func TestAnAddedItemLoadsWithItsDescriptionAndCount(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	d := fighterData()
	require.NoError(t, AddItem(d, cloak))

	cloak.Left["charges"] = 0 // the caller's copy is not the saved one
	cloak.Description.Slots[0] = "belt"
	assert.Equal(t, 1, d.Items[0].Left["charges"])
	assert.Equal(t, []string{"cloak"}, d.Items[0].Description.Slots)

	c, err := Load(context.Background(), d)
	require.NoError(t, err)

	items := c.Items()
	require.Len(t, items, 1)
	assert.Equal(t, "cloak-1", items[0].ID)
	assert.Equal(t, "Lieutenant's cloak", items[0].Description.Name)
	assert.Equal(t, 1, items[0].Left["charges"])

	items[0].Left["charges"] = 0
	items[0].Description.Name = "changed"
	assert.Equal(t, 1, c.Items()[0].Left["charges"], "a returned item is a copy")
	assert.Equal(t, "Lieutenant's cloak", c.Items()[0].Description.Name)

	d.Items[0].Left["charges"] = 0
	assert.Equal(t, 1, c.Items()[0].Left["charges"], "the sheet does not share the saved data's maps")
}

func TestItemsAreListedInIdOrder(t *testing.T) {
	_, c := loadWith(t,
		newItem(t, "b", ringDescription("b")),
		newItem(t, "a", ringDescription("a")))
	items := c.Items()
	require.Len(t, items, 2)
	assert.Equal(t, []string{"a", "b"}, []string{items[0].ID, items[1].ID})
}

// Test 5.
func TestACloakIsWornInItsSlotAndTakenOffKeepingItsCount(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	cloak.Left["charges"] = 0
	_, c := loadWith(t, cloak)

	require.NoError(t, c.EquipItem(SlotCloak, "cloak-1"))
	saved, err := c.ToData()
	require.NoError(t, err)
	assert.Equal(t, "cloak-1", saved.EquipmentSlots[SlotCloak])
	assert.Equal(t, "longsword", saved.EquipmentSlots[SlotMainHand], "the weapon is untouched")

	err = c.EquipItem(SlotRingLeft, "cloak-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be equipped in that slot")
	assert.Equal(t, "cloak-1", c.equipmentSlots.Get(SlotCloak), "a refused equip changes nothing")

	require.NoError(t, c.UnequipItem(SlotCloak))
	require.Len(t, c.Items(), 1)
	assert.Equal(t, 0, c.Items()[0].Left["charges"], "taking it off keeps its count")
	assert.Empty(t, c.equipmentSlots.Get(SlotCloak))
}

// Test 6.
func TestARingMovesBetweenItsSlotsAndTwoRingsSitTogether(t *testing.T) {
	_, c := loadWith(t,
		newItem(t, "ring-a", ringDescription("a")),
		newItem(t, "ring-b", ringDescription("b")))

	require.NoError(t, c.EquipItem(SlotRingLeft, "ring-a"))
	require.NoError(t, c.EquipItem(SlotRingRight, "ring-a"))
	assert.Equal(t, "ring-a", c.equipmentSlots.Get(SlotRingRight))
	assert.Empty(t, c.equipmentSlots.Get(SlotRingLeft), "it is one thing: it moved")

	require.NoError(t, c.EquipItem(SlotRingLeft, "ring-b"))
	assert.Equal(t, "ring-a", c.equipmentSlots.Get(SlotRingRight))
	assert.Equal(t, "ring-b", c.equipmentSlots.Get(SlotRingLeft))

	require.NoError(t, c.EquipItem(SlotRingLeft, "ring-a"))
	assert.Equal(t, "ring-a", c.equipmentSlots.Get(SlotRingLeft), "a slot holding something else is replaced")
	assert.Empty(t, c.equipmentSlots.Get(SlotRingRight))
	assert.Len(t, c.Items(), 2, "the replaced ring is still owned")
}

func TestAnItemNotOwnedCannotBeWorn(t *testing.T) {
	_, c := loadWith(t)
	err := c.EquipItem(SlotCloak, "nothing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// Test 7.
func TestTheRecordSurvivesSaveAndLoadAndTheItemStaysWorn(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	cloak.Left["charges"] = 0
	d := fighterData()
	require.NoError(t, AddItem(d, cloak))
	d.EquipmentSlots[SlotCloak] = "cloak-1"

	c, err := Load(context.Background(), d)
	require.NoError(t, err)
	saved, err := c.ToData()
	require.NoError(t, err)

	raw, err := json.Marshal(saved)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"left":{"charges":0}`)
	assert.Contains(t, string(raw), `"cloak":"cloak-1"`)

	again, err := Load(context.Background(), saved)
	require.NoError(t, err)
	assert.Equal(t, "cloak-1", again.equipmentSlots.Get(SlotCloak))
	assert.Equal(t, 0, again.Items()[0].Left["charges"])
}

// Test 8.
func TestALoadRefusesOrDropsAnItemItCannotAccept(t *testing.T) {
	good := newItem(t, "ring-a", ringDescription("a"))
	wornSlot := func(id string) magicitems.Data {
		d := cloakDescription()
		d.Slots = []string{"main_hand"}
		return newItem(t, id, d)
	}
	cases := []struct {
		name  string
		bad   func() []magicitems.Data
		badID string
		// kept: the id stays owned after a lenient load (a repeated id keeps its first).
		kept bool
	}{
		{"one id twice", func() []magicitems.Data {
			c := newItem(t, "dup", cloakDescription())
			return []magicitems.Data{c, c}
		}, "dup", true},
		{"a catalogue id", func() []magicitems.Data {
			return []magicitems.Data{newItem(t, "longsword", cloakDescription())}
		}, "longsword", false},
		{"a pool the description does not declare", func() []magicitems.Data {
			c := newItem(t, "bad", cloakDescription())
			c.Left["mana"] = 1
			return []magicitems.Data{c}
		}, "bad", false},
		{"a count above the maximum", func() []magicitems.Data {
			c := newItem(t, "bad", cloakDescription())
			c.Left["charges"] = 2
			return []magicitems.Data{c}
		}, "bad", false},
		{"a slot outside the worn slots", func() []magicitems.Data {
			return []magicitems.Data{wornSlot("bad")}
		}, "bad", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := fighterData()
			d.Items = append([]magicitems.Data{good}, tc.bad()...)
			d.EquipmentSlots[SlotRingLeft] = "ring-a"
			d.EquipmentSlots[SlotCloak] = tc.badID

			_, err := Load(context.Background(), d)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"`+tc.badID+`"`, "the refusal names the item")

			slotsBefore := EquipmentSlots{}
			for k, v := range d.EquipmentSlots {
				slotsBefore[k] = v
			}
			c, err := LoadFromData(context.Background(), d, events.NewEventBus())
			require.NoError(t, err)
			ids := []string{}
			for _, item := range c.Items() {
				ids = append(ids, item.ID)
			}
			assert.Contains(t, ids, "ring-a", "the good item is kept")
			assert.Equal(t, "ring-a", c.equipmentSlots.Get(SlotRingLeft))
			assert.Equal(t, "longsword", c.equipmentSlots.Get(SlotMainHand))
			assert.Equal(t, slotsBefore, d.EquipmentSlots, "the caller's data is not edited")

			switch {
			case tc.kept:
				assert.Contains(t, ids, tc.badID)
				assert.Equal(t, tc.badID, c.equipmentSlots.Get(SlotCloak), "the kept copy is still worn")
			case tc.badID == "longsword":
				assert.NotContains(t, ids, tc.badID)
				assert.Equal(t, "longsword", c.equipmentSlots.Get(SlotCloak),
					"a slot naming a catalogue id names the catalogue row, which is not dropped")
			default:
				assert.NotContains(t, ids, tc.badID)
				assert.Empty(t, c.equipmentSlots.Get(SlotCloak), "a slot that named the dropped item is cleared")
			}
		})
	}
}

func TestAddItemRefusesWhatALoadWouldRefuse(t *testing.T) {
	hand := cloakDescription()
	hand.Slots = []string{"armor"}
	cases := []struct {
		name string
		item magicitems.Data
		says string
	}{
		{"catalogue id", newItem(t, "longsword", cloakDescription()), `"longsword"`},
		{"hand or armour slot", newItem(t, "cloak-1", hand), "catalogue"},
		{"invalid", magicitems.Data{ID: "x"}, `"x"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := fighterData()
			err := AddItem(d, tc.item)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.says)
			assert.Empty(t, d.Items)
		})
	}

	d := fighterData()
	require.NoError(t, AddItem(d, newItem(t, "cloak-1", cloakDescription())))
	err := AddItem(d, newItem(t, "cloak-1", cloakDescription()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"cloak-1"`)
	assert.Len(t, d.Items, 1)

	require.Error(t, AddItem(nil, newItem(t, "cloak-2", cloakDescription())))
}

// Test 9.
func TestRemoveItemReturnsItAsItWasAndClearsItsSlot(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	cloak.Left["charges"] = 0
	d := fighterData()
	require.NoError(t, AddItem(d, cloak))
	require.NoError(t, AddItem(d, newItem(t, "ring-a", ringDescription("a"))))
	d.EquipmentSlots[SlotCloak] = "cloak-1"
	d.EquipmentSlots[SlotRingLeft] = "ring-a"

	got, err := RemoveItem(d, "cloak-1")
	require.NoError(t, err)
	assert.Equal(t, cloak, got)
	assert.Empty(t, d.EquipmentSlots[SlotCloak])
	assert.Equal(t, "ring-a", d.EquipmentSlots[SlotRingLeft])
	assert.Equal(t, "longsword", d.EquipmentSlots[SlotMainHand])
	require.Len(t, d.Items, 1)

	_, err = RemoveItem(d, "cloak-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"cloak-1"`)
	_, err = RemoveItem(nil, "cloak-1")
	require.Error(t, err)
}

// Test 10 (the weapon half; every existing test in this package is the rest).
func TestAWeaponStillEquipsMovesAndUnequipsWithItemsOwned(t *testing.T) {
	_, c := loadWith(t, newItem(t, "cloak-1", cloakDescription()))
	require.NoError(t, c.EquipItem(SlotCloak, "cloak-1"))

	require.NoError(t, c.EquipItem(SlotOffHand, "longsword"))
	assert.Equal(t, "longsword", c.equipmentSlots.Get(SlotOffHand))
	assert.Empty(t, c.equipmentSlots.Get(SlotMainHand))
	require.NoError(t, c.UnequipItem(SlotOffHand))
	assert.Empty(t, c.equipmentSlots.Get(SlotOffHand))
	assert.Equal(t, "cloak-1", c.equipmentSlots.Get(SlotCloak))
	assert.Error(t, c.EquipItem(SlotCloak, "longsword"), "a weapon still has no worn slot")
}

// Test 11.
func TestPlanAndPriceReadAnOwnedItemLikeArmour(t *testing.T) {
	_, c := loadWith(t, newItem(t, "cloak-1", cloakDescription()))

	plan, err := c.PlanEquipment(&PlanEquipmentInput{Slot: SlotCloak, ItemID: "cloak-1"})
	require.NoError(t, err)
	assert.Empty(t, plan.Stowed)
	assert.Equal(t, []*core.Ref{{Module: refs.Module, Type: refs.TypeEquipment, ID: "cloak-1"}}, plan.Drawn)

	_, err = c.StartTurn(context.Background(), &StartTurnInput{TurnNumber: 1, Speed: 30})
	require.NoError(t, err)

	_, err = c.PriceEquipment(&PriceEquipmentInput{Slot: SlotCloak, ItemID: "cloak-1"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrArmorInFight))

	require.NoError(t, c.EquipItem(SlotCloak, "cloak-1"))
	_, err = c.PriceEquipment(&PriceEquipmentInput{Slot: SlotCloak})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrArmorInFight))

	require.NoError(t, c.UnequipItem(SlotMainHand))
	out, err := c.PriceEquipment(&PriceEquipmentInput{Slot: SlotMainHand, ItemID: "longsword"})
	require.NoError(t, err, "a weapon change is priced as before")
	require.NotNil(t, out.Profile)
}

// Test 6. A character that owns a unique item saves and loads it still
// unique; one that owns a plain item saves no `unique` key.
func TestAUniqueItemStaysUniqueOnTheSavedSheet(t *testing.T) {
	uniqueDesc := cloakDescription()
	uniqueDesc.Unique = true
	cases := []struct {
		name   string
		desc   magicitems.Description
		unique bool
	}{
		{"unique", uniqueDesc, true},
		{"plain", cloakDescription(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := fighterData()
			require.NoError(t, AddItem(d, newItem(t, "cloak-1", tc.desc)))
			c, err := Load(context.Background(), d)
			require.NoError(t, err)
			saved, err := c.ToData()
			require.NoError(t, err)
			raw, err := json.Marshal(saved)
			require.NoError(t, err)
			assert.Equal(t, tc.unique, strings.Contains(string(raw), `"unique":true`))
			if !tc.unique {
				assert.NotContains(t, string(raw), "unique")
			}

			var loaded Data
			require.NoError(t, json.Unmarshal(raw, &loaded))
			require.Len(t, loaded.Items, 1)
			assert.Equal(t, tc.unique, loaded.Items[0].Description.Unique)
		})
	}
}
