// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// EquipPriceTestSuite pins the in-fight equip price (rpg-project#542 R1, R6)
// against a sheet readied for its turn.
type EquipPriceTestSuite struct {
	suite.Suite
	char *Character
}

func TestEquipPriceSuite(t *testing.T) {
	suite.Run(t, new(EquipPriceTestSuite))
}

func (s *EquipPriceTestSuite) SetupTest() {
	longsword := weapons.All[weapons.Longsword]
	greatsword := weapons.All[weapons.Greatsword]
	handaxe := weapons.All[weapons.Handaxe]
	shield := armor.All[armor.Shield]
	chainMail := armor.All[armor.ChainMail]

	s.char = &Character{
		id: "pricer",
		inventory: []InventoryItem{
			{Equipment: &longsword, Quantity: 1},
			{Equipment: &greatsword, Quantity: 1},
			{Equipment: &handaxe, Quantity: 1},
			{Equipment: &shield, Quantity: 1},
			{Equipment: &chainMail, Quantity: 1},
		},
		equipmentSlots: make(EquipmentSlots),
	}
	_, err := s.char.StartTurn(context.Background(), &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
}

func (s *EquipPriceTestSuite) price(slot InventorySlot, itemID string) *PriceEquipmentOutput {
	out, err := s.char.PriceEquipment(&PriceEquipmentInput{Slot: slot, ItemID: itemID})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	return out
}

// itemRefs names inventory ids as the refs a plan reports.
func itemRefs(ids ...string) []*core.Ref {
	out := make([]*core.Ref, 0, len(ids))
	for _, id := range ids {
		kind := refs.TypeEquipment
		if _, ok := weapons.All[weapons.WeaponID(id)]; ok {
			kind = refs.TypeWeapons
		} else if _, ok := armor.All[armor.ArmorID(id)]; ok {
			kind = refs.TypeArmor
		}
		out = append(out, &core.Ref{Module: refs.Module, Type: kind, ID: core.ID(id)})
	}
	return out
}

func (s *EquipPriceTestSuite) plan(slot InventorySlot, itemID string) *PlanEquipmentOutput {
	out, err := s.char.PlanEquipment(&PlanEquipmentInput{Slot: slot, ItemID: itemID})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	return out
}

// The plan needs no turn and covers every slot: body armour donned is a draw
// and doffed is a stow.
func (s *EquipPriceTestSuite) TestPlanArmourDonIsADrawAndDoffAStow() {
	_, err := s.char.ExitCombat(context.Background(), nil)
	s.Require().NoError(err)

	on := s.plan(SlotArmor, armor.ChainMail)
	s.Equal(itemRefs(armor.ChainMail), on.Drawn)
	s.Empty(on.Stowed)
	s.Equal("dnd5e:armor:chain-mail", on.Drawn[0].String())

	s.Require().NoError(s.char.EquipItem(SlotArmor, armor.ChainMail))
	off := s.plan(SlotArmor, "")
	s.Equal(itemRefs(armor.ChainMail), off.Stowed)
	s.Empty(off.Drawn)
}

func (s *EquipPriceTestSuite) TestPlanASwapIsBoth() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Handaxe))

	out := s.plan(SlotMainHand, weapons.Longsword)
	s.Equal(itemRefs(weapons.Handaxe), out.Stowed)
	s.Equal(itemRefs(weapons.Longsword), out.Drawn)
	s.Equal("dnd5e:weapons:longsword", out.Drawn[0].String())
}

func (s *EquipPriceTestSuite) TestPlanANoChangeIsEmpty() {
	s.Require().NoError(s.char.EquipItem(SlotOffHand, armor.Shield))

	same := s.plan(SlotOffHand, armor.Shield)
	s.Empty(same.Stowed)
	s.Empty(same.Drawn)

	nothing := s.plan(SlotArmor, "")
	s.Empty(nothing.Stowed)
	s.Empty(nothing.Drawn)
}

// The price reports exactly what the plan reports for the same change.
func (s *EquipPriceTestSuite) TestThePriceReadsThePlan() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))
	s.Require().NoError(s.char.EquipItem(SlotOffHand, weapons.Handaxe))

	plan := s.plan(SlotMainHand, weapons.Greatsword)
	price := s.price(SlotMainHand, weapons.Greatsword)
	s.Equal(plan.Stowed, price.Stowed)
	s.Equal(plan.Drawn, price.Drawn)
}

func action(n int) map[coreCombat.ActionType]int {
	return map[coreCombat.ActionType]int{coreCombat.ActionStandard: n}
}

func interaction(n int) map[combat.CapacityType]int {
	return map[combat.CapacityType]int{combat.CapacityObjectInteraction: n}
}

// A readied turn holds exactly one object interaction, and the next turn's
// refresh restores it after it was spent.
func (s *EquipPriceTestSuite) TestATurnHoldsOneObjectInteractionAndTheNextRestoresIt() {
	ctx := context.Background()
	s.Equal(1, s.char.CapacityLeft(combat.CapacityObjectInteraction))

	s.Require().NoError(combat.Pay(s.char, &combat.SpendProfile{Capacity: interaction(1)}))
	s.Equal(0, s.char.CapacityLeft(combat.CapacityObjectInteraction))
	s.False(combat.CanPay(s.char, &combat.SpendProfile{Capacity: interaction(1)}),
		"a turn holds one, not two")

	out, err := s.char.RefreshForTurn(ctx, &RefreshForTurnInput{TurnNumber: 2, Speed: 30})
	s.Require().NoError(err)
	s.Require().True(out.Reseeded)
	s.Equal(1, s.char.CapacityLeft(combat.CapacityObjectInteraction))
}

func (s *EquipPriceTestSuite) TestDrawingIntoAnEmptyMainHandCostsTheInteraction() {
	out := s.price(SlotMainHand, weapons.Longsword)

	s.Require().NotNil(out.Profile)
	s.Empty(out.Profile.Slots, "a draw does not touch the action")
	s.Equal(interaction(1), out.Profile.Capacity)
	s.Equal(itemRefs(weapons.Longsword), out.Drawn)
	s.Empty(out.Stowed)
}

func (s *EquipPriceTestSuite) TestStowingAHeldLongswordCostsTheAction() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))

	out := s.price(SlotMainHand, "")

	s.Require().NotNil(out.Profile)
	s.Equal(action(1), out.Profile.Slots)
	s.Empty(out.Profile.Capacity)
	s.Equal(itemRefs(weapons.Longsword), out.Stowed)
}

func (s *EquipPriceTestSuite) TestASwapCostsBoth() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Handaxe))

	out := s.price(SlotMainHand, weapons.Longsword)

	s.Require().NotNil(out.Profile)
	s.Equal(action(1), out.Profile.Slots, "the stow")
	s.Equal(interaction(1), out.Profile.Capacity, "the draw")
	s.Equal(itemRefs(weapons.Handaxe), out.Stowed)
	s.Equal(itemRefs(weapons.Longsword), out.Drawn)
}

func (s *EquipPriceTestSuite) TestADrawAfterTheInteractionIsSpentCostsTheAction() {
	s.Require().NoError(combat.Pay(s.char, &combat.SpendProfile{Capacity: interaction(1)}))

	out := s.price(SlotMainHand, weapons.Longsword)

	s.Require().NotNil(out.Profile)
	s.Equal(action(1), out.Profile.Slots)
	s.Empty(out.Profile.Capacity)
}

// R6: a shield is not a weapon. Donning and doffing it are the action, even
// with the interaction unspent.
func (s *EquipPriceTestSuite) TestAShieldIsTheActionOnAndOff() {
	on := s.price(SlotOffHand, armor.Shield)
	s.Require().NotNil(on.Profile)
	s.Equal(action(1), on.Profile.Slots)
	s.Empty(on.Profile.Capacity)

	s.Require().NoError(s.char.EquipItem(SlotOffHand, armor.Shield))
	off := s.price(SlotOffHand, "")
	s.Require().NotNil(off.Profile)
	s.Equal(action(1), off.Profile.Slots)
	s.Empty(off.Profile.Capacity)
}

// R6: body armour cannot change in a fight, on or off.
func (s *EquipPriceTestSuite) TestBodyArmourIsRefusedInAFight() {
	_, err := s.char.PriceEquipment(&PriceEquipmentInput{Slot: SlotArmor, ItemID: armor.ChainMail})
	s.Require().Error(err)
	s.True(errors.Is(err, ErrArmorInFight), "the caller translates the typed refusal")

	s.Require().NoError(s.char.EquipItem(SlotArmor, armor.ChainMail))
	_, err = s.char.PriceEquipment(&PriceEquipmentInput{Slot: SlotArmor})
	s.Require().Error(err)
	s.True(errors.Is(err, ErrArmorInFight))
}

// The occupancy rules are EquipItem's: drawing a two-hander while holding two
// items stows both, and each stow is the action.
func (s *EquipPriceTestSuite) TestATwoHanderOverTwoHeldItemsStowsBoth() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))
	s.Require().NoError(s.char.EquipItem(SlotOffHand, weapons.Handaxe))

	out := s.price(SlotMainHand, weapons.Greatsword)

	s.Require().NotNil(out.Profile)
	s.ElementsMatch(itemRefs(weapons.Longsword, weapons.Handaxe), out.Stowed)
	s.Equal(action(2), out.Profile.Slots)
	s.Equal(interaction(1), out.Profile.Capacity)
}

// An item that only changes hands is in hand before and after: free.
func (s *EquipPriceTestSuite) TestMovingAnItemBetweenHandsIsFree() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))

	out := s.price(SlotOffHand, weapons.Longsword)

	s.Nil(out.Profile)
	s.Empty(out.Stowed)
	s.Empty(out.Drawn)
}

// Pricing writes nothing: not the slots, not the ledger, not the dirty mark.
func (s *EquipPriceTestSuite) TestPricingWritesNothing() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Handaxe))
	markSaved(s.char)
	slots := maps.Clone(s.char.equipmentSlots)
	economy := *s.char.actionEconomy
	granted := maps.Clone(s.char.actionEconomy.Granted)

	s.price(SlotMainHand, weapons.Longsword)

	s.Equal(slots, s.char.equipmentSlots)
	s.Equal(granted, s.char.actionEconomy.Granted)
	economy.Granted = s.char.actionEconomy.Granted
	s.Equal(economy, *s.char.actionEconomy)
	s.False(s.char.IsDirty())
}

// No readied turn, no price: an absent economy would always read the
// interaction as spent.
func (s *EquipPriceTestSuite) TestASheetWithNoTurnIsNotPriced() {
	_, err := s.char.ExitCombat(context.Background(), nil)
	s.Require().NoError(err)

	out, err := s.char.PriceEquipment(&PriceEquipmentInput{Slot: SlotMainHand, ItemID: weapons.Longsword})
	s.Require().Error(err)
	s.Nil(out)
}

// A change EquipItem would refuse is refused with EquipItem's own answer.
func (s *EquipPriceTestSuite) TestAnEquipEquipItemRefusesIsRefused() {
	_, err := s.char.PriceEquipment(&PriceEquipmentInput{Slot: SlotMainHand, ItemID: "not-carried"})
	s.Require().Error(err)
	s.Equal(rpgerr.CodeNotFound, rpgerr.GetCode(err))

	_, err = s.char.PriceEquipment(&PriceEquipmentInput{Slot: SlotOffHand, ItemID: weapons.Greatsword})
	s.Require().Error(err)
	s.Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
}

// The priced profile is one the gate can charge, and charging it then applying
// the change lands the change with exactly that price spent.
func (s *EquipPriceTestSuite) TestThePriceIsWhatTheGateCharges() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Handaxe))
	out := s.price(SlotMainHand, weapons.Longsword)

	s.Require().NoError(out.Profile.Validate())
	s.Require().NoError(combat.Pay(s.char, out.Profile))
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))

	s.Equal(0, s.char.SlotsLeft(coreCombat.ActionStandard))
	s.Equal(0, s.char.CapacityLeft(combat.CapacityObjectInteraction))
	s.Equal(weapons.Longsword, s.char.equipmentSlots.Get(SlotMainHand))
}
