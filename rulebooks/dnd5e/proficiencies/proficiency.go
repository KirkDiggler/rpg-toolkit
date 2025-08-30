// Package proficiencies defines typed proficiency constants for D&D 5e
package proficiencies

// Armor represents an armor proficiency type
type Armor string

// Weapon represents a weapon proficiency type
type Weapon string

// Tool represents a tool proficiency type
type Tool string

// Armor proficiencies (all are categories)
const (
	ArmorLight   Armor = "light"
	ArmorMedium  Armor = "medium"
	ArmorHeavy   Armor = "heavy"
	ArmorShields Armor = "shields"
)

// Weapon proficiencies - Categories
const (
	WeaponSimple  Weapon = "simple"  // All simple weapons
	WeaponMartial Weapon = "martial" // All martial weapons
)

// Weapon proficiencies - Specific weapons
const (
	// Simple weapons that appear in class grants
	WeaponClub          Weapon = "clubs"
	WeaponDagger        Weapon = "daggers"
	WeaponDart          Weapon = "darts"
	WeaponJavelin       Weapon = "javelins"
	WeaponLightCrossbow Weapon = "light-crossbows"
	WeaponMace          Weapon = "maces"
	WeaponQuarterstaff  Weapon = "quarterstaffs"
	WeaponSickle        Weapon = "sickles"
	WeaponSling         Weapon = "slings"
	WeaponSpear         Weapon = "spears"

	// Martial weapons that appear in class grants
	WeaponHandCrossbow Weapon = "hand-crossbows"
	WeaponLongsword    Weapon = "longswords"
	WeaponRapier       Weapon = "rapiers"
	WeaponScimitar     Weapon = "scimitars"
	WeaponShortsword   Weapon = "shortswords"
)

// Tool proficiencies (all are specific)
const (
	ToolThieves   Tool = "thieves-tools"
	ToolHerbalism Tool = "herbalism-kit"
)
