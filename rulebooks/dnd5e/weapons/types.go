// Package weapons provides D&D 5e weapon definitions and data
package weapons

// WeaponCategory represents the category of weapon
type WeaponCategory string

// WeaponCategory constants classify weapons by complexity and range
const (
	CategorySimpleMelee   WeaponCategory = "simple-melee"
	CategorySimpleRanged  WeaponCategory = "simple-ranged"
	CategoryMartialMelee  WeaponCategory = "martial-melee"
	CategoryMartialRanged WeaponCategory = "martial-ranged"
)

// WeaponProperty represents special properties of weapons
type WeaponProperty string

// WeaponProperty constants define special weapon characteristics
const (
	PropertyLight      WeaponProperty = "light"
	PropertyThrown     WeaponProperty = "thrown"
	PropertyFinesse    WeaponProperty = "finesse"
	PropertyVersatile  WeaponProperty = "versatile"
	PropertyTwoHanded  WeaponProperty = "two-handed"
	PropertyAmmunition WeaponProperty = "ammunition"
	PropertyLoading    WeaponProperty = "loading"
	PropertyReach      WeaponProperty = "reach"
	PropertyHeavy      WeaponProperty = "heavy"
	PropertySpecial    WeaponProperty = "special"
)

// Weapon represents a D&D 5e weapon
type Weapon struct {
	ID         string
	Name       string
	Category   WeaponCategory
	Cost       string // "5 gp"
	Damage     string // "1d8"
	DamageType string // "slashing"
	Weight     float64
	Properties []WeaponProperty
	Range      *Range // nil for melee-only weapons
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
