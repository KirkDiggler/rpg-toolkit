// Package bundles provides constants for equipment bundle identifiers
package bundles

// BundleID represents a unique identifier for an equipment bundle
type BundleID string

// Fighter equipment bundles
const (
	// Armor bundles
	LeatherArmorLongbow BundleID = "leather-armor-longbow-bundle"

	// Weapon bundles
	MartialWeaponAndShield BundleID = "martial-weapon-and-shield"
	TwoMartialWeapons      BundleID = "two-martial-weapons"
	TwoSimpleWeapons       BundleID = "two-simple-weapons"
	SimpleWeaponAndShield  BundleID = "simple-weapon-and-shield"
)

// Explorer's packs and similar
const (
	ExplorersPack    BundleID = "explorers-pack"
	DungeoneersPack  BundleID = "dungeoneers-pack"
	BurglarsPack     BundleID = "burglars-pack"
	EntertainersPack BundleID = "entertainers-pack"
	DiplomatsPack    BundleID = "diplomats-pack"
	ScholarsPack     BundleID = "scholars-pack"
	PriestsPack      BundleID = "priests-pack"
)

// Rogue equipment bundles
const (
	RapierAndShortbow     BundleID = "rapier-and-shortbow"
	ShortswordAndShortbow BundleID = "shortsword-and-shortbow"
)

// Wizard equipment bundles
const (
	ComponentPouchBundle BundleID = "component-pouch-bundle"
	ArcaneFocusBundle    BundleID = "arcane-focus-bundle"
)

// Cleric equipment bundles
const (
	MaceAndShield     BundleID = "mace-and-shield"
	WarhamerAndShield BundleID = "warhammer-and-shield" // If dwarf
)
