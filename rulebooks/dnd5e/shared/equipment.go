// Package shared provides common types and interfaces for D&D 5e
package shared

// EquipmentID is the base type for all equipment identifiers
type EquipmentID = SelectionID

// EquipmentCategory is the base type for equipment category classifications
type EquipmentCategory string

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
