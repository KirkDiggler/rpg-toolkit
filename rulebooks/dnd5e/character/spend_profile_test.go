// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// CostCompilerTestSuite pins the one thing the gate is never allowed to know:
// why a price is what it is. A level-5 fighter's Attack action buys two swings
// and a level-1 fighter's buys one, and every number here is read off a real
// sheet — the fixtures set a class and a level and nothing else, so a test
// that passes because the economy was hand-seeded cannot exist.
type CostCompilerTestSuite struct {
	suite.Suite

	ctx context.Context
	bus events.EventBus
}

func TestCostCompilerSuite(t *testing.T) {
	suite.Run(t, new(CostCompilerTestSuite))
}

func (s *CostCompilerTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// sheetOf loads a real character of the given class and level. Nothing about
// the action economy is set here: the compiler's answer has to come from the
// class table, which is the whole point of compiling it per actor.
func (s *CostCompilerTestSuite) sheetOf(class classes.Class, level int) *Character {
	char, err := LoadFromData(s.ctx, &Data{
		ID:               "cost-" + string(class),
		PlayerID:         "cost-player",
		Name:             "Priced",
		Level:            level,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          class,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    20,
		MaxHitPoints: 20,
		ArmorClass:   16,
	}, s.bus)
	s.Require().NoError(err)

	return char
}

func (s *CostCompilerTestSuite) offHandFighter() *Character {
	char, err := LoadFromData(s.ctx, &Data{
		ID:               "cost-two-weapon-fighter",
		PlayerID:         "cost-player",
		Name:             "Two Weapons",
		Level:            1,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    20,
		MaxHitPoints: 20,
		ArmorClass:   16,
		Inventory: []InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Scimitar), Quantity: 1},
		},
		EquipmentSlots: EquipmentSlots{
			SlotMainHand: string(weapons.Shortsword),
			SlotOffHand:  string(weapons.Scimitar),
		},
	}, s.bus)
	s.Require().NoError(err)

	return char
}

func (s *CostCompilerTestSuite) martialArtsMonk(
	id string,
	inventory []InventoryItemData,
	slots EquipmentSlots,
	withMartialArts bool,
) *Character {
	var conditionData []json.RawMessage
	if withMartialArts {
		martialArts, err := conditions.NewMartialArtsCondition(conditions.MartialArtsInput{
			MemberID: id, MonkLevel: 1,
		}).ToJSON()
		s.Require().NoError(err)
		conditionData = []json.RawMessage{martialArts}
	}

	char, err := Load(s.ctx, &Data{
		ID: id, PlayerID: "cost-player", Name: "Martial Artist",
		Level: 1, ProficiencyBonus: 2, RaceID: races.Human, ClassID: classes.Monk,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 16, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 14, abilities.CHA: 8,
		},
		HitPoints: 10, MaxHitPoints: 10, ArmorClass: 15,
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple},
		Inventory:           inventory,
		EquipmentSlots:      slots,
		Conditions:          conditionData,
	})
	s.Require().NoError(err)

	return char
}

func (s *CostCompilerTestSuite) quarterstaffMonk() *Character {
	return s.martialArtsMonk(
		"cost-quarterstaff-monk",
		[]InventoryItemData{{
			Type: shared.EquipmentTypeWeapon, ID: string(weapons.Quarterstaff), Quantity: 1,
		}},
		EquipmentSlots{SlotMainHand: string(weapons.Quarterstaff)},
		true,
	)
}

func (s *CostCompilerTestSuite) banked(class classes.Class, level int) int {
	profile, err := CostOfAttack(s.sheetOf(class, level))
	s.Require().NoError(err)
	s.Require().NoError(profile.Validate())

	return profile.Grants[combat.CapacityAttack]
}

// The headline case, from both ends.
func (s *CostCompilerTestSuite) TestTheAttackActionBanksWhatTheClassTableSays() {
	s.Equal(1, s.banked(classes.Fighter, 1), "a level-1 fighter's Attack action buys one swing")
	s.Equal(2, s.banked(classes.Fighter, 5), "a level-5 fighter's Attack action buys two")
	s.Equal(3, s.banked(classes.Fighter, 11))
	s.Equal(4, s.banked(classes.Fighter, 20))
}

// Extra Attack is not a fighter feature, and the boundary is the level rather
// than the class.
func (s *CostCompilerTestSuite) TestOtherClassesArePricedByTheSameTable() {
	s.Equal(1, s.banked(classes.Monk, 4))
	s.Equal(2, s.banked(classes.Monk, 5))
	s.Equal(2, s.banked(classes.Barbarian, 5))
	s.Equal(2, s.banked(classes.Paladin, 5))
	s.Equal(2, s.banked(classes.Ranger, 5))
	s.Equal(1, s.banked(classes.Wizard, 20), "no Extra Attack at any level")
}

func (s *CostCompilerTestSuite) TestAQuarterstaffAttackGrantsOneMartialArtsBonusAttack() {
	const martialArtsBonusAttack = combat.CapacityType("martial_arts_bonus_attack")

	profile, err := CostOfAttack(s.quarterstaffMonk())
	s.Require().NoError(err)

	s.Equal(1, profile.Grants[martialArtsBonusAttack],
		"a Quarterstaff is a Monk weapon, so its Attack action grants the bonus unarmed strike")
}

func (s *CostCompilerTestSuite) TestMartialArtsBonusRequiresItsFeatureAndLegalEquipment() {
	const capacity = combat.CapacityMartialArtsBonusAttack
	quarterstaff := InventoryItemData{
		Type: shared.EquipmentTypeWeapon, ID: string(weapons.Quarterstaff), Quantity: 1,
	}

	tests := []struct {
		name             string
		inventory        []InventoryItemData
		slots            EquipmentSlots
		withMartialArts  bool
		wantMartialGrant bool
	}{
		{name: "unarmed", withMartialArts: true, wantMartialGrant: true},
		{
			name: "Quarterstaff", inventory: []InventoryItemData{quarterstaff},
			slots:           EquipmentSlots{SlotMainHand: string(weapons.Quarterstaff)},
			withMartialArts: true, wantMartialGrant: true,
		},
		{
			name: "Shortsword", inventory: []InventoryItemData{{
				Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1,
			}},
			slots:           EquipmentSlots{SlotMainHand: string(weapons.Shortsword)},
			withMartialArts: true, wantMartialGrant: true,
		},
		{
			name: "simple ranged Dart", inventory: []InventoryItemData{{
				Type: shared.EquipmentTypeWeapon, ID: string(weapons.Dart), Quantity: 1,
			}},
			slots: EquipmentSlots{SlotMainHand: string(weapons.Dart)}, withMartialArts: true,
		},
		{
			name: "Two-Handed Greatclub", inventory: []InventoryItemData{{
				Type: shared.EquipmentTypeWeapon, ID: string(weapons.Greatclub), Quantity: 1,
			}},
			slots: EquipmentSlots{SlotMainHand: string(weapons.Greatclub)}, withMartialArts: true,
		},
		{
			name: "no Martial Arts condition", inventory: []InventoryItemData{quarterstaff},
			slots: EquipmentSlots{SlotMainHand: string(weapons.Quarterstaff)},
		},
		{
			name: "wearing armor", inventory: []InventoryItemData{
				quarterstaff,
				{Type: shared.EquipmentTypeArmor, ID: string(armor.Leather), Quantity: 1},
			},
			slots: EquipmentSlots{
				SlotMainHand: string(weapons.Quarterstaff), SlotArmor: string(armor.Leather),
			},
			withMartialArts: true,
		},
		{
			name: "wielding a shield", inventory: []InventoryItemData{
				quarterstaff,
				{Type: shared.EquipmentTypeArmor, ID: string(armor.Shield), Quantity: 1},
			},
			slots: EquipmentSlots{
				SlotMainHand: string(weapons.Quarterstaff), SlotOffHand: string(armor.Shield),
			},
			withMartialArts: true,
		},
	}

	for i, tc := range tests {
		s.Run(tc.name, func() {
			profile, err := CostOfAttack(s.martialArtsMonk(
				fmt.Sprintf("martial-eligibility-%d", i), tc.inventory, tc.slots, tc.withMartialArts,
			))
			s.Require().NoError(err)
			s.Equal(tc.wantMartialGrant, profile.Grants[capacity] == 1)
		})
	}
}

func (s *CostCompilerTestSuite) TestMartialArtsTakesPriorityOverTwoWeaponFighting() {
	monk := s.martialArtsMonk(
		"martial-priority",
		[]InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Scimitar), Quantity: 1},
		},
		EquipmentSlots{
			SlotMainHand: string(weapons.Shortsword), SlotOffHand: string(weapons.Scimitar),
		},
		true,
	)

	profile, err := CostOfAttack(monk)
	s.Require().NoError(err)

	s.Equal(1, profile.Grants[combat.CapacityMartialArtsBonusAttack])
	s.NotContains(profile.Grants, combat.CapacityOffHandAttack,
		"the established Monk rule wins instead of exposing the two-weapon strike")
}

func (s *CostCompilerTestSuite) TestAQualifyingAttackActionGrantsOneOffHandAttack() {
	profile, err := CostOfAttack(s.offHandFighter())
	s.Require().NoError(err)

	s.Equal(1, profile.Grants[combat.CapacityOffHandAttack])

	ordinary, err := CostOfAttack(s.sheetOf(classes.Fighter, 1))
	s.Require().NoError(err)
	s.NotContains(ordinary.Grants, combat.CapacityOffHandAttack)
}

func (s *CostCompilerTestSuite) TestOffHandAttackSpendsBonusActionAndGrantedCapacity() {
	profile, err := CostOfOffHandAttack(s.offHandFighter())
	s.Require().NoError(err)
	s.Require().NoError(profile.Validate())

	s.Equal(map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, profile.Slots)
	s.Equal(map[combat.CapacityType]int{combat.CapacityOffHandAttack: 1}, profile.Capacity)
	s.Empty(profile.Grants)
	s.Empty(profile.Pools)
	s.Empty(profile.Requires)
}

func (s *CostCompilerTestSuite) TestOffHandAttackPriceRevalidatesEquipment() {
	_, err := CostOfOffHandAttack(s.sheetOf(classes.Fighter, 1))

	s.Require().Error(err)
	s.Contains(err.Error(), "two light melee weapons")
}

func (s *CostCompilerTestSuite) TestQualifyingFirstSwingCarriesTheOffHandGrantThroughNetting() {
	profile, err := CostOfSwing(s.offHandFighter())
	s.Require().NoError(err)

	s.Equal(1, profile.Slots[coreCombat.ActionStandard])
	s.Equal(1, profile.Grants[combat.CapacityOffHandAttack])
	s.NotContains(profile.Grants, combat.CapacityAttack)
	s.NotContains(profile.Capacity, combat.CapacityAttack)
}

// The Attack action costs the action slot and banks capacity rather than
// spending it, which is why a fighter's second swing needs no second action.
func (s *CostCompilerTestSuite) TestTheAttackActionCostsExactlyOneActionSlot() {
	profile, err := CostOfAttack(s.sheetOf(classes.Fighter, 5))
	s.Require().NoError(err)

	s.Equal(1, profile.Slots[coreCombat.ActionStandard])
	s.Len(profile.Slots, 1)
	s.Empty(profile.Capacity)
	s.Empty(profile.Pools, "nothing in v1 prices an action in pool points")
	s.Empty(profile.Requires, "nothing in v1 prices an action behind a precondition")
}

// A swing costs one banked attack and no slot; the price is inert data rather
// than an executable Strike object's accessors.
func (s *CostCompilerTestSuite) TestAStrikeCostsOneBankedAttackAndNoSlot() {
	profile, err := CostOfStrike(s.sheetOf(classes.Fighter, 5))
	s.Require().NoError(err)
	s.Require().NoError(profile.Validate())

	s.Equal(1, profile.Capacity[combat.CapacityAttack])
	s.Len(profile.Capacity, 1)
	s.Empty(profile.Slots)
	s.Empty(profile.Grants)
}

// No sheet, no price. The compilers refuse rather than compiling a default,
// the same way AssembleAttack refuses a nil character.
func (s *CostCompilerTestSuite) TestCompilingWithoutASheetIsRefused() {
	_, err := CostOfAttack(nil)
	s.Require().Error(err)

	_, err = CostOfStrike(nil)
	s.Require().Error(err)

	_, err = CostOfOffHandAttack(nil)
	s.Require().Error(err)
}

// Case (i) end to end: one action buys two swings, and the third is refused —
// on a real sheet, through the gate, with nothing between the class table and
// the bank but the profile.
func (s *CostCompilerTestSuite) TestALevelFiveFighterSwingsTwiceAndNoMore() {
	fighter := s.sheetOf(classes.Fighter, 5)
	_, err := fighter.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	attack, err := CostOfAttack(fighter)
	s.Require().NoError(err)
	s.Require().NoError(combat.Pay(fighter, attack))
	s.Equal(2, fighter.CapacityLeft(combat.CapacityAttack))
	s.Equal(0, fighter.SlotsLeft(coreCombat.ActionStandard))

	strike, err := CostOfStrike(fighter)
	s.Require().NoError(err)
	s.Require().NoError(combat.Pay(fighter, strike))
	s.Require().NoError(combat.Pay(fighter, strike))

	s.False(combat.CanPay(fighter, strike), "the third swing has nothing left to spend")
	s.Require().Error(combat.Pay(fighter, strike))

	s.False(combat.CanPay(fighter, attack), "and the action that would bank more is gone")
}

// The level-1 fighter is the same walk with one swing in it.
func (s *CostCompilerTestSuite) TestALevelOneFighterSwingsOnce() {
	fighter := s.sheetOf(classes.Fighter, 1)
	_, err := fighter.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	attack, err := CostOfAttack(fighter)
	s.Require().NoError(err)
	s.Require().NoError(combat.Pay(fighter, attack))
	s.Equal(1, fighter.CapacityLeft(combat.CapacityAttack))

	strike, err := CostOfStrike(fighter)
	s.Require().NoError(err)
	s.Require().NoError(combat.Pay(fighter, strike))
	s.False(combat.CanPay(fighter, strike))
}

func (s *CostCompilerTestSuite) TestSwingNettingCarriesCapacityStillOwed() {
	action := &combat.SpendProfile{
		Slots:  map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Grants: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}
	strike := &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 2},
	}

	merged := asOnePayment(action, strike)

	s.Require().NoError(merged.Validate())
	s.Equal(1, merged.Capacity[combat.CapacityAttack])
	s.Empty(merged.Grants)
	s.Equal(1, merged.Slots[coreCombat.ActionStandard])
}

func (s *CostCompilerTestSuite) TestSwingNettingLeavesNoZeroEntries() {
	action := &combat.SpendProfile{
		Slots:  map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Grants: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}
	strike := &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}

	merged := asOnePayment(action, strike)

	s.Require().NoError(merged.Validate())
	s.NotContains(merged.Grants, combat.CapacityAttack)
	s.NotContains(merged.Capacity, combat.CapacityAttack)
	s.Equal(1, merged.Slots[coreCombat.ActionStandard])
}

func (s *CostCompilerTestSuite) TestSwingNettingCarriesEveryCurrency() {
	const ki = coreResources.ResourceKey("ki")
	action := &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
		Pools:    map[coreResources.ResourceKey]int{ki: 1},
		Requires: map[combat.CapacityType]int{combat.CapacityAttack: 1},
	}
	strike := &combat.SpendProfile{
		Slots:    map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools:    map[coreResources.ResourceKey]int{ki: 2},
		Requires: map[combat.CapacityType]int{combat.CapacityAttack: 3},
	}

	merged := asOnePayment(action, strike)

	s.Require().NoError(merged.Validate())
	s.Equal(1, merged.Slots[coreCombat.ActionStandard])
	s.Equal(1, merged.Slots[coreCombat.ActionBonus])
	s.Equal(3, merged.Pools[ki])
	s.Equal(3, merged.Requires[combat.CapacityAttack])
}
