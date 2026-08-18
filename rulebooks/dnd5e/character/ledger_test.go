// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// A sheet is a ledger. The gate spends from characters and monsters through
// one surface and is not allowed to learn which it has — a monster's economy
// satisfies the same interface from the combat package's own side.
var _ combat.Ledger = (*Character)(nil)

// SheetLedgerTestSuite pins the sheet's side of the gate: that the keyed view
// reaches the state that is actually persisted, that a spend is a change the
// sheet REPORTS (E0/#1087 — a spend that does not dirty is a spend the
// write-back drops), and that a refused payment leaves the sheet exactly as
// clean as it found it.
type SheetLedgerTestSuite struct {
	suite.Suite

	ctx context.Context
	bus events.EventBus
}

func TestSheetLedgerSuite(t *testing.T) {
	suite.Run(t, new(SheetLedgerTestSuite))
}

func (s *SheetLedgerTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// monk is a level-5 monk with ki: the only sheet in the fixture set that can
// be charged in two currencies at once.
func (s *SheetLedgerTestSuite) monk() *Data {
	return &Data{
		ID:               "gate-monk",
		PlayerID:         "gate-player",
		Name:             "Payer",
		Level:            5,
		ProficiencyBonus: 3,
		RaceID:           races.Human,
		ClassID:          classes.Monk,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12,
			abilities.DEX: 16,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 16,
			abilities.CHA: 8,
		},
		HitPoints:    32,
		MaxHitPoints: 32,
		ArmorClass:   16,
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			resources.Ki: {Current: 3, Maximum: 5, ResetType: coreResources.ResetShortRest},
		},
	}
}

// loaded returns the monk as it comes off storage and then in a turn, marked
// clean — so whatever the test does next is the only thing that could have
// dirtied it.
func (s *SheetLedgerTestSuite) loaded() *Character {
	char, err := LoadFromData(s.ctx, s.monk(), s.bus)
	s.Require().NoError(err)

	_, err = char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	char.MarkClean()

	return char
}

// A paid sheet says so. Without this the spend serializes perfectly and is
// then discarded by the write-back, which is the failure #1087 was filed for.
func (s *SheetLedgerTestSuite) TestPayingMarksTheSheetDirty() {
	char := s.loaded()
	s.Require().False(char.IsDirty())

	profile := &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools: map[coreResources.ResourceKey]int{resources.Ki: 1},
	}

	s.Require().NoError(combat.Pay(char, profile))

	s.True(char.IsDirty(), "a spend the sheet does not report is a spend that is lost")
	s.Equal(0, char.GetActionEconomy().BonusActionsRemaining)
	s.Equal(2, char.GetResource(resources.Ki).Current())
}

// Every currency dirties on its own — a profile that only spends a slot, only
// spends capacity, or only banks capacity still has to reach storage.
func (s *SheetLedgerTestSuite) TestEveryKindOfSpendMarksTheSheet() {
	cases := map[string]*combat.SpendProfile{
		"a slot": {
			Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		},
		"a pool": {
			Pools: map[coreResources.ResourceKey]int{resources.Ki: 1},
		},
		"movement": {
			Capacity: map[combat.CapacityType]int{combat.CapacityMovement: 10},
		},
		"a grant alone": {
			Grants: map[combat.CapacityType]int{combat.CapacityAttack: 2},
		},
	}

	for name, profile := range cases {
		s.Run(name, func() {
			char := s.loaded()
			s.Require().NoError(combat.Pay(char, profile))
			s.True(char.IsDirty())
		})
	}
}

// Spending capacity marks too, once there is capacity to spend.
func (s *SheetLedgerTestSuite) TestSpendingBankedCapacityMarksTheSheet() {
	char := s.loaded()
	char.BankCapacity(combat.CapacityAttack, 1)
	char.MarkClean()

	s.Require().NoError(combat.Pay(char, &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}))
	s.True(char.IsDirty())
}

// Asking is free, in both senses: it moves nothing and it dirties nothing. A
// clean sheet that answers a question and then reports itself dirty gets
// written back over storage for no reason.
func (s *SheetLedgerTestSuite) TestCanPayLeavesTheSheetClean() {
	char := s.loaded()

	profile := &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools: map[coreResources.ResourceKey]int{resources.Ki: 1},
	}

	s.True(combat.CanPay(char, profile))
	s.False(char.IsDirty())
	s.Equal(1, char.GetActionEconomy().BonusActionsRemaining)
	s.Equal(3, char.GetResource(resources.Ki).Current())
}

// Atomicity on the real thing: the slot is affordable, the ki is not, and the
// slot must survive. A sheet that is dirty afterwards is a sheet that half-paid.
func (s *SheetLedgerTestSuite) TestARefusedPaymentChangesNothingOnTheSheet() {
	char := s.loaded()

	profile := &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Pools: map[coreResources.ResourceKey]int{resources.Ki: 4},
	}

	s.False(combat.CanPay(char, profile))
	s.Require().Error(combat.Pay(char, profile))

	s.Equal(1, char.GetActionEconomy().ActionsRemaining)
	s.Equal(3, char.GetResource(resources.Ki).Current())
	s.False(char.IsDirty(), "nothing changed, so there is nothing to save")
}

// The other order, to pin that the check is complete before any write rather
// than merely lucky about which currency it looks at first.
func (s *SheetLedgerTestSuite) TestARefusedPaymentChangesNothingWhateverIsShort() {
	char := s.loaded()
	char.SpendSlots(coreCombat.ActionStandard, 1)
	char.MarkClean()

	profile := &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Pools: map[coreResources.ResourceKey]int{resources.Ki: 1},
	}

	s.Require().Error(combat.Pay(char, profile))
	s.Equal(3, char.GetResource(resources.Ki).Current())
	s.False(char.IsDirty())
}

// A sheet that is not in combat has no economy, so it cannot pay — not even a
// profile that only grants, which would otherwise land nowhere and report
// success.
func (s *SheetLedgerTestSuite) TestASheetOutOfCombatCannotPay() {
	char, err := LoadFromData(s.ctx, s.monk(), s.bus)
	s.Require().NoError(err)
	char.MarkClean()
	s.Require().False(char.InCombat())

	profiles := []*combat.SpendProfile{
		{Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}},
		{Grants: map[combat.CapacityType]int{combat.CapacityAttack: 2}},
		{Pools: map[coreResources.ResourceKey]int{resources.Ki: 1}},
	}

	for _, profile := range profiles {
		s.False(combat.CanPay(char, profile))
		s.Require().Error(combat.Pay(char, profile))
	}

	s.False(char.IsDirty())
	s.Equal(3, char.GetResource(resources.Ki).Current())
}

// Every declared capacity round-trips through the sheet. The census's finding
// was that the bridge between the fielded economy and the persisted keyed one
// carried three of five keys; a key the sheet cannot store is a grant that
// disappears without an error, so the pin is over the whole vocabulary rather
// than over the ones we happen to compile today.
func (s *SheetLedgerTestSuite) TestEveryDeclaredCapacityRoundTripsThroughTheSheet() {
	for _, key := range combat.CapacityTypes() {
		char := s.loaded()

		before := char.CapacityLeft(key)
		char.BankCapacity(key, 3)
		s.Require().Equalf(before+3, char.CapacityLeft(key), "banked %q", key)

		char.SpendCapacity(key, 2)
		s.Require().Equalf(before+1, char.CapacityLeft(key), "spent %q", key)
	}
}

// A capacity the sheet has no name for is stored under its own key rather than
// dropped. This is the rule that keeps the bridge total as the vocabulary grows.
func (s *SheetLedgerTestSuite) TestAnUndeclaredCapacityIsStoredNotDropped() {
	char := s.loaded()
	future := combat.CapacityType("arcane_recovery_die")

	char.BankCapacity(future, 2)
	s.Equal(2, char.CapacityLeft(future))
	s.True(char.IsDirty())

	char.SpendCapacity(future, 2)
	s.Equal(0, char.CapacityLeft(future))
}

// The keyed capacity view reaches the same storage the sheet already had, so a
// gate spend and a feature grant are the same state rather than two.
func (s *SheetLedgerTestSuite) TestTheKeyedViewIsTheStoredEconomy() {
	char := s.loaded()

	char.GrantCapacity(GrantedAttacks, 2)
	s.Equal(2, char.CapacityLeft(combat.CapacityAttack), "the gate reads what a feature banked")

	char.BankCapacity(combat.CapacityAttack, 1)
	s.Equal(3, char.GetActionEconomy().Granted[GrantedAttacks], "and a feature reads what the gate banked")
}

// Movement is keyed capacity to the gate and a field on the sheet, and they
// are one thing. What a path costs is the path's business — the profile only
// ever says "spend this much".
func (s *SheetLedgerTestSuite) TestMovementIsKeyedCapacityOverTheStoredField() {
	char := s.loaded()
	s.Require().Equal(30, char.CapacityLeft(combat.CapacityMovement))

	s.Require().NoError(combat.Pay(char, &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{combat.CapacityMovement: 25},
	}))

	s.Equal(5, char.GetActionEconomy().MovementRemaining)
	s.False(combat.CanPay(char, &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{combat.CapacityMovement: 10},
	}))
}

// The three per-turn slots read from the three fields the sheet keeps, and a
// slot the sheet does not keep reads as empty rather than as unlimited.
func (s *SheetLedgerTestSuite) TestSlotsReadTheStoredEconomy() {
	char := s.loaded()

	s.Equal(1, char.SlotsLeft(coreCombat.ActionStandard))
	s.Equal(1, char.SlotsLeft(coreCombat.ActionBonus))
	s.Equal(1, char.SlotsLeft(coreCombat.ActionReaction))
	s.Equal(0, char.SlotsLeft(coreCombat.ActionFree))
	s.Equal(0, char.SlotsLeft(coreCombat.ActionMovement))

	char.SpendSlots(coreCombat.ActionReaction, 1)
	s.Equal(0, char.GetActionEconomy().ReactionsRemaining)
	s.Equal(1, char.GetActionEconomy().ActionsRemaining)
}

// A pool the sheet does not have reads as empty, so a cost in it is refused
// rather than charged to nothing.
func (s *SheetLedgerTestSuite) TestAnAbsentPoolIsEmptyRatherThanFree() {
	char := s.loaded()

	s.Equal(3, char.PoolLeft(resources.Ki))
	s.Equal(0, char.PoolLeft(coreResources.ResourceKey("sorcery_points")))

	s.False(combat.CanPay(char, &combat.SpendProfile{
		Pools: map[coreResources.ResourceKey]int{"sorcery_points": 1},
	}))
}
