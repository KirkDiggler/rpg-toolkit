// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// monkSheet is a level-1 monk holding mainHand (empty for an unarmed strike)
// with the given Strength and Dexterity and its persisted Martial Arts.
func (s *CharacterAttackTestSuite) monkSheet(mainHand weapons.WeaponID, str, dex int) *Data {
	const id = "monk"
	equipped := map[InventorySlot]string{}
	if mainHand != "" {
		equipped[SlotMainHand] = string(mainHand)
	}
	data := s.heroSheet([]proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial}, equipped)
	data.ID = id
	data.ClassID = classes.Monk
	data.AbilityScores[abilities.STR] = str
	data.AbilityScores[abilities.DEX] = dex
	martialArts, err := conditions.NewMartialArtsCondition(conditions.MartialArtsInput{
		MemberID: id, MonkLevel: 1,
	}).ToJSON()
	s.Require().NoError(err)
	data.Conditions = append(data.Conditions, martialArts)
	return data
}

func (s *CharacterAttackTestSuite) assertAbility(definition combatActions.Definition, ability abilities.Ability, modifier int) {
	s.Require().NotNil(definition.Attack)
	s.Require().NotNil(definition.Attack.Ability)
	s.Equal(ability, definition.Attack.Ability.Ability)
	s.Equal(modifier, definition.Attack.Ability.Modifier)
}

func (s *CharacterAttackTestSuite) TestMartialArtsOverridesUnarmedAbilityAndDieAtAssembly() {
	definition := s.assemble(s.monkSheet("", 10, 16), &AssembleAttackInput{Slot: SlotMainHand})

	s.assertAbility(definition, abilities.DEX, 3)
	s.Equal(3+2, definition.Attack.AttackBonus, "Dexterity plus proficiency, settled before any roll")
	s.Require().Len(definition.Attack.Damage, 1)
	s.Equal("1d4", definition.Attack.Damage[0].Dice, "the Martial Arts die replaces the unarmed 1 before it is rolled")
}

func (s *CharacterAttackTestSuite) TestMartialArtsKeepsStrengthWhenHigher() {
	definition := s.assemble(s.monkSheet("", 16, 10), &AssembleAttackInput{Slot: SlotMainHand})

	s.assertAbility(definition, abilities.STR, 3)
	s.Equal("1d4", definition.Attack.Damage[0].Dice)
}

func (s *CharacterAttackTestSuite) TestMartialArtsMonkWeaponTakesDexterityButKeepsItsDie() {
	definition := s.assemble(s.monkSheet(weapons.Quarterstaff, 10, 16), &AssembleAttackInput{Slot: SlotMainHand})

	s.assertAbility(definition, abilities.DEX, 3)
	quarterstaff, err := weapons.GetByID(weapons.Quarterstaff)
	s.Require().NoError(err)
	s.Equal(quarterstaff.Damage[0].Dice, definition.Attack.Damage[0].Dice)
}

func (s *CharacterAttackTestSuite) TestMartialArtsLeavesNonMonkWeaponAlone() {
	monk := s.monkSheet(weapons.Greataxe, 10, 16)
	definition := s.assemble(monk, &AssembleAttackInput{Slot: SlotMainHand})

	s.assertAbility(definition, abilities.STR, 0)
	greataxe, err := weapons.GetByID(weapons.Greataxe)
	s.Require().NoError(err)
	s.Equal(greataxe.Damage[0].Dice, definition.Attack.Damage[0].Dice)

	withoutMartialArts := s.monkSheet(weapons.Greataxe, 10, 16)
	withoutMartialArts.Conditions = nil
	s.Equal(s.assemble(withoutMartialArts, &AssembleAttackInput{Slot: SlotMainHand}), definition,
		"a non-monk weapon assembles exactly as it does without Martial Arts")
}

func (s *CharacterAttackTestSuite) TestMartialArtsBonusAttackUsesSameOverride() {
	monk := s.load(s.monkSheet(weapons.Quarterstaff, 10, 16))
	cost, err := CostOfMartialArtsBonusAttack(monk)
	s.Require().NoError(err)

	bonus, err := AssembleMartialArtsBonusAttack(monk, &AssembleMartialArtsBonusAttackInput{Cost: cost})
	s.Require().NoError(err)

	unarmed := s.assemble(s.monkSheet("", 10, 16), &AssembleAttackInput{Slot: SlotMainHand})
	s.assertAbility(bonus, abilities.DEX, 3)
	s.Equal(unarmed.Attack.Ability, bonus.Attack.Ability)
	s.Equal(unarmed.Attack.AttackBonus, bonus.Attack.AttackBonus)
	s.Equal(unarmed.Attack.Damage, bonus.Attack.Damage)
	s.Equal("1d4", bonus.Attack.Damage[0].Dice)
}
