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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
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
	condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1"})
	s.False(condition.IsApplied())

	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	s.True(condition.IsApplied())
	s.Error(condition.Apply(s.ctx, s.bus), "applying twice is refused")

	s.NoError(condition.Remove(s.ctx, s.bus))
	s.False(condition.IsApplied())
	s.NoError(condition.Remove(s.ctx, s.bus), "removing twice is a no-op")
}

// offer asks the condition for its override for the swing, handing it the
// level record the sheet would.
func (s *MartialArtsTestSuite) offer(
	condition *MartialArtsCondition, slot, itemID string, levels classes.LevelHolder,
) *weaponattack.Override {
	out, err := condition.WeaponAttackOverride(&weaponattack.OverrideInput{Slot: slot, ItemID: itemID, Levels: levels})
	s.Require().NoError(err)
	return out.Override
}

// The die is read from the level record handed in at each swing: the same
// condition answers a different die when the sheet's monk level changes, with
// nothing written to it.
func (s *MartialArtsTestSuite) TestOverrideDieScalesWithMonkLevel() {
	condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1"})
	for level, die := range map[int]string{1: "1d4", 4: "1d4", 5: "1d6", 11: "1d8", 17: "1d10", 20: "1d10"} {
		s.Equal(&weaponattack.Override{Dice: die, Ability: abilities.DEX},
			s.offer(condition, "main_hand", "", monkLevels(level)), "level %d", level)
	}
}

// An unarmed strike from a holder with no monk levels, or with no level record
// handed in, cannot answer its die and fails the assembly.
func (s *MartialArtsTestSuite) TestOverrideRefusesWithoutMonkLevels() {
	condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1"})
	for name, levels := range map[string]classes.LevelHolder{
		"no monk levels":  fakeLevels{classes.Fighter: 5},
		"no level record": nil,
	} {
		_, err := condition.WeaponAttackOverride(&weaponattack.OverrideInput{Slot: "main_hand", Levels: levels})
		s.Error(err, name)
	}
}

func (s *MartialArtsTestSuite) TestOverrideByWeapon() {
	condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1"})
	unarmed := &weaponattack.Override{Dice: "1d4", Ability: abilities.DEX}
	levels := monkLevels(1)

	s.Equal(unarmed, s.offer(condition, "main_hand", "", levels), "an empty hand strikes unarmed")
	s.Equal(unarmed, s.offer(condition, "", "", levels), "the bonus unarmed strike names no hand")
	s.Equal(unarmed, s.offer(condition, "main_hand", string(weapons.UnarmedStrike), levels))
	for _, monkWeapon := range []weapons.WeaponID{weapons.Quarterstaff, weapons.Club, weapons.Shortsword} {
		s.Equal(&weaponattack.Override{Ability: abilities.DEX},
			s.offer(condition, "main_hand", string(monkWeapon), levels),
			"%s keeps its own die and may use Dexterity", monkWeapon)
	}
	for _, other := range []string{string(weapons.Greataxe), string(weapons.Longbow), "not-a-weapon"} {
		s.Nil(s.offer(condition, "main_hand", other, levels), other)
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
	condition := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1"})
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	override := s.offer(condition, "main_hand", "", monkLevels(1))

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
	original := NewMartialArtsCondition(MartialArtsInput{MemberID: "monk-1"})

	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	var data MartialArtsData
	s.Require().NoError(json.Unmarshal(jsonData, &data))
	s.Equal(refs.Conditions.MartialArts(), data.Ref)
	s.Equal("monk-1", data.MemberID)
	s.NotContains(string(jsonData), "level", "the monk level is the sheet's, never stored")

	loaded := &MartialArtsCondition{}
	s.Require().NoError(loaded.loadJSON(jsonData))
	s.Equal(original.MemberID, loaded.MemberID)
}
