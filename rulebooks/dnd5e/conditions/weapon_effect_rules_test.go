// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// weaponEffectRulesSuite covers the rules that answer from the frame's weapon
// facts: each applies, does not apply for its stated reason, and depends when
// the fact it reads is unknown.
type weaponEffectRulesSuite struct{ ruleHelpers }

func TestWeaponEffectRulesSuite(t *testing.T) { suite.Run(t, new(weaponEffectRulesSuite)) }

// mutated is the rogue's frame with one change applied.
func mutated(change func(*contributions.ActionFacts)) contributions.Frame {
	frame := rogueFrame(false)
	change(&frame.Action)
	return frame
}

func (s *weaponEffectRulesSuite) expect(
	rule contributions.ActionAssessor, frame contributions.Frame,
	state contributions.Applicability, reason string,
) contributions.Answer {
	answer := s.answer(rule, frame)
	s.Equal(state, answer.Decision.Applicability, reason)
	s.Equal(reason, answer.Decision.Reason)
	return answer
}

func (s *weaponEffectRulesSuite) TestRecklessAttack() {
	rule := NewRecklessAttackCondition("rogue")

	answer := s.expect(rule, rogueFrame(false), contributions.Applies, "The attack is a melee attack on your turn")
	s.Equal("Advantage on the attack roll", answer.Benefit)

	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.Melee = contributions.Known(false) }),
		contributions.DoesNotApply, "Reckless Attack needs a melee attack")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.Opportunity = contributions.Known(true) }),
		contributions.DoesNotApply, "Reckless Attack does not cover opportunity attacks")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.Opportunity = contributions.Unknown[bool]() }),
		contributions.Depends, "Depends on the kind of attack")
	s.expect(rule, savingThrowFrame(), contributions.DoesNotApply, "Reckless Attack affects only attack rolls")
}

// TestRecklessAttackSkipsAnOpportunityAttack: the swing grants advantage
// exactly when the rule applies, and an opportunity attack is refused by the
// frame's own fact.
func (s *weaponEffectRulesSuite) TestRecklessAttackSkipsAnOpportunityAttack() {
	bus := events.NewEventBus()
	reckless := NewRecklessAttackCondition("rogue")
	s.Require().NoError(reckless.Apply(context.Background(), bus))

	for name, opportunity := range map[string]bool{"standard": false, "opportunity": true} {
		s.Run(name, func() {
			event := swungAttack(dnd5eEvents.AttackChainEvent{AttackerID: "rogue", TargetID: "goblin"}, swing{IsMelee: true})
			event.Frame.Action.Opportunity = contributions.Known(opportunity)
			answer := s.answer(reckless, event.Frame)
			final, err := s.publishAttack(bus, event)
			s.Require().NoError(err)
			applies := answer.Decision.Applicability == contributions.Applies
			s.Equal(!opportunity, applies)
			s.Equal(applies, len(final.AdvantageSources) == 1)
		})
	}
}

func (s *weaponEffectRulesSuite) TestDueling() {
	rule := NewFightingStyleDuelingCondition("rogue")

	answer := s.expect(rule, rogueFrame(false), contributions.Applies, "A melee weapon in one hand and no other weapon")
	s.Equal("+2 damage", answer.Benefit)

	s.expect(rule, mutated(func(a *contributions.ActionFacts) {
		a.Weapon = contributions.Known(refs.Weapons.UnarmedStrike().String())
	}), contributions.DoesNotApply, "Dueling needs a melee weapon")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) {
		a.Weapon = contributions.Known(refs.Weapons.Shortbow().String())
		a.RangedWeapon = contributions.Known(true)
	}), contributions.DoesNotApply, "Dueling needs a melee weapon")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.Weapon = contributions.Known("") }),
		contributions.DoesNotApply, "Dueling needs a melee weapon")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.RangedWeapon = contributions.Unknown[bool]() }),
		contributions.Depends, "Depends on the attack's weapon and grip")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.TwoHanded = contributions.Known(true) }),
		contributions.DoesNotApply, "Dueling needs the weapon in one hand")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.OffHandWeapon = contributions.Known(true) }),
		contributions.DoesNotApply, "Dueling needs no other weapon in hand")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.WeaponPool = contributions.Known(false) }),
		contributions.DoesNotApply, "Dueling requires a weapon damage pool")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.OffHandWeapon = contributions.Unknown[bool]() }),
		contributions.Depends, "Depends on the attack's weapon and grip")
}

// TestDuelingKeepsAThrownDagger: a dagger thrown from one hand is still a
// melee weapon — the weapon's category, not the attack's delivery — so Dueling
// applies though the attack is not melee.
func (s *weaponEffectRulesSuite) TestDuelingKeepsAThrownDagger() {
	thrown := mutated(func(a *contributions.ActionFacts) {
		a.Melee = contributions.Known(false)
		a.Weapon = contributions.Known(refs.Weapons.Dagger().String())
		a.RangedWeapon = contributions.Known(false)
	})

	s.expect(NewFightingStyleDuelingCondition("rogue"), thrown,
		contributions.Applies, "A melee weapon in one hand and no other weapon")
}

func (s *weaponEffectRulesSuite) TestGreatWeaponFighting() {
	rule := NewFightingStyleGreatWeaponFightingCondition("rogue", nil)
	twoHanded := mutated(func(a *contributions.ActionFacts) { a.TwoHanded = contributions.Known(true) })

	answer := s.expect(rule, twoHanded, contributions.Applies, "A melee weapon held in both hands")
	s.Equal("Rerolls 1s and 2s on the weapon's damage dice", answer.Benefit)

	s.expect(rule, rogueFrame(false), contributions.DoesNotApply, "Great Weapon Fighting needs the weapon in both hands")
	ranged := twoHanded
	ranged.Action.Weapon = contributions.Known(refs.Weapons.Longbow().String())
	ranged.Action.RangedWeapon = contributions.Known(true)
	s.expect(rule, ranged, contributions.DoesNotApply, "Great Weapon Fighting needs a melee weapon")
	unread := twoHanded
	unread.Action.RangedWeapon = contributions.Unknown[bool]()
	s.expect(rule, unread, contributions.Depends, "Depends on the attack's weapon and grip")
	unknown := twoHanded
	unknown.Action.TwoHanded = contributions.Unknown[bool]()
	s.expect(rule, unknown, contributions.Depends, "Depends on the attack's weapon and grip")
}

// TestGreatWeaponFightingLeavesAOneHandedSwingAlone: the swing rerolls only
// when its rule applies — a one-handed swing keeps its low faces.
func (s *weaponEffectRulesSuite) TestGreatWeaponFightingLeavesAOneHandedSwingAlone() {
	bus := events.NewEventBus()
	gwf := NewFightingStyleGreatWeaponFightingCondition("rogue", nil)
	s.Require().NoError(gwf.Apply(context.Background(), bus))
	event := swungDamage(&dnd5eEvents.DamageChainEvent{
		AttackerID: "rogue", TargetID: "goblin",
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			Roll:       dnd5eEvents.RollComponent{Dice: testDiceTrace(8, 1, 2)},
		}},
	}, swing{IsMelee: true, TwoHanded: false, WeaponRef: refs.Weapons.Longsword()})

	s.Require().NoError(s.publishDamage(bus, event))

	s.Equal([]int{1, 2}, event.Components[0].Roll.Dice.FinalRolls)
	s.Empty(event.Components[0].Roll.Dice.Rerolls)
}

func (s *weaponEffectRulesSuite) TestTwoWeaponFighting() {
	rule := NewFightingStyleTwoWeaponFightingCondition("rogue")
	offHand := mutated(func(a *contributions.ActionFacts) { a.OffHandAttack = contributions.Known(true) })

	answer := s.expect(rule, offHand, contributions.Applies, "This is the off-hand attack")
	s.Equal("+3 damage", answer.Benefit)

	s.expect(rule, rogueFrame(false), contributions.DoesNotApply, "Two-Weapon Fighting adds only to the off-hand attack")
	weak := offHand
	weak.Action.AbilityModifier = contributions.Known(0)
	s.expect(rule, weak, contributions.DoesNotApply, "Your ability modifier adds nothing to damage")
	unknown := offHand
	unknown.Action.AbilityModifier = contributions.Unknown[int]()
	s.expect(rule, unknown, contributions.Depends, "Depends on the off-hand attack and your ability modifier")
}

func (s *weaponEffectRulesSuite) TestMartialArts() {
	rule := NewMartialArtsCondition(MartialArtsInput{MemberID: "rogue", MonkLevel: 5})

	unarmed := mutated(func(a *contributions.ActionFacts) {
		a.Weapon = contributions.Known(refs.Weapons.UnarmedStrike().String())
		a.Finesse = contributions.Known(false)
	})
	answer := s.expect(rule, unarmed, contributions.Applies, "The attack is an unarmed strike")
	s.Equal("Can use Dexterity; deals the 1d6 Martial Arts die", answer.Benefit)

	answer = s.expect(rule, rogueFrame(false), contributions.Applies, "The attack is made with a monk weapon")
	s.Equal("Can use Dexterity", answer.Benefit)

	s.expect(rule, mutated(func(a *contributions.ActionFacts) {
		a.Weapon = contributions.Known(refs.Weapons.Longsword().String())
	}), contributions.DoesNotApply, "Martial Arts needs an unarmed strike or a monk weapon")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.Weapon = contributions.Known("") }),
		contributions.DoesNotApply, "Martial Arts needs an unarmed strike or a monk weapon")
	s.expect(rule, mutated(func(a *contributions.ActionFacts) { a.Weapon = contributions.Unknown[string]() }),
		contributions.Depends, "Depends on the attack's weapon")
}

// TestMartialArtsAgreesWithAssembly: the answer applies exactly where attack
// assembly's override offers Dexterity.
func (s *weaponEffectRulesSuite) TestMartialArtsAgreesWithAssembly() {
	monk := NewMartialArtsCondition(MartialArtsInput{MemberID: "rogue", MonkLevel: 1})
	for _, weapon := range []string{"unarmed-strike", "shortsword", "quarterstaff", "longsword", "greatclub"} {
		s.Run(weapon, func() {
			ref := refs.Weapons.ByID(weapon)
			s.Require().NotNil(ref)
			answer := s.answer(monk, mutated(func(a *contributions.ActionFacts) {
				a.Weapon = contributions.Known(ref.String())
			}))
			overridden := monk.WeaponAttackOverride("main_hand", weapon) != nil
			s.Equal(overridden, answer.Decision.Applicability == contributions.Applies)
		})
	}
}

func (s *weaponEffectRulesSuite) TestShillelagh() {
	rule, err := NewShillelaghCondition("rogue", testShillelaghConfig())
	s.Require().NoError(err)
	club := mutated(func(a *contributions.ActionFacts) {
		a.Weapon = contributions.Known(refs.Weapons.Club().String())
		a.Finesse = contributions.Known(false)
	})

	answer := s.expect(rule, club, contributions.Applies, "The attack is made with the enchanted weapon")
	s.Equal("1d8 magical damage, using Wisdom", answer.Benefit)

	otherHand := club
	otherHand.Action.WeaponSlot = contributions.Known("off_hand")
	s.expect(rule, otherHand, contributions.DoesNotApply, "Shillelagh enchants a different weapon")
	s.expect(rule, rogueFrame(false), contributions.DoesNotApply, "Shillelagh enchants a different weapon")
	unknown := club
	unknown.Action.WeaponSlot = contributions.Unknown[string]()
	s.expect(rule, unknown, contributions.Depends, "Depends on the attack's weapon")

	rule.TurnEndsLeft = 0
	s.expect(rule, club, contributions.DoesNotApply, "Shillelagh has ended")
}

// TestShillelaghAgreesWithAssembly: the answer applies exactly where attack
// assembly's override enchants the swing.
func (s *weaponEffectRulesSuite) TestShillelaghAgreesWithAssembly() {
	rule, err := NewShillelaghCondition("rogue", testShillelaghConfig())
	s.Require().NoError(err)
	for _, tc := range []struct{ slot, item string }{
		{"main_hand", "club"}, {"off_hand", "club"}, {"main_hand", "quarterstaff"},
	} {
		s.Run(tc.slot+"/"+tc.item, func() {
			answer := s.answer(rule, mutated(func(a *contributions.ActionFacts) {
				a.Weapon = contributions.Known(refs.Weapons.ByID(tc.item).String())
				a.WeaponSlot = contributions.Known(tc.slot)
			}))
			overridden := rule.WeaponAttackOverride(tc.slot, tc.item) != nil
			s.Equal(overridden, answer.Decision.Applicability == contributions.Applies)
		})
	}
}

// TestWeaponDamageHandlersRejectAZeroFrame: Dueling, Great Weapon Fighting
// and Two-Weapon Fighting fail the fold on a frame they cannot read.
func (s *weaponEffectRulesSuite) TestWeaponDamageHandlersRejectAZeroFrame() {
	for name, condition := range map[string]dnd5eEvents.ConditionBehavior{
		"dueling":               NewFightingStyleDuelingCondition("rogue"),
		"great weapon fighting": NewFightingStyleGreatWeaponFightingCondition("rogue", nil),
		"two-weapon fighting":   NewFightingStyleTwoWeaponFightingCondition("rogue"),
	} {
		s.Run(name, func() {
			bus := events.NewEventBus()
			s.Require().NoError(condition.Apply(context.Background(), bus))
			event := &dnd5eEvents.DamageChainEvent{
				AttackerID: "rogue", TargetID: "goblin",
				Components: []dnd5eEvents.DamageComponent{{
					Source:     dnd5eEvents.DamageSourceWeapon,
					Properties: []damage.Property{damage.AddsAttackAbilityModifier},
					Roll:       dnd5eEvents.RollComponent{Dice: testDiceTrace(6, 1)},
				}},
			}

			err := s.publishDamage(bus, event)

			s.Require().Error(err)
			s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
			s.Len(event.Components, 1)
		})
	}
}

// TestTwoWeaponFightingAddsTheAnswersModifier: the swing adds the number the
// rule answered with — the frame's modifier, the only one there is.
func (s *weaponEffectRulesSuite) TestTwoWeaponFightingAddsTheAnswersModifier() {
	bus := events.NewEventBus()
	twf := NewFightingStyleTwoWeaponFightingCondition("rogue")
	s.Require().NoError(twf.Apply(context.Background(), bus))
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "rogue", TargetID: "goblin",
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		}},
		Frame: mutated(func(a *contributions.ActionFacts) { a.OffHandAttack = contributions.Known(true) }),
	}

	s.Require().NoError(s.publishDamage(bus, event))

	s.Require().Len(event.Components, 2)
	s.Require().NotNil(event.Components[1].Roll.Modifier)
	s.Equal(3, *event.Components[1].Roll.Modifier, "the frame's modifier, as the row said")
}

// TestWeaponPoolRulesFailClosedWithoutAPrimaryPool: Two-Weapon Fighting and
// Divine Favor apply on the frame's weapon pool, so a fold whose components
// carry no marked primary pool fails instead of silently adding nothing.
func (s *weaponEffectRulesSuite) TestWeaponPoolRulesFailClosedWithoutAPrimaryPool() {
	favor, err := NewDivineFavorCondition(NewDivineFavorConditionInput{
		MemberID: "rogue", SourceID: "rogue", SourceRef: refs.Spells.DivineFavor(),
	})
	s.Require().NoError(err)
	for name, condition := range map[string]dnd5eEvents.ConditionBehavior{
		"two-weapon fighting": NewFightingStyleTwoWeaponFightingCondition("rogue"),
		"divine favor":        favor,
	} {
		s.Run(name, func() {
			bus := events.NewEventBus()
			s.Require().NoError(condition.Apply(context.Background(), bus))
			event := &dnd5eEvents.DamageChainEvent{
				AttackerID: "rogue", TargetID: "goblin",
				Components: []dnd5eEvents.DamageComponent{{Source: dnd5eEvents.DamageSourceSpell}},
				Frame:      mutated(func(a *contributions.ActionFacts) { a.OffHandAttack = contributions.Known(true) }),
			}

			s.Error(s.publishDamage(bus, event))
		})
	}
}

func (s *weaponEffectRulesSuite) TestTwoWeaponFightingNeedsAWeaponPool() {
	s.expect(NewFightingStyleTwoWeaponFightingCondition("rogue"), mutated(func(a *contributions.ActionFacts) {
		a.OffHandAttack = contributions.Known(true)
		a.WeaponPool = contributions.Known(false)
	}), contributions.DoesNotApply, "Two-Weapon Fighting requires a weapon damage pool")
}
