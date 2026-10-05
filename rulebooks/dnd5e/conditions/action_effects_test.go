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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type actionEffectsSuite struct{ suite.Suite }

func TestActionEffectsSuite(t *testing.T) { suite.Run(t, new(actionEffectsSuite)) }

func (s *actionEffectsSuite) blessed(member, source string) *BlessedCondition {
	condition, err := NewBlessedCondition(NewBlessedConditionInput{
		MemberID: member, SourceID: source, SourceRef: refs.Spells.Bless(),
	})
	s.Require().NoError(err)
	return condition
}

func (s *actionEffectsSuite) baned(member, source string) *BanedCondition {
	condition, err := NewBanedCondition(NewBanedConditionInput{
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

func (s *actionEffectsSuite) TestRowsMapAnswersOneToOne() {
	rage := &RagingCondition{CharacterID: "rogue", DamageBonus: 2}
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue", Level: 1})
	frame := rogueFrame(false)
	frame.Action.Ability = contributions.Known(abilities.STR)

	effects := s.assess(frame, rage, sneak)

	s.Require().Len(effects, 2)
	s.Equal(contributions.Effect{
		ID:            refs.Conditions.Raging().String(),
		Source:        contributions.Source{Ref: refs.Conditions.Raging(), Name: "Raging"},
		Description:   displayCatalog[refs.Conditions.Raging().String()].Detail,
		State:         contributions.StateApplies,
		Reason:        "The melee weapon attack uses Strength",
		Participation: contributions.ContributesNow,
		Benefit:       "+2 damage",
	}, effects[0])
	s.Equal(refs.Features.SneakAttack().String(), effects[1].ID)
	s.Equal(contributions.StateDoesNotApply, effects[1].State)
	s.Equal("Sneak Attack requires a Dexterity attack", effects[1].Reason)
	s.Empty(effects[1].Benefit)
}

func (s *actionEffectsSuite) TestSecondBlessDoesNotApplyWithReason() {
	first := s.blessed("rogue", "cleric-a")
	second := s.blessed("rogue", "cleric-b")

	effects := s.assess(rogueFrame(false), first, second)

	s.Require().Len(effects, 2)
	s.Equal(refs.Conditions.Blessed().String()+"@cleric-a", effects[0].ID)
	s.Equal(contributions.StateApplies, effects[0].State)
	s.Equal("+1d4 to the attack roll", effects[0].Benefit)
	s.Equal("cleric-a", effects[0].Source.SourceID)

	s.Equal(refs.Conditions.Blessed().String()+"@cleric-b", effects[1].ID)
	s.Equal(contributions.StateDoesNotApply, effects[1].State)
	s.Equal("Another Bless already adds to this roll", effects[1].Reason)
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
		&RagingCondition{CharacterID: "rogue", DamageBonus: 2},
	}
	frame := rogueFrame(false)

	selected, err := DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{
		Conditions: held,
		Request:    &dnd5eEvents.DescribeRollContributionsInput{Kind: dnd5eEvents.RollKindAttack},
	})
	s.Require().NoError(err)

	effects := s.assess(frame, held...)
	var assessed []contributions.DiceContribution
	for i, effect := range effects {
		if effect.State != contributions.StateApplies {
			continue
		}
		provider, ok := held[i].(dnd5eEvents.RollContributionProvider)
		if !ok {
			continue
		}
		s.Require().Implements((*contributions.ActionAssessor)(nil), provider)
		out, assessErr := held[i].(contributions.ActionAssessor).AssessAction(
			&contributions.AssessActionInput{Frame: frame})
		s.Require().NoError(assessErr)
		assessed = append(assessed, out.Answer.Roll...)
	}

	s.Require().Len(selected.Contributions, 2)
	s.Equal(selected.Contributions, assessed)
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

func (s *actionEffectsSuite) TestInspiredDoesNotApplyToAnothersAttack() {
	inspired := NewInspiredCondition("fighter", "bard", "")

	out, err := inspired.AssessAction(&contributions.AssessActionInput{Frame: rogueFrame(false)})

	s.Require().NoError(err)
	s.Equal(contributions.DoesNotApply, out.Answer.Decision.Applicability)
	s.Equal(contributions.LaterChoice, out.Answer.Participation)
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

func (s *actionEffectsSuite) TestAssessingSpendsNothing() {
	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "rogue", Level: 3})
	inspired := NewInspiredCondition("rogue", "bard", "")
	rage := &RagingCondition{CharacterID: "rogue", DamageBonus: 2, Level: 1}
	held := []dnd5eEvents.ConditionBehavior{sneak, inspired, rage}

	before := make([]json.RawMessage, len(held))
	for i, condition := range held {
		data, err := condition.ToJSON()
		s.Require().NoError(err)
		before[i] = data
	}

	frame := rogueFrame(true)
	frame.Pairs = []contributions.PairFacts{knownPair("goblin", "fighter", 1.0, contributions.StanceHostile)}
	effects := s.assess(frame, held...)
	s.Require().Len(effects, 3)
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

// unclassifiedCondition names a ref no loader or census entry knows.
type unclassifiedCondition struct{}

func (unclassifiedCondition) Ref() *core.Ref {
	return &core.Ref{Module: "dnd5e", Type: "conditions", ID: "unclassified"}
}
func (unclassifiedCondition) IsApplied() bool                               { return false }
func (unclassifiedCondition) Apply(context.Context, events.EventBus) error  { return nil }
func (unclassifiedCondition) Remove(context.Context, events.EventBus) error { return nil }
func (unclassifiedCondition) ToJSON() (json.RawMessage, error)              { return json.RawMessage(`{}`), nil }
