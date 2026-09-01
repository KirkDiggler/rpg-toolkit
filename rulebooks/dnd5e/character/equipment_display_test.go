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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
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
		{"versatile weapon", weapons.Longsword, "1d8 slashing damage · versatile"},
		{"two-handed weapon", weapons.Greatsword, "2d6 slashing damage · heavy, two-handed"},
		{"thrown weapon with range", weapons.Handaxe, "1d6 slashing damage · light, thrown 20/60"},
		{"finesse light thrown weapon", weapons.Dagger, "1d4 piercing damage · finesse, light, thrown 20/60"},
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

func (s *EquipmentDisplayTestSuite) TestWeaponStatLineIncludesEveryDamagePool() {
	detail := &equipment.EquipmentDetail{Weapon: &equipment.WeaponDetail{
		Damage: []damage.Damage{
			{Dice: "1d8", Type: damage.Slashing},
			{Dice: "1d6", Type: damage.Fire},
		},
	}}

	s.Assert().Equal("1d8 slashing damage, 1d6 fire damage", StatLine(detail))
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
// gear projects into quantity-bearing items with composed stat_line and an
// authoritative slot map, plus AC total + note derived from EffectiveAC — the
// shape rpg-api consumes with zero rules knowledge.
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

	view, viewErr := char.EquipmentView(s.ctx)

	s.Require().NoError(viewErr)
	s.Require().NotNil(view)
	s.Assert().Equal(18, view.ACTotal)
	s.Assert().Equal("16 chain mail + 2 shield", view.ACNote)
	// shield occupies off hand, so the versatile longsword is gripped
	// one-handed: its base die, not the versatile two-handed upgrade.
	s.Assert().Equal("1d8 slashing damage", view.MainHandDamage)
	s.Require().Len(view.Items, 3)
	s.Equal(EquipmentSlots{
		SlotArmor:    armor.ChainMail,
		SlotOffHand:  armor.Shield,
		SlotMainHand: weapons.Longsword,
	}, view.Equipped)

	byID := make(map[string]EquippedItemView, len(view.Items))
	for _, it := range view.Items {
		byID[it.ItemID] = it
	}

	s.Assert().Equal("AC 16 · heavy", byID[armor.ChainMail].StatLine)
	s.Assert().Equal("Chain Mail", byID[armor.ChainMail].Name)
	s.Assert().Equal("armor", byID[armor.ChainMail].Kind)
	s.Assert().Equal([]string{"armor"}, byID[armor.ChainMail].SlotKeys)
	s.Assert().Equal(1, byID[armor.ChainMail].Quantity)

	s.Assert().Equal("+2 AC", byID[armor.Shield].StatLine)
	s.Assert().Equal("Shield", byID[armor.Shield].Name)
	s.Assert().Equal("shield", byID[armor.Shield].Kind)
	s.Assert().Equal([]string{"off_hand"}, byID[armor.Shield].SlotKeys)
	s.Assert().Equal(1, byID[armor.Shield].Quantity)

	s.Assert().Equal("1d8 slashing damage · versatile", byID["longsword"].StatLine)
	s.Assert().Equal("Longsword", byID["longsword"].Name)
	s.Assert().Equal("weapon", byID["longsword"].Kind)
	s.Assert().Equal([]string{"main_hand", "off_hand"}, byID["longsword"].SlotKeys)
	s.Assert().Equal(1, byID["longsword"].Quantity)

	s.Require().Len(view.Slots, 3)
	s.Assert().Equal([]SlotDefView{
		{Key: "main_hand", DisplayLabel: "Main hand", Accepts: []string{"weapon"}},
		{Key: "off_hand", DisplayLabel: "Off hand", Accepts: []string{"weapon", "shield"}},
		{Key: "armor", DisplayLabel: "Armor", Accepts: []string{"armor"}},
	}, view.Slots)
}

func (s *EquipmentDisplayTestSuite) TestEquipmentView_ProjectsStackQuantityAndEveryOccupiedSlot() {
	handaxe := weapons.All[weapons.Handaxe]
	char := &Character{
		id:            "char-stack",
		bus:           s.bus,
		abilityScores: shared.AbilityScores{abilities.DEX: 10},
		inventory:     []InventoryItem{{Equipment: &handaxe, Quantity: 2}},
		equipmentSlots: EquipmentSlots{
			SlotMainHand: weapons.Handaxe,
			SlotOffHand:  weapons.Handaxe,
		},
	}

	view, err := char.EquipmentView(s.ctx)

	s.Require().NoError(err)
	s.Require().Len(view.Items, 1)
	s.Equal(2, view.Items[0].Quantity)
	s.Equal(EquipmentSlots{
		SlotMainHand: weapons.Handaxe,
		SlotOffHand:  weapons.Handaxe,
	}, view.Equipped)
}

// Persisted sheets predating canonical stacks can carry duplicate rows. The
// display boundary normalizes those rows so every item ID has one quantity.
func (s *EquipmentDisplayTestSuite) TestEquipmentView_AggregatesLegacyDuplicateRows() {
	handaxe := weapons.All[weapons.Handaxe]
	char := &Character{
		id:            "char-legacy-stack",
		bus:           s.bus,
		abilityScores: shared.AbilityScores{abilities.DEX: 10},
		inventory: []InventoryItem{
			{Equipment: &handaxe, Quantity: 1},
			{Equipment: &handaxe, Quantity: 2},
		},
		equipmentSlots: EquipmentSlots{SlotMainHand: weapons.Handaxe},
	}

	view, err := char.EquipmentView(s.ctx)

	s.Require().NoError(err)
	s.Require().Len(view.Items, 1)
	s.Equal(string(weapons.Handaxe), view.Items[0].ItemID)
	s.Equal(3, view.Items[0].Quantity)
}

// TestEquipmentView_EquippedIsDefensiveCopy proves the authoritative slot map
// is projected by value rather than exposing mutable character state.
func (s *EquipmentDisplayTestSuite) TestEquipmentView_EquippedIsDefensiveCopy() {
	longsword := weapons.All[weapons.Longsword]
	char := &Character{
		id:             "char-equipped-copy",
		bus:            s.bus,
		abilityScores:  shared.AbilityScores{abilities.DEX: 10},
		inventory:      []InventoryItem{{Equipment: &longsword, Quantity: 1}},
		equipmentSlots: EquipmentSlots{SlotMainHand: weapons.Longsword},
	}

	view, err := char.EquipmentView(s.ctx)
	s.Require().NoError(err)

	view.Equipped[SlotMainHand] = weapons.Handaxe
	view.Equipped[SlotOffHand] = weapons.Handaxe

	s.Equal(EquipmentSlots{SlotMainHand: weapons.Longsword}, char.equipmentSlots)
}

// TestEquipmentView_SlotsIsDefensiveCopy proves Slots (and each
// SlotDefView.Accepts) is a copy, not a reference into the shared
// package-level taxonomy — mutating one view's Slots must not leak into
// a later EquipmentView call (Copilot finding on #812: returning the
// shared slice/nested slices directly risked exactly this).
func (s *EquipmentDisplayTestSuite) TestEquipmentView_SlotsIsDefensiveCopy() {
	char := &Character{
		id:             "char-6",
		bus:            s.bus,
		abilityScores:  shared.AbilityScores{abilities.DEX: 10},
		inventory:      []InventoryItem{},
		equipmentSlots: make(EquipmentSlots),
	}

	first, viewErr := char.EquipmentView(s.ctx)

	s.Require().NoError(viewErr)
	first.Slots[0].Accepts[0] = "MUTATED"
	first.Slots = append(first.Slots, SlotDefView{Key: "mutated_slot"})

	second, viewErr := char.EquipmentView(s.ctx)

	s.Require().NoError(viewErr)
	s.Require().Len(second.Slots, 3)
	s.Assert().Equal("weapon", second.Slots[0].Accepts[0])
	s.Assert().Equal([]SlotDefView{
		{Key: "main_hand", DisplayLabel: "Main hand", Accepts: []string{"weapon"}},
		{Key: "off_hand", DisplayLabel: "Off hand", Accepts: []string{"weapon", "shield"}},
		{Key: "armor", DisplayLabel: "Armor", Accepts: []string{"armor"}},
	}, second.Slots)
}

// TestEquipmentView_CarriedItemIsAbsentFromEquipped proves inventory items
// that aren't equipped are still listed, while the authoritative slot map has
// no entry for them.
func (s *EquipmentDisplayTestSuite) TestEquipmentView_CarriedItemIsAbsentFromEquipped() {
	handaxe := weapons.All[weapons.Handaxe]

	char := &Character{
		id:             "char-2",
		bus:            s.bus,
		abilityScores:  shared.AbilityScores{abilities.DEX: 10},
		inventory:      []InventoryItem{{Equipment: &handaxe, Quantity: 1}},
		equipmentSlots: make(EquipmentSlots),
	}

	view, viewErr := char.EquipmentView(s.ctx)

	s.Require().NoError(viewErr)
	s.Require().Len(view.Items, 1)
	s.Assert().Empty(view.Equipped)
	s.Assert().Equal(1, view.Items[0].Quantity)
	s.Assert().Equal("1d6 slashing damage · light, thrown 20/60", view.Items[0].StatLine)
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

	view, viewErr := char.EquipmentView(s.ctx)

	s.Require().NoError(viewErr)
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

	view, viewErr := char.EquipmentView(s.ctx)

	s.Require().NoError(viewErr)
	s.Assert().Equal("1d10 slashing damage", view.MainHandDamage)
}

// TestEquipmentView_DualWield proves two copies from one stack can occupy
// both hands and fold the off-hand weapon's pool into the display, e.g.
// "1d4 piercing damage · off-hand 1d4 piercing damage".
func (s *EquipmentDisplayTestSuite) TestEquipmentView_DualWield() {
	dagger := weapons.All[weapons.Dagger]

	char := &Character{
		id:            "char-4",
		bus:           s.bus,
		abilityScores: shared.AbilityScores{abilities.DEX: 10},
		inventory:     []InventoryItem{{Equipment: &dagger, Quantity: 2}},
		equipmentSlots: EquipmentSlots{
			SlotMainHand: weapons.Dagger,
			SlotOffHand:  weapons.Dagger,
		},
	}

	view, viewErr := char.EquipmentView(s.ctx)

	s.Require().NoError(viewErr)
	s.Assert().Equal("1d4 piercing damage · off-hand 1d4 piercing damage", view.MainHandDamage)
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
		{"versatile, off hand free", &longsword, nil, "1d10 slashing damage"},
		{"versatile, shield in off hand", &longsword, &EquippedItem{Item: &shield}, "1d8 slashing damage"},
		{"two-handed weapon, off hand free", &greatsword, nil, "2d6 slashing damage"},
		{"dual-wield, non-versatile", &dagger, &EquippedItem{Item: &dagger}, "1d4 piercing damage · off-hand 1d4 piercing damage"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Assert().Equal(tc.expected, MainHandDamage(tc.mainWeapon, tc.offHand))
		})
	}
}
