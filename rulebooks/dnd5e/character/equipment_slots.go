package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// EquipmentSlots tracks which inventory items are equipped in combat-relevant slots.
// Values are item IDs referencing items in the character's inventory.
// Uses a map with typed InventorySlot constants as keys.
type EquipmentSlots map[InventorySlot]string

// Get returns the item ID for a given slot, or empty string if nothing equipped.
func (e EquipmentSlots) Get(slot InventorySlot) string {
	if e == nil {
		return ""
	}
	return e[slot]
}

// Set sets the item ID for a given slot.
func (e EquipmentSlots) Set(slot InventorySlot, itemID string) {
	if e == nil {
		return
	}
	e[slot] = itemID
}

// Clear removes the item from a given slot.
func (e EquipmentSlots) Clear(slot InventorySlot) {
	if e == nil {
		return
	}
	delete(e, slot)
}

// EquippedItem wraps equipment with typed accessors.
// Uses composition - the Item field holds the actual equipment.
type EquippedItem struct {
	Item equipment.Equipment
}

// AsArmor returns the item as armor, or nil if not armor.
func (e *EquippedItem) AsArmor() *armor.Armor {
	if e == nil || e.Item == nil {
		return nil
	}
	a, _ := e.Item.(*armor.Armor)
	return a
}

// AsWeapon returns the item as weapon, or nil if not weapon.
func (e *EquippedItem) AsWeapon() *weapons.Weapon {
	if e == nil || e.Item == nil {
		return nil
	}
	w, _ := e.Item.(*weapons.Weapon)
	return w
}
