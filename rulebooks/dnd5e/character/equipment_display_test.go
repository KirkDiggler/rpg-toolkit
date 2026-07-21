// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// EquipmentDisplayTestSuite covers the display projection (rpg-toolkit#811):
// StatLine composed from equipment.ResolveEquipmentDetail, ACNote composed
// from combat.ACBreakdown, and the EquipmentView accessor that gives
// rpg-api both with zero rules knowledge.
type EquipmentDisplayTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *EquipmentDisplayTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestEquipmentDisplaySuite(t *testing.T) {
	suite.Run(t, new(EquipmentDisplayTestSuite))
}

func (s *EquipmentDisplayTestSuite) TestStatLine() {
	testCases := []struct {
		name     string
		id       shared.EquipmentID
		expected string
	}{
		{"versatile weapon", weapons.Longsword, "1d8 slashing · versatile"},
		{"two-handed weapon", weapons.Greatsword, "2d6 slashing · heavy, two-handed"},
		{"thrown weapon with range", weapons.Handaxe, "1d6 slashing · light, thrown 20/60"},
		{"finesse light thrown weapon", weapons.Dagger, "1d4 piercing · finesse, light, thrown 20/60"},
		{"heavy armor, no dex bonus", armor.ChainMail, "AC 16 · heavy"},
		{"light armor", armor.Leather, "AC 11 · light"},
		{"shield", armor.Shield, "+2 AC"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			detail := equipment.ResolveEquipmentDetail(tc.id)
			s.Require().NotNil(detail)
			s.Assert().Equal(tc.expected, StatLine(detail))
		})
	}
}

func (s *EquipmentDisplayTestSuite) TestStatLine_Nil() {
	s.Assert().Equal("", StatLine(nil))
}

func (s *EquipmentDisplayTestSuite) TestStatLine_GearHasNoCombatStatLine() {
	detail := equipment.ResolveEquipmentDetail(items.ComponentPouch)
	s.Require().NotNil(detail)
	s.Assert().Equal("", StatLine(detail))
}

func (s *EquipmentDisplayTestSuite) TestACNote() {
	dexRef := refs.Abilities.Dexterity()
	unarmoredDefenseRef := refs.Conditions.UnarmoredDefense()
	armorRef := func(id shared.EquipmentID) *core.Ref {
		return &core.Ref{Module: refs.Module, Type: "armor", ID: id}
	}

	testCases := []struct {
		name       string
		components []combat.ACComponent
		expected   string
	}{
		{
			name: "armor plus shield, no dex bonus",
			components: []combat.ACComponent{
				{Type: combat.ACSourceArmor, Source: armorRef(armor.ChainMail), Value: 16},
				{Type: combat.ACSourceShield, Source: armorRef(armor.Shield), Value: 2},
			},
			expected: "16 chain mail + 2 shield",
		},
		{
			name: "light armor plus dex",
			components: []combat.ACComponent{
				{Type: combat.ACSourceArmor, Source: armorRef(armor.Leather), Value: 11},
				{Type: combat.ACSourceAbility, Source: dexRef, Value: 3},
			},
			expected: "11 leather + 3 DEX",
		},
		{
			name: "unarmored, no dex bonus",
			components: []combat.ACComponent{
				{Type: combat.ACSourceBase, Source: nil, Value: 10},
			},
			expected: "10",
		},
		{
			name: "unarmored with dex",
			components: []combat.ACComponent{
				{Type: combat.ACSourceBase, Source: nil, Value: 10},
				{Type: combat.ACSourceAbility, Source: dexRef, Value: 2},
			},
			expected: "10 + 2 DEX",
		},
		{
			name: "negative dex modifier",
			components: []combat.ACComponent{
				{Type: combat.ACSourceArmor, Source: armorRef(armor.Leather), Value: 11},
				{Type: combat.ACSourceAbility, Source: dexRef, Value: -1},
			},
			expected: "11 leather - 1 DEX",
		},
		{
			name: "unarmored defense feature component",
			components: []combat.ACComponent{
				{Type: combat.ACSourceBase, Source: nil, Value: 10},
				{Type: combat.ACSourceAbility, Source: dexRef, Value: 2},
				{Type: combat.ACSourceFeature, Source: unarmoredDefenseRef, Value: 3},
			},
			expected: "10 + 2 DEX + 3 (Unarmored Defense)",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			breakdown := &combat.ACBreakdown{Components: tc.components}
			s.Assert().Equal(tc.expected, ACNote(breakdown))
		})
	}
}

func (s *EquipmentDisplayTestSuite) TestACNote_NilOrEmpty() {
	s.Assert().Equal("", ACNote(nil))
	s.Assert().Equal("", ACNote(&combat.ACBreakdown{}))
}

// TestEquipmentView proves the full wiring: a real character's equipped
// gear projects into items with composed stat_line, plus AC total + note
// derived from EffectiveAC — the shape rpg-api consumes with zero rules
// knowledge.
func (s *EquipmentDisplayTestSuite) TestEquipmentView() {
	chainMail := armor.All[armor.ChainMail]
	shield := armor.All[armor.Shield]
	longsword := weapons.All[weapons.Longsword]

	char := &Character{
		id:            "char-1",
		bus:           s.bus,
		abilityScores: shared.AbilityScores{abilities.DEX: 10}, // +0 mod
		inventory: []InventoryItem{
			{Equipment: &chainMail, Quantity: 1},
			{Equipment: &shield, Quantity: 1},
			{Equipment: &longsword, Quantity: 1},
		},
		equipmentSlots: EquipmentSlots{
			SlotArmor:    armor.ChainMail,
			SlotOffHand:  armor.Shield,
			SlotMainHand: "longsword",
		},
	}

	view := char.EquipmentView(s.ctx)
	s.Require().NotNil(view)
	s.Assert().Equal(18, view.ACTotal)
	s.Assert().Equal("16 chain mail + 2 shield", view.ACNote)
	// shield occupies off hand, so the versatile longsword is gripped
	// one-handed: its base die, not the versatile two-handed upgrade.
	s.Assert().Equal("1d8 slashing", view.MainHandDamage)
	s.Require().Len(view.Items, 3)

	byID := make(map[string]EquippedItemView, len(view.Items))
	for _, it := range view.Items {
		byID[it.ItemID] = it
	}

	s.Assert().Equal(SlotArmor, byID[armor.ChainMail].Slot)
	s.Assert().Equal("AC 16 · heavy", byID[armor.ChainMail].StatLine)
	s.Assert().Equal("Chain Mail", byID[armor.ChainMail].Name)
	s.Assert().Equal("armor", byID[armor.ChainMail].Kind)
	s.Assert().Equal([]string{"armor"}, byID[armor.ChainMail].SlotKeys)

	s.Assert().Equal(SlotOffHand, byID[armor.Shield].Slot)
	s.Assert().Equal("+2 AC", byID[armor.Shield].StatLine)
	s.Assert().Equal("Shield", byID[armor.Shield].Name)
	s.Assert().Equal("shield", byID[armor.Shield].Kind)
	s.Assert().Equal([]string{"off_hand"}, byID[armor.Shield].SlotKeys)

	s.Assert().Equal(InventorySlot(SlotMainHand), byID["longsword"].Slot)
	s.Assert().Equal("1d8 slashing · versatile", byID["longsword"].StatLine)
	s.Assert().Equal("Longsword", byID["longsword"].Name)
	s.Assert().Equal("weapon", byID["longsword"].Kind)
	s.Assert().Equal([]string{"main_hand", "off_hand"}, byID["longsword"].SlotKeys)

	s.Require().Len(view.Slots, 3)
	s.Assert().Equal([]SlotDefView{
		{Key: "main_hand", DisplayLabel: "Main hand", Accepts: []string{"weapon"}},
		{Key: "off_hand", DisplayLabel: "Off hand", Accepts: []string{"weapon", "shield"}},
		{Key: "armor", DisplayLabel: "Armor", Accepts: []string{"armor"}},
	}, view.Slots)
}

// TestEquipmentView_CarriedItemHasNoSlot proves inventory items that
// aren't equipped are still listed, just with an empty Slot.
func (s *EquipmentDisplayTestSuite) TestEquipmentView_CarriedItemHasNoSlot() {
	handaxe := weapons.All[weapons.Handaxe]

	char := &Character{
		id:             "char-2",
		bus:            s.bus,
		abilityScores:  shared.AbilityScores{abilities.DEX: 10},
		inventory:      []InventoryItem{{Equipment: &handaxe, Quantity: 1}},
		equipmentSlots: make(EquipmentSlots),
	}

	view := char.EquipmentView(s.ctx)
	s.Require().Len(view.Items, 1)
	s.Assert().Equal(InventorySlot(""), view.Items[0].Slot)
	s.Assert().Equal("1d6 slashing · light, thrown 20/60", view.Items[0].StatLine)
	s.Assert().Equal("Handaxe", view.Items[0].Name)
	s.Assert().Equal("weapon", view.Items[0].Kind)
	s.Assert().Equal([]string{"main_hand", "off_hand"}, view.Items[0].SlotKeys)
}

// TestEquipmentView_SlotlessGear proves gear with no combat-relevant slot
// projects with an empty SlotKeys and Kind "gear".
func (s *EquipmentDisplayTestSuite) TestEquipmentView_SlotlessGear() {
	pouch := items.All[items.ComponentPouch]

	char := &Character{
		id:             "char-5",
		bus:            s.bus,
		abilityScores:  shared.AbilityScores{abilities.DEX: 10},
		inventory:      []InventoryItem{{Equipment: &pouch, Quantity: 1}},
		equipmentSlots: make(EquipmentSlots),
	}

	view := char.EquipmentView(s.ctx)
	s.Require().Len(view.Items, 1)
	s.Assert().Equal("gear", view.Items[0].Kind)
	s.Assert().Nil(view.Items[0].SlotKeys)
	s.Assert().Equal("", view.Items[0].StatLine)
}

// TestEquipmentView_VersatileFreeOffHand proves a versatile weapon grips
// two-handed (upgraded die) when the off hand is completely empty — the
// occupancy-dependent half of contract §5's main_hand_damage.
func (s *EquipmentDisplayTestSuite) TestEquipmentView_VersatileFreeOffHand() {
	longsword := weapons.All[weapons.Longsword]

	char := &Character{
		id:            "char-3",
		bus:           s.bus,
		abilityScores: shared.AbilityScores{abilities.DEX: 10},
		inventory:     []InventoryItem{{Equipment: &longsword, Quantity: 1}},
		equipmentSlots: EquipmentSlots{
			SlotMainHand: "longsword",
		},
	}

	view := char.EquipmentView(s.ctx)
	s.Assert().Equal("1d10 slashing", view.MainHandDamage)
}

// TestEquipmentView_DualWield proves dual-wielding folds the off-hand
// weapon's die into the display, e.g. "1d4 piercing · off-hand 1d4".
func (s *EquipmentDisplayTestSuite) TestEquipmentView_DualWield() {
	// Two distinct inventory entries of the same weapon type need distinct
	// IDs (EquipmentID() would otherwise collide and both slots would
	// resolve to the same inventory item). weapons.All returns copies, so
	// mutating the ID on each copy is safe — the registry is untouched.
	daggerMain := weapons.All[weapons.Dagger]
	daggerMain.ID = "dagger-main"
	daggerOff := weapons.All[weapons.Dagger]
	daggerOff.ID = "dagger-off"

	char := &Character{
		id:            "char-4",
		bus:           s.bus,
		abilityScores: shared.AbilityScores{abilities.DEX: 10},
		inventory: []InventoryItem{
			{Equipment: &daggerMain, Quantity: 1},
			{Equipment: &daggerOff, Quantity: 1},
		},
		equipmentSlots: EquipmentSlots{
			SlotMainHand: "dagger-main",
			SlotOffHand:  "dagger-off",
		},
	}

	view := char.EquipmentView(s.ctx)
	s.Assert().Equal("1d4 piercing · off-hand 1d4", view.MainHandDamage)
}

func (s *EquipmentDisplayTestSuite) TestMainHandDamage() {
	longsword := weapons.All[weapons.Longsword]
	greatsword := weapons.All[weapons.Greatsword]
	dagger := weapons.All[weapons.Dagger]
	shield := armor.All[armor.Shield]

	testCases := []struct {
		name       string
		mainWeapon *weapons.Weapon
		offHand    *EquippedItem
		expected   string
	}{
		{"no main-hand weapon", nil, nil, ""},
		{"versatile, off hand free", &longsword, nil, "1d10 slashing"},
		{"versatile, shield in off hand", &longsword, &EquippedItem{Item: &shield}, "1d8 slashing"},
		{"two-handed weapon, off hand free", &greatsword, nil, "2d6 slashing"},
		{"dual-wield, non-versatile", &dagger, &EquippedItem{Item: &dagger}, "1d4 piercing · off-hand 1d4"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Assert().Equal(tc.expected, MainHandDamage(tc.mainWeapon, tc.offHand))
		})
	}
}
