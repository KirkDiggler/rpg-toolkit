package weapons

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// Fighter weapons - just a subset for testing
// Note: Fighter gets all simple and martial weapons, but we'll add just a few for testing

// SimpleMeleeWeapons - fighter-accessible simple melee weapons (for testing)
var SimpleMeleeWeapons = map[WeaponID]Weapon{
	Club: {
		ID:         Club,
		Name:       "Club",
		Category:   CategorySimpleMelee,
		Cost:       "1 sp",
		Damage:     "1d4",
		DamageType: damage.Bludgeoning,
		Weight:     2,
		Properties: []WeaponProperty{PropertyLight},
	},
	Dagger: {
		ID:         Dagger,
		Name:       "Dagger",
		Category:   CategorySimpleMelee,
		Cost:       "2 gp",
		Damage:     "1d4",
		DamageType: damage.Piercing,
		Weight:     1,
		Properties: []WeaponProperty{PropertyFinesse, PropertyLight, PropertyThrown},
		Range:      &Range{Normal: 20, Long: 60},
	},
	Handaxe: {
		ID:         Handaxe,
		Name:       "Handaxe",
		Category:   CategorySimpleMelee,
		Cost:       "5 gp",
		Damage:     "1d6",
		DamageType: damage.Slashing,
		Weight:     2,
		Properties: []WeaponProperty{PropertyLight, PropertyThrown},
		Range:      &Range{Normal: 20, Long: 60},
	},
	Javelin: {
		ID:         Javelin,
		Name:       "Javelin",
		Category:   CategorySimpleMelee,
		Cost:       "5 sp",
		Damage:     "1d6",
		DamageType: damage.Piercing,
		Weight:     2,
		Properties: []WeaponProperty{PropertyThrown},
		Range:      &Range{Normal: 30, Long: 120},
	},
	Greatclub: {
		ID:         Greatclub,
		Name:       "Greatclub",
		Category:   CategorySimpleMelee,
		Cost:       "2 sp",
		Damage:     "1d8",
		DamageType: damage.Bludgeoning,
		Weight:     10,
		Properties: []WeaponProperty{PropertyTwoHanded},
	},
	LightHammer: {
		ID:         LightHammer,
		Name:       "Light Hammer",
		Category:   CategorySimpleMelee,
		Cost:       "2 gp",
		Damage:     "1d4",
		DamageType: damage.Bludgeoning,
		Weight:     2,
		Properties: []WeaponProperty{PropertyLight, PropertyThrown},
		Range:      &Range{Normal: 20, Long: 60},
	},
	Mace: {
		ID:         Mace,
		Name:       "Mace",
		Category:   CategorySimpleMelee,
		Cost:       "5 gp",
		Damage:     "1d6",
		DamageType: damage.Bludgeoning,
		Weight:     4,
		Properties: []WeaponProperty{},
	},
	Quarterstaff: {
		ID:         Quarterstaff,
		Name:       "Quarterstaff",
		Category:   CategorySimpleMelee,
		Cost:       "2 sp",
		Damage:     "1d6",
		DamageType: damage.Bludgeoning,
		Weight:     4,
		Properties: []WeaponProperty{PropertyVersatile},
	},
	Sickle: {
		ID:         Sickle,
		Name:       "Sickle",
		Category:   CategorySimpleMelee,
		Cost:       "1 gp",
		Damage:     "1d4",
		DamageType: damage.Slashing,
		Weight:     2,
		Properties: []WeaponProperty{PropertyLight},
	},
	Spear: {
		ID:         Spear,
		Name:       "Spear",
		Category:   CategorySimpleMelee,
		Cost:       "1 gp",
		Damage:     "1d6",
		DamageType: damage.Piercing,
		Weight:     3,
		Properties: []WeaponProperty{PropertyThrown, PropertyVersatile},
		Range:      &Range{Normal: 20, Long: 60},
	},
}

// MartialMeleeWeapons - fighter-accessible martial melee weapons (for testing)
var MartialMeleeWeapons = map[WeaponID]Weapon{
	Greatsword: {
		ID:         Greatsword,
		Name:       "Greatsword",
		Category:   CategoryMartialMelee,
		Cost:       "50 gp",
		Damage:     "2d6",
		DamageType: damage.Slashing,
		Weight:     6,
		Properties: []WeaponProperty{PropertyHeavy, PropertyTwoHanded},
	},
	Longsword: {
		ID:         Longsword,
		Name:       "Longsword",
		Category:   CategoryMartialMelee,
		Cost:       "15 gp",
		Damage:     "1d8",
		DamageType: damage.Slashing,
		Weight:     3,
		Properties: []WeaponProperty{PropertyVersatile},
	},
	Rapier: {
		ID:         Rapier,
		Name:       "Rapier",
		Category:   CategoryMartialMelee,
		Cost:       "25 gp",
		Damage:     "1d8",
		DamageType: damage.Piercing,
		Weight:     2,
		Properties: []WeaponProperty{PropertyFinesse},
	},
	Shortsword: {
		ID:         Shortsword,
		Name:       "Shortsword",
		Category:   CategoryMartialMelee,
		Cost:       "10 gp",
		Damage:     "1d6",
		DamageType: damage.Piercing,
		Weight:     2,
		Properties: []WeaponProperty{PropertyFinesse, PropertyLight},
	},
	Battleaxe: {
		ID:         Battleaxe,
		Name:       "Battleaxe",
		Category:   CategoryMartialMelee,
		Cost:       "10 gp",
		Damage:     "1d8",
		DamageType: damage.Slashing,
		Weight:     4,
		Properties: []WeaponProperty{PropertyVersatile},
	},
	Flail: {
		ID:         Flail,
		Name:       "Flail",
		Category:   CategoryMartialMelee,
		Cost:       "10 gp",
		Damage:     "1d8",
		DamageType: damage.Bludgeoning,
		Weight:     2,
		Properties: []WeaponProperty{},
	},
	Glaive: {
		ID:         Glaive,
		Name:       "Glaive",
		Category:   CategoryMartialMelee,
		Cost:       "20 gp",
		Damage:     "1d10",
		DamageType: damage.Slashing,
		Weight:     6,
		Properties: []WeaponProperty{PropertyHeavy, PropertyReach, PropertyTwoHanded},
	},
	Greataxe: {
		ID:         Greataxe,
		Name:       "Greataxe",
		Category:   CategoryMartialMelee,
		Cost:       "30 gp",
		Damage:     "1d12",
		DamageType: damage.Slashing,
		Weight:     7,
		Properties: []WeaponProperty{PropertyHeavy, PropertyTwoHanded},
	},
	Halberd: {
		ID:         Halberd,
		Name:       "Halberd",
		Category:   CategoryMartialMelee,
		Cost:       "20 gp",
		Damage:     "1d10",
		DamageType: damage.Slashing,
		Weight:     6,
		Properties: []WeaponProperty{PropertyHeavy, PropertyReach, PropertyTwoHanded},
	},
	Lance: {
		ID:         Lance,
		Name:       "Lance",
		Category:   CategoryMartialMelee,
		Cost:       "10 gp",
		Damage:     "1d12",
		DamageType: damage.Piercing,
		Weight:     6,
		Properties: []WeaponProperty{PropertyReach}, // Special: disadvantage when within 5 feet
	},
	Maul: {
		ID:         Maul,
		Name:       "Maul",
		Category:   CategoryMartialMelee,
		Cost:       "10 gp",
		Damage:     "2d6",
		DamageType: damage.Bludgeoning,
		Weight:     10,
		Properties: []WeaponProperty{PropertyHeavy, PropertyTwoHanded},
	},
	Morningstar: {
		ID:         Morningstar,
		Name:       "Morningstar",
		Category:   CategoryMartialMelee,
		Cost:       "15 gp",
		Damage:     "1d8",
		DamageType: damage.Piercing,
		Weight:     4,
		Properties: []WeaponProperty{},
	},
	Pike: {
		ID:         Pike,
		Name:       "Pike",
		Category:   CategoryMartialMelee,
		Cost:       "5 gp",
		Damage:     "1d10",
		DamageType: damage.Piercing,
		Weight:     18,
		Properties: []WeaponProperty{PropertyHeavy, PropertyReach, PropertyTwoHanded},
	},
	Scimitar: {
		ID:         Scimitar,
		Name:       "Scimitar",
		Category:   CategoryMartialMelee,
		Cost:       "25 gp",
		Damage:     "1d6",
		DamageType: damage.Slashing,
		Weight:     3,
		Properties: []WeaponProperty{PropertyFinesse, PropertyLight},
	},
	Trident: {
		ID:         Trident,
		Name:       "Trident",
		Category:   CategoryMartialMelee,
		Cost:       "5 gp",
		Damage:     "1d6",
		DamageType: damage.Piercing,
		Weight:     4,
		Properties: []WeaponProperty{PropertyThrown, PropertyVersatile},
		Range:      &Range{Normal: 20, Long: 60},
	},
	WarPick: {
		ID:         WarPick,
		Name:       "War Pick",
		Category:   CategoryMartialMelee,
		Cost:       "5 gp",
		Damage:     "1d8",
		DamageType: damage.Piercing,
		Weight:     2,
		Properties: []WeaponProperty{},
	},
	Warhammer: {
		ID:         Warhammer,
		Name:       "Warhammer",
		Category:   CategoryMartialMelee,
		Cost:       "15 gp",
		Damage:     "1d8",
		DamageType: damage.Bludgeoning,
		Weight:     2,
		Properties: []WeaponProperty{PropertyVersatile},
	},
	Whip: {
		ID:         Whip,
		Name:       "Whip",
		Category:   CategoryMartialMelee,
		Cost:       "2 gp",
		Damage:     "1d4",
		DamageType: damage.Slashing,
		Weight:     3,
		Properties: []WeaponProperty{PropertyFinesse, PropertyReach},
	},
}

// SimpleRangedWeapons - fighter-accessible simple ranged weapons (for testing)
var SimpleRangedWeapons = map[WeaponID]Weapon{
	LightCrossbow: {
		ID:         LightCrossbow,
		Name:       "Light Crossbow",
		Category:   CategorySimpleRanged,
		Cost:       "25 gp",
		Damage:     "1d8",
		DamageType: damage.Piercing,
		Weight:     5,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyLoading, PropertyTwoHanded},
		Range:      &Range{Normal: 80, Long: 320},
	},
	Shortbow: {
		ID:         Shortbow,
		Name:       "Shortbow",
		Category:   CategorySimpleRanged,
		Cost:       "25 gp",
		Damage:     "1d6",
		DamageType: damage.Piercing,
		Weight:     2,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyTwoHanded},
		Range:      &Range{Normal: 80, Long: 320},
	},
	Dart: {
		ID:         Dart,
		Name:       "Dart",
		Category:   CategorySimpleRanged,
		Cost:       "5 cp",
		Damage:     "1d4",
		DamageType: damage.Piercing,
		Weight:     0.25,
		Properties: []WeaponProperty{PropertyFinesse, PropertyThrown},
		Range:      &Range{Normal: 20, Long: 60},
	},
	Sling: {
		ID:         Sling,
		Name:       "Sling",
		Category:   CategorySimpleRanged,
		Cost:       "1 sp",
		Damage:     "1d4",
		DamageType: damage.Bludgeoning,
		Weight:     0,
		Properties: []WeaponProperty{PropertyAmmunition},
		Range:      &Range{Normal: 30, Long: 120},
	},
}

// MartialRangedWeapons - fighter-accessible martial ranged weapons (for testing)
var MartialRangedWeapons = map[WeaponID]Weapon{
	HeavyCrossbow: {
		ID:         HeavyCrossbow,
		Name:       "Heavy Crossbow",
		Category:   CategoryMartialRanged,
		Cost:       "50 gp",
		Damage:     "1d10",
		DamageType: damage.Piercing,
		Weight:     18,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyHeavy, PropertyLoading, PropertyTwoHanded},
		Range:      &Range{Normal: 100, Long: 400},
	},
	Longbow: {
		ID:         Longbow,
		Name:       "Longbow",
		Category:   CategoryMartialRanged,
		Cost:       "50 gp",
		Damage:     "1d8",
		DamageType: damage.Piercing,
		Weight:     2,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyHeavy, PropertyTwoHanded},
		Range:      &Range{Normal: 150, Long: 600},
	},
	Blowgun: {
		ID:         Blowgun,
		Name:       "Blowgun",
		Category:   CategoryMartialRanged,
		Cost:       "10 gp",
		Damage:     "1",
		DamageType: damage.Piercing,
		Weight:     1,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyLoading},
		Range:      &Range{Normal: 25, Long: 100},
	},
	HandCrossbow: {
		ID:         HandCrossbow,
		Name:       "Hand Crossbow",
		Category:   CategoryMartialRanged,
		Cost:       "75 gp",
		Damage:     "1d6",
		DamageType: damage.Piercing,
		Weight:     3,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyLight, PropertyLoading},
		Range:      &Range{Normal: 30, Long: 120},
	},
	Net: {
		ID:         Net,
		Name:       "Net",
		Category:   CategoryMartialRanged,
		Cost:       "1 gp",
		Damage:     "0", // Special: restrains target
		DamageType: damage.None,
		Weight:     3,
		Properties: []WeaponProperty{PropertyThrown}, // Special: single attack at 5/15 range
		Range:      &Range{Normal: 5, Long: 15},
	},
}

// All combines all weapon maps for easy lookup
var All = make(map[WeaponID]Weapon)

func init() {
	// Populate the All map
	for id, w := range SimpleMeleeWeapons {
		All[id] = w
	}
	for id, w := range MartialMeleeWeapons {
		All[id] = w
	}
	for id, w := range SimpleRangedWeapons {
		All[id] = w
	}
	for id, w := range MartialRangedWeapons {
		All[id] = w
	}
}

// GetByID returns a weapon by its ID
func GetByID(id WeaponID) (Weapon, error) {
	w, ok := All[id]
	if !ok {
		validWeapons := make([]string, 0, len(All))
		for k := range All {
			validWeapons = append(validWeapons, string(k))
		}
		return Weapon{}, rpgerr.New(rpgerr.CodeInvalidArgument, "invalid weapon",
			rpgerr.WithMeta("provided", string(id)),
			rpgerr.WithMeta("valid_options", validWeapons))
	}
	return w, nil
}

// GetByCategory returns all weapons in a category
func GetByCategory(cat WeaponCategory) []Weapon {
	var result []Weapon
	for _, w := range All {
		if w.Category == cat {
			result = append(result, w)
		}
	}
	return result
}

// GetSimpleWeapons returns all simple weapons
func GetSimpleWeapons() []Weapon {
	var result []Weapon
	result = append(result, GetByCategory(CategorySimpleMelee)...)
	result = append(result, GetByCategory(CategorySimpleRanged)...)
	return result
}

// GetMartialWeapons returns all martial weapons
func GetMartialWeapons() []Weapon {
	var result []Weapon
	result = append(result, GetByCategory(CategoryMartialMelee)...)
	result = append(result, GetByCategory(CategoryMartialRanged)...)
	return result
}
