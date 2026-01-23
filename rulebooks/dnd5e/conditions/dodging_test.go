// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type DodgingConditionTestSuite struct {
	suite.Suite
	ctx         context.Context
	bus         events.EventBus
	condition   *DodgingCondition
	characterID string
}

func TestDodgingConditionSuite(t *testing.T) {
	suite.Run(t, new(DodgingConditionTestSuite))
}

func (s *DodgingConditionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.characterID = "char-dodging"
	s.condition = NewDodgingCondition(s.characterID)
}

func (s *DodgingConditionTestSuite) SetupSubTest() {
	s.bus = events.NewEventBus()
}

func (s *DodgingConditionTestSuite) TestNewDodgingCondition() {
	s.Assert().Equal(s.characterID, s.condition.CharacterID)
	s.Assert().False(s.condition.IsApplied())
}

func (s *DodgingConditionTestSuite) TestApply() {
	s.Run("applies successfully", func() {
		err := s.condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		s.Assert().True(s.condition.IsApplied())
		s.Assert().Len(s.condition.subscriptionIDs, 3)
	})

	s.Run("returns error if already applied", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		err = condition.Apply(s.ctx, s.bus)
		s.Assert().Error(err)
		s.Assert().Contains(err.Error(), "already applied")
	})
}

func (s *DodgingConditionTestSuite) TestRemove() {
	s.Run("removes successfully after apply", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		err = condition.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
		s.Assert().False(condition.IsApplied())
		s.Assert().Nil(condition.subscriptionIDs)
	})

	s.Run("no-op if not applied", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
	})
}

func (s *DodgingConditionTestSuite) TestAttackChainDisadvantage() {
	s.Run("adds disadvantage when character is targeted", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		attackEvent := dnd5eEvents.AttackChainEvent{
			AttackerID: "attacker-1",
			TargetID:   s.characterID,
			IsMelee:    true,
		}

		attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
		attacks := dnd5eEvents.AttackChain.On(s.bus)
		modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
		s.Require().NoError(err)
		s.Assert().Len(finalEvent.DisadvantageSources, 1)
		s.Assert().Equal(refs.Conditions.Dodging(), finalEvent.DisadvantageSources[0].SourceRef)
		s.Assert().Equal("Dodging", finalEvent.DisadvantageSources[0].Reason)
	})

	s.Run("adds disadvantage for ranged attacks too", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		attackEvent := dnd5eEvents.AttackChainEvent{
			AttackerID: "attacker-1",
			TargetID:   s.characterID,
			IsMelee:    false,
		}

		attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
		attacks := dnd5eEvents.AttackChain.On(s.bus)
		modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
		s.Require().NoError(err)
		s.Assert().Len(finalEvent.DisadvantageSources, 1)
	})

	s.Run("does not add disadvantage when another character is targeted", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		attackEvent := dnd5eEvents.AttackChainEvent{
			AttackerID: "attacker-1",
			TargetID:   "other-character",
			IsMelee:    true,
		}

		attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
		attacks := dnd5eEvents.AttackChain.On(s.bus)
		modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
		s.Require().NoError(err)
		s.Assert().Empty(finalEvent.DisadvantageSources)
	})
}

func (s *DodgingConditionTestSuite) TestSavingThrowChainAdvantage() {
	s.Run("adds advantage on DEX saves for this character", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		saveEvent := &dnd5eEvents.SavingThrowChainEvent{
			SaverID: s.characterID,
			Ability: abilities.DEX,
			DC:      15,
		}

		saveChain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)
		saves := dnd5eEvents.SavingThrowChain.On(s.bus)
		modifiedChain, err := saves.PublishWithChain(s.ctx, saveEvent, saveChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, saveEvent)
		s.Require().NoError(err)
		s.Assert().Len(finalEvent.AdvantageSources, 1)
		s.Assert().Equal(refs.Conditions.Dodging(), finalEvent.AdvantageSources[0].SourceRef)
		s.Assert().Equal("Dodging", finalEvent.AdvantageSources[0].Name)
	})

	s.Run("does not add advantage on non-DEX saves", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		saveEvent := &dnd5eEvents.SavingThrowChainEvent{
			SaverID: s.characterID,
			Ability: abilities.CON,
			DC:      15,
		}

		saveChain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)
		saves := dnd5eEvents.SavingThrowChain.On(s.bus)
		modifiedChain, err := saves.PublishWithChain(s.ctx, saveEvent, saveChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, saveEvent)
		s.Require().NoError(err)
		s.Assert().Empty(finalEvent.AdvantageSources)
	})

	s.Run("does not add advantage for other characters", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		saveEvent := &dnd5eEvents.SavingThrowChainEvent{
			SaverID: "other-character",
			Ability: abilities.DEX,
			DC:      15,
		}

		saveChain := events.NewStagedChain[*dnd5eEvents.SavingThrowChainEvent](combat.ModifierStages)
		saves := dnd5eEvents.SavingThrowChain.On(s.bus)
		modifiedChain, err := saves.PublishWithChain(s.ctx, saveEvent, saveChain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(s.ctx, saveEvent)
		s.Require().NoError(err)
		s.Assert().Empty(finalEvent.AdvantageSources)
	})
}

func (s *DodgingConditionTestSuite) TestTurnStartRemoval() {
	s.Run("removes condition on character turn start", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		s.Assert().True(condition.IsApplied())

		// Track condition removed event
		var removedEvent *dnd5eEvents.ConditionRemovedEvent
		_, err = dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removedEvent = &event
			return nil
		})
		s.Require().NoError(err)

		// Publish turn start for this character
		err = dnd5eEvents.TurnStartTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.TurnStartEvent{
			CharacterID: s.characterID,
			Round:       1,
		})
		s.Require().NoError(err)

		// Condition should be removed
		s.Assert().False(condition.IsApplied())
		s.Require().NotNil(removedEvent)
		s.Assert().Equal(s.characterID, removedEvent.CharacterID)
		s.Assert().Equal(refs.Conditions.Dodging().String(), removedEvent.ConditionRef)
		s.Assert().Equal("turn_start", removedEvent.Reason)
	})

	s.Run("does not remove on other character turn start", func() {
		condition := NewDodgingCondition(s.characterID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Publish turn start for different character
		err = dnd5eEvents.TurnStartTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.TurnStartEvent{
			CharacterID: "other-character",
			Round:       1,
		})
		s.Require().NoError(err)

		// Condition should still be applied
		s.Assert().True(condition.IsApplied())
	})
}

func (s *DodgingConditionTestSuite) TestToJSON() {
	condition := NewDodgingCondition(s.characterID)
	data, err := condition.ToJSON()
	s.Require().NoError(err)

	// Load it back
	loaded := &DodgingCondition{}
	err = loaded.loadJSON(data)
	s.Require().NoError(err)
	s.Assert().Equal(s.characterID, loaded.CharacterID)
}
