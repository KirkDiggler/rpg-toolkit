// Package weapons provides D&D 5e weapon definitions and data
package weapons

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"

// WeaponCategory represents the category of weapon
type WeaponCategory string

const (
	// CategorySimpleMelee represents simple melee weapons
	CategorySimpleMelee WeaponCategory = "simple-melee"
	// CategorySimpleRanged represents simple ranged weapons
	CategorySimpleRanged WeaponCategory = "simple-ranged"
	// CategoryMartialMelee represents martial melee weapons
	CategoryMartialMelee WeaponCategory = "martial-melee"
	// CategoryMartialRanged represents martial ranged weapons
	CategoryMartialRanged WeaponCategory = "martial-ranged"
)

// WeaponProperty represents special properties of weapons
type WeaponProperty string

const (
	// PropertyLight indicates weapon is small and easy to handle
	PropertyLight WeaponProperty = "light"
	// PropertyThrown indicates weapon can be thrown
	PropertyThrown WeaponProperty = "thrown"
	// PropertyFinesse allows using Dexterity for attack and damage rolls
	PropertyFinesse WeaponProperty = "finesse"
	// PropertyVersatile allows one or two-handed use with different damage
	PropertyVersatile WeaponProperty = "versatile"
	// PropertyTwoHanded requires two hands to use
	PropertyTwoHanded WeaponProperty = "two-handed"
	// PropertyAmmunition requires ammunition to make ranged attacks
	PropertyAmmunition WeaponProperty = "ammunition"
	// PropertyLoading limits attacks to one per action
	PropertyLoading WeaponProperty = "loading"
	// PropertyReach adds 5 feet to attack range
	PropertyReach WeaponProperty = "reach"
	// PropertyHeavy indicates weapon is heavy and cumbersome
	PropertyHeavy WeaponProperty = "heavy"
	// PropertySpecial indicates weapon has special rules
	PropertySpecial WeaponProperty = "special"
)

// Weapon represents a D&D 5e weapon
type Weapon struct {
	ID         WeaponID
	Name       string
	Category   WeaponCategory
	Cost       string      // "5 gp"
	Damage     string      // "1d8"
	DamageType damage.Type // "slashing"
	Weight     float64
	Properties []WeaponProperty
	Range      *Range // nil for melee-only weapons
}

// GetName returns the name of the weapon
func (w Weapon) GetName() string {
	return w.Name
}

// Range represents weapon range (for thrown/ranged weapons)
type Range struct {
	Normal int
	Long   int
}

// IsSimple returns true if this is a simple weapon
func (w Weapon) IsSimple() bool {
	return w.Category == CategorySimpleMelee || w.Category == CategorySimpleRanged
}

// IsMartial returns true if this is a martial weapon
func (w Weapon) IsMartial() bool {
	return w.Category == CategoryMartialMelee || w.Category == CategoryMartialRanged
}

// IsMelee returns true if this is a melee weapon
func (w Weapon) IsMelee() bool {
	return w.Category == CategorySimpleMelee || w.Category == CategoryMartialMelee
}

// IsRanged returns true if this is a ranged weapon
func (w Weapon) IsRanged() bool {
	return w.Category == CategorySimpleRanged || w.Category == CategoryMartialRanged
}

// HasProperty returns true if the weapon has the specified property
func (w Weapon) HasProperty(prop WeaponProperty) bool {
	for _, p := range w.Properties {
		if p == prop {
			return true
		}
	}
	return false
}
