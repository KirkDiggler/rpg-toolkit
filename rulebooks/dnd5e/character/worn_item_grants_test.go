// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// sheetOf is a level 3 sheet of one class, with no item and no feature of its own.
func sheetOf(class classes.Class) *Data {
	return &Data{
		Levels: syntheticLevels(class, 3),
		ID:     "grants-1", PlayerID: "p1", Name: "Wearer",
		Level: 3, ProficiencyBonus: 2,
		RaceID: races.Human, ClassID: class,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 24, MaxHitPoints: 28,
	}
}

// wearer loads a sheet of the class owning the items, wears each item in its
// slot, and puts the character in a fight on turn 1.
func wearer(t *testing.T, class classes.Class, worn map[InventorySlot]string, items ...magicitems.Data) *Character {
	t.Helper()
	d := sheetOf(class)
	if class == classes.Fighter {
		// A fighter made through the draft carries its own Second Wind.
		made, err := newFighterDraft(t).ToCharacter(context.Background(), "grants-1", events.NewEventBus())
		require.NoError(t, err)
		d, err = made.ToData()
		require.NoError(t, err)
	}
	for _, item := range items {
		require.NoError(t, AddItem(d, item))
	}
	c, err := LoadFromData(context.Background(), d, events.NewEventBus())
	require.NoError(t, err)
	for slot, id := range worn {
		require.NoError(t, c.EquipItem(slot, id))
	}
	_, err = c.StartTurn(context.Background(), &StartTurnInput{TurnNumber: 1, Speed: 30})
	require.NoError(t, err)
	return c
}

func secondWindEntries(c *Character) []AvailableAbility {
	var out []AvailableAbility
	for _, a := range c.AvailableAbilities() {
		if a.Ref != nil && *a.Ref == *refs.Features.SecondWind() {
			out = append(out, a)
		}
	}
	return out
}

func cloakOf(c *Character, id string) magicitems.Data {
	item, ok := c.ownedItem(id)
	if !ok {
		panic("no item " + id)
	}
	return item
}

// Test 1.
func TestAWornCloakListsItsSecondWindBesideTheFightersOwn(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)

	entries := secondWindEntries(c)
	require.Len(t, entries, 2)
	var own, granted AvailableAbility
	for _, e := range entries {
		if e.Giver == nil {
			own = e
		} else {
			granted = e
		}
	}
	require.NotNil(t, own.Ref, "the fighter's own entry has a nil Giver")
	require.NotNil(t, granted.Giver)
	assert.Equal(t, AbilityGiver{ItemID: "cloak-1", ItemName: "Lieutenant's cloak", Grant: "second-wind"}, *granted.Giver)
	assert.Equal(t, "Second Wind", granted.Name)
	assert.Equal(t, 1, granted.ResourceCurrent)
	assert.Equal(t, 1, granted.ResourceMax)
	assert.True(t, granted.CanUse)
	assert.Equal(t, coreCombat.ActionBonus, granted.ActionType)
	assert.Equal(t, EconomySlotBonusAction, granted.EconomySlot)
	assert.Equal(t, TargetKindSelf, granted.TargetKind)
	assert.NotEmpty(t, granted.Description)
}

// Test 2.
func TestACloakInThePackGrantsNothing(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, nil, cloak)

	entries := secondWindEntries(c)
	require.Len(t, entries, 1)
	assert.Nil(t, entries[0].Giver)
}

// Test 3.
func TestAWizardWearingTheCloakHasOnlyTheCloaksSecondWind(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Wizard, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)

	entries := secondWindEntries(c)
	require.Len(t, entries, 1)
	require.NotNil(t, entries[0].Giver)

	_, ok, err := c.AbilityDefinition(*refs.Features.SecondWind())
	require.NoError(t, err)
	assert.False(t, ok, "AbilityDefinition answers for the character's own features only")

	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "cloak-1", Grant: "second-wind"})
	require.NoError(t, err)
	require.NoError(t, out.Definition.Validate())
	require.NotNil(t, out.Definition.Effect.Healing)
	assert.Equal(t, "1d10", out.Definition.Effect.Healing.Dice)
	require.Len(t, out.Definition.Effect.Healing.Modifiers, 1)
	mod := out.Definition.Effect.Healing.Modifiers[0]
	assert.Equal(t, 1, mod.Amount)
	require.NotNil(t, mod.Source.Ref)
	assert.Equal(t, cloakDescription().Ref, *mod.Source.Ref)
	assert.Equal(t, "Lieutenant's cloak", mod.Source.Name)
	assert.Equal(t, "Lieutenant's cloak", mod.Source.Label)
	assert.Equal(t, "cloak-1", mod.Source.SourceID)
	assert.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, out.Definition.Cost.Slots)
	assert.Equal(t, map[coreResources.ResourceKey]int{"charges": 1}, out.Definition.Cost.Pools)
}

func payCloak(t *testing.T, c *Character, id string) error {
	t.Helper()
	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: id, Grant: "second-wind"})
	require.NoError(t, err)
	return combat.Pay(combat.WithPools(c, out.Budget), out.Definition.Cost)
}

func chargesOf(c *Character, id string) int { return cloakOf(c, id).Left["charges"] }

// Test 4.
func TestTheCloaksUseIsPaidInOnePaymentFromTheCloaksOwnCount(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)
	markSaved(c)

	require.NoError(t, payCloak(t, c, "cloak-1"))

	assert.Equal(t, 0, c.actionEconomy.BonusActionsRemaining)
	assert.Equal(t, 0, chargesOf(c, "cloak-1"))
	assert.Equal(t, 1, c.GetResource(resources.SecondWind).Current(), "their own count is untouched")
	assert.True(t, c.IsDirty())

	d, err := c.ToData()
	require.NoError(t, err)
	require.Len(t, d.Items, 1)
	assert.Equal(t, 0, d.Items[0].Left["charges"])
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"left":{"charges":0}`)
}

// Test 5.
func TestAnEmptyCloakRefusesThePaymentAndLeavesTheBonusAction(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)
	require.NoError(t, payCloak(t, c, "cloak-1"))

	_, err := c.StartTurn(context.Background(), &StartTurnInput{TurnNumber: 2, Speed: 30})
	require.NoError(t, err)
	require.Equal(t, 1, c.actionEconomy.BonusActionsRemaining)

	err = payCloak(t, c, "cloak-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "charges")
	assert.Equal(t, 1, c.actionEconomy.BonusActionsRemaining, "a refused payment spends nothing")

	var own, granted AvailableAbility
	for _, e := range secondWindEntries(c) {
		if e.Giver == nil {
			own = e
		} else {
			granted = e
		}
	}
	assert.False(t, granted.CanUse)
	assert.Contains(t, granted.Reason, "Lieutenant's cloak")
	assert.Equal(t, 0, granted.ResourceCurrent)
	assert.True(t, own.CanUse, "the fighter's own is still usable")
}

// Test 6.
func TestPayingTheFightersOwnSecondWindLeavesTheCloaksCount(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)

	definition, ok, err := c.AbilityDefinition(*refs.Features.SecondWind())
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, combat.Pay(c, definition.Cost))

	assert.Equal(t, 0, c.GetResource(resources.SecondWind).Current())
	assert.Equal(t, 1, chargesOf(c, "cloak-1"))
}

// Test 7.
func TestGrantedAbilityRefusesWhatItCannotAnswerNamingTheItem(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, nil, cloak)

	_, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "ghost", Grant: "second-wind"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"ghost"`)

	_, err = c.GrantedAbility(&GrantedAbilityInput{ItemID: "cloak-1", Grant: "second-wind"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"cloak-1"`)
	assert.Contains(t, err.Error(), "not equipped")

	require.NoError(t, c.EquipItem(SlotCloak, "cloak-1"))
	_, err = c.GrantedAbility(&GrantedAbilityInput{ItemID: "cloak-1", Grant: "fireball"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"cloak-1"`)
	assert.Contains(t, err.Error(), `"fireball"`)

	_, err = c.GrantedAbility(nil)
	require.Error(t, err)

	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "cloak-1", Grant: "second-wind"})
	require.NoError(t, err)
	assert.Equal(t, 1, out.Budget.PoolLeft("charges"))
	require.NoError(t, c.UnequipItem(SlotCloak))
	assert.Equal(t, 0, out.Budget.PoolLeft("charges"), "an item taken off reads 0")
	out.Budget.SpendPool("charges", 1)
	assert.Equal(t, 1, chargesOf(c, "cloak-1"), "an item taken off spends nothing")
}

func TestTheBudgetOwnsExactlyTheItemsPools(t *testing.T) {
	cloak := newItem(t, "cloak-1", cloakDescription())
	c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, cloak)
	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "cloak-1", Grant: "second-wind"})
	require.NoError(t, err)
	assert.True(t, out.Budget.HasPool("charges"))
	assert.False(t, out.Budget.HasPool(resources.SecondWind))
	out.Budget.SpendPool("charges", 5)
	assert.Equal(t, 0, chargesOf(c, "cloak-1"), "never below zero")
}

func emptied(t *testing.T, c *Character, id string) {
	t.Helper()
	for i := range c.items {
		if c.items[i].ID == id {
			for name := range c.items[i].Left {
				c.items[i].Left[name] = 0
			}
		}
	}
}

// Test 8.
func TestARestBringsAnItemsUsesBack(t *testing.T) {
	ctx := context.Background()
	longOnly := cloakDescription()
	longOnly.Pools = map[string]magicitems.Pool{"charges": {Maximum: 1, RestoreOn: []coreResources.ResetType{coreResources.ResetLongRest}}}
	never := cloakDescription()
	never.Pools = map[string]magicitems.Pool{"charges": {Maximum: 1}}

	t.Run("short rest refills a short-rest pool", func(t *testing.T) {
		c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, newItem(t, "cloak-1", cloakDescription()))
		emptied(t, c, "cloak-1")
		_, err := c.ShortRest(ctx, &ShortRestInput{})
		require.NoError(t, err)
		assert.Equal(t, 1, chargesOf(c, "cloak-1"))
	})
	t.Run("long rest refills a short-rest pool", func(t *testing.T) {
		c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, newItem(t, "cloak-1", cloakDescription()))
		emptied(t, c, "cloak-1")
		_, err := c.LongRest(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, chargesOf(c, "cloak-1"))
	})
	t.Run("a long-rest-only pool waits for a long rest", func(t *testing.T) {
		c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, newItem(t, "cloak-1", longOnly))
		emptied(t, c, "cloak-1")
		_, err := c.ShortRest(ctx, &ShortRestInput{})
		require.NoError(t, err)
		assert.Equal(t, 0, chargesOf(c, "cloak-1"))
		_, err = c.LongRest(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, chargesOf(c, "cloak-1"))
	})
	t.Run("a pool with no restore_on never comes back", func(t *testing.T) {
		c := wearer(t, classes.Fighter, map[InventorySlot]string{SlotCloak: "cloak-1"}, newItem(t, "cloak-1", never))
		emptied(t, c, "cloak-1")
		_, err := c.ShortRest(ctx, &ShortRestInput{})
		require.NoError(t, err)
		_, err = c.LongRest(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, chargesOf(c, "cloak-1"))
	})
	t.Run("an item in the pack is refilled too", func(t *testing.T) {
		c := wearer(t, classes.Fighter, nil, newItem(t, "cloak-1", cloakDescription()))
		emptied(t, c, "cloak-1")
		_, err := c.ShortRest(ctx, &ShortRestInput{})
		require.NoError(t, err)
		assert.Equal(t, 1, chargesOf(c, "cloak-1"))
	})
}

// Test 9.
func TestTwoRingsGrantTwoSecondWindsAndPayApart(t *testing.T) {
	a := newItem(t, "ring-a", ringDescription("a"))
	b := newItem(t, "ring-b", ringDescription("b"))
	c := wearer(t, classes.Wizard, map[InventorySlot]string{SlotRingLeft: "ring-a", SlotRingRight: "ring-b"}, a, b)

	entries := secondWindEntries(c)
	require.Len(t, entries, 2)
	require.NotNil(t, entries[0].Giver)
	require.NotNil(t, entries[1].Giver)
	assert.Equal(t, "ring-a", entries[0].Giver.ItemID)
	assert.Equal(t, "ring-b", entries[1].Giver.ItemID)
	assert.Equal(t, "Ring a", entries[0].Giver.ItemName)

	require.NoError(t, payCloak(t, c, "ring-a"))
	assert.Equal(t, 0, chargesOf(c, "ring-a"))
	assert.Equal(t, 1, chargesOf(c, "ring-b"))
}

// Test 10.
func TestACharacterWithNoItemsHasNoGiver(t *testing.T) {
	c := wearer(t, classes.Fighter, nil)
	for _, a := range c.AvailableAbilities() {
		assert.Nil(t, a.Giver, a.Name)
	}
}
