// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type actionEffectsSuite struct{ suite.Suite }

func TestActionEffectsSuite(t *testing.T) { suite.Run(t, new(actionEffectsSuite)) }

func (s *actionEffectsSuite) blessed(member, source string) *LinkedCondition {
	condition, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Blessed(),
		MemberID: member, SourceID: source, SourceRef: refs.Spells.Bless(),
	})
	s.Require().NoError(err)
	return condition
}

func (s *actionEffectsSuite) baned(member, source string) *LinkedCondition {
	condition, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Baned(),
		MemberID: member, SourceID: source, SourceRef: refs.Spells.Bane(),
	})
	s.Require().NoError(err)
	return condition
}

func (s *actionEffectsSuite) assess(frame contributions.Frame, held ...dnd5eEvents.ConditionBehavior) []contributions.Effect {
	out, err := AssessActionEffects(&AssessActionEffectsInput{Conditions: held, Frame: frame})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Require().NotNil(out.Effects, "an empty listing is empty, not nil")
	return out.Effects
}

func (s *actionEffectsSuite) TestRowsKeepComposedOutputsAndSingleAnswersDistinct() {
	rage := composedRageForTest("rogue")
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue"})
	frame := rogueFrame(false)
	frame.ActorClassLevels = rogueBarbarianLevels()
	frame.Action.Ability = contributions.Known(abilities.STR)
	frame.Action.Weapon = contributions.Known(refs.Weapons.Club().String())
	frame.Action.Finesse = contributions.Known(false)

	effects := s.assess(frame, rage, sneak)

	s.Require().Len(effects, 5)
	ids := map[string]bool{}
	for i, row := range effects[:4] {
		s.False(ids[row.ID])
		ids[row.ID] = true
		s.Equal(refs.Conditions.Raging(), row.Source.Ref)
		s.Equal("Raging", row.Source.Name)
		s.Equal("rogue", row.Source.SourceID)
		s.NotEmpty(row.Description)
		s.NotEmpty(row.Reason)
		s.Equal(contributions.ContributesNow, row.Participation)
		if i == 0 {
			s.Equal(contributions.StateApplies, row.State)
			s.Equal("+2 damage", row.Benefit)
		} else {
			s.Equal(contributions.StateDoesNotApply, row.State)
			s.Empty(row.Benefit)
		}
	}
	s.Equal(refs.Conditions.SneakAttack().String(), effects[4].ID)
	s.Equal(contributions.StateDoesNotApply, effects[4].State)
	s.Equal("Sneak Attack requires a finesse or ranged weapon", effects[4].Reason)
	s.Empty(effects[4].Benefit)
}

func (s *actionEffectsSuite) TestSecondBlessDoesNotApplyWithReason() {
	first := s.blessed("rogue", "cleric-a")
	second := s.blessed("rogue", "cleric-b")

	effects := s.assess(rogueFrame(false), first, second)

	s.Require().Len(effects, 2)
	s.NotEmpty(effects[0].ID)
	s.Equal(refs.Conditions.Blessed().String(), effects[0].Source.Ref.String())
	s.Equal(contributions.StateApplies, effects[0].State)
	s.Equal("+1d4 to the attack roll", effects[0].Benefit)
	s.Equal("cleric-a", effects[0].Source.SourceID)

	s.NotEqual(effects[0].ID, effects[1].ID)
	s.Equal("cleric-b", effects[1].Source.SourceID)
	s.Equal(contributions.StateDoesNotApply, effects[1].State)
	s.Contains(effects[1].Reason, "Not selected")
	s.Contains(effects[1].Reason, "cleric-a", "the explanation identifies the older source")
	s.Empty(effects[1].Benefit)
}

func (s *actionEffectsSuite) TestBaneRowSubtracts() {
	effects := s.assess(rogueFrame(false), s.baned("rogue", "cultist"))

	s.Require().Len(effects, 1)
	s.Equal(contributions.StateApplies, effects[0].State)
	s.Equal("−1d4 to the attack roll", effects[0].Benefit)
}

func (s *actionEffectsSuite) TestSelectedRollContributionsMatchAssessedApplies() {
	held := []dnd5eEvents.ConditionBehavior{
		s.blessed("rogue", "cleric-a"),
		s.baned("rogue", "cultist-a"),
		s.blessed("rogue", "cleric-b"),
		s.baned("rogue", "cultist-b"),
		composedRageForTest("rogue"),
	}
	frame := rogueFrame(false)
	frame.ActorClassLevels = rogueBarbarianLevels()

	selected, err := DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{
		Conditions: held,
		Request:    &dnd5eEvents.DescribeRollContributionsInput{Kind: dnd5eEvents.RollKindAttack},
	})
	s.Require().NoError(err)

	effects := s.assess(frame, held...)
	var assessedSources, executedSources []string
	for _, effect := range effects {
		if effect.Selection != nil && effect.State == contributions.StateApplies {
			assessedSources = append(assessedSources, effect.Source.SourceID)
		}
	}
	for _, contribution := range selected.Contributions {
		executedSources = append(executedSources, contribution.Source.SourceID)
	}
	s.Equal([]string{"cleric-a", "cultist-a"}, assessedSources)
	s.Equal([]string{"cleric-a", "cultist-a"}, executedSources)
	s.Require().Len(selected.Contributions, 2)
	s.False(selected.Contributions[0].Subtract)
	s.True(selected.Contributions[1].Subtract)
}

func (s *actionEffectsSuite) TestInspiredAnswersLaterChoiceOnHoldersAttack() {
	inspired := NewInspiredCondition("rogue", "bard", "")

	effects := s.assess(rogueFrame(false), inspired)

	s.Require().Len(effects, 1)
	s.Equal(contributions.StateApplies, effects[0].State)
	s.Equal(contributions.LaterChoice, effects[0].Participation)
	s.Equal("May add 1d6 after seeing the roll", effects[0].Benefit)
	s.Equal(refs.Conditions.Inspired().String(), effects[0].ID)
}

// TestParticipationIsTheDescriptors: how an effect takes part is declared once,
// on its condition's descriptor. A rule's answer carries none, so no two
// declarations can disagree at runtime.
func (s *actionEffectsSuite) TestParticipationIsTheDescriptors() {
	inspired := NewInspiredCondition("rogue", "bard", "")
	blessed := s.blessed("rogue", "cleric")

	effects := s.assess(rogueFrame(false), inspired, blessed)

	s.Require().Len(effects, 2)
	s.Equal(refs.Conditions.Inspired().String(), effects[0].ID)
	s.Equal(contributions.LaterChoice, effects[0].Participation, "Inspired is offered later")
	s.NotEmpty(effects[1].ID)
	s.Equal("cleric", effects[1].Source.SourceID)
	s.Equal(contributions.ContributesNow, effects[1].Participation, "Blessed joins the roll now")
}

func (s *actionEffectsSuite) TestInspiredDoesNotApplyToAnothersAttack() {
	inspired := NewInspiredCondition("fighter", "bard", "")

	out, err := inspired.AssessAction(&contributions.AssessActionInput{Frame: rogueFrame(false)})

	s.Require().NoError(err)
	s.Equal(contributions.DoesNotApply, out.Answer.Decision.Applicability)
	s.Empty(out.Answer.Benefit)
}

func (s *actionEffectsSuite) TestInspiredOfferHandlerFailsOnZeroFrame() {
	ctx := context.Background()
	bus := events.NewEventBus()
	inspired := NewInspiredCondition("rogue", "bard", "")
	s.Require().NoError(inspired.Apply(ctx, bus))

	event := &dnd5eEvents.PostRollOfferEvent{AttackerID: "rogue", TargetID: "goblin", Roll: 12}
	chain := events.NewStagedChain[*dnd5eEvents.PostRollOfferEvent](combat.ModifierStages)
	_, err := dnd5eEvents.PostRollOfferChain.On(bus).PublishWithChain(ctx, event, chain)

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
	s.Empty(event.Offers)
}

func (s *actionEffectsSuite) TestInspiredOfferHandlerFailsOnUnrecognisedRollKind() {
	for _, roll := range []contributions.RollKind{"", "atack"} {
		ctx := context.Background()
		bus := events.NewEventBus()
		inspired := NewInspiredCondition("rogue", "bard", "")
		s.Require().NoError(inspired.Apply(ctx, bus))

		frame := rogueFrame(true)
		frame.Action.Roll = contributions.Known(roll)
		event := &dnd5eEvents.PostRollOfferEvent{AttackerID: "rogue", TargetID: "goblin", Roll: 12, Frame: frame}
		chain := events.NewStagedChain[*dnd5eEvents.PostRollOfferEvent](combat.ModifierStages)
		_, err := dnd5eEvents.PostRollOfferChain.On(bus).PublishWithChain(ctx, event, chain)

		s.Require().Error(err, "roll kind %q fails the action instead of silently withholding the die", roll)
		s.True(errors.Is(err, contributions.ErrRuleCannotAnswer))
		s.Empty(event.Offers)
	}
}

func (s *actionEffectsSuite) TestBlessAndBaneDoNotApplyToAnotherMembersAttack() {
	frame := rogueFrame(false)
	frame.Actor = "fighter"
	for name, condition := range map[string]contributions.ActionEffectsAssessor{
		"Bless": s.blessed("rogue", "cleric"),
		"Bane":  s.baned("rogue", "cultist"),
	} {
		out, err := condition.AssessActionEffects(&contributions.AssessActionInput{Frame: frame})
		s.Require().NoError(err, name)
		s.Require().Len(out.Effects, 1)
		s.Equal(contributions.StateDoesNotApply, out.Effects[0].State, name)
		s.Contains(out.Effects[0].Reason, "recipient")
		s.Empty(out.Effects[0].Benefit, name)
	}

	effects := s.assess(frame, s.blessed("rogue", "cleric"))
	s.Require().Len(effects, 1)
	s.Equal(contributions.StateDoesNotApply, effects[0].State,
		"another member's Bless in the listing reads does-not-apply, not an error")
}

func (s *actionEffectsSuite) TestAssessingSpendsNothing() {
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue"})
	inspired := NewInspiredCondition("rogue", "bard", "")
	rage := composedRageForTest("rogue")
	held := []dnd5eEvents.ConditionBehavior{sneak, inspired, rage}

	before := make([]json.RawMessage, len(held))
	for i, condition := range held {
		data, err := condition.ToJSON()
		s.Require().NoError(err)
		before[i] = data
	}

	frame := rogueFrame(true)
	frame.ActorClassLevels = rogueBarbarianLevels()
	frame.Pairs = []contributions.PairFacts{knownPair("goblin", "fighter", 1.0, contributions.StanceHostile)}
	effects := s.assess(frame, held...)
	s.Require().Len(effects, 6)
	s.Equal(contributions.StateApplies, effects[0].State, "Sneak Attack applies and is still unspent")

	for i, condition := range held {
		after, err := condition.ToJSON()
		s.Require().NoError(err)
		s.Equal(string(before[i]), string(after), "%T changed while being read", condition)
		s.False(condition.IsApplied())
	}
}

func (s *actionEffectsSuite) TestNotBearingYieldsNoRowAndEmptyIsNotNil() {
	effects := s.assess(rogueFrame(false), &UnarmoredDefenseCondition{MemberID: "rogue"})
	s.Empty(effects)
	s.Empty(s.assess(rogueFrame(false)))
}

func (s *actionEffectsSuite) TestRefusesInvalidInput() {
	for name, in := range map[string]*AssessActionEffectsInput{
		"nil input":  nil,
		"zero frame": {Conditions: []dnd5eEvents.ConditionBehavior{s.blessed("rogue", "cleric")}},
		"unknown ref": {
			Conditions: []dnd5eEvents.ConditionBehavior{unclassifiedCondition{}},
			Frame:      rogueFrame(false),
		},
		"nil condition": {Conditions: []dnd5eEvents.ConditionBehavior{nil}, Frame: rogueFrame(false)},
		"duplicate id": {
			Conditions: []dnd5eEvents.ConditionBehavior{s.blessed("rogue", "cleric"), s.blessed("rogue", "cleric")},
			Frame:      rogueFrame(false),
		},
	} {
		out, err := AssessActionEffects(in)
		s.Error(err, name)
		s.Nil(out, name)
	}
}

// unclassifiedCondition names a ref the registry does not hold.
type unclassifiedCondition struct{}

func (unclassifiedCondition) Ref() *core.Ref {
	return &core.Ref{Module: "dnd5e", Type: "conditions", ID: "unclassified"}
}
func (unclassifiedCondition) IsApplied() bool                               { return false }
func (unclassifiedCondition) Apply(context.Context, events.EventBus) error  { return nil }
func (unclassifiedCondition) Remove(context.Context, events.EventBus) error { return nil }
func (unclassifiedCondition) ToJSON() (json.RawMessage, error)              { return json.RawMessage(`{}`), nil }

// rogueBarbarianLevels is the fixture rogue's class levels when it also holds
// a Rage: one rogue level and one barbarian level.
func rogueBarbarianLevels() contributions.ClassLevels {
	return contributions.KnownClassLevels(
		contributions.ClassLevel{Class: classes.Rogue, Levels: 1},
		contributions.ClassLevel{Class: classes.Barbarian, Levels: 1})
}
