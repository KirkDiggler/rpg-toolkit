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
	SlotMainHand  InventorySlot = "main_hand"
	SlotOffHand   InventorySlot = "off_hand"
	SlotArmor     InventorySlot = "armor"
	SlotHelm      InventorySlot = "helm"
	SlotGloves    InventorySlot = "gloves"
	SlotBoots     InventorySlot = "boots"
	SlotCloak     InventorySlot = "cloak"
	SlotAmulet    InventorySlot = "amulet"
	SlotRingLeft  InventorySlot = "ring_left"
	SlotRingRight InventorySlot = "ring_right"
	SlotBelt      InventorySlot = "belt"
)
