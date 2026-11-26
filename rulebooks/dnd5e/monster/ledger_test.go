// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
)

// goblinBeforeTheSlice is the JSON of NewGoblin("goblin-1").ToData() captured on
// origin/main at 9ccd3917, before the sheet held an economy.
const goblinBeforeTheSlice = `{"id":"goblin-1","name":"Goblin","ref":"dnd5e:monsters:goblin","hit_points":7,"max_hit_points":7,"armor_class":15,"ability_scores":{"cha":8,"con":10,"dex":14,"int":10,"str":8,"wis":8},"proficiency_bonus":2,"experience":50,"speed":{"walk":30},"senses":{"passive_perception":9},"actions":[{"description":"Make a melee attack against a creature in reach.","ref":"dnd5e:weapons:scimitar","name":"Scimitar","attack":{"category":"weapon","delivery":{"melee":{"reach_feet":5}},"attack_bonus":4,"ability":{"ability":"dex","modifier":2},"weapon":{"ref":"dnd5e:weapons:scimitar","two_handed":false},"damage":[{"dice":"1d6","type":"slashing","properties":["adds-attack-ability-modifier"]}]}},{"description":"Make a ranged attack against a creature within range.","ref":"dnd5e:weapons:shortbow","name":"Shortbow","attack":{"category":"weapon","delivery":{"ranged":{"normal_feet":80,"long_feet":320}},"attack_bonus":4,"ability":{"ability":"dex","modifier":2},"weapon":{"ref":"dnd5e:weapons:shortbow","two_handed":true},"damage":[{"dice":"1d6","type":"piercing","properties":["adds-attack-ability-modifier"]}]}}],"conditions":[{"ref":"dnd5e:monster_traits:nimble_escape","owner_id":"goblin-1"}]}`

func freshWolf(t *testing.T) *monster.Monster {
	t.Helper()

	m, err := monster.Load(context.Background(), &monster.Data{
		ID: "wolf-1", Name: "Wolf", HitPoints: 11, MaxHitPoints: 11, ArmorClass: 13,
	})
	require.NoError(t, err)

	return m
}

func profileOf(slot coreCombat.ActionType) *combat.SpendProfile {
	return &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{slot: 1}}
}

func economyKey(t *testing.T, m *monster.Monster) bool {
	t.Helper()

	raw, err := json.Marshal(m.ToData())
	require.NoError(t, err)

	var asMap map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &asMap))
	_, has := asMap["economy"]

	return has
}

// Test 1.
func TestAMonsterThatSpentNothingSavesNoAccount(t *testing.T) {
	raw, err := json.Marshal(monsters.NewGoblin("goblin-1").ToData())
	require.NoError(t, err)

	require.NotContains(t, string(raw), "economy")
	require.Equal(t, goblinBeforeTheSlice, string(raw), "byte for byte what it saved before the slice")
}

// Test 2.
func TestEachSlotIsPaidOnce(t *testing.T) {
	for _, slot := range []coreCombat.ActionType{
		coreCombat.ActionBonus, coreCombat.ActionStandard, coreCombat.ActionReaction,
	} {
		m := freshWolf(t)

		require.NoError(t, combat.Pay(m, profileOf(slot)), slot)
		require.Equal(t, 0, m.SlotsLeft(slot), slot)
		require.Error(t, combat.Pay(m, profileOf(slot)), "a second payment is refused: %s", slot)
		require.Equal(t, 0, m.SlotsLeft(slot), "and nothing moved: %s", slot)

		for _, other := range []coreCombat.ActionType{
			coreCombat.ActionBonus, coreCombat.ActionStandard, coreCombat.ActionReaction,
		} {
			if other != slot {
				require.Equal(t, 1, m.SlotsLeft(other), "paying %s left %s alone", slot, other)
			}
		}
	}
}

// Test 3.
func TestAProfileThatCannotBePaidWholeIsPaidNotAtAll(t *testing.T) {
	m := freshWolf(t)

	profile := &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools: map[coreResources.ResourceKey]int{"dnd5e:resources:ki": 1},
	}

	require.Error(t, combat.Pay(m, profile), "a monster has no counts, so a cost naming a pool is refused")
	require.Equal(t, 1, m.SlotsLeft(coreCombat.ActionBonus))
	require.False(t, m.IsDirty())
}

// Test 4.
func TestAskingSpendsNothing(t *testing.T) {
	m := freshWolf(t)

	require.True(t, m.InCombat())
	require.True(t, combat.CanPay(m, profileOf(coreCombat.ActionBonus)))
	require.Equal(t, 1, m.SlotsLeft(coreCombat.ActionStandard))
	require.Equal(t, 1, m.CapacityLeft(combat.CapacityObjectInteraction))

	require.False(t, m.IsDirty(), "a read does not mark the sheet")
	require.False(t, economyKey(t, m), "and does not make an account")
}

// Test 5.
func TestItsOwnTurnStartGivesTheTurnBackAndOnlyItsOwn(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	m := freshWolf(t)
	require.NoError(t, m.SheetKeeper().Apply(ctx, bus))

	turnStart := func(id string) {
		require.NoError(t, dnd5eEvents.TurnStartTopic.On(bus).Publish(ctx,
			dnd5eEvents.TurnStartEvent{SubjectID: id, Round: 2}))
	}

	turnStart(m.GetID())
	require.False(t, m.IsDirty(), "a turn start on a monster that spent nothing is not a write")

	for _, slot := range []coreCombat.ActionType{
		coreCombat.ActionBonus, coreCombat.ActionStandard, coreCombat.ActionReaction,
	} {
		require.NoError(t, combat.Pay(m, profileOf(slot)))
	}

	turnStart("someone-else")
	for _, slot := range []coreCombat.ActionType{
		coreCombat.ActionBonus, coreCombat.ActionStandard, coreCombat.ActionReaction,
	} {
		require.Equal(t, 0, m.SlotsLeft(slot), "another member's turn changes nothing: %s", slot)
	}

	turnStart(m.GetID())
	for _, slot := range []coreCombat.ActionType{
		coreCombat.ActionBonus, coreCombat.ActionStandard, coreCombat.ActionReaction,
	} {
		require.True(t, combat.CanPay(m, profileOf(slot)), slot)
	}
	require.False(t, economyKey(t, m), "a full turn is saved as no account")
	require.True(t, m.IsDirty(), "giving the turn back is a change worth saving")
}

// Test 6.
func TestSaveAndLoadCarryTheAccount(t *testing.T) {
	m := freshWolf(t)
	require.NoError(t, combat.Pay(m, profileOf(coreCombat.ActionBonus)))

	raw, err := json.Marshal(m.ToData())
	require.NoError(t, err)
	require.Contains(t, string(raw), `"economy":{"actions_remaining":1,"bonus_actions_remaining":0,`+
		`"reactions_remaining":1,"object_interactions_remaining":1}`)

	var data monster.Data
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := monster.Load(context.Background(), &data)
	require.NoError(t, err)

	require.Equal(t, 0, loaded.SlotsLeft(coreCombat.ActionBonus))
	require.Equal(t, 1, loaded.SlotsLeft(coreCombat.ActionStandard))
	require.Equal(t, 1, loaded.SlotsLeft(coreCombat.ActionReaction))
}

// Test 7.
func TestTheSheetAndItsSavedFormShareNothing(t *testing.T) {
	m := freshWolf(t)
	require.NoError(t, combat.Pay(m, profileOf(coreCombat.ActionBonus)))

	saved := m.ToData()
	saved.Economy.BonusActionsRemaining = 1
	saved.Economy.ActionsRemaining = 0
	require.Equal(t, 0, m.SlotsLeft(coreCombat.ActionBonus), "changing what ToData handed out does not change the monster")
	require.Equal(t, 1, m.SlotsLeft(coreCombat.ActionStandard))

	given := m.ToData()
	loaded, err := monster.Load(context.Background(), given)
	require.NoError(t, err)
	given.Economy.BonusActionsRemaining = 1
	given.Economy.ReactionsRemaining = 0
	require.Equal(t, 0, loaded.SlotsLeft(coreCombat.ActionBonus), "changing what was loaded does not change the monster")
	require.Equal(t, 1, loaded.SlotsLeft(coreCombat.ActionReaction))
}

// Test 8.
func TestASpendMarksTheSheetAndARefusalDoesNot(t *testing.T) {
	m := freshWolf(t)

	require.NoError(t, combat.Pay(m, profileOf(coreCombat.ActionStandard)))
	require.True(t, m.IsDirty())

	spent, err := monster.Load(context.Background(), m.ToData())
	require.NoError(t, err)
	require.False(t, spent.IsDirty(), "a loaded sheet starts clean")
	require.Error(t, combat.Pay(spent, profileOf(coreCombat.ActionStandard)))
	require.False(t, spent.IsDirty(), "a refusal marks nothing")
}

// Test 9.
func TestDataSlotsLeft(t *testing.T) {
	var d monster.Data
	for _, slot := range []coreCombat.ActionType{
		coreCombat.ActionStandard, coreCombat.ActionBonus, coreCombat.ActionReaction,
	} {
		require.Equal(t, 1, d.SlotsLeft(slot), "a nil account is a full turn: %s", slot)
	}

	d.Economy = &combat.ActionEconomy{ActionsRemaining: 0, BonusActionsRemaining: 1, ReactionsRemaining: 0}
	require.Equal(t, 0, d.SlotsLeft(coreCombat.ActionStandard))
	require.Equal(t, 1, d.SlotsLeft(coreCombat.ActionBonus))
	require.Equal(t, 0, d.SlotsLeft(coreCombat.ActionReaction))
	require.Equal(t, 0, d.SlotsLeft(coreCombat.ActionFree))
	require.Equal(t, 0, d.SlotsLeft(coreCombat.ActionMovement))
}

// Test 10, the ledger side: the reaction's three doors still read one account.
func TestTheReactionDoorsReadTheOneAccount(t *testing.T) {
	m := freshWolf(t)
	require.True(t, m.CanReact())

	require.NoError(t, combat.Pay(m, profileOf(coreCombat.ActionReaction)))
	require.False(t, m.CanReact())
	require.ErrorIs(t, m.SpendReaction(), monster.ErrReactionSpent)

	other := freshWolf(t)
	require.NoError(t, other.SpendReaction())
	require.Equal(t, 0, other.SlotsLeft(coreCombat.ActionReaction))
	require.Error(t, combat.Pay(other, profileOf(coreCombat.ActionReaction)))
}

// An old blob that said the reaction was spent loads as a full turn.
func TestAnOldBlobWithTheRemovedFieldLoadsAsAFullTurn(t *testing.T) {
	var data monster.Data
	require.NoError(t, json.Unmarshal([]byte(
		`{"id":"wolf-1","name":"Wolf","hit_points":11,"max_hit_points":11,"armor_class":13,"reaction_spent":true}`,
	), &data))

	m, err := monster.Load(context.Background(), &data)
	require.NoError(t, err)
	require.True(t, m.CanReact())
}
