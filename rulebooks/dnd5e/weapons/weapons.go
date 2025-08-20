package weapons

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Fighter weapons - just a subset for testing
// Note: Fighter gets all simple and martial weapons, but we'll add just a few for testing

// SimpleMeleeWeapons - fighter-accessible simple melee weapons (for testing)
var SimpleMeleeWeapons = map[string]Weapon{
	"club": {
		ID:         "club",
		Name:       "Club",
		Category:   CategorySimpleMelee,
		Cost:       "1 sp",
		Damage:     "1d4",
		DamageType: "bludgeoning",
		Weight:     2,
		Properties: []WeaponProperty{PropertyLight},
	},
	"dagger": {
		ID:         "dagger",
		Name:       "Dagger",
		Category:   CategorySimpleMelee,
		Cost:       "2 gp",
		Damage:     "1d4",
		DamageType: "piercing",
		Weight:     1,
		Properties: []WeaponProperty{PropertyFinesse, PropertyLight, PropertyThrown},
		Range:      &Range{Normal: 20, Long: 60},
	},
	"handaxe": {
		ID:         "handaxe",
		Name:       "Handaxe",
		Category:   CategorySimpleMelee,
		Cost:       "5 gp",
		Damage:     "1d6",
		DamageType: "slashing",
		Weight:     2,
		Properties: []WeaponProperty{PropertyLight, PropertyThrown},
		Range:      &Range{Normal: 20, Long: 60},
	},
	"javelin": {
		ID:         "javelin",
		Name:       "Javelin",
		Category:   CategorySimpleMelee,
		Cost:       "5 sp",
		Damage:     "1d6",
		DamageType: "piercing",
		Weight:     2,
		Properties: []WeaponProperty{PropertyThrown},
		Range:      &Range{Normal: 30, Long: 120},
	},
}

// MartialMeleeWeapons - fighter-accessible martial melee weapons (for testing)
var MartialMeleeWeapons = map[string]Weapon{
	"greatsword": {
		ID:         "greatsword",
		Name:       "Greatsword",
		Category:   CategoryMartialMelee,
		Cost:       "50 gp",
		Damage:     "2d6",
		DamageType: "slashing",
		Weight:     6,
		Properties: []WeaponProperty{PropertyHeavy, PropertyTwoHanded},
	},
	"longsword": {
		ID:         "longsword",
		Name:       "Longsword",
		Category:   CategoryMartialMelee,
		Cost:       "15 gp",
		Damage:     "1d8",
		DamageType: "slashing",
		Weight:     3,
		Properties: []WeaponProperty{PropertyVersatile},
	},
	"rapier": {
		ID:         "rapier",
		Name:       "Rapier",
		Category:   CategoryMartialMelee,
		Cost:       "25 gp",
		Damage:     "1d8",
		DamageType: "piercing",
		Weight:     2,
		Properties: []WeaponProperty{PropertyFinesse},
	},
	"shortsword": {
		ID:         "shortsword",
		Name:       "Shortsword",
		Category:   CategoryMartialMelee,
		Cost:       "10 gp",
		Damage:     "1d6",
		DamageType: "piercing",
		Weight:     2,
		Properties: []WeaponProperty{PropertyFinesse, PropertyLight},
	},
}

// SimpleRangedWeapons - fighter-accessible simple ranged weapons (for testing)
var SimpleRangedWeapons = map[string]Weapon{
	"light-crossbow": {
		ID:         "light-crossbow",
		Name:       "Light Crossbow",
		Category:   CategorySimpleRanged,
		Cost:       "25 gp",
		Damage:     "1d8",
		DamageType: "piercing",
		Weight:     5,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyLoading, PropertyTwoHanded},
		Range:      &Range{Normal: 80, Long: 320},
	},
	"shortbow": {
		ID:         "shortbow",
		Name:       "Shortbow",
		Category:   CategorySimpleRanged,
		Cost:       "25 gp",
		Damage:     "1d6",
		DamageType: "piercing",
		Weight:     2,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyTwoHanded},
		Range:      &Range{Normal: 80, Long: 320},
	},
}

// MartialRangedWeapons - fighter-accessible martial ranged weapons (for testing)
var MartialRangedWeapons = map[string]Weapon{
	"heavy-crossbow": {
		ID:         "heavy-crossbow",
		Name:       "Heavy Crossbow",
		Category:   CategoryMartialRanged,
		Cost:       "50 gp",
		Damage:     "1d10",
		DamageType: "piercing",
		Weight:     18,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyHeavy, PropertyLoading, PropertyTwoHanded},
		Range:      &Range{Normal: 100, Long: 400},
	},
	"longbow": {
		ID:         "longbow",
		Name:       "Longbow",
		Category:   CategoryMartialRanged,
		Cost:       "50 gp",
		Damage:     "1d8",
		DamageType: "piercing",
		Weight:     2,
		Properties: []WeaponProperty{PropertyAmmunition, PropertyHeavy, PropertyTwoHanded},
		Range:      &Range{Normal: 150, Long: 600},
	},
}

// All combines all weapon maps for easy lookup
var All = make(map[string]Weapon)

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
func GetByID(id string) (Weapon, error) {
	w, ok := All[id]
	if !ok {
		validWeapons := make([]string, 0, len(All))
		for k := range All {
			validWeapons = append(validWeapons, k)
		}
		return Weapon{}, rpgerr.New(rpgerr.CodeInvalidArgument, "invalid weapon",
			rpgerr.WithMeta("provided", id),
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
