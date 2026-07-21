// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// EquipOccupancyTestSuite ports the two-handed occupancy behaviors from
// rpg-dnd5e-web#557 src/concepts/equipment/fixtures.test.ts — the fixture
// reducer's applyIntent semantics are the acceptance spec for this rule
// (rpg-toolkit#811). The fixture file's other describe block
// (targetSlotFor) covers client-side click-targeting, a UI convenience
// with no state-mutating equivalent here — see the PR's scope-decisions.
type EquipOccupancyTestSuite struct {
	suite.Suite
	char *Character
}

func (s *EquipOccupancyTestSuite) SetupTest() {
	longsword := weapons.All[weapons.Longsword]
	greatsword := weapons.All[weapons.Greatsword]
	handaxe := weapons.All[weapons.Handaxe]
	shield := armor.All[armor.Shield]
	chainMail := armor.All[armor.ChainMail]

	s.char = &Character{
		inventory: []InventoryItem{
			{Equipment: &longsword, Quantity: 1},
			{Equipment: &greatsword, Quantity: 1},
			{Equipment: &handaxe, Quantity: 1},
			{Equipment: &shield, Quantity: 1},
			{Equipment: &chainMail, Quantity: 1},
		},
		equipmentSlots: make(EquipmentSlots),
	}
}

func TestEquipOccupancySuite(t *testing.T) {
	suite.Run(t, new(EquipOccupancyTestSuite))
}

// equips a carried item into an empty compatible slot
func (s *EquipOccupancyTestSuite) TestEquipsIntoEmptyCompatibleSlot() {
	err := s.char.EquipItem(SlotMainHand, weapons.Longsword)
	s.Require().NoError(err)
	s.Assert().Equal(weapons.Longsword, s.char.equipmentSlots[SlotMainHand])
}

// two-handed equip clears the off hand
func (s *EquipOccupancyTestSuite) TestTwoHandedEquipClearsOffHand() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))
	s.Require().NoError(s.char.EquipItem(SlotOffHand, armor.Shield))

	err := s.char.EquipItem(SlotMainHand, weapons.Greatsword)
	s.Require().NoError(err)

	s.Assert().Equal(weapons.Greatsword, s.char.equipmentSlots[SlotMainHand])
	_, offHandOccupied := s.char.equipmentSlots[SlotOffHand]
	s.Assert().False(offHandOccupied)
}

// equipping the off hand while a two-hander is held frees the main hand
func (s *EquipOccupancyTestSuite) TestEquippingOffHandFreesMainHandFromTwoHander() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Greatsword))

	err := s.char.EquipItem(SlotOffHand, armor.Shield)
	s.Require().NoError(err)

	s.Assert().Equal(armor.Shield, s.char.equipmentSlots[SlotOffHand])
	_, mainHandOccupied := s.char.equipmentSlots[SlotMainHand]
	s.Assert().False(mainHandOccupied)
}

// unequip empties only the named slot
func (s *EquipOccupancyTestSuite) TestUnequipEmptiesOnlyNamedSlot() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))
	s.Require().NoError(s.char.EquipItem(SlotArmor, armor.ChainMail))

	s.char.UnequipItem(SlotMainHand)

	_, mainHandOccupied := s.char.equipmentSlots[SlotMainHand]
	s.Assert().False(mainHandOccupied)
	s.Assert().Equal(armor.ChainMail, s.char.equipmentSlots[SlotArmor])
}

// rejects an incompatible slot (no state change)
func (s *EquipOccupancyTestSuite) TestRejectsIncompatibleSlot() {
	s.Require().NoError(s.char.EquipItem(SlotArmor, armor.ChainMail))

	err := s.char.EquipItem(SlotMainHand, armor.ChainMail)

	s.Require().Error(err)
	s.Assert().Equal(armor.ChainMail, s.char.equipmentSlots[SlotArmor])
	_, mainHandOccupied := s.char.equipmentSlots[SlotMainHand]
	s.Assert().False(mainHandOccupied)
}

// moving an equipped item between slots vacates its old slot
func (s *EquipOccupancyTestSuite) TestMovingEquippedItemVacatesOldSlot() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))

	err := s.char.EquipItem(SlotOffHand, weapons.Longsword)
	s.Require().NoError(err)

	s.Assert().Equal(weapons.Longsword, s.char.equipmentSlots[SlotOffHand])
	_, mainHandOccupied := s.char.equipmentSlots[SlotMainHand]
	s.Assert().False(mainHandOccupied)
}

// equipping into an occupied slot returns the previous occupant to
// inventory (i.e. it's simply no longer equipped anywhere; it stays in
// the character's inventory list)
func (s *EquipOccupancyTestSuite) TestEquippingOccupiedSlotSwapsOccupant() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))

	err := s.char.EquipItem(SlotMainHand, weapons.Handaxe)
	s.Require().NoError(err)

	s.Assert().Equal(weapons.Handaxe, s.char.equipmentSlots[SlotMainHand])
	// longsword is no longer equipped anywhere, but remains in inventory
	for _, id := range s.char.equipmentSlots {
		s.Assert().NotEqual(weapons.Longsword, id)
	}
	found := false
	for _, invItem := range s.char.inventory {
		if invItem.Equipment.EquipmentID() == weapons.Longsword {
			found = true
		}
	}
	s.Assert().True(found, "unequipped item should remain in inventory")
}

// TestTwoHandedEquipVacatesStaleSlot is a regression test for a Copilot
// finding on #812: the two-handed branch's early return used to run
// before the vacate-old-slot loop, so if EquipmentSlots ever held the
// same itemID under a stale key OTHER than off_hand (unreachable through
// EquipItem alone — equipmentFitsSlot only ever lets a two-handed weapon
// occupy main hand — but possible via corrupted/legacy persisted state,
// since LoadFromData copies EquipmentSlots verbatim with no validation),
// that stale mapping would survive equipping the two-hander into main
// hand, leaving the same item nominally equipped in two slots at once.
// (A stale off_hand entry specifically doesn't exercise this: the
// two-handed branch already unconditionally clears off_hand as its own
// occupancy rule, incidentally masking that case — this uses armor,
// which nothing in the two-handed branch touches, to isolate the bug.)
func (s *EquipOccupancyTestSuite) TestTwoHandedEquipVacatesStaleSlot() {
	// Simulate corrupted/legacy state directly — greatsword already
	// (invalidly) mapped under armor, as no legitimate EquipItem call
	// could produce.
	s.char.equipmentSlots[SlotArmor] = weapons.Greatsword

	err := s.char.EquipItem(SlotMainHand, weapons.Greatsword)
	s.Require().NoError(err)

	s.Assert().Equal(weapons.Greatsword, s.char.equipmentSlots[SlotMainHand])
	_, armorSlotStillHoldsIt := s.char.equipmentSlots[SlotArmor]
	s.Assert().False(armorSlotStillHoldsIt, "stale armor-slot mapping must be vacated, not left duplicated")
}
