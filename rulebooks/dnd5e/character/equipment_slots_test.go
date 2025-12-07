package character

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

type EquipmentSlotsTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *EquipmentSlotsTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestEquipmentSlotsSuite(t *testing.T) {
	suite.Run(t, new(EquipmentSlotsTestSuite))
}

// Test EquipmentSlots map

func (s *EquipmentSlotsTestSuite) TestEquipmentSlots_Get() {
	slots := EquipmentSlots{
		SlotMainHand: "longsword",
		SlotArmor:    "chain-mail",
	}

	s.Assert().Equal("longsword", slots.Get(SlotMainHand))
	s.Assert().Equal("chain-mail", slots.Get(SlotArmor))
	s.Assert().Equal("", slots.Get(SlotOffHand))
}

func (s *EquipmentSlotsTestSuite) TestEquipmentSlots_Set() {
	slots := make(EquipmentSlots)

	slots.Set(SlotMainHand, "longsword")
	slots.Set(SlotArmor, "chain-mail")

	s.Assert().Equal("longsword", slots[SlotMainHand])
	s.Assert().Equal("chain-mail", slots[SlotArmor])
}

func (s *EquipmentSlotsTestSuite) TestEquipmentSlots_Clear() {
	slots := EquipmentSlots{
		SlotMainHand: "longsword",
	}

	slots.Clear(SlotMainHand)

	_, exists := slots[SlotMainHand]
	s.Assert().False(exists)
}

func (s *EquipmentSlotsTestSuite) TestEquipmentSlots_NilSafe() {
	var slots EquipmentSlots

	// Should not panic
	s.Assert().Equal("", slots.Get(SlotMainHand))
}

// Test EquippedItem wrapper

func (s *EquipmentSlotsTestSuite) TestEquippedItem_AsArmor() {
	chainMail := armor.All[armor.ChainMail]

	equipped := &EquippedItem{Item: &chainMail}

	result := equipped.AsArmor()
	s.Require().NotNil(result)
	s.Assert().Equal(armor.ChainMail, result.ID)
	s.Assert().Equal(armor.CategoryHeavy, result.Category)
	s.Assert().Equal(16, result.AC)
}

func (s *EquipmentSlotsTestSuite) TestEquippedItem_AsArmor_NotArmor() {
	longsword := weapons.All["longsword"]

	equipped := &EquippedItem{Item: &longsword}

	result := equipped.AsArmor()
	s.Assert().Nil(result)
}

func (s *EquipmentSlotsTestSuite) TestEquippedItem_AsArmor_Nil() {
	var equipped *EquippedItem

	result := equipped.AsArmor()
	s.Assert().Nil(result)
}

func (s *EquipmentSlotsTestSuite) TestEquippedItem_AsWeapon() {
	longsword := weapons.All["longsword"]

	equipped := &EquippedItem{Item: &longsword}

	result := equipped.AsWeapon()
	s.Require().NotNil(result)
	s.Assert().Equal("longsword", result.ID)
}

func (s *EquipmentSlotsTestSuite) TestEquippedItem_AsWeapon_NotWeapon() {
	chainMail := armor.All[armor.ChainMail]

	equipped := &EquippedItem{Item: &chainMail}

	result := equipped.AsWeapon()
	s.Assert().Nil(result)
}

func (s *EquipmentSlotsTestSuite) TestEquippedItem_AsWeapon_Nil() {
	var equipped *EquippedItem

	result := equipped.AsWeapon()
	s.Assert().Nil(result)
}

// Test Character equipment methods

func (s *EquipmentSlotsTestSuite) TestCharacter_GetEquippedSlot_Armor() {
	chainMail := armor.All[armor.ChainMail]

	char := &Character{
		inventory: []InventoryItem{
			{Equipment: &chainMail, Quantity: 1},
		},
		equipmentSlots: EquipmentSlots{
			SlotArmor: armor.ChainMail,
		},
	}

	equipped := char.GetEquippedSlot(SlotArmor)
	s.Require().NotNil(equipped)

	armorItem := equipped.AsArmor()
	s.Require().NotNil(armorItem)
	s.Assert().Equal(armor.ChainMail, armorItem.ID)
	s.Assert().Equal(16, armorItem.AC)
}

func (s *EquipmentSlotsTestSuite) TestCharacter_GetEquippedSlot_Weapon() {
	longsword := weapons.All["longsword"]

	char := &Character{
		inventory: []InventoryItem{
			{Equipment: &longsword, Quantity: 1},
		},
		equipmentSlots: EquipmentSlots{
			SlotMainHand: "longsword",
		},
	}

	equipped := char.GetEquippedSlot(SlotMainHand)
	s.Require().NotNil(equipped)

	weaponItem := equipped.AsWeapon()
	s.Require().NotNil(weaponItem)
	s.Assert().Equal("longsword", weaponItem.ID)
}

func (s *EquipmentSlotsTestSuite) TestCharacter_GetEquippedSlot_Empty() {
	char := &Character{
		inventory:      []InventoryItem{},
		equipmentSlots: EquipmentSlots{},
	}

	equipped := char.GetEquippedSlot(SlotArmor)
	s.Assert().Nil(equipped)
}

func (s *EquipmentSlotsTestSuite) TestCharacter_GetEquippedSlot_ItemNotInInventory() {
	// Slot references an item that's not in inventory
	char := &Character{
		inventory: []InventoryItem{},
		equipmentSlots: EquipmentSlots{
			SlotArmor: armor.ChainMail,
		},
	}

	equipped := char.GetEquippedSlot(SlotArmor)
	s.Assert().Nil(equipped)
}

func (s *EquipmentSlotsTestSuite) TestCharacter_EquipItem() {
	chainMail := armor.All[armor.ChainMail]

	char := &Character{
		inventory: []InventoryItem{
			{Equipment: &chainMail, Quantity: 1},
		},
		equipmentSlots: make(EquipmentSlots),
	}

	err := char.EquipItem(SlotArmor, armor.ChainMail)
	s.Require().NoError(err)

	s.Assert().Equal(armor.ChainMail, char.equipmentSlots[SlotArmor])
}

func (s *EquipmentSlotsTestSuite) TestCharacter_EquipItem_NotInInventory() {
	char := &Character{
		inventory:      []InventoryItem{},
		equipmentSlots: make(EquipmentSlots),
	}

	err := char.EquipItem(SlotArmor, armor.ChainMail)
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "not found")
}

func (s *EquipmentSlotsTestSuite) TestCharacter_EquipItem_NilMap() {
	// Test that EquipItem initializes the map if nil
	chainMail := armor.All[armor.ChainMail]

	char := &Character{
		inventory: []InventoryItem{
			{Equipment: &chainMail, Quantity: 1},
		},
		equipmentSlots: nil,
	}

	err := char.EquipItem(SlotArmor, armor.ChainMail)
	s.Require().NoError(err)

	s.Assert().Equal(armor.ChainMail, char.equipmentSlots[SlotArmor])
}

func (s *EquipmentSlotsTestSuite) TestCharacter_UnequipItem() {
	char := &Character{
		equipmentSlots: EquipmentSlots{
			SlotArmor: armor.ChainMail,
		},
	}

	char.UnequipItem(SlotArmor)

	_, exists := char.equipmentSlots[SlotArmor]
	s.Assert().False(exists)
}

// Test persistence roundtrip

func (s *EquipmentSlotsTestSuite) TestEquipmentSlots_Persistence() {
	chainMail := armor.All[armor.ChainMail]
	longsword := weapons.All["longsword"]
	shieldItem := armor.All[armor.Shield]

	char := &Character{
		id:    "test-char",
		name:  "Test Character",
		level: 1,
		bus:   s.bus,
		inventory: []InventoryItem{
			{Equipment: &chainMail, Quantity: 1},
			{Equipment: &longsword, Quantity: 1},
			{Equipment: &shieldItem, Quantity: 1},
		},
		equipmentSlots: EquipmentSlots{
			SlotArmor:    armor.ChainMail,
			SlotMainHand: "longsword",
			SlotOffHand:  armor.Shield,
		},
	}

	// Convert to data
	data := char.ToData()

	s.Assert().Equal(armor.ChainMail, data.EquipmentSlots[SlotArmor])
	s.Assert().Equal("longsword", data.EquipmentSlots[SlotMainHand])
	s.Assert().Equal(armor.Shield, data.EquipmentSlots[SlotOffHand])

	// Load from data
	loaded, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Verify equipment slots restored
	equippedArmor := loaded.GetEquippedSlot(SlotArmor)
	s.Require().NotNil(equippedArmor)
	s.Assert().Equal(armor.ChainMail, equippedArmor.AsArmor().ID)

	equippedWeapon := loaded.GetEquippedSlot(SlotMainHand)
	s.Require().NotNil(equippedWeapon)
	s.Assert().Equal("longsword", equippedWeapon.AsWeapon().ID)

	equippedShield := loaded.GetEquippedSlot(SlotOffHand)
	s.Require().NotNil(equippedShield)
	s.Assert().Equal(armor.Shield, equippedShield.AsArmor().ID)
}

// Test armor properties accessible through EquippedItem

func (s *EquipmentSlotsTestSuite) TestEquippedItem_ArmorProperties() {
	// Test that we can access armor-specific properties like Category, MaxDexBonus
	chainMail := armor.All[armor.ChainMail]
	leather := armor.All[armor.Leather]
	shieldItem := armor.All[armor.Shield]

	s.Run("heavy armor has no dex bonus", func() {
		equipped := &EquippedItem{Item: &chainMail}
		armorItem := equipped.AsArmor()

		s.Assert().Equal(armor.CategoryHeavy, armorItem.Category)
		s.Require().NotNil(armorItem.MaxDexBonus)
		s.Assert().Equal(0, *armorItem.MaxDexBonus)
	})

	s.Run("light armor has unlimited dex bonus", func() {
		equipped := &EquippedItem{Item: &leather}
		armorItem := equipped.AsArmor()

		s.Assert().Equal(armor.CategoryLight, armorItem.Category)
		s.Assert().Nil(armorItem.MaxDexBonus) // nil means unlimited
	})

	s.Run("shield is shield category", func() {
		equipped := &EquippedItem{Item: &shieldItem}
		armorItem := equipped.AsArmor()

		s.Assert().Equal(armor.CategoryShield, armorItem.Category)
		s.Assert().Equal(2, armorItem.AC) // +2 AC
	})
}

// Test typed slot constants

func (s *EquipmentSlotsTestSuite) TestEquipmentSlots_TypedConstants() {
	// Verify that the slot constants are typed InventorySlot
	slots := make(EquipmentSlots)

	// These should compile and work because keys are typed
	slots[SlotMainHand] = "longsword"
	slots[SlotOffHand] = "shield"
	slots[SlotArmor] = "chain-mail"

	s.Assert().Equal("longsword", slots[SlotMainHand])
	s.Assert().Equal("shield", slots[SlotOffHand])
	s.Assert().Equal("chain-mail", slots[SlotArmor])
}
