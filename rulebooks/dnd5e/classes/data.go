package classes

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Data contains all the game mechanics data for a class
type Data struct {
	ID             Class // The class this data represents
	HitDice        int
	PrimaryAbility abilities.Ability
	SavingThrows   []abilities.Ability

	// Proficiencies
	Armor   []proficiencies.Armor
	Weapons []proficiencies.Weapon
	Tools   []proficiencies.Tool // Some classes get tool proficiencies

	// Skill proficiencies
	SkillCount int            // Number of skills to choose
	SkillList  []skills.Skill // Available skills to choose from

	// Spellcasting
	//
	// The ability is the only spellcasting fact that does not vary by level.
	// How many cantrips and spells a class knows, and what slots it has, are
	// per-level facts and live in its [SpellProgression] — they were three
	// scalars here, each holding only the level-1 value (design R4.5).
	SpellcastingAbility abilities.Ability // Empty if not a spellcaster

	// Subclass information
	SubclassLevel    int        // Level when subclass is chosen (0 = no subclass)
	SubclassLabel    string     // e.g., "Divine Domain", "Martial Archetype"
	SubclassChoiceID string     // The choice ID for this subclass (e.g., "fighter-archetype")
	Subclasses       []Subclass // Available subclass options

	// Starting equipment (simplified for now)
	// TODO: Add equipment choices
}

// ClassData is the lookup map for all class data
var ClassData = map[Class]*Data{
	Fighter: {
		ID:             Fighter,
		HitDice:        10,
		PrimaryAbility: abilities.STR,
		SavingThrows:   []abilities.Ability{abilities.STR, abilities.CON},
		Armor: []proficiencies.Armor{
			proficiencies.ArmorLight,
			proficiencies.ArmorMedium,
			proficiencies.ArmorHeavy,
			proficiencies.ArmorShields,
		},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponMartial,
		},
		SkillCount: 2,
		SkillList: []skills.Skill{
			skills.Acrobatics,
			skills.AnimalHandling,
			skills.Athletics,
			skills.History,
			skills.Insight,
			skills.Intimidation,
			skills.Perception,
			skills.Survival,
		},
		SubclassLevel:    3,
		SubclassLabel:    "Martial Archetype",
		SubclassChoiceID: "fighter-archetype",
		Subclasses: []Subclass{
			Champion,
			BattleMaster,
			EldritchKnight,
		},
	},

	Wizard: {
		ID:             Wizard,
		HitDice:        6,
		PrimaryAbility: abilities.INT,
		SavingThrows:   []abilities.Ability{abilities.INT, abilities.WIS},
		Armor:          []proficiencies.Armor{},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponDagger,
			proficiencies.WeaponDart,
			proficiencies.WeaponSling,
			proficiencies.WeaponQuarterstaff,
			proficiencies.WeaponLightCrossbow,
		},
		SkillCount: 2,
		SkillList: []skills.Skill{
			skills.Arcana,
			skills.History,
			skills.Insight,
			skills.Investigation,
			skills.Medicine,
			skills.Religion,
		},
		SpellcastingAbility: abilities.INT,
		SubclassLevel:       2,
		SubclassLabel:       "Arcane Tradition",
		SubclassChoiceID:    "wizard-tradition",
		Subclasses: []Subclass{
			Evocation,
			Abjuration,
			Conjuration,
			Divination,
			Enchantment,
			Illusion,
			Necromancy,
			Transmutation,
		},
	},

	Cleric: {
		ID:             Cleric,
		HitDice:        8,
		PrimaryAbility: abilities.WIS,
		SavingThrows:   []abilities.Ability{abilities.WIS, abilities.CHA},
		Armor: []proficiencies.Armor{
			proficiencies.ArmorLight,
			proficiencies.ArmorMedium,
			proficiencies.ArmorShields,
		},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponSimple,
		},
		SkillCount: 2,
		SkillList: []skills.Skill{
			skills.History,
			skills.Insight,
			skills.Medicine,
			skills.Persuasion,
			skills.Religion,
		},
		SpellcastingAbility: abilities.WIS,
		SubclassLevel:       1,
		SubclassLabel:       "Divine Domain",
		SubclassChoiceID:    "cleric-domain",
		Subclasses: []Subclass{
			LifeDomain,
			LightDomain,
			NatureDomain,
			TempestDomain,
			TrickeryDomain,
			WarDomain,
			KnowledgeDomain,
		},
	},

	Rogue: {
		ID:             Rogue,
		HitDice:        8,
		PrimaryAbility: abilities.DEX,
		SavingThrows:   []abilities.Ability{abilities.DEX, abilities.INT},
		Armor: []proficiencies.Armor{
			proficiencies.ArmorLight,
		},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponHandCrossbow,
			proficiencies.WeaponLongsword,
			proficiencies.WeaponRapier,
			proficiencies.WeaponShortsword,
		},
		Tools: []proficiencies.Tool{
			proficiencies.ToolThieves,
		},
		SkillCount: 4, // Rogues get more skills
		SkillList: []skills.Skill{
			skills.Acrobatics,
			skills.Athletics,
			skills.Deception,
			skills.Insight,
			skills.Intimidation,
			skills.Investigation,
			skills.Perception,
			skills.Performance,
			skills.Persuasion,
			skills.SleightOfHand,
			skills.Stealth,
		},
		SubclassLevel:    3,
		SubclassLabel:    "Roguish Archetype",
		SubclassChoiceID: "rogue-archetype",
		Subclasses: []Subclass{
			Thief,
			Assassin,
			ArcaneTrickster,
		},
	},

	Barbarian: {
		ID:             Barbarian,
		HitDice:        12,
		PrimaryAbility: abilities.STR,
		SavingThrows:   []abilities.Ability{abilities.STR, abilities.CON},
		Armor: []proficiencies.Armor{
			proficiencies.ArmorLight,
			proficiencies.ArmorMedium,
			proficiencies.ArmorShields,
		},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponMartial,
		},
		SkillCount: 2,
		SkillList: []skills.Skill{
			skills.AnimalHandling,
			skills.Athletics,
			skills.Intimidation,
			skills.Nature,
			skills.Perception,
			skills.Survival,
		},
		SubclassLevel:    3,
		SubclassLabel:    "Primal Path",
		SubclassChoiceID: "barbarian-path",
		Subclasses: []Subclass{
			Berserker,
			Totem,
		},
	},

	Bard: {
		ID:             Bard,
		HitDice:        8,
		PrimaryAbility: abilities.CHA,
		SavingThrows:   []abilities.Ability{abilities.DEX, abilities.CHA},
		Armor: []proficiencies.Armor{
			proficiencies.ArmorLight,
		},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponHandCrossbow,
			proficiencies.WeaponLongsword,
			proficiencies.WeaponRapier,
			proficiencies.WeaponShortsword,
		},
		Tools:               []proficiencies.Tool{}, // Bards choose 3 musical instruments
		SkillCount:          3,
		SkillList:           []skills.Skill{}, // Bards can choose any 3 skills
		SpellcastingAbility: abilities.CHA,
		SubclassLevel:       3,
		SubclassLabel:       "Bard College",
		SubclassChoiceID:    "bard-college",
		Subclasses: []Subclass{
			Lore,
			Valor,
		},
	},

	Druid: {
		ID:             Druid,
		HitDice:        8,
		PrimaryAbility: abilities.WIS,
		SavingThrows:   []abilities.Ability{abilities.INT, abilities.WIS},
		Armor: []proficiencies.Armor{
			proficiencies.ArmorLight,
			proficiencies.ArmorMedium,
			proficiencies.ArmorShields,
			// Note: Druids won't wear metal armor
		},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponClub,
			proficiencies.WeaponDagger,
			proficiencies.WeaponDart,
			proficiencies.WeaponJavelin,
			proficiencies.WeaponMace,
			proficiencies.WeaponQuarterstaff,
			proficiencies.WeaponScimitar,
			proficiencies.WeaponSickle,
			proficiencies.WeaponSling,
			proficiencies.WeaponSpear,
		},
		Tools: []proficiencies.Tool{
			proficiencies.ToolHerbalism,
		},
		SkillCount: 2,
		SkillList: []skills.Skill{
			skills.Arcana,
			skills.AnimalHandling,
			skills.Insight,
			skills.Medicine,
			skills.Nature,
			skills.Perception,
			skills.Religion,
			skills.Survival,
		},
		SpellcastingAbility: abilities.WIS,
		SubclassLevel:       2,
		SubclassLabel:       "Druid Circle",
		SubclassChoiceID:    "druid-circle",
		Subclasses: []Subclass{
			CircleLand,
			CircleMoon,
		},
	},

	Monk: {
		ID:             Monk,
		HitDice:        8,
		PrimaryAbility: abilities.DEX,
		SavingThrows:   []abilities.Ability{abilities.STR, abilities.DEX},
		Armor:          []proficiencies.Armor{},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponShortsword,
		},
		Tools:      []proficiencies.Tool{}, // Monks can choose one type of artisan's tools or one musical instrument
		SkillCount: 2,
		SkillList: []skills.Skill{
			skills.Acrobatics,
			skills.Athletics,
			skills.History,
			skills.Insight,
			skills.Religion,
			skills.Stealth,
		},
	},

	Paladin: {
		ID:             Paladin,
		HitDice:        10,
		PrimaryAbility: abilities.STR,
		SavingThrows:   []abilities.Ability{abilities.WIS, abilities.CHA},
		Armor: []proficiencies.Armor{
			proficiencies.ArmorLight,
			proficiencies.ArmorMedium,
			proficiencies.ArmorHeavy,
			proficiencies.ArmorShields,
		},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponMartial,
		},
		SkillCount: 2,
		SkillList: []skills.Skill{
			skills.Athletics,
			skills.Insight,
			skills.Intimidation,
			skills.Medicine,
			skills.Persuasion,
			skills.Religion,
		},
		// Paladins don't get spellcasting until level 2
		SpellcastingAbility: abilities.CHA,
	},

	Ranger: {
		ID:             Ranger,
		HitDice:        10,
		PrimaryAbility: abilities.DEX,
		SavingThrows:   []abilities.Ability{abilities.STR, abilities.DEX},
		Armor: []proficiencies.Armor{
			proficiencies.ArmorLight,
			proficiencies.ArmorMedium,
			proficiencies.ArmorShields,
		},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponSimple,
			proficiencies.WeaponMartial,
		},
		SkillCount: 3,
		SkillList: []skills.Skill{
			skills.AnimalHandling,
			skills.Athletics,
			skills.Insight,
			skills.Investigation,
			skills.Nature,
			skills.Perception,
			skills.Stealth,
			skills.Survival,
		},
		// Rangers don't get spellcasting until level 2
		SpellcastingAbility: abilities.WIS,
	},

	Sorcerer: {
		ID:             Sorcerer,
		HitDice:        6,
		PrimaryAbility: abilities.CHA,
		SavingThrows:   []abilities.Ability{abilities.CON, abilities.CHA},
		Armor:          []proficiencies.Armor{},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponDagger,
			proficiencies.WeaponDart,
			proficiencies.WeaponSling,
			proficiencies.WeaponQuarterstaff,
			proficiencies.WeaponLightCrossbow,
		},
		SkillCount: 2,
		SkillList: []skills.Skill{
			skills.Arcana,
			skills.Deception,
			skills.Insight,
			skills.Intimidation,
			skills.Persuasion,
			skills.Religion,
		},
		SpellcastingAbility: abilities.CHA,
	},

	Warlock: {
		ID:             Warlock,
		HitDice:        8,
		PrimaryAbility: abilities.CHA,
		SavingThrows:   []abilities.Ability{abilities.WIS, abilities.CHA},
		Armor: []proficiencies.Armor{
			proficiencies.ArmorLight,
		},
		Weapons: []proficiencies.Weapon{
			proficiencies.WeaponSimple,
		},
		SkillCount: 2,
		SkillList: []skills.Skill{
			skills.Arcana,
			skills.Deception,
			skills.History,
			skills.Intimidation,
			skills.Investigation,
			skills.Nature,
			skills.Religion,
		},
		SpellcastingAbility: abilities.CHA,
	},
}

// GetData returns the class data for a given class ID
func GetData(classID Class) *Data {
	return ClassData[classID]
}

// Name returns the display name of the class
func (d *Data) Name() string {
	return Name(d.ID)
}

// Description returns the description of the class
func (d *Data) Description() string {
	return Description(d.ID)
}
