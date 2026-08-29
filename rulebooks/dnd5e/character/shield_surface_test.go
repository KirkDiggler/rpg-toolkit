// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// A character is a combat.Combatant, for the reason the monster side states.
var _ combat.Combatant = (*Character)(nil)

// ShieldSurfaceTestSuite pins the character half of the member surface's
// shield question.
//
// HasShieldEquipped itself is unchanged here. What changed is WHO may ask it:
// it used to be reachable only by a condition holding this exact sheet through
// the owner handle, and it is now a question on combat.Member, which is the
// surface a rule reads a participant through. It had no direct test of its own
// under the old arrangement, and it is about to carry Unarmored Movement.
type ShieldSurfaceTestSuite struct {
	suite.Suite
	char *Character
}

func (s *ShieldSurfaceTestSuite) SetupTest() {
	longsword := weapons.All[weapons.Longsword]
	shield := armor.All[armor.Shield]
	chainMail := armor.All[armor.ChainMail]

	s.char = &Character{
		inventory: []InventoryItem{
			{Equipment: &longsword, Quantity: 1},
			{Equipment: &shield, Quantity: 1},
			{Equipment: &chainMail, Quantity: 1},
		},
		equipmentSlots: make(EquipmentSlots),
	}
}

func TestShieldSurfaceSuite(t *testing.T) { suite.Run(t, new(ShieldSurfaceTestSuite)) }

// A weapon in hand is not a shield. This is also the only test that reaches
// the nil guard: AsArmor answers nil for a weapon, so a version that dropped
// the guard and read the category straight off would panic here.
func (s *ShieldSurfaceTestSuite) TestAWeaponInHandIsNotAShield() {
	s.Require().NoError(s.char.EquipItem(SlotMainHand, weapons.Longsword))
	s.Require().NoError(s.char.EquipItem(SlotArmor, armor.ChainMail))

	s.Assert().False(s.char.HasShieldEquipped(),
		"the body armour is in the armour slot, which this question does not read at all")
}

// BODY ARMOUR IN A HAND SLOT is not a shield, and this is the test that says
// so — the earlier version of it put chain mail in SlotArmor and proved
// nothing, because the armour slot is not one of the two this reads. Dropping
// the category check survived that test and dies against this one.
//
// Written straight into the slot for TestAShieldInTheMainHandIsStillAShield's
// reason: CompatibleSlots gives non-shield armour the armour slot and nothing
// else, so the equip path cannot produce this and a stored sheet can.
func (s *ShieldSurfaceTestSuite) TestBodyArmourInAHandSlotIsNotAShield() {
	chainMail := armor.All[armor.ChainMail]
	s.Require().NotContains(CompatibleSlots(&chainMail), SlotOffHand,
		"if body armour becomes hand-equippable, this test stops being about persisted data")

	s.char.equipmentSlots[SlotOffHand] = armor.ChainMail

	s.Assert().False(s.char.HasShieldEquipped(),
		"a shield is a CATEGORY of armour, not any armour that happens to be in a hand")
}

func (s *ShieldSurfaceTestSuite) TestAShieldInTheOffHandIsAShield() {
	s.Require().NoError(s.char.EquipItem(SlotOffHand, armor.Shield))

	s.Assert().True(s.char.HasShieldEquipped())
}

// The main-hand arm is reachable from PERSISTED data, not from the equip path.
// CompatibleSlots gives a shield the off hand and nothing else, so EquipItem
// can never put one in the main hand — asserted here so that if the equip rule
// ever changes, this test says what this arm was actually for. A stored sheet
// can name any slot, which is why the loop reads both, so the slot is written
// the way a load writes it rather than through a call that would refuse.
func (s *ShieldSurfaceTestSuite) TestAShieldInTheMainHandIsStillAShield() {
	shield := armor.All[armor.Shield]
	s.Require().NotContains(CompatibleSlots(&shield), SlotMainHand,
		"if equipping a shield to the main hand becomes legal, this test is no longer about persisted data")

	s.char.equipmentSlots[SlotMainHand] = armor.Shield

	s.Assert().True(s.char.HasShieldEquipped())
}
