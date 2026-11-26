// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
)

func oneOf(slot coreCombat.ActionType) *combat.SpendProfile {
	return &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{slot: 1}}
}

func costOf(t *testing.T, sheet *monster.Data, action int) *combat.SpendProfile {
	t.Helper()
	out, err := monster.CostOfAction(&monster.CostOfActionInput{Sheet: sheet, Action: sheet.Actions[action]})
	require.NoError(t, err)
	require.NotNil(t, out.Cost)
	return out.Cost
}

func disengageOf(t *testing.T, sheet *monster.Data) *combat.SpendProfile {
	t.Helper()
	out, err := monster.CostOfDisengage(&monster.CostOfDisengageInput{Sheet: sheet})
	require.NoError(t, err)
	require.NotNil(t, out.Cost)
	return out.Cost
}

// Test 1.
func TestAListedActionThatStatesNoCostCostsOneAction(t *testing.T) {
	sheets := map[string]*monster.Data{
		"goblin":      monsters.NewGoblin("goblin-1").ToData(),
		"wolf":        monsters.NewWolf("wolf-1").ToData(),
		"goblin boss": monsters.NewGoblinBoss("boss-1").ToData(),
	}
	for name, sheet := range sheets {
		require.NotEmpty(t, sheet.Actions, name)
		for i, action := range sheet.Actions {
			require.Nil(t, action.Cost, "%s %s states no cost", name, action.Name)
			require.Equal(t, oneOf(coreCombat.ActionStandard), costOf(t, sheet, i), "%s %s", name, action.Name)
		}
	}
}

// Test 2.
func TestAListedActionThatStatesItsOwnCostCostsThat(t *testing.T) {
	sheet := monsters.NewGoblin("goblin-1").ToData()
	stated := &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools: map[coreResources.ResourceKey]int{"dnd5e:resources:ki": 1},
	}
	action := sheet.Actions[0]
	action.Cost = stated

	out, err := monster.CostOfAction(&monster.CostOfActionInput{Sheet: sheet, Action: action})
	require.NoError(t, err)
	require.Equal(t, stated, out.Cost)
	require.NotSame(t, stated, out.Cost, "a copy, never the definition's own pointer")

	out.Cost.Slots[coreCombat.ActionStandard] = 9
	out.Cost.Pools["dnd5e:resources:ki"] = 9
	require.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, stated.Slots)
	require.Equal(t, map[coreResources.ResourceKey]int{"dnd5e:resources:ki": 1}, stated.Pools)
}

// Test 3.
func TestTheGateTakesAListedActionsPrice(t *testing.T) {
	goblin := monsters.NewGoblin("goblin-1")
	cost := costOf(t, goblin.ToData(), 0)

	require.NoError(t, combat.Pay(goblin, cost))
	require.Equal(t, 0, goblin.SlotsLeft(coreCombat.ActionStandard))
	require.Equal(t, 1, goblin.SlotsLeft(coreCombat.ActionBonus))
	require.Equal(t, 1, goblin.SlotsLeft(coreCombat.ActionReaction))
	require.Error(t, combat.Pay(goblin, cost), "a second is refused")
}

// Test 4.
func TestDisengageIsABonusActionWithNimbleEscapeAndAnActionWithout(t *testing.T) {
	require.Equal(t, oneOf(coreCombat.ActionBonus), disengageOf(t, monsters.NewGoblin("goblin-1").ToData()))
	require.Equal(t, oneOf(coreCombat.ActionStandard), disengageOf(t, monsters.NewWolf("wolf-1").ToData()))
}

// Test 5.
func TestTheGateTakesDisengageThenTheScimitarInOneTurn(t *testing.T) {
	goblin := monsters.NewGoblin("goblin-1")
	sheet := goblin.ToData()

	require.NoError(t, combat.Pay(goblin, disengageOf(t, sheet)))
	require.NoError(t, combat.Pay(goblin, costOf(t, sheet, 0)))
	require.Error(t, combat.Pay(goblin, disengageOf(t, sheet)), "a second Disengage is refused")
}

// Test 6.
func TestAStoredTraitThatCannotBeReadIsAnErrorNotAPrice(t *testing.T) {
	sheet := &monster.Data{ID: "bad-1", Conditions: []json.RawMessage{json.RawMessage(`{not json`)}}

	out, err := monster.CostOfDisengage(&monster.CostOfDisengageInput{Sheet: sheet})
	require.Error(t, err)
	require.Nil(t, out)
	require.Contains(t, err.Error(), "bad-1")
}

// Test 7.
func TestNilIsRefusedByName(t *testing.T) {
	_, err := monster.CostOfAction(nil)
	require.Equal(t, rpgerr.CodeNil, rpgerr.GetCode(err))
	_, err = monster.CostOfAction(&monster.CostOfActionInput{})
	require.Equal(t, rpgerr.CodeNil, rpgerr.GetCode(err))
	_, err = monster.CostOfDisengage(nil)
	require.Equal(t, rpgerr.CodeNil, rpgerr.GetCode(err))
	_, err = monster.CostOfDisengage(&monster.CostOfDisengageInput{})
	require.Equal(t, rpgerr.CodeNil, rpgerr.GetCode(err))
}

func dashOf(t *testing.T, sheet *monster.Data) *combat.SpendProfile {
	t.Helper()
	out, err := monster.CostOfDash(&monster.CostOfDashInput{Sheet: sheet})
	require.NoError(t, err)
	require.NotNil(t, out.Cost)
	return out.Cost
}

// Dash Test 1.
func TestADashIsExactlyOneActionAndGrantsNothing(t *testing.T) {
	sheets := map[string]*monster.Data{
		"goblin": monsters.NewGoblin("goblin-1").ToData(),
		"wolf":   monsters.NewWolf("wolf-1").ToData(),
	}
	for name, sheet := range sheets {
		cost := dashOf(t, sheet)
		require.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}, cost.Slots, name)
		require.Equal(t, oneOf(coreCombat.ActionStandard), cost, name)
		require.Empty(t, cost.Capacity, name)
		require.Empty(t, cost.Pools, name)
		require.Empty(t, cost.Grants, name)
		require.Empty(t, cost.Requires, name)
	}
}

// Dash Test 2.
func TestEachDashPriceIsItsOwnProfile(t *testing.T) {
	sheet := monsters.NewGoblin("goblin-1").ToData()
	first := dashOf(t, sheet)
	first.Slots[coreCombat.ActionStandard] = 9
	first.Slots[coreCombat.ActionBonus] = 9

	require.Equal(t, oneOf(coreCombat.ActionStandard), dashOf(t, sheet))
}

// Dash Test 3.
func TestTheGateTakesADashAndLeavesTheBonusActionAndReaction(t *testing.T) {
	goblin := monsters.NewGoblin("goblin-1")
	sheet := goblin.ToData()

	require.NoError(t, combat.Pay(goblin, dashOf(t, sheet)))
	require.Equal(t, 0, goblin.SlotsLeft(coreCombat.ActionStandard))
	require.Equal(t, 1, goblin.SlotsLeft(coreCombat.ActionBonus))
	require.Equal(t, 1, goblin.SlotsLeft(coreCombat.ActionReaction))

	require.NoError(t, combat.Pay(goblin, disengageOf(t, sheet)), "its Disengage is a bonus action, still there")
	require.Error(t, combat.Pay(goblin, costOf(t, sheet, 0)), "its scimitar is an action, gone")
}

// Dash Test 4.
func TestADashRefusesNilByName(t *testing.T) {
	_, err := monster.CostOfDash(nil)
	require.Equal(t, rpgerr.CodeNil, rpgerr.GetCode(err))
	_, err = monster.CostOfDash(&monster.CostOfDashInput{})
	require.Equal(t, rpgerr.CodeNil, rpgerr.GetCode(err))
}
