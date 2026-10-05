// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// attackEffectRulesSuite covers the rules that answer for an effect on the
// holder's own attack beyond the first five: each applies, does not apply for
// its stated reason, and depends when the fact it reads is unknown.
type attackEffectRulesSuite struct{ ruleHelpers }

// ruleHelpers asks a rule and publishes attack and damage folds; the rule
// suites embed it.
type ruleHelpers struct{ suite.Suite }

func TestAttackEffectRulesSuite(t *testing.T) { suite.Run(t, new(attackEffectRulesSuite)) }

func (s *ruleHelpers) answer(rule contributions.ActionAssessor, frame contributions.Frame) contributions.Answer {
	out, err := rule.AssessAction(&contributions.AssessActionInput{Frame: frame})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Require().NoError(out.Answer.Decision.Validate())
	s.Equal(contributions.ContributesNow, out.Answer.Participation)
	if out.Answer.Decision.Applicability != contributions.Applies {
		s.Empty(out.Answer.Benefit, "an answer that does not apply carries no benefit")
	}
	return out.Answer
}

func savingThrowFrame() contributions.Frame {
	frame := rogueFrame(false)
	frame.Action.Roll = contributions.Known(contributions.RollKindSavingThrow)
	return frame
}

// rollEffects are the holder-only attack-roll effects, with the answer each
// gives on the rogue's own attack.
func rollEffects() map[string]struct {
	rule        contributions.ActionAssessor
	applies     string
	benefit     string
	notOwner    string
	onlyAttacks string
} {
	return map[string]struct {
		rule        contributions.ActionAssessor
		applies     string
		benefit     string
		notOwner    string
		onlyAttacks string
	}{
		"prone": {
			NewProneCondition("rogue"),
			"You are prone", "Disadvantage on the attack roll",
			"Prone affects only its holder's attacks", "Prone affects only attack rolls",
		},
		"hidden": {
			NewHiddenCondition("rogue"),
			"You are hidden; attacking ends it", "Advantage on the attack roll",
			"Hidden affects only its holder's attacks", "Hidden affects only attack rolls",
		},
		"helped": {
			NewHelpedCondition("rogue", "cleric"),
			"An ally is helping you", "Advantage on the attack roll",
			"Help affects only the helped creature's attack", "Help affects only attack rolls",
		},
		"vicious mockery": {
			NewViciousMockeryCondition("rogue", "bard", refs.Spells.ViciousMockery().String()),
			"You were mocked", "Disadvantage on the attack roll",
			"Vicious Mockery affects only the mocked creature's attack", "Vicious Mockery affects only attack rolls",
		},
		"true strike": {
			NewTrueStrikeCondition("rogue", "goblin", refs.Spells.TrueStrike().String()),
			"This is True Strike's chosen target", "Advantage on the attack roll",
			"True Strike affects only its caster's attack", "True Strike affects only attack rolls",
		},
		"improved critical": {
			NewImprovedCriticalCondition(ImprovedCriticalInput{MemberID: "rogue", Threshold: 18}),
			"Your attacks score a critical hit on a lower roll", "Critical hit on a roll of 18 or higher",
			"Improved Critical affects only its holder's attacks", "Improved Critical affects only attack rolls",
		},
	}
}

func (s *attackEffectRulesSuite) TestRollEffectsApplyToTheHoldersAttack() {
	for name, tc := range rollEffects() {
		s.Run(name, func() {
			answer := s.answer(tc.rule, rogueFrame(false))
			s.Equal(contributions.Applies, answer.Decision.Applicability)
			s.Equal(tc.applies, answer.Decision.Reason)
			s.Equal(tc.benefit, answer.Benefit)
		})
	}
}

func (s *attackEffectRulesSuite) TestRollEffectsDoNotApplyToAnothersAttack() {
	frame := rogueFrame(false)
	frame.Actor = "fighter"
	for name, tc := range rollEffects() {
		s.Run(name, func() {
			answer := s.answer(tc.rule, frame)
			s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
			s.Equal(tc.notOwner, answer.Decision.Reason)
		})
	}
}

func (s *attackEffectRulesSuite) TestRollEffectsDoNotApplyToASavingThrow() {
	for name, tc := range rollEffects() {
		s.Run(name, func() {
			answer := s.answer(tc.rule, savingThrowFrame())
			s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
			s.Equal(tc.onlyAttacks, answer.Decision.Reason)
		})
	}
}

func (s *attackEffectRulesSuite) TestRollEffectsRefuseAnInvalidFrame() {
	for name, tc := range rollEffects() {
		s.Run(name, func() {
			_, err := tc.rule.AssessAction(&contributions.AssessActionInput{})
			s.Require().Error(err)
			s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
			_, err = tc.rule.AssessAction(nil)
			s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
		})
	}
}

func (s *attackEffectRulesSuite) TestTrueStrikeAnswersOnlyForItsTarget() {
	rule := NewTrueStrikeCondition("rogue", "goblin", refs.Spells.TrueStrike().String())

	other := rogueFrame(false)
	other.Target = contributions.Known("orc")
	answer := s.answer(rule, other)
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("True Strike helps only against its chosen target", answer.Decision.Reason)

	unknown := rogueFrame(false)
	unknown.Target = contributions.Unknown[string]()
	answer = s.answer(rule, unknown)
	s.Equal(contributions.Depends, answer.Decision.Applicability)
	s.Equal("Depends on the target", answer.Decision.Reason)
}

// TestArcheryAnswersFromTheWeapon: Archery reads the weapon's category, so a
// ranged spell attack — not melee, no ranged weapon — gets nothing.
func (s *attackEffectRulesSuite) TestArcheryAnswersFromTheWeapon() {
	rule := NewFightingStyleArcheryCondition("rogue")

	ranged := rogueFrame(false)
	ranged.Action.Melee = contributions.Known(false)
	ranged.Action.Weapon = contributions.Known(refs.Weapons.Shortbow().String())
	ranged.Action.Finesse = contributions.Known(false)
	ranged.Action.RangedWeapon = contributions.Known(true)
	answer := s.answer(rule, ranged)
	s.Equal(contributions.Applies, answer.Decision.Applicability)
	s.Equal("The attack is made with a ranged weapon", answer.Decision.Reason)
	s.Equal("+2 to the attack roll", answer.Benefit)

	answer = s.answer(rule, rogueFrame(false))
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Archery adds only to attacks with ranged weapons", answer.Decision.Reason)

	spell := rogueFrame(false)
	spell.Action.Melee = contributions.Known(false)
	spell.Action.WeaponPool = contributions.Known(false)
	spell.Action.Weapon = contributions.Known("")
	spell.Action.Finesse = contributions.Known(false)
	answer = s.answer(rule, spell)
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability, "a ranged spell attack is not a ranged weapon")

	unknown := rogueFrame(false)
	unknown.Action.RangedWeapon = contributions.Unknown[bool]()
	answer = s.answer(rule, unknown)
	s.Equal(contributions.Depends, answer.Decision.Applicability)
	s.Equal("Depends on the attack's weapon", answer.Decision.Reason)

	other := ranged
	other.Actor = "fighter"
	answer = s.answer(rule, other)
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Archery affects only its holder's attacks", answer.Decision.Reason)

	answer = s.answer(rule, savingThrowFrame())
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Archery affects only attack rolls", answer.Decision.Reason)
}

func (s *attackEffectRulesSuite) divineFavor() *DivineFavorCondition {
	favored, err := NewDivineFavorCondition(NewDivineFavorConditionInput{
		MemberID: "rogue", SourceID: "rogue", SourceRef: refs.Spells.DivineFavor(),
	})
	s.Require().NoError(err)
	return favored
}

func (s *attackEffectRulesSuite) TestDivineFavorAnswersFromTheWeaponPool() {
	rule := s.divineFavor()

	answer := s.answer(rule, rogueFrame(false))
	s.Equal(contributions.Applies, answer.Decision.Applicability)
	s.Equal("The attack is a weapon attack", answer.Decision.Reason)
	s.Equal("+1d4 radiant damage", answer.Benefit)

	spell := rogueFrame(false)
	spell.Action.WeaponPool = contributions.Known(false)
	answer = s.answer(rule, spell)
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Divine Favor requires a weapon attack", answer.Decision.Reason)

	unknown := rogueFrame(false)
	unknown.Action.WeaponPool = contributions.Unknown[bool]()
	answer = s.answer(rule, unknown)
	s.Equal(contributions.Depends, answer.Decision.Applicability)
	s.Equal("Depends on the attack's weapon", answer.Decision.Reason)

	other := rogueFrame(false)
	other.Actor = "fighter"
	answer = s.answer(rule, other)
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Divine Favor affects only its caster's attacks", answer.Decision.Reason)

	answer = s.answer(rule, savingThrowFrame())
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Divine Favor adds only to weapon attacks", answer.Decision.Reason)
}

func (s *attackEffectRulesSuite) TestBrutalCriticalAnswersFromTheWeaponPool() {
	rule := NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: "rogue", Level: 9})

	answer := s.answer(rule, rogueFrame(false))
	s.Equal(contributions.Applies, answer.Decision.Applicability)
	s.Equal("The attack has a weapon damage die", answer.Decision.Reason)
	s.Equal("+1 weapon damage die on a critical hit", answer.Benefit)

	answer = s.answer(NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: "rogue", Level: 13}), rogueFrame(false))
	s.Equal("+2 weapon damage dice on a critical hit", answer.Benefit)

	answer = s.answer(NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: "rogue", Level: 5}), rogueFrame(false))
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Brutal Critical adds dice from 9th level", answer.Decision.Reason)

	spell := rogueFrame(false)
	spell.Action.WeaponPool = contributions.Known(false)
	answer = s.answer(rule, spell)
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Brutal Critical requires a weapon damage die", answer.Decision.Reason)

	unknown := rogueFrame(false)
	unknown.Action.WeaponPool = contributions.Unknown[bool]()
	answer = s.answer(rule, unknown)
	s.Equal(contributions.Depends, answer.Decision.Applicability)
	s.Equal("Depends on the attack's weapon", answer.Decision.Reason)

	other := rogueFrame(false)
	other.Actor = "fighter"
	answer = s.answer(rule, other)
	s.Equal(contributions.DoesNotApply, answer.Decision.Applicability)
	s.Equal("Brutal Critical affects only its holder's attacks", answer.Decision.Reason)
}

// TestRowsListTheNewAnswers: the listing maps each answer onto its row, a
// row that does not apply keeps its reason, and the still-unanswering
// effect stays unavailable beside them.
func (s *attackEffectRulesSuite) TestRowsListTheNewAnswers() {
	out, err := AssessActionEffects(&AssessActionEffectsInput{
		Conditions: []dnd5eEvents.ConditionBehavior{
			NewProneCondition("rogue"),
			NewFightingStyleArcheryCondition("rogue"),
			s.sanctuary(),
		},
		Frame: rogueFrame(false),
	})
	s.Require().NoError(err)
	s.Require().Len(out.Effects, 3)

	s.Equal(refs.Conditions.Prone().String(), out.Effects[0].ID)
	s.Equal(contributions.StateApplies, out.Effects[0].State)
	s.Equal("Disadvantage on the attack roll", out.Effects[0].Benefit)

	s.Equal(refs.Conditions.FightingStyleArchery().String(), out.Effects[1].ID)
	s.Equal(contributions.StateDoesNotApply, out.Effects[1].State)
	s.Equal("Archery adds only to attacks with ranged weapons", out.Effects[1].Reason)
	s.Empty(out.Effects[1].Benefit)

	s.Equal(refs.Conditions.Sanctuary().String()+"@cleric", out.Effects[2].ID)
	s.Equal(contributions.StateUnavailable, out.Effects[2].State)
}

func (s *attackEffectRulesSuite) sanctuary() *SanctuaryCondition {
	ward, err := NewSanctuaryCondition(NewSanctuaryConditionInput{
		MemberID: "rogue", SourceID: "cleric", SourceRef: refs.Spells.Sanctuary(),
	})
	s.Require().NoError(err)
	return ward
}

func (s *ruleHelpers) publishAttack(
	bus events.EventBus, event dnd5eEvents.AttackChainEvent,
) (dnd5eEvents.AttackChainEvent, error) {
	ctx := context.Background()
	c := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.AttackChain.On(bus).PublishWithChain(ctx, event, c)
	if err != nil {
		return event, err
	}
	return modified.Execute(ctx, event)
}

// TestAskingDoesNotConsumeTheEffect: reading Help's answer spends nothing,
// so the attack that follows still has the advantage.
func (s *attackEffectRulesSuite) TestAskingDoesNotConsumeTheEffect() {
	bus := events.NewEventBus()
	helped := NewHelpedCondition("rogue", "cleric")
	s.Require().NoError(helped.Apply(context.Background(), bus))

	for range 2 {
		answer := s.answer(helped, rogueFrame(false))
		s.Equal(contributions.Applies, answer.Decision.Applicability)
	}
	s.True(helped.IsApplied())

	final, err := s.publishAttack(bus, framedAttack(dnd5eEvents.AttackChainEvent{AttackerID: "rogue", TargetID: "goblin", IsMelee: true}))
	s.Require().NoError(err)
	s.Require().Len(final.AdvantageSources, 1)
	s.Equal(refs.Conditions.Helped(), final.AdvantageSources[0].SourceRef)
}

// TestArcheryExecutionAgreesWithItsAnswer: the swing adds +2 exactly when the
// rule answers that it applies.
func (s *attackEffectRulesSuite) TestArcheryExecutionAgreesWithItsAnswer() {
	bus := events.NewEventBus()
	archery := NewFightingStyleArcheryCondition("rogue")
	s.Require().NoError(archery.Apply(context.Background(), bus))

	for name, melee := range map[string]bool{"ranged": false, "melee": true} {
		s.Run(name, func() {
			weapon := refs.Weapons.Shortbow()
			if melee {
				weapon = refs.Weapons.Shortsword()
			}
			event := framedAttack(dnd5eEvents.AttackChainEvent{AttackerID: "rogue", TargetID: "goblin", IsMelee: melee, WeaponRef: weapon, AttackBonus: 5})
			answer := s.answer(archery, event.Frame)
			final, err := s.publishAttack(bus, event)
			s.Require().NoError(err)
			if answer.Decision.Applicability == contributions.Applies {
				s.Equal(7, final.AttackBonus)
			} else {
				s.Equal(5, final.AttackBonus)
			}
			s.Equal(!melee, answer.Decision.Applicability == contributions.Applies)
		})
	}
}

// TestTrueStrikeFailsAnAttackWithNoTarget: execution never reads an unknown
// target as "not the chosen one".
func (s *attackEffectRulesSuite) TestTrueStrikeFailsAnAttackWithNoTarget() {
	bus := events.NewEventBus()
	strike := NewTrueStrikeCondition("rogue", "goblin", refs.Spells.TrueStrike().String())
	s.Require().NoError(strike.Apply(context.Background(), bus))

	_, err := s.publishAttack(bus, framedAttack(dnd5eEvents.AttackChainEvent{AttackerID: "rogue", IsMelee: true}))

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
	s.True(strike.IsApplied(), "a failed attack does not consume True Strike")
}

// TestAttackChainHandlersRejectAMissingFrame: every attack-chain handler that
// asks a rule fails the attack on an event with no frame rather than switching
// itself off, and adds nothing.
func (s *attackEffectRulesSuite) TestAttackChainHandlersRejectAMissingFrame() {
	for name, condition := range map[string]dnd5eEvents.ConditionBehavior{
		"prone":             NewProneCondition("rogue"),
		"hidden":            NewHiddenCondition("rogue"),
		"helped":            NewHelpedCondition("rogue", "cleric"),
		"vicious mockery":   NewViciousMockeryCondition("rogue", "bard", refs.Spells.ViciousMockery().String()),
		"true strike":       NewTrueStrikeCondition("rogue", "goblin", refs.Spells.TrueStrike().String()),
		"improved critical": NewImprovedCriticalCondition(ImprovedCriticalInput{MemberID: "rogue", Threshold: 19}),
		"archery":           NewFightingStyleArcheryCondition("rogue"),
		"reckless attack":   NewRecklessAttackCondition("rogue"),
	} {
		s.Run(name, func() {
			bus := events.NewEventBus()
			s.Require().NoError(condition.Apply(context.Background(), bus))

			final, err := s.publishAttack(bus, dnd5eEvents.AttackChainEvent{
				AttackerID: "rogue", TargetID: "goblin", IsMelee: false, AttackBonus: 5, CriticalThreshold: 20,
			})

			s.Require().Error(err)
			s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
			s.Empty(final.AdvantageSources)
			s.Empty(final.DisadvantageSources)
			s.Equal(5, final.AttackBonus)
			s.Equal(20, final.CriticalThreshold)
		})
	}
}

func (s *ruleHelpers) publishDamage(bus events.EventBus, event *dnd5eEvents.DamageChainEvent) error {
	ctx := context.Background()
	c := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(bus).PublishWithChain(ctx, event, c)
	if err != nil {
		return err
	}
	_, err = modified.Execute(ctx, event)
	return err
}

// TestDamageHandlersRejectAZeroFrame: Divine Favor and Brutal Critical fail
// the fold on a frame they cannot read instead of switching themselves off.
func (s *attackEffectRulesSuite) TestDamageHandlersRejectAZeroFrame() {
	for name, condition := range map[string]dnd5eEvents.ConditionBehavior{
		"divine favor":    s.divineFavor(),
		"brutal critical": NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: "rogue", Level: 9}),
	} {
		s.Run(name, func() {
			bus := events.NewEventBus()
			s.Require().NoError(condition.Apply(context.Background(), bus))
			event := &dnd5eEvents.DamageChainEvent{
				AttackerID: "rogue", TargetID: "goblin", IsCritical: true,
				WeaponDamageDice: "1d8",
				Components: []dnd5eEvents.DamageComponent{{
					Source:     dnd5eEvents.DamageSourceWeapon,
					Properties: []damage.Property{damage.AddsAttackAbilityModifier},
				}},
			}

			err := s.publishDamage(bus, event)

			s.Require().Error(err)
			s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
			s.Len(event.Components, 1)
		})
	}
}
