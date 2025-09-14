// Package fightingstyles provides D&D 5e fighting style definitions
package fightingstyles

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"

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

// Name returns the display name of the fighting style
func Name(f FightingStyle) string {
	switch f {
	case Archery:
		return "Archery"
	case Defense:
		return "Defense"
	case Dueling:
		return "Dueling"
	case GreatWeaponFighting:
		return "Great Weapon Fighting"
	case Protection:
		return "Protection"
	case TwoWeaponFighting:
		return "Two-Weapon Fighting"
	default:
		return f
	}
}

// Description returns the mechanical description of the fighting style
func Description(f FightingStyle) string {
	switch f {
	case Archery:
		return "You gain a +2 bonus to attack rolls you make with ranged weapons."
	case Defense:
		return "While you are wearing armor, you gain a +1 bonus to AC."
	case Dueling:
		return "When you are wielding a melee weapon in one hand and no other weapons, you gain a +2 bonus to damage rolls with that weapon." //nolint:lll
	case GreatWeaponFighting:
		return "When you roll a 1 or 2 on a damage die for an attack you make with a melee weapon that you are wielding with two hands, you can reroll the die and must use the new roll. The weapon must have the two-handed or versatile property." //nolint:lll
	case Protection:
		return "When a creature you can see attacks a target other than you that is within 5 feet of you, you can use your reaction to impose disadvantage on the attack roll. You must be wielding a shield." //nolint:lll
	case TwoWeaponFighting:
		return "When you engage in two-weapon fighting, you can add your ability modifier to the damage of the second attack." //nolint:lll
	default:
		return ""
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
