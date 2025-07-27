package validation

import (
	"github.com/KirkDiggler/rpg-toolkit/items"
)

// Character represents the minimal interface needed for equipment validation
type Character interface {
	// GetID returns the character's unique identifier
	GetID() string

	// GetStrength returns the character's strength score
	GetStrength() int

	// GetProficiencies returns a list of the character's proficiencies
	GetProficiencies() []string

	// GetEquippedItems returns currently equipped items by slot
	GetEquippedItems() map[string]items.Item

	// GetAttunedItems returns currently attuned items
	GetAttunedItems() []items.Item

	// GetAttunementLimit returns the maximum number of attuned items allowed
	GetAttunementLimit() int

	// GetClass returns the character's class for restriction checking
	GetClass() string

	// GetRace returns the character's race for restriction checking
	GetRace() string

	// GetAlignment returns the character's alignment for restriction checking
	GetAlignment() string
}

// EquipmentValidator validates equipment actions
type EquipmentValidator interface {
	// CanEquip checks if a character can equip an item to a specific slot
	CanEquip(character Character, item items.EquippableItem, slot string) error

	// CanUnequip checks if a character can unequip an item from a specific slot
	CanUnequip(character Character, slot string) error

	// CanAttune checks if a character can attune to an item
	CanAttune(character Character, item items.EquippableItem) error

	// CanUseWeapon checks if a character can effectively use a weapon
	CanUseWeapon(character Character, weapon items.WeaponItem) error

	// CanWearArmor checks if a character can effectively wear armor
	CanWearArmor(character Character, armor items.ArmorItem) error

	// ValidateEquipmentSet checks if the entire equipment set is valid
	ValidateEquipmentSet(character Character) []error
}

// Context provides additional context for validation
type Context struct {
	// IgnoreProficiency allows equipping without proficiency (with penalties)
	IgnoreProficiency bool

	// IgnoreStrength allows equipping without strength requirement (with penalties)
	IgnoreStrength bool

	// ForceEquip bypasses most validation (admin/debug mode)
	ForceEquip bool
}

// EquipmentValidatorWithContext extends EquipmentValidator with context support
type EquipmentValidatorWithContext interface {
	EquipmentValidator

	// CanEquipWithContext checks if a character can equip an item with context
	CanEquipWithContext(character Character, item items.EquippableItem, slot string, ctx Context) error
}
