// Package fighter provides the D&D 5e Fighter class implementation
package fighter

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/bundles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Get returns the complete Fighter class data
func Get() *class.Data {
	return &class.Data{
		ID:          classes.Fighter,
		Name:        "Fighter",
		Description: "A master of martial combat, skilled with a variety of weapons and armor",

		// Core mechanics
		HitDice:           10,
		HitPointsPerLevel: 6, // 1d10 average (5.5) + 0.5 = 6

		// Proficiencies
		ArmorProficiencies:  []string{"light", "medium", "heavy", "shields"},
		WeaponProficiencies: []string{"simple", "martial"},
		ToolProficiencies:   []string{}, // None by default
		SavingThrows: []abilities.Ability{
			abilities.STR,
			abilities.CON,
		},

		// Skills - Choose 2 from the list
		SkillProficiencyCount: 2,
		SkillOptions: []skills.Skill{
			skills.Acrobatics,
			skills.AnimalHandling,
			skills.Athletics,
			skills.History,
			skills.Insight,
			skills.Intimidation,
			skills.Perception,
			skills.Survival,
		},

		// Starting equipment (guaranteed items)
		StartingEquipment: []class.EquipmentData{
			// All fighters get chain mail OR leather armor + longbow + 20 arrows
			// We'll handle this as a choice instead
		},

		// Equipment choices
		EquipmentChoices: GetEquipmentChoices(),

		// Class features by level
		Features: GetFeatures(),

		// No spellcasting
		Spellcasting: nil,

		// Fighter resources (Second Wind, Action Surge, etc.)
		Resources: GetResources(),

		// Martial Archetype at 3rd level
		SubclassLevel: 3,
		Subclasses:    []class.SubclassData{}, // TODO: Add Champion, Battle Master, Eldritch Knight
	}
}

// GetEquipmentChoices returns the Fighter's equipment choices
func GetEquipmentChoices() []class.EquipmentChoiceData {
	return []class.EquipmentChoiceData{
		// Choice 1: Chain mail OR leather armor + longbow + 20 arrows
		{
			ID:          "fighter-armor-choice",
			Description: "Choose your starting armor",
			Choose:      1,
			Options: []class.EquipmentOption{
				{
					ID: "chain-mail",
					Items: []class.EquipmentData{
						{ItemID: "chain-mail", Quantity: 1},
					},
				},
				{
					ID: "leather-longbow",
					Items: []class.EquipmentData{
						{ItemID: "leather", Quantity: 1},
						{ItemID: "longbow", Quantity: 1},
						{ItemID: "arrow", Quantity: 20},
					},
				},
			},
		},
		// Choice 2: Martial weapon and shield OR two martial weapons
		{
			ID:          "fighter-weapon-choice",
			Description: "Choose your weapons",
			Choose:      1,
			Options: []class.EquipmentOption{
				{
					ID: string(bundles.MartialWeaponAndShield),
					Items: []class.EquipmentData{
						// This will be expanded to let player choose ANY martial weapon
						{ItemID: "martial-weapon-and-shield", Quantity: 1},
					},
				},
				{
					ID: string(bundles.TwoMartialWeapons),
					Items: []class.EquipmentData{
						// This will be expanded to let player choose ANY 2 martial weapons
						{ItemID: "two-martial-weapons", Quantity: 1},
					},
				},
			},
		},
		// Choice 3: Light crossbow + 20 bolts OR two handaxes
		{
			ID:          "fighter-ranged-choice",
			Description: "Choose your ranged weapons",
			Choose:      1,
			Options: []class.EquipmentOption{
				{
					ID: "crossbow-bolts",
					Items: []class.EquipmentData{
						{ItemID: "light-crossbow", Quantity: 1},
						{ItemID: "crossbow-bolt", Quantity: 20},
					},
				},
				{
					ID: "two-handaxes",
					Items: []class.EquipmentData{
						{ItemID: "handaxe", Quantity: 2},
					},
				},
			},
		},
		// Choice 4: Dungeoneer's pack OR explorer's pack
		{
			ID:          "fighter-pack-choice",
			Description: "Choose your equipment pack",
			Choose:      1,
			Options: []class.EquipmentOption{
				{
					ID: string(bundles.DungeoneersPack),
					Items: []class.EquipmentData{
						{ItemID: "dungeoneers-pack", Quantity: 1},
					},
				},
				{
					ID: string(bundles.ExplorersPack),
					Items: []class.EquipmentData{
						{ItemID: "explorers-pack", Quantity: 1},
					},
				},
			},
		},
	}
}

// GetEquipmentChoicesAsChoices converts equipment choices to the choices.Choice format
// This is what the game server actually uses
func GetEquipmentChoicesAsChoices() []choices.Choice {
	return []choices.Choice{
		// Choice 1: Chain mail OR leather armor + longbow + 20 arrows
		{
			ID:          choices.ChoiceID("fighter-armor-choice"),
			Category:    choices.CategoryEquipment,
			Description: "Choose your starting armor",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{
					ItemType: choices.ItemTypeArmor,
					ItemID:   "chain-mail",
					Display:  "Chain mail",
				},
				choices.BundleOption{
					ID: "leather-longbow",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeArmor, ItemID: "leather", Quantity: 1},
						{ItemType: choices.ItemTypeWeapon, ItemID: "longbow", Quantity: 1},
						{ItemType: choices.ItemTypeGear, ItemID: "arrow", Quantity: 20},
					},
				},
			},
		},
		// Choice 2: Martial weapon and shield OR two martial weapons
		{
			ID:          choices.ChoiceID("fighter-weapon-choice"),
			Category:    choices.CategoryEquipment,
			Description: "Choose your weapons",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				// Martial weapon + shield (player chooses the weapon)
				choices.BundleOption{
					ID: string(bundles.MartialWeaponAndShield),
					Items: []choices.CountedItem{
						// This is a special bundle that expands to weapon choice + shield
						{ItemType: choices.ItemTypeWeapon, ItemID: "martial-weapon-choice", Quantity: 1},
						{ItemType: choices.ItemTypeArmor, ItemID: "shield", Quantity: 1},
					},
				},
				// Two martial weapons (player chooses both)
				choices.BundleOption{
					ID: string(bundles.TwoMartialWeapons),
					Items: []choices.CountedItem{
						// This is a special bundle that expands to 2 weapon choices
						{ItemType: choices.ItemTypeWeapon, ItemID: "martial-weapon-choice", Quantity: 2},
					},
				},
			},
		},
		// Choice 3: Light crossbow + 20 bolts OR two handaxes
		{
			ID:          choices.ChoiceID("fighter-ranged-choice"),
			Category:    choices.CategoryEquipment,
			Description: "Choose your ranged weapons",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.BundleOption{
					ID: "crossbow-bolts",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeWeapon, ItemID: "light-crossbow", Quantity: 1},
						{ItemType: choices.ItemTypeGear, ItemID: "crossbow-bolt", Quantity: 20},
					},
				},
				choices.BundleOption{
					ID: "two-handaxes",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeWeapon, ItemID: "handaxe", Quantity: 2},
					},
				},
			},
		},
		// Choice 4: Dungeoneer's pack OR explorer's pack
		{
			ID:          choices.ChoiceID("fighter-pack-choice"),
			Category:    choices.CategoryEquipment,
			Description: "Choose your equipment pack",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{
					ItemType: choices.ItemTypeGear,
					ItemID:   string(bundles.DungeoneersPack),
					Display:  "Dungeoneer's Pack",
				},
				choices.SingleOption{
					ItemType: choices.ItemTypeGear,
					ItemID:   string(bundles.ExplorersPack),
					Display:  "Explorer's Pack",
				},
			},
		},
	}
}

// GetMartialWeaponChoice returns a choice for selecting any martial weapon
// This is used when a bundle includes "choose a martial weapon"
func GetMartialWeaponChoice() choices.Choice {
	return choices.Choice{
		ID:          choices.ChoiceID("fighter-martial-weapon"),
		Category:    choices.CategoryEquipment,
		Description: "Choose a martial weapon",
		Choose:      1,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.WeaponCategoryOption{
				Category: weapons.CategoryMartialMelee,
			},
			choices.WeaponCategoryOption{
				Category: weapons.CategoryMartialRanged,
			},
		},
	}
}

// GetSkillChoices returns the Fighter's skill proficiency choices
func GetSkillChoices() choices.Choice {
	return choices.Choice{
		ID:          choices.ChoiceID("fighter-skills"),
		Category:    choices.CategorySkill,
		Description: "Choose two skills",
		Choose:      2,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.SkillListOption{
				Skills: []skills.Skill{
					skills.Acrobatics,
					skills.AnimalHandling,
					skills.Athletics,
					skills.History,
					skills.Insight,
					skills.Intimidation,
					skills.Perception,
					skills.Survival,
				},
			},
		},
	}
}