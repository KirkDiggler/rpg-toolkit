// Package items provides infrastructure for defining and managing game items
// without imposing specific game rules or mechanics.
//
// Purpose:
// This package defines what items ARE (properties, types, categories) while
// remaining agnostic to game-specific rules about how they work or what
// they do mechanically.
//
// Scope:
//   - Item definitions and properties
//   - Item types and categories
//   - Equipment slots and compatibility
//   - Item requirements (strength, proficiency)
//   - Magical properties and attunement
//   - Item stacking and quantities
//   - Item quality and condition
//
// Non-Goals:
//   - Combat mechanics: How weapons deal damage is game-specific
//   - Economy: Item values and pricing are game-specific
//   - Crafting rules: How items are created is game-specific
//   - Class/race restrictions: Specific limitations are game rules
//   - Inventory management: Storage is handled by containers package
//   - Item effects: What items DO is handled by effects/features
//
// Integration:
// This package integrates with:
//   - core: Items implement Entity interface
//   - events: Publishes item-related events
//   - effects: Items may have associated effects
//   - proficiency: Items may require proficiencies
//
// Games define specific items and their mechanics, while this package
// provides the framework for representing them.
//
// Example:
//
//	// Define an item
//	sword := items.NewWeapon("longsword", items.WeaponConfig{
//	    Damage:      "1d8",
//	    DamageType:  "slashing",
//	    Properties:  []string{"versatile"},
//	    Weight:      3,
//	    Value:       15,
//	    Proficiency: "martial_weapons",
//	})
//
//	// Check requirements
//	validator := validation.NewEquipmentValidator()
//	if err := validator.CanEquip(character, sword, items.SlotMainHand); err != nil {
//	    // Handle validation error
//	}
//
//	// Magical items
//	ring := items.NewMagical("ring_of_protection", items.MagicalConfig{
//	    Rarity:           "rare",
//	    RequiresAttunement: true,
//	    Slots:            []string{"ring"},
//	})
package items
