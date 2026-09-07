// Package shared provides common types and interfaces for D&D 5e
package shared

// EquipmentID is the base type for all equipment identifiers
type EquipmentID = SelectionID

// EquipmentCategory is the base type for equipment category classifications.
//
// Category values live here, not in the packages that use them (equipment,
// tools, weapons, armor), so every package that tags an item with a
// category and every package that queries by category shares one
// vocabulary. Before this, equipment.CategoryMusicalInstruments
// ("musical-instruments") and tools' own internal CategoryMusical
// ("musical") were different strings bridged only by a switch statement in
// equipment.GetByCategory — defining the value once here removes that
// class of mismatch instead of adding another translation layer for it.
type EquipmentCategory string

const (
	// CategoryMusicalInstruments selects all musical instruments.
	CategoryMusicalInstruments EquipmentCategory = "musical-instruments"
	// CategoryDruidicFoci selects druidic focuses.
	CategoryDruidicFoci EquipmentCategory = "druidic-foci"
	// CategoryHolySymbols selects holy symbols.
	CategoryHolySymbols EquipmentCategory = "holy-symbols"
	// CategoryArcaneFoci selects arcane focuses.
	CategoryArcaneFoci EquipmentCategory = "arcane-foci"
	// CategoryArtisanTools selects all artisan's tool kits.
	CategoryArtisanTools EquipmentCategory = "artisan-tools"
)

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
