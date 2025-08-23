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
		// Choice 2: Light crossbow + 20 bolts OR two handaxes
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
		// Choice 3: Dungeoneer's pack OR explorer's pack
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

// GetEquipmentChoicesAsChoices returns Fighter equipment choices in the choices format.
// This provides the actual weapon category expansion that the game server uses.
// Following D&D 5e RAW: Fighter gets 4 equipment choices.
func GetEquipmentChoicesAsChoices() []choices.Choice {
	return []choices.Choice{
		// Choice 1: Chain mail OR leather armor + longbow + 20 arrows
		{
			ID:          choices.ChoiceID("fighter-armor-choice"),
			Category:    choices.CategoryEquipment,
			Description: "(a) chain mail or (b) leather armor, longbow, and 20 arrows",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{
					ItemType: choices.ItemTypeArmor,
					ItemID:   "chain-mail",
					Display:  "Chain mail",
				},
				choices.BundleOption{
					ID:      "leather-longbow",
					Display: "Leather armor, longbow, and 20 arrows",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeArmor, ItemID: "leather-armor", Quantity: 1},
						{ItemType: choices.ItemTypeWeapon, ItemID: "longbow", Quantity: 1},
						{ItemType: choices.ItemTypeGear, ItemID: "arrow", Quantity: 20},
					},
				},
			},
		},
		// Choice 2a: First martial weapon (or weapon + shield choice)
		// We split the complex choice into separate choices for simplicity
		{
			ID:          choices.ChoiceID("fighter-primary-weapon"),
			Category:    choices.CategoryEquipment,
			Description: "Choose your first martial weapon",
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
		},
		// Choice 2b: Shield OR second martial weapon
		// This gives the player the choice between defensive (shield) or offensive (second weapon)
		{
			ID:          choices.ChoiceID("fighter-secondary-equipment"),
			Category:    choices.CategoryEquipment,
			Description: "(a) a shield or (b) a second martial weapon",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{
					ItemType: choices.ItemTypeArmor,
					ItemID:   "shield",
					Display:  "Shield",
				},
				choices.WeaponCategoryOption{
					Category: weapons.CategoryMartialMelee,
				},
				choices.WeaponCategoryOption{
					Category: weapons.CategoryMartialRanged,
				},
			},
		},
		// Choice 3: Light crossbow + 20 bolts OR two handaxes
		{
			ID:          choices.ChoiceID("fighter-ranged-choice"),
			Category:    choices.CategoryEquipment,
			Description: "(a) a light crossbow and 20 bolts or (b) two handaxes",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.BundleOption{
					ID:      "crossbow-bolts",
					Display: "Light crossbow and 20 bolts",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeWeapon, ItemID: "crossbow-light", Quantity: 1},
						{ItemType: choices.ItemTypeGear, ItemID: "crossbow-bolt", Quantity: 20},
					},
				},
				choices.BundleOption{
					ID:      "two-handaxes",
					Display: "Two handaxes",
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
			Description: "(a) a dungeoneer's pack or (b) an explorer's pack",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{
					ItemType: choices.ItemTypeGear,
					ItemID:   "dungeoneers-pack",
					Display:  "Dungeoneer's Pack",
				},
				choices.SingleOption{
					ItemType: choices.ItemTypeGear,
					ItemID:   "explorers-pack",
					Display:  "Explorer's Pack",
				},
			},
		},
	}
}

// GetAllChoices returns all Fighter choices including equipment and skills
func GetAllChoices() []choices.Choice {
	choices := GetEquipmentChoicesAsChoices()
	choices = append(choices, GetSkillChoices())
	return choices
}

// GetWeaponSelectionChoices returns the actual weapon selection choices based on the configuration chosen
func GetWeaponSelectionChoices(weaponConfig string) []choices.Choice {
	switch weaponConfig {
	case "weapon-and-shield":
		// Choose one martial weapon, automatically get a shield
		return []choices.Choice{
			{
				ID:          choices.ChoiceID("fighter-primary-weapon"),
				Category:    choices.CategoryEquipment,
				Description: "Choose your martial weapon",
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
			},
		}
	case "two-weapons":
		// Choose two martial weapons
		return []choices.Choice{
			{
				ID:          choices.ChoiceID("fighter-first-weapon"),
				Category:    choices.CategoryEquipment,
				Description: "Choose your first martial weapon",
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
			},
			{
				ID:          choices.ChoiceID("fighter-second-weapon"),
				Category:    choices.CategoryEquipment,
				Description: "Choose your second martial weapon",
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
			},
		}
	default:
		return nil
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
