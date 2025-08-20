package fighter

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/bundles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// WeaponProficiencies returns all weapons a fighter is proficient with
func WeaponProficiencies() []string {
	// Fighters are proficient with all simple and martial weapons
	var profs []string

	for id := range weapons.SimpleMeleeWeapons {
		profs = append(profs, id)
	}
	for id := range weapons.MartialMeleeWeapons {
		profs = append(profs, id)
	}
	for id := range weapons.SimpleRangedWeapons {
		profs = append(profs, id)
	}
	for id := range weapons.MartialRangedWeapons {
		profs = append(profs, id)
	}

	return profs
}

// StartingEquipmentChoice1 returns the fighter's first equipment choice
// (a) chain mail or (b) leather armor, longbow, and 20 arrows
func StartingEquipmentChoice1() choices.Choice {
	return choices.Choice{
		ID:          choices.FighterEquipment1,
		Category:    choices.CategoryEquipment,
		Description: "Choose your starting armor",
		Choose:      1,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.SingleOption{
				ItemType: choices.ItemTypeArmor,
				ItemID:   string(armor.ChainMail),
			},
			choices.BundleOption{
				ID: string(bundles.LeatherArmorLongbow),
				Items: []choices.CountedItem{
					{ItemType: choices.ItemTypeArmor, ItemID: string(armor.Leather), Quantity: 1},
					{ItemType: choices.ItemTypeWeapon, ItemID: "longbow", Quantity: 1},
					{ItemType: choices.ItemTypeGear, ItemID: "arrow", Quantity: 20},
				},
			},
		},
	}
}

// StartingEquipmentChoice2 returns the fighter's weapon choice
// (a) a martial weapon and a shield or (b) two martial weapons
func StartingEquipmentChoice2() choices.Choice {
	return choices.Choice{
		ID:          choices.FighterEquipment2,
		Category:    choices.CategoryEquipment,
		Description: "Choose your starting weapons",
		Choose:      1,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.BundleOption{
				ID: string(bundles.MartialWeaponAndShield),
				Items: []choices.CountedItem{
					{ItemType: choices.ItemTypeWeapon, ItemID: "martial-weapon-choice", Quantity: 1},
					{ItemType: choices.ItemTypeArmor, ItemID: string(armor.Shield), Quantity: 1},
				},
			},
			choices.BundleOption{
				ID: string(bundles.TwoMartialWeapons),
				Items: []choices.CountedItem{
					{ItemType: choices.ItemTypeWeapon, ItemID: "martial-weapon-choice-1", Quantity: 1},
					{ItemType: choices.ItemTypeWeapon, ItemID: "martial-weapon-choice-2", Quantity: 1},
				},
			},
		},
	}
}

// MartialWeaponChoice returns a choice of martial weapon
func MartialWeaponChoice() choices.Choice {
	return choices.Choice{
		ID:          choices.ChoiceID("martial-weapon-choice"),
		Category:    choices.CategoryEquipment,
		Description: "Choose a martial weapon",
		Choose:      1,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			// Using WeaponCategoryOption to represent choosing from a category
			choices.WeaponCategoryOption{
				Category: weapons.CategoryMartialMelee,
			},
			choices.WeaponCategoryOption{
				Category: weapons.CategoryMartialRanged,
			},
		},
	}
}

// SimpleWeaponChoice returns a choice of simple weapon
func SimpleWeaponChoice() choices.Choice {
	return choices.Choice{
		ID:          choices.ChoiceID("simple-weapon-choice"),
		Category:    choices.CategoryEquipment,
		Description: "Choose a simple weapon",
		Choose:      1,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.WeaponCategoryOption{
				Category: weapons.CategorySimpleMelee,
			},
			choices.WeaponCategoryOption{
				Category: weapons.CategorySimpleRanged,
			},
		},
	}
}
