package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
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

// ToData converts the inventory item to its persistent form
func (i InventoryItem) ToData() InventoryItemData {
	return InventoryItemData{
		Type:     i.Equipment.EquipmentType(),
		ID:       i.Equipment.EquipmentID(),
		Quantity: i.Quantity,
	}
}

// AddInventoryItem adds one item to a character's stored inventory, merging
// its quantity into an existing stack of the same type and ID or appending a
// new stack otherwise. The runtime counterpart to Draft.compileInventory's
// draft-time equivalent — the only path that puts an item into a character's
// Data outside character creation.
//
// item.Quantity must be strictly positive: this is the one seam a caller
// could use to shrink or invert an existing stack (a negative quantity would
// subtract from it in the merge branch below), so it is rejected here rather
// than trusted — the same "reject, don't silently reinterpret" rule
// DecrementVendorStock applies on the vendor side of the same exchange.
func AddInventoryItem(data *Data, item InventoryItemData) error {
	if data == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "character data is required")
	}
	if item.ID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "item id is required")
	}
	if item.Quantity <= 0 {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument, "item quantity must be positive, got %d", item.Quantity)
	}

	equip, err := equipment.GetByID(shared.SelectionID(item.ID))
	if err != nil {
		return rpgerr.Newf(rpgerr.CodeNotFound, "item %q not found in equipment catalog", item.ID)
	}
	if equip.EquipmentType() != item.Type {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"item %q type %q does not match catalog type %q", item.ID, item.Type, equip.EquipmentType())
	}

	for i := range data.Inventory {
		if data.Inventory[i].Type == item.Type && data.Inventory[i].ID == item.ID {
			data.Inventory[i].Quantity += item.Quantity
			return nil
		}
	}
	data.Inventory = append(data.Inventory, InventoryItemData{
		Type: item.Type, ID: item.ID, Quantity: item.Quantity,
	})
	return nil
}

// RemoveInventoryItem removes quantity units of an item from a character's
// stored inventory, decrementing an existing stack or removing it entirely
// once exhausted — the inverse of AddInventoryItem, and Sell's underlying
// primitive (rpg-toolkit#1537).
//
// A stack decremented to exactly zero is REMOVED from Inventory rather than
// left at Quantity: 0 — a zero-or-negative persisted quantity is not a value
// this package's own loadInventory accepts (load.go's strict-mode rejection,
// lenient-mode drop-and-warn), the same invariant npcs.DecrementVendorStock
// learned to protect the hard way (rpg-toolkit#1508). Leaving one behind
// here would write data this same package's next load could not accept.
//
// item.Quantity must be strictly positive, the same "reject, don't silently
// reinterpret" rule AddInventoryItem applies — a negative quantity here
// would otherwise ADD to the stack instead of removing from it.
//
// Returns rpgerr.CodeNotFound if the character does not own at least
// quantity units of Type/ID — covering both "never had it" and "has fewer
// than requested" with the one meaning that matters to a seller: you don't
// have that to sell.
func RemoveInventoryItem(data *Data, item InventoryItemData) error {
	if data == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "character data is required")
	}
	if item.ID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "item id is required")
	}
	if item.Quantity <= 0 {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument, "item quantity must be positive, got %d", item.Quantity)
	}

	for i := range data.Inventory {
		if data.Inventory[i].Type != item.Type || data.Inventory[i].ID != item.ID {
			continue
		}
		if data.Inventory[i].Quantity < item.Quantity {
			return rpgerr.Newf(rpgerr.CodeNotFound,
				"insufficient quantity of %q owned: have %d, need %d",
				item.ID, data.Inventory[i].Quantity, item.Quantity)
		}

		remaining := data.Inventory[i].Quantity - item.Quantity
		if remaining == 0 {
			data.Inventory = append(data.Inventory[:i:i], data.Inventory[i+1:]...)
		} else {
			data.Inventory[i].Quantity = remaining
		}
		return nil
	}

	return rpgerr.Newf(rpgerr.CodeNotFound, "item %q not in inventory", item.ID)
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
