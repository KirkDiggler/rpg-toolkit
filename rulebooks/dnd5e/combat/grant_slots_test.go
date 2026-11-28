// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
)

func givesAnAction() *combat.SpendProfile {
	return &combat.SpendProfile{
		GrantSlots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
	}
}

// Test 1: the gate charges, then gives.
func TestThePriceGivesItsSlotAfterItsCharges(t *testing.T) {
	ledger := newSpyLedger()
	ledger.pools["x"] = 1

	profile := &combat.SpendProfile{
		Pools:      map[coreResources.ResourceKey]int{"x": 1},
		GrantSlots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
	}

	require.NoError(t, combat.Pay(ledger, profile))
	require.Equal(t, []string{"pool:x", "grant:" + string(coreCombat.ActionStandard)}, ledger.order)
	require.Equal(t, 1, ledger.slots[coreCombat.ActionStandard])
	require.Equal(t, 0, ledger.pools["x"])
}

// Test 2: all or none.
func TestAPriceThatCannotBePaidGivesNothing(t *testing.T) {
	ledger := newSpyLedger()

	profile := &combat.SpendProfile{
		Pools:      map[coreResources.ResourceKey]int{"x": 1},
		GrantSlots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
	}

	require.Error(t, combat.Pay(ledger, profile))
	require.Zero(t, ledger.writes, "a refused payment gives nothing")
	require.Empty(t, ledger.order)
}

// Test 3: charged first, then given; the gift does not pay for itself.
func TestTheGiftDoesNotPayForItself(t *testing.T) {
	profile := &combat.SpendProfile{
		Slots:      map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		GrantSlots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
	}

	withOne := combat.NewActionEconomy()
	require.Equal(t, 1, withOne.SlotsLeft(coreCombat.ActionStandard))
	require.NoError(t, combat.Pay(withOne, profile))
	require.Equal(t, 1, withOne.SlotsLeft(coreCombat.ActionStandard))

	withNone := combat.NewActionEconomy()
	withNone.ActionsRemaining = 0
	require.Error(t, combat.Pay(withNone, profile))
	require.Equal(t, 0, withNone.SlotsLeft(coreCombat.ActionStandard))
}

// Test 4: the price is judged.
func TestASlotGrantIsJudgedLikeASlotCost(t *testing.T) {
	bad := map[string]*combat.SpendProfile{
		"movement": {GrantSlots: map[coreCombat.ActionType]int{coreCombat.ActionMovement: 1}},
		"free":     {GrantSlots: map[coreCombat.ActionType]int{coreCombat.ActionFree: 1}},
		"zero":     {GrantSlots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 0}},
		"negative": {GrantSlots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: -1}},
	}
	for name, profile := range bad {
		t.Run(name, func(t *testing.T) {
			err := profile.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), "slot grant")
			require.False(t, combat.CanPay(combat.NewActionEconomy(), profile))
		})
	}

	// A price that only gives a slot moves something: out of combat it is refused.
	var notInCombat *combat.ActionEconomy
	require.False(t, combat.CanPay(notInCombat, givesAnAction()))
	require.Error(t, combat.Pay(notInCombat, givesAnAction()))
	require.True(t, combat.CanPay(combat.NewActionEconomy(), givesAnAction()))
}

// Test 5 (the economy and the pool wrapper).
func TestTheEconomyAnswersAndAPooledLedgerForwards(t *testing.T) {
	economy := combat.NewActionEconomy()
	require.NoError(t, combat.Pay(economy, givesAnAction()))
	require.Equal(t, 2, economy.SlotsLeft(coreCombat.ActionStandard))

	bonus := &combat.SpendProfile{GrantSlots: map[coreCombat.ActionType]int{
		coreCombat.ActionBonus: 1, coreCombat.ActionReaction: 2,
	}}
	require.NoError(t, combat.Pay(economy, bonus))
	require.Equal(t, 2, economy.SlotsLeft(coreCombat.ActionBonus))
	require.Equal(t, 3, economy.SlotsLeft(coreCombat.ActionReaction))

	base := combat.NewActionEconomy()
	pooled := combat.WithPools(base, nil)
	require.NoError(t, combat.Pay(pooled, givesAnAction()))
	require.Equal(t, 2, base.SlotsLeft(coreCombat.ActionStandard))

	pooled = combat.WithPools(base, fakeSource{})
	require.NoError(t, combat.Pay(pooled, givesAnAction()))
	require.Equal(t, 3, base.SlotsLeft(coreCombat.ActionStandard), "the grant reaches the base ledger")

	var gone *combat.ActionEconomy
	require.NotPanics(t, func() { gone.GrantSlots(coreCombat.ActionStandard, 1) })
}

type fakeSource struct{}

func (fakeSource) HasPool(coreResources.ResourceKey) bool   { return false }
func (fakeSource) PoolLeft(coreResources.ResourceKey) int   { return 0 }
func (fakeSource) SpendPool(coreResources.ResourceKey, int) {}

// Test 7: a stored price round-trips, and one without it is as it was.
func TestAStoredPriceCarriesItsSlotGrant(t *testing.T) {
	raw, err := json.Marshal(givesAnAction())
	require.NoError(t, err)
	require.Contains(t, string(raw), `"grant_slots"`)

	var back combat.SpendProfile
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Equal(t, *givesAnAction(), back)

	plain, err := json.Marshal(&combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
	})
	require.NoError(t, err)
	require.NotContains(t, string(plain), "grant_slots")
	require.JSONEq(t, `{"slots":{"bonus_action":1}}`, string(plain))
}

// Test 9: CanPay reads.
func TestAskingAboutAGivenSlotWritesNothing(t *testing.T) {
	spy := newSpyLedger()
	require.True(t, combat.CanPay(spy, givesAnAction()))
	require.Zero(t, spy.writes)

	economy := combat.NewActionEconomy()
	require.True(t, combat.CanPay(economy, givesAnAction()))
	require.Equal(t, 1, economy.SlotsLeft(coreCombat.ActionStandard))
}

// Test 6: a copy of a price carries the slot grant, deeply.
func TestACopyOfAPriceCarriesItsSlotGrant(t *testing.T) {
	original := givesAnAction()
	copied := actions.CloneSpendProfile(original)

	require.Equal(t, original.GrantSlots, copied.GrantSlots)

	copied.GrantSlots[coreCombat.ActionStandard] = 5
	require.Equal(t, 1, original.GrantSlots[coreCombat.ActionStandard], "the copy shares nothing")
}
