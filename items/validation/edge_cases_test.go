package validation_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/items"
	"github.com/KirkDiggler/rpg-toolkit/items/validation"
)

type EdgeCasesTestSuite struct {
	suite.Suite
	validator *validation.BasicValidator
}

func (s *EdgeCasesTestSuite) SetupTest() {
	s.validator = validation.NewBasicValidator(validation.BasicValidatorConfig{
		DefaultAttunementLimit: 3,
	})
}

func TestEdgeCasesSuite(t *testing.T) {
	suite.Run(t, new(EdgeCasesTestSuite))
}

// Test edge cases and complex scenarios

func (s *EdgeCasesTestSuite) TestTwoHandedWeapon_PreventsDualWield() {
	character := &mockCharacter{
		id:            "char1",
		proficiencies: []string{"martial_weapons"},
		equippedItems: make(map[string]items.Item),
	}

	// First, equip a two-handed weapon
	greatsword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "greatsword",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand", "off_hand"}, // Two-handed requires both slots
		},
		proficiency: "martial_weapons",
		twoHanded:   true,
	}

	err := s.validator.CanEquip(character, greatsword, "main_hand")
	s.Require().NoError(err)

	// Simulate the weapon being equipped
	character.equippedItems["main_hand"] = greatsword
	character.equippedItems["off_hand"] = greatsword // Two-handed uses both

	// Now try to equip something in off-hand
	dagger := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "dagger",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand", "off_hand"},
			requiredSlots: []string{"off_hand"},
		},
		proficiency: "simple_weapons",
	}

	err = s.validator.CanEquip(character, dagger, "off_hand")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrSlotOccupied)
}

func (s *EdgeCasesTestSuite) TestMultiSlotItem() {
	character := &mockCharacter{
		id:            "char1",
		equippedItems: make(map[string]items.Item),
	}

	// Create a special item that requires multiple slots (like a full suit)
	fullPlate := &mockArmorItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "full-plate-suit",
				itemType: "armor",
			},
			validSlots:    []string{"body"},
			requiredSlots: []string{"body", "legs", "arms"}, // Takes multiple slots
		},
	}

	// Put items in the slots it needs
	character.equippedItems["legs"] = &mockEquippableItem{
		mockItem: mockItem{id: "leather-pants"},
	}

	err := s.validator.CanEquip(character, fullPlate, "body")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrSlotOccupied)
}

func (s *EdgeCasesTestSuite) TestReequipSameItem() {
	sword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "longsword",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand", "off_hand"},
			requiredSlots: []string{"main_hand"},
		},
	}

	character := &mockCharacter{
		id:            "char1",
		proficiencies: []string{"martial_weapons"},
		equippedItems: map[string]items.Item{
			"main_hand": sword, // Same sword already equipped
		},
	}

	// Should be able to re-equip the same item
	err := s.validator.CanEquip(character, sword, "main_hand")
	s.Require().NoError(err)
}

func (s *EdgeCasesTestSuite) TestVersatileWeapon_OneHanded() {
	character := &mockCharacter{
		id:            "char1",
		proficiencies: []string{"martial_weapons"},
		equippedItems: make(map[string]items.Item),
	}

	// Versatile weapon can be used one or two-handed
	longsword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "longsword",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
		proficiency: "martial_weapons",
		versatile:   true,
		twoHanded:   false, // Not inherently two-handed
	}

	shield := &mockEquippableItem{
		mockItem: mockItem{
			id:       "shield",
			itemType: "armor",
		},
		validSlots:    []string{"off_hand"},
		requiredSlots: []string{"off_hand"},
	}

	// Should be able to use with shield
	err := s.validator.CanEquip(character, longsword, "main_hand")
	s.Require().NoError(err)

	character.equippedItems["main_hand"] = longsword

	err = s.validator.CanEquip(character, shield, "off_hand")
	s.Require().NoError(err)
}

func (s *EdgeCasesTestSuite) TestAttunementLimit_Zero() {
	character := &mockCharacter{
		id:              "char1",
		equippedItems:   make(map[string]items.Item),
		attunedItems:    []items.Item{},
		attunementLimit: 0, // Will use default
	}

	// Add 3 attuned items (default limit)
	for i := 0; i < 3; i++ {
		character.attunedItems = append(character.attunedItems,
			&mockEquippableItem{mockItem: mockItem{id: string(rune('a' + i))}})
	}

	magicRing := &mockEquippableItem{
		mockItem:           mockItem{id: "ring-of-protection"},
		attunable:          true,
		requiresAttunement: true,
	}

	err := s.validator.CanAttune(character, magicRing)
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrAttunementLimit)
}

func (s *EdgeCasesTestSuite) TestNoProficiencyRequired() {
	character := &mockCharacter{
		id:            "char1",
		proficiencies: []string{}, // No proficiencies
		equippedItems: make(map[string]items.Item),
	}

	// Weapon with no proficiency requirement
	club := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "club",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
		proficiency: "", // No proficiency required
	}

	err := s.validator.CanEquip(character, club, "main_hand")
	s.Require().NoError(err)
}

func (s *EdgeCasesTestSuite) TestArmorNoStrengthRequirement() {
	character := &mockCharacter{
		id:            "char1",
		strength:      8, // Very low strength
		proficiencies: []string{"light_armor"},
		equippedItems: make(map[string]items.Item),
	}

	// Light armor with no strength requirement
	leather := &mockArmorItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "leather-armor",
				itemType: "armor",
			},
			validSlots:    []string{"body"},
			requiredSlots: []string{"body"},
		},
		strengthReq: 0, // No requirement
		proficiency: "light_armor",
	}

	err := s.validator.CanEquip(character, leather, "body")
	s.Require().NoError(err)
}

func (s *EdgeCasesTestSuite) TestValidateEquipmentSet_MultipleErrors() {
	// Create invalid equipment setup with multiple issues
	twoHandedSword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem:      mockItem{id: "greatsword"},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
		twoHanded:   true,
		proficiency: "martial_weapons",
	}

	shield := &mockEquippableItem{
		mockItem:      mockItem{id: "shield"},
		validSlots:    []string{"off_hand"},
		requiredSlots: []string{"off_hand"},
	}

	heavyArmor := &mockArmorItem{
		mockEquippableItem: mockEquippableItem{
			mockItem:      mockItem{id: "plate-armor"},
			validSlots:    []string{"body"},
			requiredSlots: []string{"body"},
		},
		strengthReq: 15,
		proficiency: "heavy_armor",
	}

	character := &mockCharacter{
		id:            "char1",
		strength:      10,                         // Too low for heavy armor
		proficiencies: []string{"simple_weapons"}, // Missing martial and heavy armor
		equippedItems: map[string]items.Item{
			"main_hand": twoHandedSword,
			"off_hand":  shield,
			"body":      heavyArmor,
		},
	}

	errors := s.validator.ValidateEquipmentSet(character)
	s.Assert().GreaterOrEqual(len(errors), 3) // At least 3 errors expected

	// Check that we got the expected error types
	var hasTwoHandedConflict, hasProficiencyError, hasStrengthError bool
	for _, err := range errors {
		equipErr, ok := err.(*core.EquipmentError)
		if ok {
			if equipErr.Err == core.ErrTwoHandedConflict {
				hasTwoHandedConflict = true
			}
			if equipErr.Err == core.ErrMissingProficiency {
				hasProficiencyError = true
			}
			if equipErr.Err == core.ErrInsufficientStrength {
				hasStrengthError = true
			}
		}
	}

	s.Assert().True(hasTwoHandedConflict, "Expected two-handed conflict error")
	s.Assert().True(hasProficiencyError, "Expected proficiency error")
	s.Assert().True(hasStrengthError, "Expected strength error")
}

func (s *EdgeCasesTestSuite) TestEmptyEquipmentSet() {
	character := &mockCharacter{
		id:            "char1",
		equippedItems: make(map[string]items.Item), // No equipment
	}

	errors := s.validator.ValidateEquipmentSet(character)
	s.Assert().Empty(errors)
}

func (s *EdgeCasesTestSuite) TestNilEventBus() {
	// Create validator without event bus
	validator := validation.NewBasicValidator(validation.BasicValidatorConfig{})

	character := &mockCharacter{
		id:            "char1",
		proficiencies: []string{"simple_weapons"},
		equippedItems: make(map[string]items.Item),
	}

	dagger := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "dagger",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
		proficiency: "simple_weapons",
	}

	// Should work without event bus
	err := validator.CanEquip(character, dagger, "main_hand")
	s.Require().NoError(err)
}
