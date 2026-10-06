// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

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

// TestMartialArtsFoldLeavesTheRolledDieAlone pins that Martial Arts touches no
// roll: with the condition applied, a damage fold leaves the faces already
// rolled for the assembled Martial Arts die untouched, and an attack fold
// leaves the assembled bonus alone. The end-to-end count — a monk's unarmed
// hit rolls its damage die exactly once — needs the strike machine and lives
// in resolution (rpg-toolkit#1939, TestMonkUnarmedHitRollsItsDamageDieOnce).
func (s *MartialArtsTestSuite) TestMartialArtsFoldLeavesTheRolledDieAlone() {
	condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1", MonkLevel: 1})
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	override := condition.WeaponAttackOverride("main_hand", "")

	event := swungDamage(&dnd5eEvents.DamageChainEvent{
		AttackerID: "monk-1", TargetID: "goblin",
		WeaponDamageDice: override.Dice,
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.UnarmedStrike(), Name: "Unarmed Strike"},
				Dice:   testDiceTrace(4, 3),
			},
			DamageType: damage.Bludgeoning,
		}},
	}, swing{AbilityUsed: abilities.DEX, IsMelee: true, WeaponRef: refs.Weapons.UnarmedStrike()})
	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(s.ctx, withEventFrame(event), chain)
	s.Require().NoError(err)
	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)
	s.Require().Len(folded.Components, 1)
	s.Equal([]int{3}, folded.Components[0].Roll.Dice.OriginalRolls, "no re-roll replaced the assembled die")
	s.Equal([]int{3}, folded.Components[0].Roll.Dice.FinalRolls)
	s.Equal("1d4", folded.WeaponDamageDice)
	ability, _ := folded.Frame.Action.Ability.Get()
	s.Equal(abilities.DEX, ability)

	attack := swungAttack(dnd5eEvents.AttackChainEvent{AttackerID: "monk-1", AttackBonus: 5}, swing{WeaponRef: refs.Weapons.UnarmedStrike()})
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modifiedAttack, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(s.ctx, attack, attackChain)
	s.Require().NoError(err)
	foldedAttack, err := modifiedAttack.Execute(s.ctx, attack)
	s.Require().NoError(err)
	s.Equal(5, foldedAttack.AttackBonus, "the attack bonus was settled at assembly")
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
