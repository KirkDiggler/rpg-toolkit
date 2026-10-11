// Package fightingstyles provides D&D 5e fighting style definitions
package fightingstyles

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// FightingStyle represents a combat style that provides specific benefits
type FightingStyle = shared.SelectionID

// Fighting style constants
const (
	// Unspecified means fighting style not yet chosen (invalid for classes that get one)
	Unspecified FightingStyle = ""

	// Archery grants +2 to attack rolls with ranged weapons
	Archery FightingStyle = "archery"

	// Defense grants +1 to AC while wearing armor
	Defense FightingStyle = "defense"

	// Dueling grants +2 damage with one-handed weapons when no other weapon
	Dueling FightingStyle = "dueling"

	// GreatWeaponFighting allows reroll 1s and 2s on damage with two-handed weapons
	GreatWeaponFighting FightingStyle = "great_weapon_fighting"

	// Protection allows imposing disadvantage on attacks against nearby allies
	Protection FightingStyle = "protection"

	// TwoWeaponFighting adds ability modifier to off-hand damage
	TwoWeaponFighting FightingStyle = "two_weapon_fighting"
)

// ConditionRef is the condition a fighting style grants. ok is false for
// Unspecified and for a style this package does not name.
//
// The style's display name and detail are its condition's display, read from
// the conditions registry; this package holds neither.
func ConditionRef(f FightingStyle) (ref *core.Ref, ok bool) {
	switch f {
	case Archery:
		return refs.Conditions.FightingStyleArchery(), true
	case Defense:
		return refs.Conditions.FightingStyleDefense(), true
	case Dueling:
		return refs.Conditions.FightingStyleDueling(), true
	case GreatWeaponFighting:
		return refs.Conditions.FightingStyleGreatWeaponFighting(), true
	case Protection:
		return refs.Conditions.FightingStyleProtection(), true
	case TwoWeaponFighting:
		return refs.Conditions.FightingStyleTwoWeaponFighting(), true
	default:
		return nil, false
	}
}

// All returns all available fighting styles
func All() []FightingStyle {
	return []FightingStyle{
		Archery,
		Defense,
		Dueling,
		GreatWeaponFighting,
		Protection,
		TwoWeaponFighting,
	}
}

// ValidStyles returns the fighting styles available to a specific class
// This could be expanded to take class as a parameter if needed
func ValidStyles() []FightingStyle {
	// For now, return all styles
	// Later could filter based on class (e.g., Rangers don't get Great Weapon Fighting)
	return All()
}

// IsImplemented returns true if the fighting style has been implemented
func IsImplemented(style FightingStyle) bool {
	switch style {
	case Archery, GreatWeaponFighting, Dueling, TwoWeaponFighting, Defense, Protection:
		return true
	default:
		return false
	}
}
