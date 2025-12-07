// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

type EffectiveACTestSuite struct {
	suite.Suite
	ctx      context.Context
	eventBus events.EventBus
}

func (s *EffectiveACTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.eventBus = events.NewEventBus()
}

// TestUnarmoredCharacter tests unarmored AC calculation (10 + DEX)
func (s *EffectiveACTestSuite) TestUnarmoredCharacter() {
	// Create character with DEX 14 (modifier +2)
	char := &Character{
		id:   "test-char",
		name: "Unarmored Test",
		abilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14, // +2 modifier
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		equipmentSlots: make(EquipmentSlots),
		inventory:      []InventoryItem{},
		bus:            s.eventBus,
	}

	// Calculate AC
	breakdown := char.EffectiveAC(s.ctx)

	// Verify total AC = 10 + 2 = 12
	s.Assert().Equal(12, breakdown.Total, "Unarmored AC should be 10 + DEX modifier")

	// Verify components
	s.Require().Len(breakdown.Components, 2, "Should have base and DEX components")
	s.Assert().Equal(combat.ACSourceBase, breakdown.Components[0].Type)
	s.Assert().Equal(10, breakdown.Components[0].Value)
	s.Assert().Equal(combat.ACSourceAbility, breakdown.Components[1].Type)
	s.Assert().Equal(2, breakdown.Components[1].Value)
}

// TestHeavyArmor tests heavy armor with no DEX bonus
func (s *EffectiveACTestSuite) TestHeavyArmor() {
	chainMail := armor.All[armor.ChainMail] // AC 16, MaxDexBonus = 0

	// Create character with DEX 14 (modifier +2, but should be ignored)
	char := &Character{
		id:   "test-char",
		name: "Heavy Armor Test",
		abilityScores: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 14, // +2 modifier, but capped at 0
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		equipmentSlots: make(EquipmentSlots),
		inventory: []InventoryItem{
			{
				Equipment: &chainMail,
				Quantity:  1,
			},
		},
		bus: s.eventBus,
	}

	// Equip chain mail
	char.equipmentSlots.Set(SlotArmor, armor.ChainMail)

	// Calculate AC
	breakdown := char.EffectiveAC(s.ctx)

	// Verify total AC = 16 (no DEX bonus for heavy armor)
	s.Assert().Equal(16, breakdown.Total, "Heavy armor AC should not include DEX")

	// Verify components - should only have armor component
	s.Require().Len(breakdown.Components, 1, "Heavy armor should only have armor component")
	s.Assert().Equal(combat.ACSourceArmor, breakdown.Components[0].Type)
	s.Assert().Equal(16, breakdown.Components[0].Value)
	s.Assert().NotNil(breakdown.Components[0].Source)
	s.Assert().Equal("dnd5e", breakdown.Components[0].Source.Module)
	s.Assert().Equal("armor", breakdown.Components[0].Source.Type)
	s.Assert().Equal(armor.ChainMail, breakdown.Components[0].Source.ID)
}

// TestMediumArmorDexCap tests medium armor with DEX cap
func (s *EffectiveACTestSuite) TestMediumArmorDexCap() {
	scaleMail := armor.All[armor.ScaleMail] // AC 14, MaxDexBonus = 2

	testCases := []struct {
		name           string
		dex            int
		expectedDexMod int
		expectedTotal  int
	}{
		{
			name:           "Low DEX (below cap)",
			dex:            12, // +1 modifier
			expectedDexMod: 1,
			expectedTotal:  15, // 14 + 1
		},
		{
			name:           "Exactly at cap",
			dex:            14, // +2 modifier
			expectedDexMod: 2,
			expectedTotal:  16, // 14 + 2
		},
		{
			name:           "High DEX (above cap)",
			dex:            18, // +4 modifier, but capped at +2
			expectedDexMod: 2,
			expectedTotal:  16, // 14 + 2 (capped)
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			char := &Character{
				id:   "test-char",
				name: "Medium Armor Test",
				abilityScores: shared.AbilityScores{
					abilities.STR: 10,
					abilities.DEX: tc.dex,
					abilities.CON: 10,
					abilities.INT: 10,
					abilities.WIS: 10,
					abilities.CHA: 10,
				},
				equipmentSlots: make(EquipmentSlots),
				inventory: []InventoryItem{
					{
						Equipment: &scaleMail,
						Quantity:  1,
					},
				},
				bus: s.eventBus,
			}

			// Equip scale mail
			char.equipmentSlots.Set(SlotArmor, armor.ScaleMail)

			// Calculate AC
			breakdown := char.EffectiveAC(s.ctx)

			// Verify total AC
			s.Assert().Equal(tc.expectedTotal, breakdown.Total,
				"Medium armor AC should cap DEX bonus at +2")

			// Verify components
			s.Require().Len(breakdown.Components, 2, "Should have armor and DEX components")
			s.Assert().Equal(combat.ACSourceArmor, breakdown.Components[0].Type)
			s.Assert().Equal(14, breakdown.Components[0].Value)
			s.Assert().Equal(combat.ACSourceAbility, breakdown.Components[1].Type)
			s.Assert().Equal(tc.expectedDexMod, breakdown.Components[1].Value)
		})
	}
}

// TestLightArmor tests light armor with unlimited DEX bonus
func (s *EffectiveACTestSuite) TestLightArmor() {
	leather := armor.All[armor.Leather] // AC 11, MaxDexBonus = nil (unlimited)

	// Create character with DEX 18 (modifier +4)
	char := &Character{
		id:   "test-char",
		name: "Light Armor Test",
		abilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 18, // +4 modifier
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		equipmentSlots: make(EquipmentSlots),
		inventory: []InventoryItem{
			{
				Equipment: &leather,
				Quantity:  1,
			},
		},
		bus: s.eventBus,
	}

	// Equip leather armor
	char.equipmentSlots.Set(SlotArmor, armor.Leather)

	// Calculate AC
	breakdown := char.EffectiveAC(s.ctx)

	// Verify total AC = 11 + 4 = 15
	s.Assert().Equal(15, breakdown.Total, "Light armor AC should include full DEX bonus")

	// Verify components
	s.Require().Len(breakdown.Components, 2, "Should have armor and DEX components")
	s.Assert().Equal(combat.ACSourceArmor, breakdown.Components[0].Type)
	s.Assert().Equal(11, breakdown.Components[0].Value)
	s.Assert().Equal(combat.ACSourceAbility, breakdown.Components[1].Type)
	s.Assert().Equal(4, breakdown.Components[1].Value)
}

// TestShieldBonus tests that shield adds +2 AC
func (s *EffectiveACTestSuite) TestShieldBonus() {
	leather := armor.All[armor.Leather] // AC 11
	shield := armor.All[armor.Shield]   // AC +2

	// Create character with DEX 14 (modifier +2)
	char := &Character{
		id:   "test-char",
		name: "Shield Test",
		abilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14, // +2 modifier
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		equipmentSlots: make(EquipmentSlots),
		inventory: []InventoryItem{
			{
				Equipment: &leather,
				Quantity:  1,
			},
			{
				Equipment: &shield,
				Quantity:  1,
			},
		},
		bus: s.eventBus,
	}

	// Equip leather armor and shield
	char.equipmentSlots.Set(SlotArmor, armor.Leather)
	char.equipmentSlots.Set(SlotOffHand, armor.Shield)

	// Calculate AC
	breakdown := char.EffectiveAC(s.ctx)

	// Verify total AC = 11 (armor) + 2 (DEX) + 2 (shield) = 15
	s.Assert().Equal(15, breakdown.Total, "Shield should add +2 to AC")

	// Verify components
	s.Require().Len(breakdown.Components, 3, "Should have armor, DEX, and shield components")
	s.Assert().Equal(combat.ACSourceArmor, breakdown.Components[0].Type)
	s.Assert().Equal(11, breakdown.Components[0].Value)
	s.Assert().Equal(combat.ACSourceAbility, breakdown.Components[1].Type)
	s.Assert().Equal(2, breakdown.Components[1].Value)
	s.Assert().Equal(combat.ACSourceShield, breakdown.Components[2].Type)
	s.Assert().Equal(2, breakdown.Components[2].Value)
	s.Assert().NotNil(breakdown.Components[2].Source)
	s.Assert().Equal("dnd5e", breakdown.Components[2].Source.Module)
	s.Assert().Equal("armor", breakdown.Components[2].Source.Type)
	s.Assert().Equal(armor.Shield, breakdown.Components[2].Source.ID)
}

// TestBreakdownTransparency tests that breakdown shows all sources
func (s *EffectiveACTestSuite) TestBreakdownTransparency() {
	chainMail := armor.All[armor.ChainMail] // AC 16, MaxDexBonus = 0
	shield := armor.All[armor.Shield]       // AC +2

	char := &Character{
		id:   "test-char",
		name: "Breakdown Test",
		abilityScores: shared.AbilityScores{
			abilities.STR: 15,
			abilities.DEX: 10, // +0 modifier
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		equipmentSlots: make(EquipmentSlots),
		inventory: []InventoryItem{
			{
				Equipment: &chainMail,
				Quantity:  1,
			},
			{
				Equipment: &shield,
				Quantity:  1,
			},
		},
		bus: s.eventBus,
	}

	// Equip chain mail and shield
	char.equipmentSlots.Set(SlotArmor, armor.ChainMail)
	char.equipmentSlots.Set(SlotOffHand, armor.Shield)

	// Calculate AC
	breakdown := char.EffectiveAC(s.ctx)

	// Verify total
	s.Assert().Equal(18, breakdown.Total, "AC should be 16 (armor) + 2 (shield)")

	// Verify each component has proper Type and Source
	s.Require().Len(breakdown.Components, 2, "Should have armor and shield components")

	// Armor component
	armorComp := breakdown.Components[0]
	s.Assert().Equal(combat.ACSourceArmor, armorComp.Type)
	s.Assert().Equal(16, armorComp.Value)
	s.Require().NotNil(armorComp.Source)
	s.Assert().Equal("dnd5e", armorComp.Source.Module)
	s.Assert().Equal("armor", armorComp.Source.Type)
	s.Assert().Equal(armor.ChainMail, armorComp.Source.ID)

	// Shield component
	shieldComp := breakdown.Components[1]
	s.Assert().Equal(combat.ACSourceShield, shieldComp.Type)
	s.Assert().Equal(2, shieldComp.Value)
	s.Require().NotNil(shieldComp.Source)
	s.Assert().Equal("dnd5e", shieldComp.Source.Module)
	s.Assert().Equal("armor", shieldComp.Source.Type)
	s.Assert().Equal(armor.Shield, shieldComp.Source.ID)
}

// TestNoEquipment tests character with no equipment
func (s *EffectiveACTestSuite) TestNoEquipment() {
	char := &Character{
		id:   "test-char",
		name: "No Equipment Test",
		abilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 10, // +0 modifier
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		equipmentSlots: make(EquipmentSlots),
		inventory:      []InventoryItem{},
		bus:            s.eventBus,
	}

	// Calculate AC
	breakdown := char.EffectiveAC(s.ctx)

	// Verify total AC = 10 (base only, no DEX bonus since modifier is 0)
	s.Assert().Equal(10, breakdown.Total, "Unarmored with no DEX bonus should be 10")

	// Verify components - only base component since DEX is 0
	s.Require().Len(breakdown.Components, 1, "Should only have base component")
	s.Assert().Equal(combat.ACSourceBase, breakdown.Components[0].Type)
	s.Assert().Equal(10, breakdown.Components[0].Value)
}

// TestNegativeDexModifier tests that negative DEX reduces AC
func (s *EffectiveACTestSuite) TestNegativeDexModifier() {
	leather := armor.All[armor.Leather] // AC 11, unlimited DEX

	char := &Character{
		id:   "test-char",
		name: "Negative DEX Test",
		abilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 8, // -1 modifier
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		equipmentSlots: make(EquipmentSlots),
		inventory: []InventoryItem{
			{
				Equipment: &leather,
				Quantity:  1,
			},
		},
		bus: s.eventBus,
	}

	// Equip leather armor
	char.equipmentSlots.Set(SlotArmor, armor.Leather)

	// Calculate AC
	breakdown := char.EffectiveAC(s.ctx)

	// Verify total AC = 11 + (-1) = 10
	s.Assert().Equal(10, breakdown.Total, "Negative DEX should reduce AC")

	// Verify components
	s.Require().Len(breakdown.Components, 2, "Should have armor and DEX components")
	s.Assert().Equal(combat.ACSourceArmor, breakdown.Components[0].Type)
	s.Assert().Equal(11, breakdown.Components[0].Value)
	s.Assert().Equal(combat.ACSourceAbility, breakdown.Components[1].Type)
	s.Assert().Equal(-1, breakdown.Components[1].Value)
}

// TestNoModifiers tests AC calculation with event bus but no modifying conditions/features
func (s *EffectiveACTestSuite) TestNoModifiers() {
	leather := armor.All[armor.Leather]

	char := &Character{
		id:   "test-char",
		name: "No Modifiers Test",
		abilityScores: shared.AbilityScores{
			abilities.STR: 10,
			abilities.DEX: 14, // +2 modifier
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		equipmentSlots: make(EquipmentSlots),
		inventory: []InventoryItem{
			{
				Equipment: &leather,
				Quantity:  1,
			},
		},
		bus: s.eventBus, // Event bus present but no modifiers registered
	}

	// Equip leather armor
	char.equipmentSlots.Set(SlotArmor, armor.Leather)

	// Calculate AC - should work with event bus but no modifiers
	breakdown := char.EffectiveAC(s.ctx)

	// Verify total AC = 11 + 2 = 13
	s.Assert().Equal(13, breakdown.Total, "AC calculation should work with no modifiers")

	// Verify components
	s.Require().Len(breakdown.Components, 2, "Should have armor and DEX components")
}

func TestEffectiveACTestSuite(t *testing.T) {
	suite.Run(t, new(EffectiveACTestSuite))
}
