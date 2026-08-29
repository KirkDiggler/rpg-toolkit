// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type HelpedConditionTestSuite struct {
	suite.Suite
	ctx         context.Context
	bus         events.EventBus
	condition   *HelpedCondition
	characterID string
	helperID    string
}

func TestHelpedConditionSuite(t *testing.T) {
	suite.Run(t, new(HelpedConditionTestSuite))
}

func (s *HelpedConditionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.characterID = "ally-1"
	s.helperID = "helper-1"
	s.condition = NewHelpedCondition(s.characterID, s.helperID)
}

func (s *HelpedConditionTestSuite) TestNewHelpedCondition() {
	s.Equal(s.characterID, s.condition.CharacterID)
	s.Equal(s.helperID, s.condition.HelperID)
	s.False(s.condition.IsApplied())
}

func (s *HelpedConditionTestSuite) TestApply() {
	s.Run("applies successfully", func() {
		err := s.condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		s.True(s.condition.IsApplied())
		s.Len(s.condition.subscriptionIDs, 2)
	})

	s.Run("returns error if already applied", func() {
		condition := NewHelpedCondition(s.characterID, s.helperID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		err = condition.Apply(s.ctx, s.bus)
		s.Require().Error(err)
		s.Contains(err.Error(), "already applied")
	})
}

func (s *HelpedConditionTestSuite) TestRemove() {
	s.Run("removes successfully after apply", func() {
		condition := NewHelpedCondition(s.characterID, s.helperID)
		err := condition.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		err = condition.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
		s.False(condition.IsApplied())
	})

	s.Run("no-op if not applied", func() {
		condition := NewHelpedCondition(s.characterID, s.helperID)
		err := condition.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
	})
}

func (s *HelpedConditionTestSuite) TestAttackChain_AllyAttackGetsAdvantageAndConsumes() {
	condition := NewHelpedCondition(s.characterID, s.helperID)
	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	_, err = dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removedEvent = &event
			return nil
		})
	s.Require().NoError(err)

	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: s.characterID,
		TargetID:   "goblin-1",
	}
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	s.Require().Len(finalEvent.AdvantageSources, 1, "the helped ally's attack gets advantage")
	s.Equal(refs.Conditions.Helped(), finalEvent.AdvantageSources[0].SourceRef)
	s.Equal(s.helperID, finalEvent.AdvantageSources[0].SourceID, "advantage is attributed to the helper")

	s.False(condition.IsApplied(), "Helped is consumed on the ally's attack")
	s.Require().NotNil(removedEvent)
	s.Equal(s.characterID, removedEvent.MemberID)
	s.Equal("consumed", removedEvent.Reason)
}

func (s *HelpedConditionTestSuite) TestAttackChain_OtherCharacterAttackIsUntouched() {
	condition := NewHelpedCondition(s.characterID, s.helperID)
	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	attackEvent := dnd5eEvents.AttackChainEvent{
		AttackerID: "someone-else",
		TargetID:   "goblin-1",
	}
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)

	s.Empty(finalEvent.AdvantageSources)
	s.True(condition.IsApplied())
}

func (s *HelpedConditionTestSuite) TestTurnStartRemoval_SafetyNetAtHelpersTurn() {
	condition := NewHelpedCondition(s.characterID, s.helperID)
	err := condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	_, err = dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			removedEvent = &event
			return nil
		})
	s.Require().NoError(err)

	s.Run("does not remove on the ally's own turn start", func() {
		err = dnd5eEvents.TurnStartTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.TurnStartEvent{
			SubjectID: s.characterID,
			Round:     1,
		})
		s.Require().NoError(err)
		s.True(condition.IsApplied(), "the trigger is the HELPER's turn, not the ally's")
	})

	s.Run("removes on the helper's turn start", func() {
		err = dnd5eEvents.TurnStartTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.TurnStartEvent{
			SubjectID: s.helperID,
			Round:     1,
		})
		s.Require().NoError(err)
		s.False(condition.IsApplied())
		s.Require().NotNil(removedEvent)
		s.Equal(s.characterID, removedEvent.MemberID)
		s.Equal("turn_start", removedEvent.Reason)
	})
}

func (s *HelpedConditionTestSuite) TestToJSON() {
	condition := NewHelpedCondition(s.characterID, s.helperID)
	data, err := condition.ToJSON()
	s.Require().NoError(err)

	loaded := &HelpedCondition{}
	err = loaded.loadJSON(data)
	s.Require().NoError(err)
	s.Equal(s.characterID, loaded.CharacterID)
	s.Equal(s.helperID, loaded.HelperID)
}

// TestLoaderRoundTrip proves HelpedCondition is registered in loader.go (R3).
func (s *HelpedConditionTestSuite) TestLoaderRoundTrip() {
	original := NewHelpedCondition(s.characterID, s.helperID)
	data, err := original.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(data)
	s.Require().NoError(err)

	helped, ok := loaded.(*HelpedCondition)
	s.Require().True(ok, "loaded condition should be a *HelpedCondition")
	s.Equal(s.characterID, helped.CharacterID)
	s.Equal(s.helperID, helped.HelperID)

	err = helped.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(helped.IsApplied())

	attackEvent := dnd5eEvents.AttackChainEvent{AttackerID: s.characterID, TargetID: "goblin-1"}
	attackChain := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	attacks := dnd5eEvents.AttackChain.On(s.bus)
	modifiedChain, err := attacks.PublishWithChain(s.ctx, attackEvent, attackChain)
	s.Require().NoError(err)
	finalEvent, err := modifiedChain.Execute(s.ctx, attackEvent)
	s.Require().NoError(err)
	s.Require().Len(finalEvent.AdvantageSources, 1, "reloaded condition still grants advantage")
}

// TestFactoryRoundTrip proves HelpedCondition is registered in factory.go (R3).
func (s *HelpedConditionTestSuite) TestFactoryRoundTrip() {
	output, err := CreateFromRef(&CreateFromRefInput{
		Ref:         refs.Conditions.Helped().String(),
		Config:      json.RawMessage(`{"helper_id":"` + s.helperID + `"}`),
		CharacterID: s.characterID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(output)

	helped, ok := output.Condition.(*HelpedCondition)
	s.Require().True(ok)
	s.Equal(s.characterID, helped.CharacterID)
	s.Equal(s.helperID, helped.HelperID)
}
