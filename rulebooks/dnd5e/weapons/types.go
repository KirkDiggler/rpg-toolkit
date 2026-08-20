// Package weapons provides D&D 5e weapon definitions and data
package weapons

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// WeaponCategory represents the category of weapon
type WeaponCategory = shared.EquipmentCategory

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
	ID             WeaponID
	Name           string
	Category       WeaponCategory
	Cost           string // "5 gp"
	Damage         []damage.Damage
	Weight         float64
	Properties     []WeaponProperty
	Range          *Range          // nil for melee-only weapons
	AmmunitionType ammunition.Type // Type of ammunition this weapon uses
}

// EquipmentID returns the unique identifier for this weapon
func (w *Weapon) EquipmentID() shared.EquipmentID {
	return w.ID
}

// EquipmentType returns the equipment type (always TypeWeapon)
func (w *Weapon) EquipmentType() shared.EquipmentType {
	return shared.EquipmentTypeWeapon
}

// EquipmentName returns the name of the weapon
func (w *Weapon) EquipmentName() string {
	return w.Name
}

// EquipmentWeight returns the weight in pounds
func (w *Weapon) EquipmentWeight() float32 {
	return float32(w.Weight)
}

// EquipmentValue returns the value in copper pieces
func (w *Weapon) EquipmentValue() int {
	// TODO: Parse cost string (e.g., "5 gp") and convert to copper
	// For now, return a placeholder
	return 0
}

// EquipmentDescription returns a description of the weapon
func (w *Weapon) EquipmentDescription() string {
	// Build description from damage and properties
	damages := make([]string, len(w.Damage))
	for i, pool := range w.Damage {
		damages[i] = fmt.Sprintf("%s %s damage", pool.Dice, pool.Type)
	}
	desc := strings.Join(damages, ", ")
	if len(w.Properties) > 0 {
		desc += " ("
		for i, prop := range w.Properties {
			if i > 0 {
				desc += ", "
			}
			desc += string(prop)
		}
		desc += ")"
	}
	return desc
}

// PrimaryDamage returns the sole damage pool that receives the attack's
// ability modifier. A weapon without exactly one marked pool has no primary
// damage pool.
func (w Weapon) PrimaryDamage() (damage.Damage, bool) {
	index, ok := w.primaryDamageIndex()
	if !ok {
		return damage.Damage{}, false
	}
	return w.Damage[index], true
}

func (w Weapon) primaryDamageIndex() (int, bool) {
	index := -1
	for i, pool := range w.Damage {
		if !pool.HasProperty(damage.AddsAttackAbilityModifier) {
			continue
		}
		if index != -1 {
			return -1, false
		}
		index = i
	}
	return index, index != -1
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

// RequiresAmmunition returns true if this weapon needs ammunition to fire
func (w Weapon) RequiresAmmunition() bool {
	return w.HasProperty(PropertyAmmunition)
}

// GetAmmunitionType returns the type of ammunition this weapon uses
// Returns empty string if weapon doesn't require ammunition
func (w Weapon) GetAmmunitionType() ammunition.Type {
	if w.RequiresAmmunition() {
		return w.AmmunitionType
	}
	return ""
}
