// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package items_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

func attachedRing(t *testing.T, id string) (*items.Item, events.EventBus) {
	t.Helper()
	bus := events.NewEventBus()
	ring, err := items.New(ringConfig(id, 1, nil))
	require.NoError(t, err)
	require.NoError(t, ring.Attach(context.Background(), items.AttachInput{Bus: bus, HolderID: "rook"}))
	return ring, bus
}

func ringCost() *combat.SpendProfile {
	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools: map[coreResources.ResourceKey]int{chargePool: 1},
	}
}

func charges(ring *items.Item) int { return ring.Data().Pools[chargePool] }

func TestRingPaysTheBonusActionAndItsUseInOnePayment(t *testing.T) {
	ring, _ := attachedRing(t, "ring")
	economy := combat.NewActionEconomy()
	calc, err := payRing(context.Background(), ring, "rook", economy, &fixedRoller{face: 6})
	require.NoError(t, err)
	require.Equal(t, 7, calc.Total)
	require.Zero(t, economy.BonusActionsRemaining)
	require.Zero(t, charges(ring))
}

func TestSpentBonusActionRefusesAndTheRingKeepsItsUse(t *testing.T) {
	ring, _ := attachedRing(t, "ring")
	economy := combat.NewActionEconomy()
	economy.BonusActionsRemaining = 0
	roller := &fixedRoller{face: 6}
	_, err := payRing(context.Background(), ring, "rook", economy, roller)
	require.Error(t, err)
	require.Equal(t, 1, charges(ring))
	require.Zero(t, roller.calls)
}

func TestEmptyRingRefusesNamingItsPoolAndKeepsTheBonusAction(t *testing.T) {
	ring, _ := attachedRing(t, "ring")
	_, err := payRing(context.Background(), ring, "rook", combat.NewActionEconomy(), &fixedRoller{face: 6})
	require.NoError(t, err)
	economy := combat.NewActionEconomy()
	_, err = payRing(context.Background(), ring, "rook", economy, &fixedRoller{face: 6})
	require.Error(t, err)
	require.ErrorContains(t, err, "charges: 1 needed, 0 left")
	require.Equal(t, 1, economy.BonusActionsRemaining)
	require.Zero(t, charges(ring))
}

func TestRingHeldBySomeoneElseCannotPay(t *testing.T) {
	ring, _ := attachedRing(t, "ring")
	economy := combat.NewActionEconomy()
	_, err := payRing(context.Background(), ring, "someone else", economy, &fixedRoller{face: 6})
	require.ErrorIs(t, err, items.ErrUnavailable)
	err = combat.Pay(combat.WithPools(economy, ring.Budget("someone else")), ringCost())
	require.ErrorContains(t, err, "1 needed, 0 left")
	require.Equal(t, 1, charges(ring))
	require.Equal(t, 1, economy.BonusActionsRemaining)
}

func TestDetachedRingCannotPay(t *testing.T) {
	ring, _ := attachedRing(t, "ring")
	require.NoError(t, ring.Detach(context.Background()))
	economy := combat.NewActionEconomy()
	_, err := payRing(context.Background(), ring, "rook", economy, &fixedRoller{face: 6})
	require.ErrorIs(t, err, items.ErrUnavailable)
	err = combat.Pay(combat.WithPools(economy, ring.Budget("rook")), ringCost())
	require.ErrorContains(t, err, "1 needed, 0 left")
	require.Equal(t, 1, charges(ring))
	require.Equal(t, 1, economy.BonusActionsRemaining)
}

func TestPayingWithOneRingLeavesTheOtherAlone(t *testing.T) {
	first, bus := attachedRing(t, "first")
	second, err := items.New(ringConfig("second", 1, nil))
	require.NoError(t, err)
	require.NoError(t, second.Attach(context.Background(), items.AttachInput{Bus: bus, HolderID: "rook"}))
	_, err = payRing(context.Background(), first, "rook", combat.NewActionEconomy(), &fixedRoller{face: 6})
	require.NoError(t, err)
	require.Zero(t, charges(first))
	require.Equal(t, 1, charges(second))
}

func TestAPoolTheRingDoesNotDeclareIsNotTheRings(t *testing.T) {
	ring, _ := attachedRing(t, "ring")
	budget := ring.Budget("rook")
	require.True(t, budget.HasPool(chargePool))
	require.False(t, budget.HasPool("ki"))
	economy := combat.NewActionEconomy()
	cost := &combat.SpendProfile{Pools: map[coreResources.ResourceKey]int{"ki": 1}}
	err := combat.Pay(combat.WithPools(economy, budget), cost)
	require.ErrorContains(t, err, "ki: 1 needed, 0 left")
	require.Equal(t, 1, charges(ring))
}

func TestPaidUseSurvivesSaveTransferAndRest(t *testing.T) {
	ctx := context.Background()
	ring, bus := attachedRing(t, "ring")
	_, err := payRing(ctx, ring, "rook", combat.NewActionEconomy(), &fixedRoller{face: 6})
	require.NoError(t, err)
	data := ring.Data()
	require.Zero(t, data.Pools[chargePool])
	restored, err := items.New(ringConfig("ring", 1, &data))
	require.NoError(t, err)
	require.Zero(t, charges(restored))
	require.NoError(t, ring.Detach(ctx))
	require.NoError(t, ring.Attach(ctx, items.AttachInput{Bus: bus, HolderID: "vale"}))
	require.Zero(t, charges(ring))
	require.NoError(t, items.RecoveryTopic.On(bus).Publish(ctx, items.RecoveryEvent{HolderID: "vale", Trigger: shortRest}))
	require.Equal(t, 1, charges(ring))
}
