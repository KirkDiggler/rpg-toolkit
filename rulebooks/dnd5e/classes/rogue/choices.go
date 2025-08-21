// Package rogue provides rogue class-specific choices and features
package rogue

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/bundles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// SkillChoices returns the rogue's skill proficiency choices
func SkillChoices() choices.Choice {
	return choices.Choice{
		ID:       choices.RogueSkills,
		Category: choices.CategorySkill,
		Description: "Choose four skills from Acrobatics, Athletics, Deception, Insight, Intimidation, " +
			"Investigation, Perception, Performance, Persuasion, Sleight of Hand, and Stealth",
		Choose: 4, // Rogues get more skills!
		Source: choices.SourceClass,
		Options: []choices.Option{
			choices.SkillListOption{
				Skills: []skills.Skill{
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
			},
		},
	}
}

// EquipmentChoices returns the rogue's starting equipment choices
func EquipmentChoices() []choices.Choice {
	return []choices.Choice{
		// Choice 1: Rapier or shortsword
		{
			ID:          choices.RogueEquipment1,
			Category:    choices.CategoryEquipment,
			Description: "Choose your primary weapon",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{ItemID: "rapier"},
				choices.SingleOption{ItemID: "shortsword"},
			},
		},
		// Choice 2: Shortbow and quiver of 20 arrows or shortsword
		{
			ID:          choices.RogueEquipment2,
			Category:    choices.CategoryEquipment,
			Description: "Choose your secondary weapon",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.BundleOption{
					ID: "shortbow-and-arrows",
					Items: []choices.CountedItem{
						{ItemType: choices.ItemTypeWeapon, ItemID: "shortbow", Quantity: 1},
						{ItemType: choices.ItemTypeGear, ItemID: "arrow", Quantity: 20},
					},
				},
				choices.SingleOption{ItemID: "shortsword"},
			},
		},
		// Choice 3: Burglar's pack, dungeoneer's pack, or explorer's pack
		{
			ID:          choices.RogueEquipment3,
			Category:    choices.CategoryEquipment,
			Description: "Choose an equipment pack",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{ItemID: string(bundles.BurglarsPack)},
				choices.SingleOption{ItemID: string(bundles.DungeoneersPack)},
				choices.SingleOption{ItemID: string(bundles.ExplorersPack)},
			},
		},
		// Note: Rogues also get leather armor, two daggers, and thieves' tools automatically
	}
}

// AllChoices returns all rogue choices
func AllChoices() []choices.Choice {
	choices := []choices.Choice{
		SkillChoices(),
	}
	choices = append(choices, EquipmentChoices()...)
	return choices
}
