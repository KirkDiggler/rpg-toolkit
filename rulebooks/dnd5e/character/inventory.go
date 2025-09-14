package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
)

// InventoryItem represents an item in a character's inventory with quantity
type InventoryItem struct {
	Equipment equipment.Equipment `json:"equipment"`
	Quantity  int                 `json:"quantity"`
}

// GetTotalWeight returns the total weight of this stack of items
func (i InventoryItem) GetTotalWeight() float32 {
	return i.Equipment.EquipmentWeight() * float32(i.Quantity)
}

// InventorySlot represents where an item can be equipped
type InventorySlot string

const (
	// SlotMainHand represents the main hand
	SlotMainHand InventorySlot = "main_hand"
	// SlotOffHand represents the off hand
	SlotOffHand InventorySlot = "off_hand"
	// SlotArmor represents the armor
	SlotArmor InventorySlot = "armor"
	// SlotHelm represents the helm
	SlotHelm InventorySlot = "helm"
	// SlotBoots represents the boots
	SlotBoots InventorySlot = "boots"
	// SlotCloak represents the cloak
	SlotCloak InventorySlot = "cloak"
	// SlotAmulet represents the amulet
	SlotAmulet InventorySlot = "amulet"
	// SlotRingLeft represents the left ring
	SlotRingLeft InventorySlot = "ring_left"
	// SlotRingRight represents the right ring
	SlotRingRight InventorySlot = "ring_right"
	// SlotBelt represents the belt
	SlotBelt InventorySlot = "belt"
)
