package fighter_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class/fighter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type FighterTestSuite struct {
	suite.Suite
}

func TestFighterSuite(t *testing.T) {
	suite.Run(t, new(FighterTestSuite))
}

func (s *FighterTestSuite) TestFighterBasics() {
	f := fighter.Get()
	
	s.Run("has correct class ID", func() {
		s.Equal(classes.Fighter, f.ID)
		s.Equal("Fighter", f.Name)
	})
	
	s.Run("has correct hit dice and HP", func() {
		s.Equal(10, f.HitDice)
		s.Equal(6, f.HitPointsPerLevel)
	})
	
	s.Run("has correct proficiencies", func() {
		// Armor
		s.Contains(f.ArmorProficiencies, "light")
		s.Contains(f.ArmorProficiencies, "medium")
		s.Contains(f.ArmorProficiencies, "heavy")
		s.Contains(f.ArmorProficiencies, "shields")
		
		// Weapons
		s.Contains(f.WeaponProficiencies, "simple")
		s.Contains(f.WeaponProficiencies, "martial")
		
		// Saving throws
		s.Len(f.SavingThrows, 2)
		s.Contains(f.SavingThrows, abilities.STR)
		s.Contains(f.SavingThrows, abilities.CON)
	})
	
	s.Run("has correct skill options", func() {
		s.Equal(2, f.SkillProficiencyCount)
		s.Len(f.SkillOptions, 8)
		s.Contains(f.SkillOptions, skills.Athletics)
		s.Contains(f.SkillOptions, skills.Intimidation)
		s.Contains(f.SkillOptions, skills.Survival)
	})
	
	s.Run("has subclass at level 3", func() {
		s.Equal(3, f.SubclassLevel)
	})
}

func (s *FighterTestSuite) TestFighterEquipmentChoices() {
	f := fighter.Get()
	
	s.Run("has 4 equipment choices", func() {
		s.Len(f.EquipmentChoices, 4)
	})
	
	s.Run("armor choice", func() {
		armorChoice := f.EquipmentChoices[0]
		s.Equal("fighter-armor-choice", armorChoice.ID)
		s.Equal(1, armorChoice.Choose)
		s.Len(armorChoice.Options, 2)
		
		// Option 1: Chain mail
		chainMail := armorChoice.Options[0]
		s.Equal("chain-mail", chainMail.ID)
		s.Len(chainMail.Items, 1)
		s.Equal("chain-mail", chainMail.Items[0].ItemID)
		
		// Option 2: Leather + longbow + arrows
		leatherBundle := armorChoice.Options[1]
		s.Equal("leather-longbow", leatherBundle.ID)
		s.Len(leatherBundle.Items, 3)
		s.Equal("leather", leatherBundle.Items[0].ItemID)
		s.Equal("longbow", leatherBundle.Items[1].ItemID)
		s.Equal("arrow", leatherBundle.Items[2].ItemID)
		s.Equal(20, leatherBundle.Items[2].Quantity)
	})
	
	s.Run("weapon choice", func() {
		weaponChoice := f.EquipmentChoices[1]
		s.Equal("fighter-weapon-choice", weaponChoice.ID)
		s.Equal(1, weaponChoice.Choose)
		s.Len(weaponChoice.Options, 2)
		
		// These are bundles that will expand later
		s.Equal("martial-weapon-and-shield", weaponChoice.Options[0].ID)
		s.Equal("two-martial-weapons", weaponChoice.Options[1].ID)
	})
	
	s.Run("ranged choice", func() {
		rangedChoice := f.EquipmentChoices[2]
		s.Equal("fighter-ranged-choice", rangedChoice.ID)
		s.Len(rangedChoice.Options, 2)
		
		// Crossbow option
		crossbow := rangedChoice.Options[0]
		s.Equal("crossbow-bolts", crossbow.ID)
		s.Equal("light-crossbow", crossbow.Items[0].ItemID)
		s.Equal("crossbow-bolt", crossbow.Items[1].ItemID)
		s.Equal(20, crossbow.Items[1].Quantity)
		
		// Handaxe option
		handaxes := rangedChoice.Options[1]
		s.Equal("two-handaxes", handaxes.ID)
		s.Equal("handaxe", handaxes.Items[0].ItemID)
		s.Equal(2, handaxes.Items[0].Quantity)
	})
	
	s.Run("pack choice", func() {
		packChoice := f.EquipmentChoices[3]
		s.Equal("fighter-pack-choice", packChoice.ID)
		s.Len(packChoice.Options, 2)
		s.Equal("dungeoneers-pack", packChoice.Options[0].ID)
		s.Equal("explorers-pack", packChoice.Options[1].ID)
	})
}

func (s *FighterTestSuite) TestFighterEquipmentAsChoices() {
	equipChoices := fighter.GetEquipmentChoicesAsChoices()
	
	s.Run("has 4 choices in choice format", func() {
		s.Len(equipChoices, 4)
	})
	
	s.Run("armor choice uses proper Option types", func() {
		armorChoice := equipChoices[0]
		s.Equal(choices.ChoiceID("fighter-armor-choice"), armorChoice.ID)
		s.Equal(choices.CategoryEquipment, armorChoice.Category)
		s.Equal(choices.SourceClass, armorChoice.Source)
		s.Len(armorChoice.Options, 2)
		
		// Chain mail is a single option
		chainMail, ok := armorChoice.Options[0].(choices.SingleOption)
		s.True(ok)
		s.Equal("chain-mail", chainMail.ItemID)
		s.Equal(choices.ItemTypeArmor, chainMail.ItemType)
		
		// Leather bundle
		leatherBundle, ok := armorChoice.Options[1].(choices.BundleOption)
		s.True(ok)
		s.Equal("leather-longbow", leatherBundle.ID)
		s.Len(leatherBundle.Items, 3)
	})
	
	s.Run("weapon choice uses bundles", func() {
		weaponChoice := equipChoices[1]
		s.Equal(choices.ChoiceID("fighter-weapon-choice"), weaponChoice.ID)
		
		// Both options are bundles
		bundle1, ok := weaponChoice.Options[0].(choices.BundleOption)
		s.True(ok)
		s.Equal("martial-weapon-and-shield", bundle1.ID)
		
		bundle2, ok := weaponChoice.Options[1].(choices.BundleOption)
		s.True(ok)
		s.Equal("two-martial-weapons", bundle2.ID)
	})
}

func (s *FighterTestSuite) TestFighterSkillChoice() {
	skillChoice := fighter.GetSkillChoices()
	
	s.Run("skill choice is properly structured", func() {
		s.Equal(choices.ChoiceID("fighter-skills"), skillChoice.ID)
		s.Equal(choices.CategorySkill, skillChoice.Category)
		s.Equal(2, skillChoice.Choose)
		s.Equal(choices.SourceClass, skillChoice.Source)
		
		// Should have one option that's a skill list
		s.Len(skillChoice.Options, 1)
		skillList, ok := skillChoice.Options[0].(choices.SkillListOption)
		s.True(ok)
		s.Len(skillList.Skills, 8)
		s.Contains(skillList.Skills, skills.Athletics)
		s.Contains(skillList.Skills, skills.Survival)
	})
}

func (s *FighterTestSuite) TestFighterMartialWeaponChoice() {
	weaponChoice := fighter.GetMartialWeaponChoice()
	
	s.Run("martial weapon choice expands to categories", func() {
		s.Equal(choices.ChoiceID("fighter-martial-weapon"), weaponChoice.ID)
		s.Equal(choices.CategoryEquipment, weaponChoice.Category)
		s.Equal(1, weaponChoice.Choose)
		s.Len(weaponChoice.Options, 2)
		
		// Should have melee and ranged categories
		melee, ok := weaponChoice.Options[0].(choices.WeaponCategoryOption)
		s.True(ok)
		s.Equal("martial-melee", string(melee.Category))
		
		ranged, ok := weaponChoice.Options[1].(choices.WeaponCategoryOption)
		s.True(ok)
		s.Equal("martial-ranged", string(ranged.Category))
	})
}

func (s *FighterTestSuite) TestFighterFeatures() {
	features := fighter.GetFeatures()
	
	s.Run("has level 1 features", func() {
		level1 := features[1]
		s.Len(level1, 2)
		
		// Fighting Style
		fightingStyle := level1[0]
		s.Equal("fighting-style", fightingStyle.ID)
		s.Equal("Fighting Style", fightingStyle.Name)
		s.NotNil(fightingStyle.Choice)
		s.Contains(fightingStyle.Choice.From, "archery")
		s.Contains(fightingStyle.Choice.From, "defense")
		
		// Second Wind
		secondWind := level1[1]
		s.Equal("second-wind", secondWind.ID)
		s.Equal("Second Wind", secondWind.Name)
	})
	
	s.Run("has action surge at level 2", func() {
		level2 := features[2]
		s.Len(level2, 1)
		s.Equal("action-surge", level2[0].ID)
	})
	
	s.Run("has extra attack at level 5", func() {
		level5 := features[5]
		s.Len(level5, 1)
		s.Equal("extra-attack", level5[0].ID)
	})
	
	s.Run("has indomitable at level 9", func() {
		level9 := features[9]
		s.Len(level9, 1)
		s.Equal("indomitable", level9[0].ID)
	})
}

func (s *FighterTestSuite) TestFighterResources() {
	resources := fighter.GetResources()
	
	s.Run("has 3 resource types", func() {
		s.Len(resources, 3)
	})
	
	s.Run("second wind resource", func() {
		var secondWind *class.ResourceData
		for i := range resources {
			if resources[i].Name == "Second Wind" {
				secondWind = &resources[i]
				break
			}
		}
		require.NotNil(s.T(), secondWind)
		s.Equal("1", secondWind.MaxFormula)
		s.Equal(shared.ResetTypeShortRest, secondWind.Resets)
	})
	
	s.Run("action surge resource", func() {
		var actionSurge *class.ResourceData
		for i := range resources {
			if resources[i].Name == "Action Surge" {
				actionSurge = &resources[i]
				break
			}
		}
		require.NotNil(s.T(), actionSurge)
		s.Equal(shared.ResetTypeShortRest, actionSurge.Resets)
		s.Equal(1, actionSurge.UsesPerLevel[1])
		s.Equal(2, actionSurge.UsesPerLevel[17])
	})
	
	s.Run("indomitable resource", func() {
		var indomitable *class.ResourceData
		for i := range resources {
			if resources[i].Name == "Indomitable" {
				indomitable = &resources[i]
				break
			}
		}
		require.NotNil(s.T(), indomitable)
		s.Equal(shared.ResetTypeLongRest, indomitable.Resets)
		s.Equal(1, indomitable.UsesPerLevel[9])
		s.Equal(2, indomitable.UsesPerLevel[13])
		s.Equal(3, indomitable.UsesPerLevel[17])
	})
}