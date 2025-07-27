package items

import "github.com/KirkDiggler/rpg-toolkit/core"

// Item represents any game item that can be owned, equipped, or used.
// Purpose: Core interface that all items must implement
type Item interface {
	core.Entity

	// GetWeight returns the item's weight in the game's unit system
	GetWeight() float64

	// GetValue returns the item's monetary value
	GetValue() int

	// GetProperties returns a list of item properties
	GetProperties() []string

	// IsStackable returns true if multiple instances can stack
	IsStackable() bool

	// GetMaxStack returns the maximum stack size (0 for unlimited)
	GetMaxStack() int
}

// EquippableItem represents an item that can be equipped to a slot
type EquippableItem interface {
	Item

	// GetValidSlots returns slots where this item can be equipped
	GetValidSlots() []string

	// GetRequiredSlots returns all slots this item occupies when equipped
	GetRequiredSlots() []string

	// IsAttunable returns true if this item can be attuned
	IsAttunable() bool

	// RequiresAttunement returns true if attunement is required to gain benefits
	RequiresAttunement() bool
}

// WeaponItem represents an equippable weapon
type WeaponItem interface {
	EquippableItem

	// GetDamage returns the damage dice expression (e.g., "1d8")
	GetDamage() string

	// GetDamageType returns the type of damage dealt
	GetDamageType() string

	// GetRange returns the weapon's range (0 for melee)
	GetRange() int

	// GetRequiredProficiency returns the proficiency needed to use effectively
	GetRequiredProficiency() string

	// IsTwoHanded returns true if the weapon requires two hands
	IsTwoHanded() bool

	// IsVersatile returns true if the weapon can be used one or two-handed
	IsVersatile() bool

	// IsFinesse returns true if the weapon has the finesse property
	IsFinesse() bool
}

// ArmorItem represents equippable armor
type ArmorItem interface {
	EquippableItem

	// GetArmorClass returns the base armor class
	GetArmorClass() int

	// GetMaxDexBonus returns the maximum dexterity bonus allowed (-1 for no limit)
	GetMaxDexBonus() int

	// GetStrengthRequirement returns the minimum strength score needed
	GetStrengthRequirement() int

	// GetRequiredProficiency returns the proficiency needed to wear effectively
	GetRequiredProficiency() string

	// GetStealthDisadvantage returns true if the armor imposes stealth disadvantage
	GetStealthDisadvantage() bool
}

// ConsumableItem represents items that are used up
type ConsumableItem interface {
	Item

	// GetUses returns the number of uses
	GetUses() int

	// IsConsumable returns true if the item is destroyed when uses reach 0
	IsConsumable() bool
}
