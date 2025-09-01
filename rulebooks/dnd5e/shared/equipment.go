// Package shared provides common types and interfaces for D&D 5e
package shared

// Equipment represents any item that can be owned, carried, or equipped
// This interface unifies weapons, armor, tools, packs, and mundane items
// for selection and inventory purposes
type Equipment interface {
	// GetID returns the unique identifier for this equipment
	GetID() string

	// GetType returns the category of equipment
	GetType() EquipmentType

	// GetName returns the display name
	GetName() string

	// GetWeight returns the weight in pounds
	GetWeight() float32
}

// EquipmentType categorizes different kinds of equipment
type EquipmentType string

const (
	// EquipmentTypeWeapon represents weapons (swords, bows, etc.)
	EquipmentTypeWeapon EquipmentType = "weapon"

	// EquipmentTypeArmor represents armor and shields
	EquipmentTypeArmor EquipmentType = "armor"

	// EquipmentTypeTool represents tools (thieves' tools, artisan's tools, etc.)
	EquipmentTypeTool EquipmentType = "tool"

	// EquipmentTypePack represents equipment packs (explorer's pack, priest's pack, etc.)
	EquipmentTypePack EquipmentType = "pack"

	// EquipmentTypeItem represents mundane items (rope, torch, bedroll, etc.)
	EquipmentTypeItem EquipmentType = "item"

	// EquipmentTypeAmmunition represents ammunition (arrows, bolts, etc.)
	EquipmentTypeAmmunition EquipmentType = "ammunition"
)
