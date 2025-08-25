package choices

// FightingStyle represents a fighting style choice
type FightingStyle string

// Fighting Styles
const (
	FightingStyleArchery             FightingStyle = "archery"
	FightingStyleDefense             FightingStyle = "defense"
	FightingStyleDueling             FightingStyle = "dueling"
	FightingStyleGreatWeaponFighting FightingStyle = "great-weapon-fighting"
	FightingStyleProtection          FightingStyle = "protection"
	FightingStyleTwoWeaponFighting   FightingStyle = "two-weapon-fighting"
)

// ArmorID represents a specific armor
type ArmorID string

// Common Armor
const (
	LeatherArmor ArmorID = "leather-armor"
	ScaleMail    ArmorID = "scale-mail"
	ChainMail    ArmorID = "chain-mail"
	Shield       ArmorID = "shield"
)

// PackID represents an equipment pack
type PackID string

// Equipment Packs
const (
	BurglarsPack     PackID = "burglars-pack"
	DiplomatsPack    PackID = "diplomats-pack"
	DungeoneersPack  PackID = "dungeoneers-pack"
	EntertainersPack PackID = "entertainers-pack"
	ExplorersPack    PackID = "explorers-pack"
	PriestsPack      PackID = "priests-pack"
	ScholarsPack     PackID = "scholars-pack"
)

// ToolID represents a specific tool
type ToolID string

// Common Tools
const (
	ThievesTools ToolID = "thieves-tools"
)

// FocusID represents a spellcasting focus
type FocusID string

// Spellcasting Focuses
const (
	ArcaneFocus    FocusID = "arcane-focus"
	ComponentPouch FocusID = "component-pouch"
	HolySymbol     FocusID = "holy-symbol"
)

// InstrumentID represents a musical instrument
type InstrumentID string

// Common Instruments
const (
	Lute  InstrumentID = "lute"
	Flute InstrumentID = "flute"
	Drum  InstrumentID = "drum"
	Lyre  InstrumentID = "lyre"
	Horn  InstrumentID = "horn"
	Viol  InstrumentID = "viol"

	// Placeholder for any instrument choice
	AnyInstrument InstrumentID = "musical-instrument"
)

// AncestryID represents a draconic ancestry
type AncestryID string

// Draconic Ancestries
const (
	AncestryBlack  AncestryID = "black"
	AncestryBlue   AncestryID = "blue"
	AncestryBrass  AncestryID = "brass"
	AncestryBronze AncestryID = "bronze"
	AncestryCopper AncestryID = "copper"
	AncestryGold   AncestryID = "gold"
	AncestryGreen  AncestryID = "green"
	AncestryRed    AncestryID = "red"
	Ancestrysilver AncestryID = "silver"
	AncestryWhite  AncestryID = "white"
)
