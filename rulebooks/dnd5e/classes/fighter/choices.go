// Package fighter provides fighter class-specific choices and features
package fighter

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/bundles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// SkillChoices returns the fighter's skill proficiency choices
func SkillChoices() choices.Choice {
	return choices.Choice{
		ID:          choices.FighterSkills,
		Category:    choices.CategorySkill,
		Description: "Choose two skills from Acrobatics, Animal Handling, Athletics, History, Insight, Intimidation, Perception, and Survival",
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

// EquipmentChoices returns the fighter's starting equipment choices
func EquipmentChoices() []choices.Choice {
	return []choices.Choice{
		// Choice 1: Chain mail or leather armor, longbow, and 20 arrows
		{
			ID:          choices.FighterEquipment1,
			Category:    choices.CategoryEquipment,
			Description: "Choose your armor",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{ItemID: string(armor.ChainMail)},
				choices.BundleOption{
					ID: "leather-and-bow",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeArmor, ItemID: string(armor.Leather), Quantity: 1},
						{ItemType: choices.ItemTypeWeapon, ItemID: "longbow", Quantity: 1},
						{ItemType: choices.ItemTypeGear, ItemID: "arrow", Quantity: 20},
					},
				},
			},
		},
		// Choice 2: Martial weapon and shield or two martial weapons
		{
			ID:          choices.FighterEquipment2,
			Category:    choices.CategoryEquipment,
			Description: "Choose your primary weapons",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.BundleOption{
					ID: "weapon-and-shield",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeWeapon, ItemID: "longsword", Quantity: 1},
						{ItemType: choices.ItemTypeArmor, ItemID: string(armor.Shield), Quantity: 1},
					},
				},
				choices.BundleOption{
					ID: "two-weapons",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeWeapon, ItemID: "longsword", Quantity: 1},
						{ItemType: choices.ItemTypeWeapon, ItemID: "shortsword", Quantity: 1},
					},
				},
			},
		},
		// Choice 3: Light crossbow and 20 bolts or two handaxes
		{
			ID:          choices.FighterEquipment3,
			Category:    choices.CategoryEquipment,
			Description: "Choose your secondary weapons",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.BundleOption{
					ID: "crossbow-and-bolts",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeWeapon, ItemID: "light-crossbow", Quantity: 1},
						{ItemType: choices.ItemTypeGear, ItemID: "bolt", Quantity: 20},
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
		// Choice 4: Dungeoneer's pack or explorer's pack
		{
			ID:          choices.FighterEquipment5,
			Category:    choices.CategoryEquipment,
			Description: "Choose an equipment pack",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{ItemID: string(bundles.DungeoneersPack)},
				choices.SingleOption{ItemID: string(bundles.ExplorersPack)},
			},
		},
	}
}

// AllChoices returns all fighter choices
func AllChoices() []choices.Choice {
	choices := []choices.Choice{
		SkillChoices(),
	}
	choices = append(choices, EquipmentChoices()...)
	return choices
}
