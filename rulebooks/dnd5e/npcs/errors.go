package npcs

import "errors"

var (
	// ErrNoVendorData reports an attempt to load a missing vendor payload.
	ErrNoVendorData = errors.New("dnd5e npcs: vendor data is required")

	// ErrNoVendorNPC reports a vendor payload without generic NPC data.
	ErrNoVendorNPC = errors.New("dnd5e npcs: vendor npc data is required")

	// ErrNoInventory reports a vendor payload without inventory data.
	ErrNoInventory = errors.New("dnd5e npcs: vendor inventory is required")

	// ErrNoStock reports vendor inventory with no display stock.
	ErrNoStock = errors.New("dnd5e npcs: vendor inventory needs at least one stock entry")

	// ErrNoEquipmentType reports stock without a D&D equipment type.
	ErrNoEquipmentType = errors.New("dnd5e npcs: stock equipment type is required")

	// ErrNoEquipmentID reports stock without a D&D equipment id.
	ErrNoEquipmentID = errors.New("dnd5e npcs: stock equipment id is required")

	// ErrEquipmentNotFound reports stock that cannot be resolved through the D&D equipment registries.
	ErrEquipmentNotFound = errors.New("dnd5e npcs: stock equipment not found")

	// ErrEquipmentTypeMismatch reports stock whose stored type disagrees with the resolved equipment.
	ErrEquipmentTypeMismatch = errors.New("dnd5e npcs: stock equipment type mismatch")

	// ErrNoStockMode reports stock availability without a mode.
	ErrNoStockMode = errors.New("dnd5e npcs: stock mode is required")

	// ErrUnknownStockMode reports stock availability with an unsupported mode.
	ErrUnknownStockMode = errors.New("dnd5e npcs: stock mode is unknown")

	// ErrInvalidStockQuantity reports limited stock without a positive quantity.
	ErrInvalidStockQuantity = errors.New("dnd5e npcs: limited stock quantity must be positive")

	// ErrNoWorldVendor reports a world declaration config without a vendor.
	ErrNoWorldVendor = errors.New("dnd5e npcs: world vendor is required")

	// ErrNoWorldEntity reports a world declaration config without an entity id.
	ErrNoWorldEntity = errors.New("dnd5e npcs: world entity id is required")

	// ErrNoWorldMembership reports a world declaration config without a membership relation.
	ErrNoWorldMembership = errors.New("dnd5e npcs: world membership relation is required")

	// ErrOutOfStock reports a DecrementVendorStock request this vendor cannot
	// fulfill — either the item is not a stocked row at all, or a
	// StockModeLimited row does not hold as much as requested. One sentinel
	// covers both: from a caller's perspective "never carried it" and "no
	// longer has enough of it" are the same refusal.
	ErrOutOfStock = errors.New("dnd5e npcs: insufficient vendor stock")
)
