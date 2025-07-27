package validation_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/items"
	"github.com/KirkDiggler/rpg-toolkit/items/validation"
)

// Test implementations

// mockItem implements items.Item
type mockItem struct {
	id         string
	itemType   string
	weight     float64
	value      int
	properties []string
	stackable  bool
	maxStack   int
}

func (m *mockItem) GetID() string           { return m.id }
func (m *mockItem) GetType() string         { return m.itemType }
func (m *mockItem) GetWeight() float64      { return m.weight }
func (m *mockItem) GetValue() int           { return m.value }
func (m *mockItem) GetProperties() []string { return m.properties }
func (m *mockItem) IsStackable() bool       { return m.stackable }
func (m *mockItem) GetMaxStack() int        { return m.maxStack }

// mockEquippableItem implements items.EquippableItem
type mockEquippableItem struct {
	mockItem
	validSlots         []string
	requiredSlots      []string
	attunable          bool
	requiresAttunement bool
}

func (m *mockEquippableItem) GetValidSlots() []string    { return m.validSlots }
func (m *mockEquippableItem) GetRequiredSlots() []string { return m.requiredSlots }
func (m *mockEquippableItem) IsAttunable() bool          { return m.attunable }
func (m *mockEquippableItem) RequiresAttunement() bool   { return m.requiresAttunement }

// mockWeaponItem implements items.WeaponItem
type mockWeaponItem struct {
	mockEquippableItem
	damage      string
	damageType  string
	weaponRange int
	proficiency string
	twoHanded   bool
	versatile   bool
	finesse     bool
}

func (m *mockWeaponItem) GetDamage() string              { return m.damage }
func (m *mockWeaponItem) GetDamageType() string          { return m.damageType }
func (m *mockWeaponItem) GetRange() int                  { return m.weaponRange }
func (m *mockWeaponItem) GetRequiredProficiency() string { return m.proficiency }
func (m *mockWeaponItem) IsTwoHanded() bool              { return m.twoHanded }
func (m *mockWeaponItem) IsVersatile() bool              { return m.versatile }
func (m *mockWeaponItem) IsFinesse() bool                { return m.finesse }

// mockArmorItem implements items.ArmorItem
type mockArmorItem struct {
	mockEquippableItem
	armorClass    int
	maxDexBonus   int
	strengthReq   int
	proficiency   string
	stealthDisadv bool
}

func (m *mockArmorItem) GetArmorClass() int             { return m.armorClass }
func (m *mockArmorItem) GetMaxDexBonus() int            { return m.maxDexBonus }
func (m *mockArmorItem) GetStrengthRequirement() int    { return m.strengthReq }
func (m *mockArmorItem) GetRequiredProficiency() string { return m.proficiency }
func (m *mockArmorItem) GetStealthDisadvantage() bool   { return m.stealthDisadv }

// mockCharacter implements validation.Character
type mockCharacter struct {
	id              string
	strength        int
	proficiencies   []string
	equippedItems   map[string]items.Item
	attunedItems    []items.Item
	attunementLimit int
	class           string
	race            string
	alignment       string
}

func (m *mockCharacter) GetID() string                           { return m.id }
func (m *mockCharacter) GetStrength() int                        { return m.strength }
func (m *mockCharacter) GetProficiencies() []string              { return m.proficiencies }
func (m *mockCharacter) GetEquippedItems() map[string]items.Item { return m.equippedItems }
func (m *mockCharacter) GetAttunedItems() []items.Item           { return m.attunedItems }
func (m *mockCharacter) GetAttunementLimit() int                 { return m.attunementLimit }
func (m *mockCharacter) GetClass() string                        { return m.class }
func (m *mockCharacter) GetRace() string                         { return m.race }
func (m *mockCharacter) GetAlignment() string                    { return m.alignment }

// Test Suite

type BasicValidatorTestSuite struct {
	suite.Suite
	validator *validation.BasicValidator
}

func (s *BasicValidatorTestSuite) SetupTest() {
	s.validator = validation.NewBasicValidator(validation.BasicValidatorConfig{
		DefaultAttunementLimit: 3,
		ClassRestrictions: map[string][]string{
			"holy-avenger":         {"paladin"},
			"robe-of-the-archmagi": {"wizard", "sorcerer", "warlock"},
		},
		RaceRestrictions: map[string][]string{
			"boots-of-elvenkind": {"elf", "half-elf"},
			"dwarven-thrower":    {"dwarf"},
		},
		AlignmentRestrictions: map[string][]string{
			"holy-avenger": {"lawful good", "neutral good", "chaotic good"},
			"blackrazor":   {"lawful evil", "neutral evil", "chaotic evil"},
		},
	})
}

func TestBasicValidatorSuite(t *testing.T) {
	suite.Run(t, new(BasicValidatorTestSuite))
}

// Test Cases

func (s *BasicValidatorTestSuite) TestCanEquip_ValidWeapon() {
	character := &mockCharacter{
		id:            "char1",
		strength:      15,
		proficiencies: []string{"martial_weapons", "simple_weapons"},
		equippedItems: make(map[string]items.Item),
		class:         "fighter",
		race:          "human",
		alignment:     "lawful good",
	}

	sword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "longsword",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand", "off_hand"},
			requiredSlots: []string{"main_hand"},
		},
		proficiency: "martial_weapons",
	}

	err := s.validator.CanEquip(character, sword, "main_hand")
	s.Require().NoError(err)
}

func (s *BasicValidatorTestSuite) TestCanEquip_InvalidSlot() {
	character := &mockCharacter{
		id:            "char1",
		equippedItems: make(map[string]items.Item),
	}

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

	err := s.validator.CanEquip(character, sword, "head")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrIncompatibleSlot)
}

func (s *BasicValidatorTestSuite) TestCanEquip_SlotOccupied() {
	existingSword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{id: "existing-sword"},
		},
	}

	character := &mockCharacter{
		id:            "char1",
		proficiencies: []string{"martial_weapons"},
		equippedItems: map[string]items.Item{
			"main_hand": existingSword,
		},
	}

	newSword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "new-sword",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
		proficiency: "martial_weapons",
	}

	err := s.validator.CanEquip(character, newSword, "main_hand")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrSlotOccupied)
}

func (s *BasicValidatorTestSuite) TestCanEquip_MissingProficiency() {
	character := &mockCharacter{
		id:            "char1",
		proficiencies: []string{"simple_weapons"}, // No martial weapons
		equippedItems: make(map[string]items.Item),
	}

	sword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "longsword",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
		proficiency: "martial_weapons",
	}

	err := s.validator.CanEquip(character, sword, "main_hand")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrMissingProficiency)
}

func (s *BasicValidatorTestSuite) TestCanEquip_InsufficientStrength() {
	character := &mockCharacter{
		id:            "char1",
		strength:      12, // Too low
		proficiencies: []string{"heavy_armor"},
		equippedItems: make(map[string]items.Item),
	}

	armor := &mockArmorItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "plate-armor",
				itemType: "armor",
			},
			validSlots:    []string{"body"},
			requiredSlots: []string{"body"},
		},
		strengthReq: 15,
		proficiency: "heavy_armor",
	}

	err := s.validator.CanEquip(character, armor, "body")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrInsufficientStrength)
}

func (s *BasicValidatorTestSuite) TestCanEquip_ClassRestriction() {
	character := &mockCharacter{
		id:            "char1",
		class:         "fighter", // Not a paladin
		equippedItems: make(map[string]items.Item),
	}

	holyAvenger := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "holy-avenger",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
	}

	err := s.validator.CanEquip(character, holyAvenger, "main_hand")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrClassRestriction)
}

func (s *BasicValidatorTestSuite) TestCanEquip_RaceRestriction() {
	character := &mockCharacter{
		id:            "char1",
		race:          "human", // Not a dwarf
		equippedItems: make(map[string]items.Item),
	}

	dwarvenThrower := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "dwarven-thrower",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
	}

	err := s.validator.CanEquip(character, dwarvenThrower, "main_hand")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrRaceRestriction)
}

func (s *BasicValidatorTestSuite) TestCanEquip_AlignmentRestriction() {
	character := &mockCharacter{
		id:            "char1",
		class:         "paladin",     // Correct class
		alignment:     "lawful evil", // Evil alignment
		equippedItems: make(map[string]items.Item),
	}

	holyAvenger := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "holy-avenger",
				itemType: "weapon",
			},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
	}

	err := s.validator.CanEquip(character, holyAvenger, "main_hand")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrAlignmentRestriction)
}

func (s *BasicValidatorTestSuite) TestCanEquip_RequiresAttunement() {
	character := &mockCharacter{
		id:            "char1",
		equippedItems: make(map[string]items.Item),
		attunedItems: []items.Item{
			&mockEquippableItem{mockItem: mockItem{id: "ring1"}},
			&mockEquippableItem{mockItem: mockItem{id: "ring2"}},
			&mockEquippableItem{mockItem: mockItem{id: "ring3"}},
		},
		attunementLimit: 3,
	}

	magicSword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{
				id:       "flame-tongue",
				itemType: "weapon",
			},
			validSlots:         []string{"main_hand"},
			requiredSlots:      []string{"main_hand"},
			requiresAttunement: true,
			attunable:          true,
		},
	}

	err := s.validator.CanEquip(character, magicSword, "main_hand")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrAttunementLimit)
}

func (s *BasicValidatorTestSuite) TestValidateEquipmentSet_TwoHandedConflict() {
	twoHandedSword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem:      mockItem{id: "greatsword"},
			validSlots:    []string{"main_hand"},
			requiredSlots: []string{"main_hand"},
		},
		twoHanded: true,
	}

	shield := &mockEquippableItem{
		mockItem:      mockItem{id: "shield"},
		validSlots:    []string{"off_hand"},
		requiredSlots: []string{"off_hand"},
	}

	character := &mockCharacter{
		id: "char1",
		equippedItems: map[string]items.Item{
			"main_hand": twoHandedSword,
			"off_hand":  shield,
		},
	}

	errors := s.validator.ValidateEquipmentSet(character)
	s.Require().NotEmpty(errors)

	// Should have at least the two-handed conflict
	var hasTwoHandedConflict bool
	for _, err := range errors {
		equipErr, ok := err.(*core.EquipmentError)
		if ok && equipErr.Err == core.ErrTwoHandedConflict {
			hasTwoHandedConflict = true
			break
		}
	}
	s.Assert().True(hasTwoHandedConflict, "Should have two-handed conflict error")
}

func (s *BasicValidatorTestSuite) TestCanAttune_NotAttunable() {
	character := &mockCharacter{
		id: "char1",
	}

	regularSword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem:  mockItem{id: "longsword"},
			attunable: false,
		},
	}

	err := s.validator.CanAttune(character, regularSword)
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrRequiresAttunement)
}

func (s *BasicValidatorTestSuite) TestCanAttune_Valid() {
	character := &mockCharacter{
		id:              "char1",
		attunedItems:    []items.Item{}, // No items attuned
		attunementLimit: 3,
	}

	magicRing := &mockEquippableItem{
		mockItem:  mockItem{id: "ring-of-protection"},
		attunable: true,
	}

	err := s.validator.CanAttune(character, magicRing)
	s.Require().NoError(err)
}

func (s *BasicValidatorTestSuite) TestCanUnequip_EmptySlot() {
	character := &mockCharacter{
		id:            "char1",
		equippedItems: make(map[string]items.Item),
	}

	err := s.validator.CanUnequip(character, "main_hand")
	s.Require().Error(err)
	s.Assert().ErrorIs(err, core.ErrIncompatibleSlot)
}

func (s *BasicValidatorTestSuite) TestCanUnequip_Valid() {
	sword := &mockWeaponItem{
		mockEquippableItem: mockEquippableItem{
			mockItem: mockItem{id: "longsword"},
		},
	}

	character := &mockCharacter{
		id: "char1",
		equippedItems: map[string]items.Item{
			"main_hand": sword,
		},
	}

	err := s.validator.CanUnequip(character, "main_hand")
	s.Require().NoError(err)
}
