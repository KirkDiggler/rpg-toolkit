// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// CompatibleSlotsTestSuite covers CompatibleSlots — the exported, single
// source of truth for slot compatibility that EquipItem validates against
// and EquipmentView projects as each item's SlotKeys (rpg-toolkit#811).
type CompatibleSlotsTestSuite struct {
	suite.Suite
}

func TestCompatibleSlotsSuite(t *testing.T) {
	suite.Run(t, new(CompatibleSlotsTestSuite))
}

func (s *CompatibleSlotsTestSuite) TestCompatibleSlots() {
	greatsword := weapons.All[weapons.Greatsword]
	dagger := weapons.All[weapons.Dagger]
	shield := armor.All[armor.Shield]
	chainMail := armor.All[armor.ChainMail]
	componentPouch := items.All[items.ComponentPouch]

	testCases := []struct {
		name     string
		item     equipment.Equipment
		expected []InventorySlot
	}{
		{"two-handed weapon", &greatsword, []InventorySlot{SlotMainHand}},
		{"one-handed weapon", &dagger, []InventorySlot{SlotMainHand, SlotOffHand}},
		{"shield", &shield, []InventorySlot{SlotOffHand}},
		{"armor", &chainMail, []InventorySlot{SlotArmor}},
		{"slotless gear", &componentPouch, nil},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Assert().Equal(tc.expected, CompatibleSlots(tc.item))
		})
	}
}

func (s *CompatibleSlotsTestSuite) TestItemKind() {
	greatsword := weapons.All[weapons.Greatsword]
	shield := armor.All[armor.Shield]
	chainMail := armor.All[armor.ChainMail]
	componentPouch := items.All[items.ComponentPouch]

	testCases := []struct {
		name     string
		item     equipment.Equipment
		expected string
	}{
		{"weapon", &greatsword, "weapon"},
		{"shield", &shield, "shield"},
		{"armor", &chainMail, "armor"},
		{"gear", &componentPouch, "gear"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Assert().Equal(tc.expected, itemKind(tc.item))
		})
	}
}
