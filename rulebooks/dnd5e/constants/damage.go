package constants

// DamageType represents types of damage in D&D 5e
type DamageType string

// Damage type constants
const (
	DamageAcid        DamageType = "acid"
	DamageBludgeoning DamageType = "bludgeoning"
	DamageCold        DamageType = "cold"
	DamageFire        DamageType = "fire"
	DamageForce       DamageType = "force"
	DamageLightning   DamageType = "lightning"
	DamageNecrotic    DamageType = "necrotic"
	DamagePiercing    DamageType = "piercing"
	DamagePoison      DamageType = "poison"
	DamagePsychic     DamageType = "psychic"
	DamageRadiant     DamageType = "radiant"
	DamageSlashing    DamageType = "slashing"
	DamageThunder     DamageType = "thunder"
)

// Display returns the human-readable name of the damage type
func (d DamageType) Display() string {
	switch d {
	case DamageAcid:
		return "Acid"
	case DamageBludgeoning:
		return "Bludgeoning"
	case DamageCold:
		return "Cold"
	case DamageFire:
		return "Fire"
	case DamageForce:
		return "Force"
	case DamageLightning:
		return "Lightning"
	case DamageNecrotic:
		return "Necrotic"
	case DamagePiercing:
		return "Piercing"
	case DamagePoison:
		return "Poison"
	case DamagePsychic:
		return "Psychic"
	case DamageRadiant:
		return "Radiant"
	case DamageSlashing:
		return "Slashing"
	case DamageThunder:
		return "Thunder"
	default:
		return string(d)
	}
}

// IsPhysical returns true if this is physical damage (bludgeoning, piercing, slashing)
func (d DamageType) IsPhysical() bool {
	switch d {
	case DamageBludgeoning, DamagePiercing, DamageSlashing:
		return true
	default:
		return false
	}
}

// WeaponProperty represents weapon properties in D&D 5e
type WeaponProperty string

// Weapon property constants
const (
	WeaponAmmunition WeaponProperty = "ammunition"
	WeaponFinesse    WeaponProperty = "finesse"
	WeaponHeavy      WeaponProperty = "heavy"
	WeaponLight      WeaponProperty = "light"
	WeaponLoading    WeaponProperty = "loading"
	WeaponRange      WeaponProperty = "range"
	WeaponReach      WeaponProperty = "reach"
	WeaponSpecial    WeaponProperty = "special"
	WeaponThrown     WeaponProperty = "thrown"
	WeaponTwoHanded  WeaponProperty = "two-handed"
	WeaponVersatile  WeaponProperty = "versatile"
)

// Display returns the human-readable name of the weapon property
func (w WeaponProperty) Display() string {
	switch w {
	case WeaponAmmunition:
		return "Ammunition"
	case WeaponFinesse:
		return "Finesse"
	case WeaponHeavy:
		return "Heavy"
	case WeaponLight:
		return "Light"
	case WeaponLoading:
		return "Loading"
	case WeaponRange:
		return "Range"
	case WeaponReach:
		return "Reach"
	case WeaponSpecial:
		return "Special"
	case WeaponThrown:
		return "Thrown"
	case WeaponTwoHanded:
		return "Two-Handed"
	case WeaponVersatile:
		return "Versatile"
	default:
		return string(w)
	}
}
