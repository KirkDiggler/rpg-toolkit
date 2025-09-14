package choices

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"

// ChoiceID represents a unique identifier for a choice requirement
type ChoiceID string

// OptionID represents a unique identifier for an option within a choice
type OptionID = shared.SelectionID

// Class skill choice IDs
const (
	FighterSkills   ChoiceID = "fighter-skills"
	RogueSkills     ChoiceID = "rogue-skills"
	WizardSkills    ChoiceID = "wizard-skills"
	ClericSkills    ChoiceID = "cleric-skills"
	RangerSkills    ChoiceID = "ranger-skills"
	BarbarianSkills ChoiceID = "barbarian-skills"
	BardSkills      ChoiceID = "bard-skills"
	DruidSkills     ChoiceID = "druid-skills"
	MonkSkills      ChoiceID = "monk-skills"
	PaladinSkills   ChoiceID = "paladin-skills"
	SorcererSkills  ChoiceID = "sorcerer-skills"
	WarlockSkills   ChoiceID = "warlock-skills"
)

// Fighting style choice IDs
const (
	FighterFightingStyle ChoiceID = "fighter-fighting-style"
	RangerFightingStyle  ChoiceID = "ranger-fighting-style"
	PaladinFightingStyle ChoiceID = "paladin-fighting-style"
)

// Expertise choice IDs
const (
	RogueExpertise1 ChoiceID = "rogue-expertise-1" // Level 1
	RogueExpertise6 ChoiceID = "rogue-expertise-6" // Level 6
	BardExpertise3  ChoiceID = "bard-expertise-3"  // Level 3
	BardExpertise10 ChoiceID = "bard-expertise-10" // Level 10
)

// Fighter equipment choice IDs
const (
	FighterArmor            ChoiceID = "fighter-armor"
	FighterWeaponsPrimary   ChoiceID = "fighter-weapons-primary"
	FighterWeaponsSecondary ChoiceID = "fighter-weapons-secondary"
	FighterPack             ChoiceID = "fighter-pack"
	// Martial weapon choices are now embedded in the equipment options
)

// Barbarian equipment choice IDs
const (
	BarbarianWeaponsPrimary   ChoiceID = "barbarian-weapons-primary"
	BarbarianWeaponsSecondary ChoiceID = "barbarian-weapons-secondary"
	BarbarianPack             ChoiceID = "barbarian-pack"
)

// Rogue equipment choice IDs
const (
	RogueWeaponsPrimary   ChoiceID = "rogue-weapons-primary"
	RogueWeaponsSecondary ChoiceID = "rogue-weapons-secondary"
	RoguePack             ChoiceID = "rogue-pack"
)

// Wizard equipment choice IDs
const (
	WizardWeaponsPrimary ChoiceID = "wizard-weapons-primary"
	WizardFocus          ChoiceID = "wizard-focus"
	WizardPack           ChoiceID = "wizard-pack"
)

// Cleric equipment choice IDs
const (
	ClericWeapons         ChoiceID = "cleric-weapons"
	ClericArmor           ChoiceID = "cleric-armor"
	ClericSecondaryWeapon ChoiceID = "cleric-secondary-weapon"
	ClericPack            ChoiceID = "cleric-pack"
	ClericHolySymbol      ChoiceID = "cleric-holy-symbol"
)

// Bard equipment choice IDs
const (
	BardWeaponsPrimary ChoiceID = "bard-weapons-primary"
	BardPack           ChoiceID = "bard-pack"
	BardInstrument     ChoiceID = "bard-instrument"
)

// Druid equipment choice IDs
const (
	DruidWeaponsPrimary   ChoiceID = "druid-weapons-primary"
	DruidWeaponsSecondary ChoiceID = "druid-weapons-secondary"
	DruidFocus            ChoiceID = "druid-focus"
)

// Monk equipment choice IDs
const (
	MonkWeaponsPrimary ChoiceID = "monk-weapons-primary"
	MonkPack           ChoiceID = "monk-pack"
)

// Paladin equipment choice IDs
const (
	PaladinWeaponsPrimary   ChoiceID = "paladin-weapons-primary"
	PaladinWeaponsSecondary ChoiceID = "paladin-weapons-secondary"
	PaladinPack             ChoiceID = "paladin-pack"
	PaladinHolySymbol       ChoiceID = "paladin-holy-symbol"
)

// Ranger equipment choice IDs
const (
	RangerArmor          ChoiceID = "ranger-armor"
	RangerWeaponsPrimary ChoiceID = "ranger-weapons-primary"
	RangerPack           ChoiceID = "ranger-pack"
)

// Sorcerer equipment choice IDs
const (
	SorcererWeaponsPrimary ChoiceID = "sorcerer-weapons-primary"
	SorcererFocus          ChoiceID = "sorcerer-focus"
	SorcererPack           ChoiceID = "sorcerer-pack"
)

// Warlock equipment choice IDs
const (
	WarlockWeaponsPrimary   ChoiceID = "warlock-weapons-primary"
	WarlockFocus            ChoiceID = "warlock-focus"
	WarlockPack             ChoiceID = "warlock-pack"
	WarlockWeaponsSecondary ChoiceID = "warlock-weapons-secondary"
)

// Race skill choice IDs
const (
	HalfElfSkills ChoiceID = "half-elf-skills"
)

// Race language choice IDs
const (
	HumanLanguage   ChoiceID = "human-language"
	HalfElfLanguage ChoiceID = "half-elf-language"
	HighElfLanguage ChoiceID = "high-elf-language"
	DwarfLanguage   ChoiceID = "dwarf-language"
)

// Race cantrip choice IDs
const (
	HighElfCantrip ChoiceID = "high-elf-cantrip"
)

// Tool proficiency choice IDs
const (
	MonkTools            ChoiceID = "monk-tools"
	BardTools            ChoiceID = "bard-tools"
	BardInstruments      ChoiceID = "bard-instruments"
	GuildArtisanTools    ChoiceID = "guild-artisan-tools"
	DwarfToolProficiency ChoiceID = "dwarf-tool-proficiency"
)

// Subclass choice IDs
const (
	FighterArchetype ChoiceID = "fighter-archetype" // Level 3
	RogueArchetype   ChoiceID = "rogue-archetype"   // Level 3
	WizardSchool     ChoiceID = "wizard-school"     // Level 2
	ClericDomain     ChoiceID = "cleric-domain"     // Level 1
	BarbarianPath    ChoiceID = "barbarian-path"    // Level 3
	BardCollege      ChoiceID = "bard-college"      // Level 3
	DruidCircle      ChoiceID = "druid-circle"      // Level 2
	MonkTradition    ChoiceID = "monk-tradition"    // Level 3
	PaladinOath      ChoiceID = "paladin-oath"      // Level 3
	RangerArchetype  ChoiceID = "ranger-archetype"  // Level 3
	SorcererOrigin   ChoiceID = "sorcerer-origin"   // Level 1
	WarlockPatron    ChoiceID = "warlock-patron"    // Level 1
)

// Spell choice IDs
const (
	WizardCantrips1   ChoiceID = "wizard-cantrips-1"
	WizardSpells1     ChoiceID = "wizard-spells-1"
	ClericCantrips1   ChoiceID = "cleric-cantrips-1"
	BardCantrips1     ChoiceID = "bard-cantrips-1"
	BardSpells1       ChoiceID = "bard-spells-1"
	DruidCantrips1    ChoiceID = "druid-cantrips-1"
	SorcererCantrips1 ChoiceID = "sorcerer-cantrips-1"
	SorcererSpells1   ChoiceID = "sorcerer-spells-1"
	WarlockCantrips1  ChoiceID = "warlock-cantrips-1"
	WarlockSpells1    ChoiceID = "warlock-spells-1"
)

// BackgroundData choice IDs
const (
	AcolyteLanguages     ChoiceID = "acolyte-languages"
	CriminalTools        ChoiceID = "criminal-tools"
	EntertainerTools     ChoiceID = "entertainer-tools"
	FolkHeroTools        ChoiceID = "folk-hero-tools"
	GuildArtisanLanguage ChoiceID = "guild-artisan-language"
	HermitLanguage       ChoiceID = "hermit-language"
	NobleLanguage        ChoiceID = "noble-language"
	OutlanderLanguage    ChoiceID = "outlander-language"
	SageLanguages        ChoiceID = "sage-languages"
	SoldierTools         ChoiceID = "soldier-tools"
)

// Equipment option IDs - Fighter
const (
	FighterArmorChainMail      OptionID = "fighter-armor-a"
	FighterArmorLeather        OptionID = "fighter-armor-b"
	FighterWeaponMartialShield OptionID = "fighter-weapon-a"
	FighterWeaponTwoMartial    OptionID = "fighter-weapon-b"
	FighterRangedCrossbow      OptionID = "fighter-ranged-a"
	FighterRangedHandaxes      OptionID = "fighter-ranged-b"
	FighterPackDungeoneer      OptionID = "fighter-pack-a"
	FighterPackExplorer        OptionID = "fighter-pack-b"
)

// Equipment option IDs - Barbarian
const (
	BarbarianWeaponGreataxe    OptionID = "barbarian-weapon-a"
	BarbarianWeaponMartial     OptionID = "barbarian-weapon-b"
	BarbarianSecondaryHandaxes OptionID = "barbarian-secondary-a"
	BarbarianSecondarySimple   OptionID = "barbarian-secondary-b"
	BarbarianPackExplorer      OptionID = "barbarian-pack-a"
	BarbarianPackDungeoneer    OptionID = "barbarian-pack-b"
)

// Equipment option IDs - Rogue
const (
	RogueWeaponRapier        OptionID = "rogue-weapon-a"
	RogueWeaponShortsword    OptionID = "rogue-weapon-b"
	RogueSecondaryShortbow   OptionID = "rogue-secondary-a"
	RogueSecondaryShortsword OptionID = "rogue-secondary-b"
	RoguePackBurglar         OptionID = "rogue-pack-a"
	RoguePackDungeoneer      OptionID = "rogue-pack-b"
	RoguePackExplorer        OptionID = "rogue-pack-c"
)

// Equipment option IDs - Wizard
const (
	WizardWeaponQuarterstaff OptionID = "wizard-weapon-a"
	WizardWeaponDagger       OptionID = "wizard-weapon-b"
	WizardFocusComponent     OptionID = "wizard-focus-a"
	WizardFocusStaff         OptionID = "wizard-focus-b"
	WizardPackScholar        OptionID = "wizard-pack-a"
	WizardPackExplorer       OptionID = "wizard-pack-b"
)

// Equipment option IDs - Cleric
const (
	ClericWeaponMace        OptionID = "cleric-weapon-a"
	ClericWeaponWarhammer   OptionID = "cleric-weapon-b"
	ClericArmorScale        OptionID = "cleric-armor-a"
	ClericArmorLeather      OptionID = "cleric-armor-b"
	ClericArmorChainMail    OptionID = "cleric-armor-c"
	ClericSecondaryShortbow OptionID = "cleric-secondary-a"
	ClericSecondarySimple   OptionID = "cleric-secondary-b"
	ClericPackPriest        OptionID = "cleric-pack-a"
	ClericPackExplorer      OptionID = "cleric-pack-b"
	ClericHolyAmulet        OptionID = "cleric-holy-a"
	ClericHolyEmblem        OptionID = "cleric-holy-b"
)

// Equipment option IDs - Bard
const (
	BardWeaponRapier    OptionID = "bard-weapon-a"
	BardWeaponLongsword OptionID = "bard-weapon-b"
	BardWeaponSimple    OptionID = "bard-weapon-c"
	BardPackDiplomat    OptionID = "bard-pack-a"
	BardPackEntertainer OptionID = "bard-pack-b"
	BardInstrumentLute  OptionID = "bard-instrument-a"
	BardInstrumentOther OptionID = "bard-instrument-b"
)

// Equipment option IDs - Druid
const (
	DruidWeaponShield      OptionID = "druid-weapon-a"
	DruidWeaponSimple      OptionID = "druid-weapon-b"
	DruidSecondaryScimitar OptionID = "druid-secondary-a"
	DruidSecondaryMelee    OptionID = "druid-secondary-b"
	DruidFocusOption       OptionID = "druid-focus-a"
)

// Equipment option IDs - Monk
const (
	MonkWeaponShortsword OptionID = "monk-weapon-a"
	MonkWeaponSimple     OptionID = "monk-weapon-b"
	MonkPackDungeoneer   OptionID = "monk-pack-a"
	MonkPackExplorer     OptionID = "monk-pack-b"
)

// Equipment option IDs - Paladin
const (
	PaladinWeaponMartialShield OptionID = "paladin-weapon-a"
	PaladinWeaponTwoMartial    OptionID = "paladin-weapon-b"
	PaladinSecondaryJavelins   OptionID = "paladin-secondary-a"
	PaladinSecondarySimple     OptionID = "paladin-secondary-b"
	PaladinPackPriest          OptionID = "paladin-pack-a"
	PaladinPackExplorer        OptionID = "paladin-pack-b"
	PaladinHolySymbolOption    OptionID = "paladin-holy-a"
)

// Equipment option IDs - Ranger
const (
	RangerArmorScale        OptionID = "ranger-armor-a"
	RangerArmorLeather      OptionID = "ranger-armor-b"
	RangerWeaponShortswords OptionID = "ranger-weapon-a"
	RangerWeaponSimpleMelee OptionID = "ranger-weapon-b"
	RangerPackDungeoneer    OptionID = "ranger-pack-a"
	RangerPackExplorer      OptionID = "ranger-pack-b"
)

// Equipment option IDs - Sorcerer
const (
	SorcererWeaponCrossbow OptionID = "sorcerer-weapon-a"
	SorcererWeaponSimple   OptionID = "sorcerer-weapon-b"
	SorcererFocusComponent OptionID = "sorcerer-focus-a"
	SorcererFocusArcane    OptionID = "sorcerer-focus-b"
	SorcererPackDungeoneer OptionID = "sorcerer-pack-a"
	SorcererPackExplorer   OptionID = "sorcerer-pack-b"
)

// Equipment option IDs - Warlock
const (
	WarlockWeaponCrossbow  OptionID = "warlock-weapon-a"
	WarlockWeaponSimple    OptionID = "warlock-weapon-b"
	WarlockFocusComponent  OptionID = "warlock-focus-a"
	WarlockFocusArcane     OptionID = "warlock-focus-b"
	WarlockPackScholar     OptionID = "warlock-pack-a"
	WarlockPackDungeoneer  OptionID = "warlock-pack-b"
	WarlockWeaponSecondary OptionID = "warlock-weapon-c"
)
