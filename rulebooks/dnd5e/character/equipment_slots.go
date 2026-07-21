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

// CompatibleSlots returns every InventorySlot item may be equipped into —
// the single source of truth for slot compatibility, ported from the
// occupancy semantics in rpg-dnd5e-web#557
// src/concepts/equipment/fixtures.ts (applyIntent/targetSlotFor): weapons
// fit main or off hand, two-handed weapons fit main hand only, shields fit
// off hand, other armor fits the armor slot, and everything else (tools,
// packs, ammunition, misc gear) has no combat-relevant slot today.
// EquipItem validates against this, and EquipmentView projects it as each
// item's SlotKeys — rpg-api must not reconstruct this rule itself.
func CompatibleSlots(item equipment.Equipment) []InventorySlot {
	switch it := item.(type) {
	case *weapons.Weapon:
		if it.HasProperty(weapons.PropertyTwoHanded) {
			return []InventorySlot{SlotMainHand}
		}
		return []InventorySlot{SlotMainHand, SlotOffHand}
	case *armor.Armor:
		if it.Category == shieldCategory {
			return []InventorySlot{SlotOffHand}
		}
		return []InventorySlot{SlotArmor}
	default:
		return nil
	}
}

// equipmentFitsSlot reports whether item may be equipped into slot —
// membership in CompatibleSlots(item).
func equipmentFitsSlot(item equipment.Equipment, slot InventorySlot) bool {
	for _, s := range CompatibleSlots(item) {
		if s == slot {
			return true
		}
	}
	return false
}

// isTwoHanded reports whether item is a weapon with the two-handed property.
func isTwoHanded(item equipment.Equipment) bool {
	w, ok := item.(*weapons.Weapon)
	return ok && w.HasProperty(weapons.PropertyTwoHanded)
}

// itemKind classifies item into the display vocabulary EquippedItemView
// uses ("weapon" | "shield" | "armor" | "gear"), mirroring
// rpg-dnd5e-web#557 fixtures.ts's ItemFixture.kind.
func itemKind(item equipment.Equipment) string {
	switch it := item.(type) {
	case *weapons.Weapon:
		return "weapon"
	case *armor.Armor:
		if it.Category == shieldCategory {
			return "shield"
		}
		return "armor"
	default:
		return "gear"
	}
}
