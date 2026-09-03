// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// The economy a monster's turn is driven from is a ledger. Monsters take no
// gated action in v1, but the gate must not be shaped so that they never can:
// a monster holds a caller-supplied *ActionEconomy where a character holds a
// persisted sheet, and the gate is not allowed to know which it has.
var _ combat.Ledger = (*combat.ActionEconomy)(nil)

type ActionEconomyLedgerTestSuite struct {
	suite.Suite
}

func TestActionEconomyLedgerSuite(t *testing.T) {
	suite.Run(t, new(ActionEconomyLedgerTestSuite))
}

// An economy that does not exist is not in combat, so the gate refuses it
// rather than dereferencing it.
func (s *ActionEconomyLedgerTestSuite) TestNilEconomyIsNotInCombat() {
	var economy *combat.ActionEconomy
	s.False(economy.InCombat())
	s.False(combat.CanPay(economy, &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
	}))
}

func (s *ActionEconomyLedgerTestSuite) TestNewEconomyIsInCombat() {
	s.True(combat.NewActionEconomy().InCombat())
}

// The three per-turn slots read and write the three fields that have always
// held them.
func (s *ActionEconomyLedgerTestSuite) TestSlotsAreTheThreeFields() {
	economy := combat.NewActionEconomy()

	s.Equal(1, economy.SlotsLeft(coreCombat.ActionStandard))
	s.Equal(1, economy.SlotsLeft(coreCombat.ActionBonus))
	s.Equal(1, economy.SlotsLeft(coreCombat.ActionReaction))

	economy.SpendSlots(coreCombat.ActionBonus, 1)
	s.Equal(0, economy.BonusActionsRemaining)
	s.Equal(1, economy.ActionsRemaining)
	s.Equal(1, economy.ReactionsRemaining)
}

// A slot this economy does not keep reads as nothing left, so any cost keyed
// to it is refused rather than quietly granted.
func (s *ActionEconomyLedgerTestSuite) TestUnknownSlotHasNothingLeft() {
	economy := combat.NewActionEconomy()
	s.Equal(0, economy.SlotsLeft(coreCombat.ActionFree))
	s.Equal(0, economy.SlotsLeft(coreCombat.ActionMovement))
}

// The keyed view and the named fields are one piece of state, not two. This is
// the half of the census's lossy-bridge finding that is testable here: a
// keyed write that the fielded accessors cannot see is a spend that legacy
// code will overwrite.
func (s *ActionEconomyLedgerTestSuite) TestKeyedCapacityIsTheFieldedCapacity() {
	economy := combat.NewActionEconomy()

	economy.BankCapacity(combat.CapacityAttack, 2)
	s.Equal(2, economy.AttacksRemaining)

	economy.BankCapacity(combat.CapacityOffHandAttack, 1)
	s.Equal(1, economy.OffHandAttacksRemaining)

	economy.BankCapacity(combat.CapacityFlurryStrike, 2)
	s.Equal(2, economy.FlurryStrikesRemaining)

	economy.BankCapacity(combat.CapacityDeathSave, 1)
	s.Equal(1, economy.DeathSavesRemaining)

	economy.SetMovement(30)
	s.Equal(30, economy.CapacityLeft(combat.CapacityMovement))

	economy.SpendCapacity(combat.CapacityMovement, 10)
	s.Equal(20, economy.MovementRemaining)
}

// Every declared capacity round-trips. A key the ledger cannot store is a
// grant that vanishes and a cost that is free, and neither reports anything.
func (s *ActionEconomyLedgerTestSuite) TestEveryDeclaredCapacityRoundTrips() {
	for _, key := range combat.CapacityTypes() {
		economy := combat.NewActionEconomy()

		economy.BankCapacity(key, 3)
		s.Require().Equalf(3, economy.CapacityLeft(key), "banked %q", key)

		economy.SpendCapacity(key, 2)
		s.Require().Equalf(1, economy.CapacityLeft(key), "spent %q", key)
	}
}

// A capacity this economy has no field for never reaches it: the profile that
// named it does not validate, so the gate refuses before anything is charged.
// That is what closing the vocabulary buys — a fielded ledger cannot store a
// key it was never told about, and the alternative to refusing is a grant that
// evaporates without a word.
func (s *ActionEconomyLedgerTestSuite) TestAnUndeclaredCapacityNeverReachesTheEconomy() {
	economy := combat.NewActionEconomy()
	future := combat.CapacityType("arcane_recovery_die")

	s.Equal(0, economy.CapacityLeft(future))

	profile := &combat.SpendProfile{
		Grants: map[combat.CapacityType]int{future: 2},
	}
	s.False(combat.CanPay(economy, profile))
	s.Require().Error(combat.Pay(economy, profile))
	s.Equal(0, economy.CapacityLeft(future))
}

// An economy holds no pools, so a pool cost is refused rather than charged to
// nothing. A monster with no ki cannot pay for Flurry of Blows by not paying.
func (s *ActionEconomyLedgerTestSuite) TestNoPoolsMeansAPoolCostIsRefused() {
	economy := combat.NewActionEconomy()
	s.Equal(0, economy.PoolLeft("ki"))

	s.False(combat.CanPay(economy, &combat.SpendProfile{
		Pools: map[coreResources.ResourceKey]int{"ki": 1},
	}))
}

// The shape a monster's turn actually takes: one action-costing profile paid
// out of the economy the caller handed in.
func (s *ActionEconomyLedgerTestSuite) TestAMonstersEconomyPaysForAnAction() {
	economy := combat.NewActionEconomy()
	profile := &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
	}

	s.True(combat.CanPay(economy, profile))
	s.Require().NoError(combat.Pay(economy, profile))
	s.Equal(0, economy.ActionsRemaining)

	s.False(combat.CanPay(economy, profile))
	s.Require().Error(combat.Pay(economy, profile))
}
