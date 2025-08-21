package fighter_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/bundles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes/fighter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFighterWeaponProficiencies(t *testing.T) {
	profs := fighter.WeaponProficiencies()

	// Fighter should be proficient with all our test weapons
	profMap := make(map[string]bool)
	for _, p := range profs {
		profMap[p] = true
	}

	// Check simple weapons
	assert.True(t, profMap["club"])
	assert.True(t, profMap["dagger"])
	assert.True(t, profMap["handaxe"])
	assert.True(t, profMap["javelin"])

	// Check martial weapons
	assert.True(t, profMap["longsword"])
	assert.True(t, profMap["greatsword"])
	assert.True(t, profMap["rapier"])
	assert.True(t, profMap["shortsword"])

	// Check ranged weapons
	assert.True(t, profMap["shortbow"])
	assert.True(t, profMap["longbow"])
	assert.True(t, profMap["light-crossbow"])
	assert.True(t, profMap["heavy-crossbow"])

	// Should have at least our test weapons (12)
	assert.GreaterOrEqual(t, len(profs), 12)
}

func TestFighterStartingEquipmentChoice1(t *testing.T) {
	choice := fighter.StartingEquipmentChoice1()

	assert.Equal(t, choices.FighterEquipment1, choice.ID)
	assert.Equal(t, choices.CategoryEquipment, choice.Category)
	assert.Equal(t, 1, choice.Choose)
	assert.Equal(t, choices.SourceClass, choice.Source)

	require.Len(t, choice.Options, 2)

	// First option should be chain mail
	opt1, ok := choice.Options[0].(choices.SingleOption)
	require.True(t, ok)
	assert.Equal(t, choices.ItemTypeArmor, opt1.ItemType)
	assert.Equal(t, string(armor.ChainMail), opt1.ItemID)

	// Second option should be leather armor + longbow bundle
	opt2, ok := choice.Options[1].(choices.BundleOption)
	require.True(t, ok)
	assert.Equal(t, string(bundles.LeatherArmorLongbow), opt2.ID)
	assert.Len(t, opt2.Items, 3)

	// Check bundle contents
	assert.Equal(t, string(armor.Leather), opt2.Items[0].ItemID)
	assert.Equal(t, "longbow", opt2.Items[1].ItemID)
	assert.Equal(t, "arrow", opt2.Items[2].ItemID)
	assert.Equal(t, 20, opt2.Items[2].Quantity)
}

func TestFighterStartingEquipmentChoice2(t *testing.T) {
	choice := fighter.StartingEquipmentChoice2()

	assert.Equal(t, choices.FighterEquipment2, choice.ID)
	assert.Equal(t, choices.CategoryEquipment, choice.Category)
	assert.Equal(t, 1, choice.Choose)
	assert.Equal(t, choices.SourceClass, choice.Source)

	require.Len(t, choice.Options, 2)

	// Both options should be bundles
	opt1, ok := choice.Options[0].(choices.BundleOption)
	require.True(t, ok)
	assert.Equal(t, string(bundles.MartialWeaponAndShield), opt1.ID)
	assert.Len(t, opt1.Items, 2)

	opt2, ok := choice.Options[1].(choices.BundleOption)
	require.True(t, ok)
	assert.Equal(t, string(bundles.TwoMartialWeapons), opt2.ID)
	assert.Len(t, opt2.Items, 2)
}

func TestMartialWeaponChoice(t *testing.T) {
	choice := fighter.MartialWeaponChoice()

	assert.Equal(t, choices.ChoiceID("martial-weapon-choice"), choice.ID)
	assert.Equal(t, choices.CategoryEquipment, choice.Category)
	assert.Equal(t, 1, choice.Choose)
	assert.Equal(t, choices.SourceClass, choice.Source)

	require.Len(t, choice.Options, 2)

	// Should have martial melee and martial ranged options
	opt1, ok := choice.Options[0].(choices.WeaponCategoryOption)
	require.True(t, ok)
	assert.Equal(t, weapons.CategoryMartialMelee, opt1.Category)

	opt2, ok := choice.Options[1].(choices.WeaponCategoryOption)
	require.True(t, ok)
	assert.Equal(t, weapons.CategoryMartialRanged, opt2.Category)
}

func TestSimpleWeaponChoice(t *testing.T) {
	choice := fighter.SimpleWeaponChoice()

	assert.Equal(t, choices.ChoiceID("simple-weapon-choice"), choice.ID)
	assert.Equal(t, choices.CategoryEquipment, choice.Category)
	assert.Equal(t, 1, choice.Choose)
	assert.Equal(t, choices.SourceClass, choice.Source)

	require.Len(t, choice.Options, 2)

	// Should have simple melee and simple ranged options
	opt1, ok := choice.Options[0].(choices.WeaponCategoryOption)
	require.True(t, ok)
	assert.Equal(t, weapons.CategorySimpleMelee, opt1.Category)

	opt2, ok := choice.Options[1].(choices.WeaponCategoryOption)
	require.True(t, ok)
	assert.Equal(t, weapons.CategorySimpleRanged, opt2.Category)
}
