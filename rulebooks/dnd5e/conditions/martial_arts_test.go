// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// MartialArtsTestSuite pins Martial Arts at attack assembly. The comparison
// of Dexterity against the weapon's own ability is made by weaponattack from
// the wielder's scores (pinned in character's assembly tests); this suite pins
// what the condition offers and that it no longer touches a roll.
type MartialArtsTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestMartialArtsTestSuite(t *testing.T) {
	suite.Run(t, new(MartialArtsTestSuite))
}

func (s *MartialArtsTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// TestApplyAndRemove verifies basic apply/remove functionality
func (s *MartialArtsTestSuite) TestApplyAndRemove() {
	condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1", MonkLevel: 1})
	s.False(condition.IsApplied())

	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	s.True(condition.IsApplied())
	s.Error(condition.Apply(s.ctx, s.bus), "applying twice is refused")

	s.NoError(condition.Remove(s.ctx, s.bus))
	s.False(condition.IsApplied())
	s.NoError(condition.Remove(s.ctx, s.bus), "removing twice is a no-op")
}

func (s *MartialArtsTestSuite) TestOverrideDieScalesWithMonkLevel() {
	for level, die := range map[int]string{1: "1d4", 4: "1d4", 5: "1d6", 11: "1d8", 17: "1d10", 20: "1d10"} {
		condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1", MonkLevel: level})
		s.Equal(&weaponattack.Override{Dice: die, Ability: abilities.DEX},
			condition.WeaponAttackOverride("main_hand", ""), "level %d", level)
	}
}

func (s *MartialArtsTestSuite) TestOverrideByWeapon() {
	condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1", MonkLevel: 1})
	unarmed := &weaponattack.Override{Dice: "1d4", Ability: abilities.DEX}

	s.Equal(unarmed, condition.WeaponAttackOverride("main_hand", ""), "an empty hand strikes unarmed")
	s.Equal(unarmed, condition.WeaponAttackOverride("", ""), "the bonus unarmed strike names no hand")
	s.Equal(unarmed, condition.WeaponAttackOverride("main_hand", string(weapons.UnarmedStrike)))
	for _, monkWeapon := range []weapons.WeaponID{weapons.Quarterstaff, weapons.Club, weapons.Shortsword} {
		s.Equal(&weaponattack.Override{Ability: abilities.DEX},
			condition.WeaponAttackOverride("main_hand", string(monkWeapon)),
			"%s keeps its own die and may use Dexterity", monkWeapon)
	}
	for _, other := range []string{string(weapons.Greataxe), string(weapons.Longbow), "not-a-weapon"} {
		s.Nil(condition.WeaponAttackOverride("main_hand", other), other)
	}
}

func (s *MartialArtsTestSuite) TestMonkWeaponDetection() {
	for id, want := range map[weapons.WeaponID]bool{
		weapons.Shortsword:   true,
		weapons.Club:         true,
		weapons.Quarterstaff: true,
		weapons.Greatsword:   false,
		weapons.Longbow:      false,
	} {
		weapon, err := weapons.GetByID(id)
		s.Require().NoError(err)
		s.Equal(want, isMonkWeapon(&weapon), string(id))
	}
}

// TestMartialArtsRollsItsDieOnce stands in for the strike: the assembled
// unarmed pool is rolled once by its owner, then the damage and attack chains
// fold with Martial Arts applied. A counting roller sees exactly one roll —
// nothing rolls the weapon die and then discards it for the Martial Arts die.
func (s *MartialArtsTestSuite) TestMartialArtsRollsItsDieOnce() {
	condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1", MonkLevel: 1})
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	override := condition.WeaponAttackOverride("main_hand", "")

	roller := &countingRoller{}
	pool, err := dice.ParseNotation(override.Dice)
	s.Require().NoError(err)
	rolled := pool.RollContext(s.ctx, roller)
	s.Require().NoError(rolled.Error())
	s.Equal(1, roller.rollNCalls+roller.rollCalls, "the assembled 1d4 is rolled once")

	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "monk-1", TargetID: "goblin", AbilityUsed: abilities.DEX, IsMelee: true,
		WeaponRef: refs.Weapons.UnarmedStrike(), WeaponDamageDice: override.Dice,
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
				Dice:   testDiceTrace(4, 3),
			},
			DamageType: damage.Bludgeoning,
		}},
	}
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(s.ctx, withEventFrame(event), chain)
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)
	s.Equal([]int{3}, folded.Components[0].Roll.Dice.FinalRolls, "the fold leaves the rolled die alone")
	s.Equal(abilities.DEX, folded.AbilityUsed)

	attack := dnd5eEvents.AttackChainEvent{AttackerID: "monk-1", WeaponRef: refs.Weapons.UnarmedStrike(), AttackBonus: 5}
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modifiedAttack, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(s.ctx, attack, attackChain)
	s.Require().NoError(err)
	foldedAttack, err := modifiedAttack.Execute(s.ctx, attack)
	s.Require().NoError(err)
	s.Equal(5, foldedAttack.AttackBonus, "the attack bonus was settled at assembly")

	s.Equal(1, roller.rollNCalls+roller.rollCalls)
}

// TestSerialization tests JSON serialization round-trip
func (s *MartialArtsTestSuite) TestSerialization() {
	original := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1", MonkLevel: 5})

	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	var data MartialArtsData
	s.Require().NoError(json.Unmarshal(jsonData, &data))
	s.Equal(refs.Conditions.MartialArts(), data.Ref)
	s.Equal("monk-1", data.MemberID)
	s.Equal(5, data.MonkLevel)

	loaded := &MartialArtsCondition{}
	s.Require().NoError(loaded.loadJSON(jsonData))
	s.Equal(original.MemberID, loaded.MemberID)
	s.Equal(original.MonkLevel, loaded.MonkLevel)
}
