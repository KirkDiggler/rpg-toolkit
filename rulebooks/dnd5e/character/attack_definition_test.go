// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type CharacterAttackTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestAssembleAttackSuite(t *testing.T) {
	suite.Run(t, new(CharacterAttackTestSuite))
}

func (s *CharacterAttackTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CharacterAttackTestSuite) heroSheet(
	profs []proficiencies.Weapon, equipped map[InventorySlot]string,
) *Data {
	inventory := make([]InventoryItemData, 0, len(equipped))
	slots := EquipmentSlots{}
	for slot, id := range equipped {
		inventory = append(inventory, InventoryItemData{
			Type: shared.EquipmentTypeWeapon, ID: id, Quantity: 1,
		})
		slots[slot] = id
	}

	return &Data{
		ID:       "hero",
		PlayerID: "player-1",
		Name:     "Grog",
		Level:    1,
		ClassID:  classes.Fighter,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:           14,
		MaxHitPoints:        14,
		ArmorClass:          14,
		ProficiencyBonus:    2,
		WeaponProficiencies: profs,
		Inventory:           inventory,
		EquipmentSlots:      slots,
	}
}

func (s *CharacterAttackTestSuite) martialHero() *Data {
	return s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial},
		map[InventorySlot]string{SlotMainHand: string(weapons.Longsword)},
	)
}

func (s *CharacterAttackTestSuite) load(data *Data) *Character {
	character, err := Load(s.ctx, data)
	s.Require().NoError(err)
	return character
}

func (s *CharacterAttackTestSuite) assemble(data *Data, in *AssembleAttackInput) combatActions.Definition {
	definition, err := AssembleAttack(s.load(data), in)
	s.Require().NoError(err)
	return definition
}

func (s *CharacterAttackTestSuite) mainHand() *AssembleAttackInput {
	return &AssembleAttackInput{Slot: SlotMainHand}
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_LongswordCarriesStaticEvidence() {
	cost := &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1}}
	definition := s.assemble(s.martialHero(), &AssembleAttackInput{Slot: SlotMainHand, Cost: cost})

	s.Equal(*refs.Weapons.Longsword(), definition.Ref)
	s.Equal("Longsword", definition.Name)
	s.Equal(cost, definition.Cost)
	s.Require().NotNil(definition.Attack)
	s.Equal(combatActions.AttackCategoryWeapon, definition.Attack.Category)
	s.Equal(&combatActions.MeleeDelivery{ReachFeet: 5}, definition.Attack.Delivery.Melee)
	s.Equal(5, definition.Attack.AttackBonus)
	s.Equal(&combatActions.AbilityContribution{Ability: abilities.STR, Modifier: 3}, definition.Attack.Ability)
	s.Equal(refs.Weapons.Longsword(), definition.Attack.Weapon.Ref)
	s.Equal([]damage.Damage{{
		Dice:       "1d8",
		Type:       damage.Slashing,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
	}}, definition.Attack.Damage)
	s.Require().NoError(definition.Validate())
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_LongbowIsRanged() {
	data := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponMartial},
		map[InventorySlot]string{SlotMainHand: string(weapons.Longbow)},
	)
	cost := &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1}}

	definition := s.assemble(data, &AssembleAttackInput{Slot: SlotMainHand, Cost: cost})

	s.Require().NotNil(definition.Attack)
	s.Equal(combatActions.AttackCategoryWeapon, definition.Attack.Category)
	s.Equal(&combatActions.RangedDelivery{NormalFeet: 150, LongFeet: 600}, definition.Attack.Delivery.Ranged)
	s.Equal(abilities.DEX, definition.Attack.Ability.Ability)
	s.Equal(refs.Weapons.Longbow(), definition.Attack.Weapon.Ref)
	s.Equal(cost, definition.Cost)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_ThrownMeleeWeaponRemainsMelee() {
	data := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponSimple},
		map[InventorySlot]string{SlotMainHand: string(weapons.Dagger)},
	)

	definition := s.assemble(data, s.mainHand())

	s.Equal(&combatActions.MeleeDelivery{ReachFeet: 5}, definition.Attack.Delivery.Melee)
	s.Nil(definition.Attack.Delivery.Ranged)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_FinesseUsesTheBetterAbility() {
	data := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponSimple},
		map[InventorySlot]string{SlotMainHand: string(weapons.Dagger)},
	)
	data.AbilityScores[abilities.STR] = 10
	data.AbilityScores[abilities.DEX] = 18

	dexterity := s.assemble(data, s.mainHand())
	s.Equal(&combatActions.AbilityContribution{Ability: abilities.DEX, Modifier: 4}, dexterity.Attack.Ability)
	s.Equal(6, dexterity.Attack.AttackBonus)

	data.AbilityScores[abilities.STR] = 18
	data.AbilityScores[abilities.DEX] = 10
	strength := s.assemble(data, s.mainHand())
	s.Equal(&combatActions.AbilityContribution{Ability: abilities.STR, Modifier: 4}, strength.Attack.Ability)
	s.Equal(6, strength.Attack.AttackBonus)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_RangedFinesseMayUseStrength() {
	data := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponSimple},
		map[InventorySlot]string{SlotMainHand: string(weapons.Dart)},
	)
	data.AbilityScores[abilities.STR] = 18
	data.AbilityScores[abilities.DEX] = 10

	definition := s.assemble(data, s.mainHand())

	s.Equal(&combatActions.AbilityContribution{Ability: abilities.STR, Modifier: 4}, definition.Attack.Ability)
	s.Equal(6, definition.Attack.AttackBonus)
	s.Require().NotNil(definition.Attack.Delivery.Ranged)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_ProficiencyChangesAccuracyOnly() {
	equipped := map[InventorySlot]string{SlotMainHand: string(weapons.Longsword)}
	trained := s.assemble(s.heroSheet([]proficiencies.Weapon{proficiencies.WeaponMartial}, equipped), s.mainHand())
	untrained := s.assemble(s.heroSheet([]proficiencies.Weapon{proficiencies.WeaponSimple}, equipped), s.mainHand())

	s.Equal(2, trained.Attack.AttackBonus-untrained.Attack.AttackBonus)
	s.Equal(trained.Attack.Damage, untrained.Attack.Damage)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_VersatileGripChangesOnlyTheDie() {
	oneHanded := s.assemble(s.martialHero(), s.mainHand())
	twoHanded := s.assemble(s.martialHero(), &AssembleAttackInput{Slot: SlotMainHand, TwoHanded: true})

	s.Equal("1d8", oneHanded.Attack.Damage[0].Dice)
	s.Equal("1d10", twoHanded.Attack.Damage[0].Dice)
	s.Equal(oneHanded.Attack.AttackBonus, twoHanded.Attack.AttackBonus)
	s.False(oneHanded.Attack.Weapon.TwoHanded)
	s.True(twoHanded.Attack.Weapon.TwoHanded)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_TwoHandedPropertyIsStaticEvidence() {
	data := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponMartial},
		map[InventorySlot]string{SlotMainHand: string(weapons.Greatsword)},
	)

	definition := s.assemble(data, s.mainHand())

	s.True(definition.Attack.Weapon.TwoHanded)
	s.Equal("2d6", definition.Attack.Damage[0].Dice)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_ReachPropertyChangesMeleeReach() {
	data := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponMartial},
		map[InventorySlot]string{SlotMainHand: string(weapons.Glaive)},
	)

	definition := s.assemble(data, s.mainHand())

	s.Equal(&combatActions.MeleeDelivery{ReachFeet: 10}, definition.Attack.Delivery.Melee)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_EmptyHandIsAnUnarmedStrike() {
	definition := s.assemble(s.heroSheet(nil, nil), s.mainHand())

	s.Equal(*refs.Weapons.UnarmedStrike(), definition.Ref)
	s.Equal("Unarmed Strike", definition.Name)
	s.Equal(5, definition.Attack.AttackBonus, "every character is proficient with unarmed strikes")
	s.Equal(&combatActions.MeleeDelivery{ReachFeet: 5}, definition.Attack.Delivery.Melee)
	s.Equal("1d1", definition.Attack.Damage[0].Dice)
	s.Equal(damage.Bludgeoning, definition.Attack.Damage[0].Type)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_OffHandSwingMayAlsoBeUnarmed() {
	definition := s.assemble(s.heroSheet(nil, nil), &AssembleAttackInput{Slot: SlotOffHand})

	s.Equal(*refs.Weapons.UnarmedStrike(), definition.Ref)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_RecordsTheOtherHandWeapon() {
	data := s.heroSheet(
		[]proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial},
		map[InventorySlot]string{
			SlotMainHand: string(weapons.Longsword),
			SlotOffHand:  string(weapons.Dagger),
		},
	)

	definition := s.assemble(data, s.mainHand())

	s.Equal(refs.Weapons.Dagger(), definition.Attack.Weapon.OffHandWeaponRef)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_PreservesEveryDamagePool() {
	original := weapons.All[weapons.Longsword]
	modified := original
	modified.Damage = []damage.Damage{
		{Dice: "1d8", Type: damage.Slashing, Properties: []damage.Property{damage.AddsAttackAbilityModifier}},
		{Dice: "1d4", Type: damage.Fire, Properties: []damage.Property{damage.DoesNotCrit}},
	}
	weapons.All[weapons.Longsword] = modified
	s.T().Cleanup(func() { weapons.All[weapons.Longsword] = original })

	definition := s.assemble(s.martialHero(), s.mainHand())

	s.Equal(modified.Damage, definition.Attack.Damage)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_DoesNotAliasCostOrCatalogDamage() {
	cost := &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1}}
	definition := s.assemble(s.martialHero(), &AssembleAttackInput{Slot: SlotMainHand, Cost: cost})

	cost.Capacity[combat.CapacityAttack] = 9
	definition.Attack.Damage[0].Properties[0] = damage.DoesNotCrit
	definition.Attack.Weapon.Ref.ID = "changed"

	s.Equal(1, definition.Cost.Capacity[combat.CapacityAttack])
	fresh := s.assemble(s.martialHero(), s.mainHand())
	s.Equal([]damage.Property{damage.AddsAttackAbilityModifier}, fresh.Attack.Damage[0].Properties)
	s.Equal("longsword", refs.Weapons.Longsword().ID)
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_RefusesUnreadableEquipment() {
	s.Run("corrupt slot", func() {
		data := s.heroSheet(nil, nil)
		data.EquipmentSlots = EquipmentSlots{SlotMainHand: "phantom-blade"}

		_, err := AssembleAttack(s.load(data), s.mainHand())

		s.Require().Error(err)
		s.Contains(err.Error(), "phantom-blade")
		s.Contains(err.Error(), "not in the inventory")
	})

	s.Run("armor in requested slot", func() {
		data := s.heroSheet(nil, nil)
		data.Inventory = append(data.Inventory, InventoryItemData{
			Type: shared.EquipmentTypeArmor, ID: string(armor.Leather), Quantity: 1,
		})
		data.EquipmentSlots[SlotArmor] = string(armor.Leather)

		_, err := AssembleAttack(s.load(data), &AssembleAttackInput{Slot: SlotArmor})

		s.Require().Error(err)
		s.Contains(err.Error(), "holds no weapon")
	})

	s.Run("no slot", func() {
		_, err := AssembleAttack(s.load(s.martialHero()), &AssembleAttackInput{})
		s.Require().Error(err)
	})

	s.Run("nil inputs", func() {
		_, err := AssembleAttack(nil, s.mainHand())
		s.Require().Error(err)
		_, err = AssembleAttack(s.load(s.martialHero()), nil)
		s.Require().Error(err)
	})
}

func (s *CharacterAttackTestSuite) TestCostOfSwing_FirstSwingNetsTheAttackGrant() {
	fighter := s.load(s.martialHero())

	profile, err := CostOfSwing(fighter)

	s.Require().NoError(err)
	s.Equal(1, profile.Slots[coreCombat.ActionStandard])
	s.NotContains(profile.Capacity, combat.CapacityAttack)
	s.NotContains(profile.Grants, combat.CapacityAttack)
	s.Require().NoError(profile.Validate())
}

func (s *CharacterAttackTestSuite) TestCostOfSwing_ExtraAttackLeavesLaterCapacityBanked() {
	data := s.martialHero()
	data.Level = 5
	fighter := s.load(data)

	profile, err := CostOfSwing(fighter)

	s.Require().NoError(err)
	s.Equal(1, profile.Slots[coreCombat.ActionStandard])
	s.Equal(1, profile.Grants[combat.CapacityAttack])
	s.NotContains(profile.Capacity, combat.CapacityAttack)
}

func (s *CharacterAttackTestSuite) TestCostOfSwing_AlreadyBankedAttackCostsOnlyCapacity() {
	fighter := s.load(s.martialHero())
	_, err := fighter.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	fighter.BankCapacity(combat.CapacityAttack, 1)

	profile, err := CostOfSwing(fighter)

	s.Require().NoError(err)
	s.Equal(1, profile.Capacity[combat.CapacityAttack])
	s.Empty(profile.Slots)
	s.Empty(profile.Grants)
}
