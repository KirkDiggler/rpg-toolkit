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
	WeaponShortbow      Weapon = "shortbows"
	WeaponSickle        Weapon = "sickles"
	WeaponSling         Weapon = "slings"
	WeaponSpear         Weapon = "spears"

	// Martial weapons that appear in class grants
	WeaponHandCrossbow Weapon = "hand-crossbows"
	WeaponLongbow      Weapon = "longbows"
	WeaponLongsword    Weapon = "longswords"
	WeaponRapier       Weapon = "rapiers"
	WeaponScimitar     Weapon = "scimitars"
	WeaponShortsword   Weapon = "shortswords"
)

// Tool proficiencies - Artisan's Tools
const (
	ToolAlchemist     Tool = "alchemist-supplies"
	ToolBrewer        Tool = "brewer-supplies"
	ToolCalligrapher  Tool = "calligrapher-supplies"
	ToolCarpenter     Tool = "carpenter-tools"
	ToolCartographer  Tool = "cartographer-tools"
	ToolCobbler       Tool = "cobbler-tools"
	ToolCook          Tool = "cook-utensils"
	ToolGlassblower   Tool = "glassblower-tools"
	ToolJeweler       Tool = "jeweler-tools"
	ToolLeatherworker Tool = "leatherworker-tools"
	ToolMason         Tool = "mason-tools"
	ToolPainter       Tool = "painter-supplies"
	ToolPotter        Tool = "potter-tools"
	ToolSmith         Tool = "smith-tools"
	ToolTinker        Tool = "tinker-tools"
	ToolWeaver        Tool = "weaver-tools"
	ToolWoodcarver    Tool = "woodcarver-tools"
)

// Tool proficiencies - Gaming Sets
const (
	ToolDiceSet         Tool = "dice-set"
	ToolPlayingCardSet  Tool = "playing-card-set"
	ToolDragonchessSet  Tool = "dragonchess-set"
	ToolThreeDragonAnte Tool = "three-dragon-ante-set"
)

// Tool proficiencies - Musical Instruments
const (
	ToolBagpipes Tool = "bagpipes"
	ToolDrum     Tool = "drum"
	ToolDulcimer Tool = "dulcimer"
	ToolFlute    Tool = "flute"
	ToolLute     Tool = "lute"
	ToolLyre     Tool = "lyre"
	ToolHorn     Tool = "horn"
	ToolPanFlute Tool = "pan-flute"
	ToolShawm    Tool = "shawm"
	ToolViol     Tool = "viol"
)

// Tool proficiencies - Other Tools
const (
	ToolDisguiseKit  Tool = "disguise-kit"
	ToolForgeryKit   Tool = "forgery-kit"
	ToolHerbalism    Tool = "herbalism-kit"
	ToolNavigator    Tool = "navigator-tools"
	ToolPoisoner     Tool = "poisoner-kit"
	ToolThieves      Tool = "thieves-tools"
	ToolVehicleLand  Tool = "vehicles-land"
	ToolVehicleWater Tool = "vehicles-water"
)
