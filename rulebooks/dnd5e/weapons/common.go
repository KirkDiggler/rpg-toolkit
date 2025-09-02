package weapons

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"

// Common weapon IDs used in character creation and equipment choices
// These are typed strings to provide compile-time safety for frequently used weapons

// WeaponID represents a specific weapon (alias of shared.EquipmentID)
type WeaponID = shared.EquipmentID

// Simple Melee Weapons
const (
	Club         WeaponID = "club"
	Dagger       WeaponID = "dagger"
	Greatclub    WeaponID = "greatclub"
	Handaxe      WeaponID = "handaxe"
	Javelin      WeaponID = "javelin"
	LightHammer  WeaponID = "light-hammer"
	Mace         WeaponID = "mace"
	Quarterstaff WeaponID = "quarterstaff"
	Sickle       WeaponID = "sickle"
	Spear        WeaponID = "spear"
)

// Simple Ranged Weapons
const (
	LightCrossbow WeaponID = "light-crossbow"
	Dart          WeaponID = "dart"
	Shortbow      WeaponID = "shortbow"
	Sling         WeaponID = "sling"
)

// Martial Melee Weapons
const (
	Battleaxe   WeaponID = "battleaxe"
	Flail       WeaponID = "flail"
	Glaive      WeaponID = "glaive"
	Greataxe    WeaponID = "greataxe"
	Greatsword  WeaponID = "greatsword"
	Halberd     WeaponID = "halberd"
	Lance       WeaponID = "lance"
	Longsword   WeaponID = "longsword"
	Maul        WeaponID = "maul"
	Morningstar WeaponID = "morningstar"
	Pike        WeaponID = "pike"
	Rapier      WeaponID = "rapier"
	Scimitar    WeaponID = "scimitar"
	Shortsword  WeaponID = "shortsword"
	Trident     WeaponID = "trident"
	WarPick     WeaponID = "war-pick"
	Warhammer   WeaponID = "warhammer"
	Whip        WeaponID = "whip"
)

// Martial Ranged Weapons
const (
	Blowgun       WeaponID = "blowgun"
	HandCrossbow  WeaponID = "hand-crossbow"
	HeavyCrossbow WeaponID = "heavy-crossbow"
	Longbow       WeaponID = "longbow"
	Net           WeaponID = "net"
)

// Ammunition
const (
	Arrows20 WeaponID = "arrows-20"
	Bolts20  WeaponID = "bolts-20"
)

// Category placeholders for choice requirements
// These are used when a choice allows any weapon from a category
const (
	AnySimpleWeapon  WeaponID = "simple-weapon"
	AnyMartialWeapon WeaponID = "martial-weapon"
	AnyWeapon        WeaponID = "any-weapon"
)
