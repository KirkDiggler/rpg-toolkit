// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// RageDescribedSuite pins that a barbarian's sheet answers Rage's description
// with her own numbers, that Rage is used from it, that its own code refuses
// and touches nothing, and that her row in the list of abilities is as it was
// (rpg-project#1131).
type RageDescribedSuite struct {
	suite.Suite

	ctx  context.Context
	bus  events.EventBus
	char *Character
}

func TestRageDescribedSuite(t *testing.T) { suite.Run(t, new(RageDescribedSuite)) }

func (s *RageDescribedSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	char, err := newBarbarianDraft(s.T()).ToCharacter(s.ctx, "described-barbarian", s.bus)
	s.Require().NoError(err)
	s.char = char
	_, err = s.char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	s.Require().Equal(1, s.char.actionEconomy.BonusActionsRemaining)
}

func (s *RageDescribedSuite) ragingCount() int {
	count := 0
	for _, condition := range s.char.GetConditions() {
		if condition.Ref().String() == refs.Conditions.Raging().String() {
			count++
		}
	}
	return count
}

func (s *RageDescribedSuite) rageRow() AvailableAbility {
	for _, ability := range s.char.buildAvailableAbilities() {
		if ability.Ref.ID == refs.Features.Rage().ID {
			return ability
		}
	}
	s.FailNow("rage is not among the barbarian's abilities")
	return AvailableAbility{}
}

func (s *RageDescribedSuite) TestALevelOneBarbariansDescription() {
	definition, ok, err := s.char.AbilityDefinition(*refs.Features.Rage())

	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(definition)
	s.Equal(*refs.Features.Rage(), definition.Ref)
	s.Require().NotNil(definition.Cost)
	s.Equal(map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, definition.Cost.Slots)
	s.Equal(map[coreResources.ResourceKey]int{resources.RageCharges: 1}, definition.Cost.Pools)
	s.Require().NotNil(definition.Effect)
	s.Equal(actions.EffectSelf, definition.Effect.To)
	s.Nil(definition.Effect.Healing)
	s.Require().NotNil(definition.Effect.Condition)
	s.Equal(*refs.Conditions.Raging(), definition.Effect.Condition.Ref)
	s.JSONEq(`{"damage_bonus":{"barbarian_level":true}}`, string(definition.Effect.Condition.Parameters))
	s.Require().NoError(definition.Validate())
}

func (s *RageDescribedSuite) TestAtLevel20TheDescriptionNamesNoPool() {
	rage, err := features.LoadJSON(json.RawMessage(`{"ref":{"module":"dnd5e","type":"features","id":"rage"},"id":"rage-1","name":"Rage"}`))
	s.Require().NoError(err)
	barbarian := &Character{
		id: "b20", classID: classes.Barbarian, levels: syntheticLevels(classes.Barbarian, 20),
		features: []features.Feature{rage},
	}

	definition, ok, err := barbarian.AbilityDefinition(*refs.Features.Rage())

	s.Require().NoError(err)
	s.Require().True(ok)
	s.Require().NotNil(definition.Cost)
	s.Equal(map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, definition.Cost.Slots)
	s.Empty(definition.Cost.Pools, "unlimited rages: no charge is named")
}

func (s *RageDescribedSuite) TestARageAlreadyRagingIsRefusedWithTheRowsSentence() {
	s.char.actionEconomy.BonusActionsRemaining = 2 // so the refusal is Rage's alone
	s.Require().NoError(useRageFromItsDescription(s.ctx, s.T(), s.char, s.bus))
	s.Require().Equal(1, s.ragingCount())

	definition, ok, err := s.char.AbilityDefinition(*refs.Features.Rage())

	s.Require().Error(err)
	s.False(ok)
	s.Nil(definition)
	s.Contains(err.Error(), "already raging")
	s.Contains(err.Error(), s.rageRow().Reason, "the description and the row refuse in the same words")
}

func (s *RageDescribedSuite) TestAnOwnerWithNoBarbarianLevelIsRefused() {
	rage, err := features.LoadJSON(json.RawMessage(`{"ref":{"module":"dnd5e","type":"features","id":"rage"},"id":"rage-1","name":"Rage"}`))
	s.Require().NoError(err)
	fighter := &Character{
		id: "f1", classID: classes.Fighter, levels: syntheticLevels(classes.Fighter, 3),
		features: []features.Feature{rage},
	}

	definition, ok, err := fighter.AbilityDefinition(*refs.Features.Rage())

	s.Require().Error(err)
	s.False(ok)
	s.Nil(definition)
	s.Contains(err.Error(), "owner holds no barbarian levels")
}

func (s *RageDescribedSuite) TestNoChargeLeftStillGetsADescriptionAndTheGateRefusesThePayment() {
	s.Require().NoError(s.char.UseResource(resources.RageCharges, 2))
	s.Require().Equal(0, s.char.GetResource(resources.RageCharges).Current())

	definition, ok, err := s.char.AbilityDefinition(*refs.Features.Rage())

	s.Require().NoError(err, "a want of a charge is the gate's to refuse")
	s.Require().True(ok)
	s.Require().Error(combat.Pay(s.char, definition.Cost))
	s.Equal(1, s.char.actionEconomy.BonusActionsRemaining, "the bonus action was not spent either")
	s.Equal(0, s.char.GetResource(resources.RageCharges).Current())
}

func (s *RageDescribedSuite) TestUsedFromItsDescription() {
	s.Require().Equal(2, s.char.GetResource(resources.RageCharges).Current())

	s.Require().NoError(useRageFromItsDescription(s.ctx, s.T(), s.char, s.bus))

	s.Equal(0, s.char.actionEconomy.BonusActionsRemaining, "her bonus action is spent")
	s.Equal(1, s.char.GetResource(resources.RageCharges).Current(), "her charges go 2 to 1")
	s.Require().Equal(1, s.ragingCount())
	for _, condition := range s.char.GetConditions() {
		if condition.Ref().String() != refs.Conditions.Raging().String() {
			continue
		}
		composed, ok := condition.(*conditions.ComposedCondition)
		s.Require().True(ok)
		raw, err := composed.ToJSON()
		s.Require().NoError(err)
		var saved conditions.ComposedData
		s.Require().NoError(json.Unmarshal(raw, &saved))
		s.Equal(s.char.GetID(), saved.MemberID)
		s.Require().NotNil(saved.Origin)
		s.Equal(*refs.Features.Rage(), saved.Origin.Ref)
		s.Equal(s.char.GetID(), saved.Origin.EntityID)
	}
}

func (s *RageDescribedSuite) TestTheFeaturesOwnCodeRefusesAndTouchesNothing() {
	published := 0
	_, err := dnd5eEvents.ConditionAppliedTopic.On(s.bus).Subscribe(s.ctx,
		func(context.Context, dnd5eEvents.ConditionAppliedEvent) error { published++; return nil })
	s.Require().NoError(err)
	rage := s.char.GetFeature("rage")
	s.Require().NotNil(rage)

	direct := rage.Activate(s.ctx, s.char, features.FeatureInput{Bus: s.bus})
	out, err := s.char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: refs.Features.Rage()})

	s.Require().Error(direct)
	s.True(errors.Is(direct, features.ErrUsedFromItsDescription), "got %v", direct)
	s.Require().NoError(err)
	s.False(out.Success)
	s.Equal(features.ErrUsedFromItsDescription.Error(), out.Error)
	s.Equal(2, s.char.GetResource(resources.RageCharges).Current(), "no charge moved")
	s.Equal(1, s.char.actionEconomy.BonusActionsRemaining, "no bonus action moved")
	s.Zero(s.ragingCount(), "no status was put on her")
	s.Zero(published, "nothing was published")
}

func (s *RageDescribedSuite) TestHerRowIsAsItWas() {
	s.char.actionEconomy.BonusActionsRemaining = 2 // so a refusal after raging is Rage's alone
	row := s.rageRow()
	s.Equal(coreCombat.ActionBonus, row.ActionType)
	s.True(row.CanUse, row.Reason)
	s.Equal(2, row.ResourceCurrent)
	s.Equal(2, row.ResourceMax)
	s.Equal(TargetKindSelf, row.TargetKind)

	s.Require().NoError(useRageFromItsDescription(s.ctx, s.T(), s.char, s.bus))
	raging := s.rageRow()
	s.False(raging.CanUse)
	s.Contains(raging.Reason, "already raging")
	s.Equal(1, raging.ResourceCurrent, "her charges are still shown")
}

func (s *RageDescribedSuite) TestHerRowReadsUnavailableWithNoCharge() {
	s.Require().NoError(s.char.UseResource(resources.RageCharges, 2))
	s.char.actionEconomy.BonusActionsRemaining = 1

	row := s.rageRow()

	s.False(row.CanUse)
	s.Contains(row.Reason, "no rage uses remaining")
	s.Equal(0, row.ResourceCurrent)
}
