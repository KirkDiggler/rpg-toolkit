package races

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Data contains all the game mechanics data for a race
type Data struct {
	ID               Race // The race this data represents
	Speed            int
	Size             string // "Small", "Medium", "Large"
	AbilityIncreases map[abilities.Ability]int
	Traits           []Trait

	// Automatic proficiencies granted by race
	Skills    []skills.Skill
	Weapons   []proficiencies.Weapon
	Tools     []proficiencies.Tool
	Languages []languages.Language

	// Choices the player must make
	SkillChoice    *Choice
	LanguageChoice *Choice
	ToolChoice     *Choice
	AbilityChoice  *Choice // For Half-Elf's +1 to two abilities

	// Subraces if applicable
	Subraces map[Subrace]*SubraceData
}

// SubraceData contains additional modifications for a subrace
type SubraceData struct {
	AbilityIncreases map[abilities.Ability]int
	Traits           []Trait
	Skills           []skills.Skill
	Weapons          []proficiencies.Weapon
	Armor            []proficiencies.Armor
}

// Trait represents a racial trait or feature
type Trait struct {
	ID          string
	Name        string
	Description string
}

// Choice represents a choice the player must make
type Choice struct {
	Type        string   // "skill", "language", "tool", "ability"
	Count       int      // Number to choose
	Options     []string // Available options (empty means "any")
	Description string
}

// RaceData is the lookup map for all race data
var RaceData = map[Race]*Data{
	Human: {
		ID:    Human,
		Speed: 30,
		Size:  "Medium",
		AbilityIncreases: map[abilities.Ability]int{
			abilities.STR: 1,
			abilities.DEX: 1,
			abilities.CON: 1,
			abilities.INT: 1,
			abilities.WIS: 1,
			abilities.CHA: 1,
		},
		Languages: []languages.Language{
			languages.Common,
		},
		LanguageChoice: &Choice{
			Type:        "language",
			Count:       1,
			Options:     []string{}, // Empty means "any"
			Description: "You can speak, read, and write one extra language of your choice",
		},
	},

	Dwarf: {
		ID:    Dwarf,
		Speed: 25,
		Size:  "Medium",
		AbilityIncreases: map[abilities.Ability]int{
			abilities.CON: 2,
		},
		Traits: []Trait{
			{
				ID:          "darkvision",
				Name:        "Darkvision",
				Description: "You can see in dim light within 60 feet as if it were bright light",
			},
			{
				ID:          "dwarven-resilience",
				Name:        "Dwarven Resilience",
				Description: "You have advantage on saving throws against poison, and resistance to poison damage",
			},
			{
				ID:          "stonecunning",
				Name:        "Stonecunning",
				Description: "Whenever you make a History check related to stonework, add double your proficiency bonus",
			},
		},
		Weapons: []proficiencies.Weapon{
			// TODO: Add specific weapon constants
			// proficiencies.WeaponBattleaxe,
			// proficiencies.WeaponHandaxe,
			// proficiencies.WeaponLightHammer,
			// proficiencies.WeaponWarhammer,
		},
		Languages: []languages.Language{
			languages.Common,
			languages.Dwarvish,
		},
		ToolChoice: &Choice{
			Type:        "tool",
			Count:       1,
			Options:     []string{"smith's tools", "brewer's supplies", "mason's tools"},
			Description: "You gain proficiency with one of these artisan's tools",
		},
		Subraces: map[Subrace]*SubraceData{
			HillDwarf: {
				AbilityIncreases: map[abilities.Ability]int{
					abilities.WIS: 1,
				},
				Traits: []Trait{
					{
						ID:          "dwarven-toughness",
						Name:        "Dwarven Toughness",
						Description: "Your hit point maximum increases by 1, and increases by 1 every time you gain a level",
					},
				},
			},
			MountainDwarf: {
				AbilityIncreases: map[abilities.Ability]int{
					abilities.STR: 2,
				},
				Armor: []proficiencies.Armor{
					proficiencies.ArmorLight,
					proficiencies.ArmorMedium,
				},
			},
		},
	},

	Elf: {
		ID:    Elf,
		Speed: 30,
		Size:  "Medium",
		AbilityIncreases: map[abilities.Ability]int{
			abilities.DEX: 2,
		},
		Traits: []Trait{
			{
				ID:          "darkvision",
				Name:        "Darkvision",
				Description: "You can see in dim light within 60 feet as if it were bright light",
			},
			{
				ID:          "keen-senses",
				Name:        "Keen Senses",
				Description: "You have proficiency in the Perception skill",
			},
			{
				ID:          "fey-ancestry",
				Name:        "Fey Ancestry",
				Description: "You have advantage on saving throws against being charmed, and magic can't put you to sleep",
			},
			{
				ID:          "trance",
				Name:        "Trance",
				Description: "Elves don't need to sleep. Instead, they meditate deeply for 4 hours a day",
			},
		},
		Skills: []skills.Skill{
			skills.Perception,
		},
		Languages: []languages.Language{
			languages.Common,
			languages.Elvish,
		},
		Subraces: map[Subrace]*SubraceData{
			HighElf: {
				AbilityIncreases: map[abilities.Ability]int{
					abilities.INT: 1,
				},
				Weapons: []proficiencies.Weapon{
					proficiencies.WeaponLongsword,
					proficiencies.WeaponShortsword,
					proficiencies.WeaponLongbow,
					proficiencies.WeaponShortbow,
				},
				// TODO: Add cantrip choice
			},
			WoodElf: {
				AbilityIncreases: map[abilities.Ability]int{
					abilities.WIS: 1,
				},
				Weapons: []proficiencies.Weapon{
					proficiencies.WeaponLongsword,
					proficiencies.WeaponShortsword,
					proficiencies.WeaponLongbow,
					proficiencies.WeaponShortbow,
				},
				Traits: []Trait{
					{
						ID:          "fleet-of-foot",
						Name:        "Fleet of Foot",
						Description: "Your base walking speed increases to 35 feet",
					},
					{
						ID:          "mask-of-the-wild",
						Name:        "Mask of the Wild",
						Description: "You can attempt to hide even when lightly obscured by natural phenomena",
					},
				},
			},
		},
	},

	Halfling: {
		ID:    Halfling,
		Speed: 25,
		Size:  "Small",
		AbilityIncreases: map[abilities.Ability]int{
			abilities.DEX: 2,
		},
		Traits: []Trait{
			{
				ID:          "lucky",
				Name:        "Lucky",
				Description: "When you roll a 1 on an attack roll, ability check, or saving throw, you can reroll the die",
			},
			{
				ID:          "brave",
				Name:        "Brave",
				Description: "You have advantage on saving throws against being frightened",
			},
			{
				ID:          "halfling-nimbleness",
				Name:        "Halfling Nimbleness",
				Description: "You can move through the space of any creature that is of a size larger than yours",
			},
		},
		Languages: []languages.Language{
			languages.Common,
			languages.Halfling,
		},
		Subraces: map[Subrace]*SubraceData{
			LightfootHalfling: {
				AbilityIncreases: map[abilities.Ability]int{
					abilities.CHA: 1,
				},
				Traits: []Trait{
					{
						ID:          "naturally-stealthy",
						Name:        "Naturally Stealthy",
						Description: "You can hide when obscured only by a creature at least one size larger than you",
					},
				},
			},
			StoutHalfling: {
				AbilityIncreases: map[abilities.Ability]int{
					abilities.CON: 1,
				},
				Traits: []Trait{
					{
						ID:          "stout-resilience",
						Name:        "Stout Resilience",
						Description: "You have advantage on saving throws against poison, and resistance to poison damage",
					},
				},
			},
		},
	},

	Dragonborn: {
		ID:    Dragonborn,
		Speed: 30,
		Size:  "Medium",
		AbilityIncreases: map[abilities.Ability]int{
			abilities.STR: 2,
			abilities.CHA: 1,
		},
		Traits: []Trait{
			{
				ID:          "draconic-ancestry",
				Name:        "Draconic Ancestry",
				Description: "You have draconic ancestry. Choose one type of dragon from the Draconic Ancestry table",
			},
			{
				ID:          "breath-weapon",
				Name:        "Breath Weapon",
				Description: "You can use your action to exhale destructive energy (2d6 damage, increases with level)",
			},
			{
				ID:          "damage-resistance",
				Name:        "Damage Resistance",
				Description: "You have resistance to the damage type associated with your draconic ancestry",
			},
		},
		Languages: []languages.Language{
			languages.Common,
			languages.Draconic,
		},
	},

	Gnome: {
		ID:    Gnome,
		Speed: 25,
		Size:  "Small",
		AbilityIncreases: map[abilities.Ability]int{
			abilities.INT: 2,
		},
		Traits: []Trait{
			{
				ID:          "darkvision",
				Name:        "Darkvision",
				Description: "You can see in dim light within 60 feet as if it were bright light",
			},
			{
				ID:          "gnome-cunning",
				Name:        "Gnome Cunning",
				Description: "You have advantage on all Intelligence, Wisdom, and Charisma saving throws against magic",
			},
		},
		Languages: []languages.Language{
			languages.Common,
			languages.Gnomish,
		},
		Subraces: map[Subrace]*SubraceData{
			ForestGnome: {
				AbilityIncreases: map[abilities.Ability]int{
					abilities.DEX: 1,
				},
				Traits: []Trait{
					{
						ID:          "natural-illusionist",
						Name:        "Natural Illusionist",
						Description: "You know the minor illusion cantrip",
					},
					{
						ID:          "speak-with-small-beasts",
						Name:        "Speak with Small Beasts",
						Description: "You can communicate simple ideas with Small or smaller beasts",
					},
				},
			},
			RockGnome: {
				AbilityIncreases: map[abilities.Ability]int{
					abilities.CON: 1,
				},
				Traits: []Trait{
					{
						ID:          "artificers-lore",
						Name:        "Artificer's Lore",
						Description: "Add twice your proficiency bonus to History checks related to magic items, alchemical objects, or technological devices", //nolint:lll
					},
					{
						ID:          "tinker",
						Name:        "Tinker",
						Description: "You have proficiency with artisan's tools (tinker's tools) and can create small devices",
					},
				},
			},
		},
	},

	HalfElf: {
		ID:    HalfElf,
		Speed: 30,
		Size:  "Medium",
		AbilityIncreases: map[abilities.Ability]int{
			abilities.CHA: 2,
		},
		AbilityChoice: &Choice{
			Type:        "ability",
			Count:       2,
			Options:     []string{}, // Will need special handling - any except CHA
			Description: "Increase two different ability scores of your choice by 1",
		},
		Traits: []Trait{
			{
				ID:          "darkvision",
				Name:        "Darkvision",
				Description: "You can see in dim light within 60 feet as if it were bright light",
			},
			{
				ID:          "fey-ancestry",
				Name:        "Fey Ancestry",
				Description: "You have advantage on saving throws against being charmed, and magic can't put you to sleep",
			},
		},
		Languages: []languages.Language{
			languages.Common,
			languages.Elvish,
		},
		LanguageChoice: &Choice{
			Type:        "language",
			Count:       1,
			Options:     []string{},
			Description: "You can speak, read, and write one extra language of your choice",
		},
		SkillChoice: &Choice{
			Type:        "skill",
			Count:       2,
			Options:     []string{}, // Any skills
			Description: "You gain proficiency in two skills of your choice",
		},
	},

	HalfOrc: {
		ID:    HalfOrc,
		Speed: 30,
		Size:  "Medium",
		AbilityIncreases: map[abilities.Ability]int{
			abilities.STR: 2,
			abilities.CON: 1,
		},
		Traits: []Trait{
			{
				ID:          "darkvision",
				Name:        "Darkvision",
				Description: "You can see in dim light within 60 feet as if it were bright light",
			},
			{
				ID:          "menacing",
				Name:        "Menacing",
				Description: "You gain proficiency in the Intimidation skill",
			},
			{
				ID:          "relentless-endurance",
				Name:        "Relentless Endurance",
				Description: "When reduced to 0 hit points but not killed, you can drop to 1 hit point instead (once per long rest)", //nolint:lll
			},
			{
				ID:          "savage-attacks",
				Name:        "Savage Attacks",
				Description: "When you score a critical hit with a melee weapon, roll one of the weapon's damage dice one additional time", //nolint:lll
			},
		},
		Skills: []skills.Skill{
			skills.Intimidation,
		},
		Languages: []languages.Language{
			languages.Common,
			languages.Orc,
		},
	},

	Tiefling: {
		ID:    Tiefling,
		Speed: 30,
		Size:  "Medium",
		AbilityIncreases: map[abilities.Ability]int{
			abilities.INT: 1,
			abilities.CHA: 2,
		},
		Traits: []Trait{
			{
				ID:          "darkvision",
				Name:        "Darkvision",
				Description: "You can see in dim light within 60 feet as if it were bright light",
			},
			{
				ID:          "hellish-resistance",
				Name:        "Hellish Resistance",
				Description: "You have resistance to fire damage",
			},
			{
				ID:          "infernal-legacy",
				Name:        "Infernal Legacy",
				Description: "You know thaumaturgy cantrip. At 3rd level, cast hellish rebuke once per day. At 5th level, cast darkness once per day", //nolint:lll
			},
		},
		Languages: []languages.Language{
			languages.Common,
			languages.Infernal,
		},
	},
}

// GetData returns the race data for a given race ID
func GetData(raceID Race) *Data {
	return RaceData[raceID]
}

// GetSubraceData returns the subrace data for a given race and subrace
func GetSubraceData(raceID Race, subraceID Subrace) *SubraceData {
	raceData := RaceData[raceID]
	if raceData == nil {
		return nil
	}
	return raceData.Subraces[subraceID]
}

// Name returns the display name of the race
func (d *Data) Name() string {
	return Name(d.ID)
}

// Description returns the description of the race
func (d *Data) Description() string {
	return Description(d.ID)
}
