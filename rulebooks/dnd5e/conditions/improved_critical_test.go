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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type ImprovedCriticalTestSuite struct {
	suite.Suite
	bus       events.EventBus
	condition *ImprovedCriticalCondition
}

func (s *ImprovedCriticalTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.condition = NewImprovedCriticalCondition(ImprovedCriticalInput{
		CharacterID: "champion-fighter",
		Threshold:   19, // Champion level 3
	})
}

func (s *ImprovedCriticalTestSuite) TearDownTest() {
	if s.condition != nil && s.condition.IsApplied() {
		_ = s.condition.Remove(context.Background(), s.bus)
	}
}

// TestNewImprovedCriticalCondition verifies condition creation
func (s *ImprovedCriticalTestSuite) TestNewImprovedCriticalCondition() {
	s.Run("creates with explicit threshold", func() {
		cond := NewImprovedCriticalCondition(ImprovedCriticalInput{
			CharacterID: "test-char",
			Threshold:   18, // Superior Critical
		})

		s.Assert().Equal("test-char", cond.CharacterID)
		s.Assert().Equal(18, cond.Threshold)
		s.Assert().False(cond.IsApplied())
	})

	s.Run("defaults to 19 when threshold is 0", func() {
		cond := NewImprovedCriticalCondition(ImprovedCriticalInput{
			CharacterID: "test-char",
			Threshold:   0,
		})

		s.Assert().Equal(19, cond.Threshold)
	})
}

// TestApplyAndRemove verifies lifecycle management
func (s *ImprovedCriticalTestSuite) TestApplyAndRemove() {
	ctx := context.Background()

	s.Run("applies successfully", func() {
		err := s.condition.Apply(ctx, s.bus)
		s.Require().NoError(err)
		s.Assert().True(s.condition.IsApplied())
	})

	s.Run("fails to apply twice", func() {
		err := s.condition.Apply(ctx, s.bus)
		s.Require().Error(err)
	})

	s.Run("removes successfully", func() {
		err := s.condition.Remove(ctx, s.bus)
		s.Require().NoError(err)
		s.Assert().False(s.condition.IsApplied())
	})

	s.Run("remove is idempotent", func() {
		err := s.condition.Remove(ctx, s.bus)
		s.Require().NoError(err)
	})
}

// TestCriticalThresholdModification verifies that Improved Critical lowers the crit threshold
func (s *ImprovedCriticalTestSuite) TestCriticalThresholdModification() {
	ctx := context.Background()
	err := s.condition.Apply(ctx, s.bus)
	s.Require().NoError(err)

	s.Run("lowers threshold to 19 for champion attacks", func() {
		event := combat.AttackChainEvent{
			AttackerID:        "champion-fighter",
			TargetID:          "goblin",
			AttackRoll:        19,
			AttackBonus:       5,
			TargetAC:          15,
			IsNaturalTwenty:   false,
			IsNaturalOne:      false,
			CriticalThreshold: 20, // Default threshold
		}

		// Create and execute chain
		chain := events.NewStagedChain[combat.AttackChainEvent](combat.ModifierStages)
		attackChain := combat.AttackChain.On(s.bus)

		modifiedChain, err := attackChain.PublishWithChain(ctx, event, chain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(ctx, event)
		s.Require().NoError(err)

		// Verify threshold was lowered to 19
		s.Assert().Equal(19, finalEvent.CriticalThreshold)
	})

	s.Run("does not modify attacks by other characters", func() {
		event := combat.AttackChainEvent{
			AttackerID:        "other-fighter",
			TargetID:          "goblin",
			AttackRoll:        19,
			AttackBonus:       5,
			TargetAC:          15,
			IsNaturalTwenty:   false,
			IsNaturalOne:      false,
			CriticalThreshold: 20,
		}

		chain := events.NewStagedChain[combat.AttackChainEvent](combat.ModifierStages)
		attackChain := combat.AttackChain.On(s.bus)

		modifiedChain, err := attackChain.PublishWithChain(ctx, event, chain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(ctx, event)
		s.Require().NoError(err)

		// Threshold should remain 20
		s.Assert().Equal(20, finalEvent.CriticalThreshold)
	})

	s.Run("does not raise threshold if already lower", func() {
		// Test with a lower starting threshold
		// First remove the existing condition with threshold 19
		err := s.condition.Remove(ctx, s.bus)
		s.Require().NoError(err)

		// Create condition with threshold 18 (Superior Critical)
		superiorCrit := NewImprovedCriticalCondition(ImprovedCriticalInput{
			CharacterID: "champion-fighter",
			Threshold:   18,
		})
		err = superiorCrit.Apply(ctx, s.bus)
		s.Require().NoError(err)
		defer func() {
			_ = superiorCrit.Remove(ctx, s.bus)
			// Reapply the original condition for other tests
			_ = s.condition.Apply(ctx, s.bus)
		}()

		event := combat.AttackChainEvent{
			AttackerID:        "champion-fighter",
			TargetID:          "goblin",
			AttackRoll:        18,
			AttackBonus:       5,
			TargetAC:          15,
			IsNaturalTwenty:   false,
			IsNaturalOne:      false,
			CriticalThreshold: 20,
		}

		chain := events.NewStagedChain[combat.AttackChainEvent](combat.ModifierStages)
		attackChain := combat.AttackChain.On(s.bus)

		modifiedChain, err := attackChain.PublishWithChain(ctx, event, chain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(ctx, event)
		s.Require().NoError(err)

		// Should be lowered to 18
		s.Assert().Equal(18, finalEvent.CriticalThreshold)
	})
}

// TestSerialization verifies JSON serialization
func (s *ImprovedCriticalTestSuite) TestSerialization() {
	s.Run("serializes to JSON", func() {
		data, err := s.condition.ToJSON()
		s.Require().NoError(err)

		var decoded ImprovedCriticalData
		err = json.Unmarshal(data, &decoded)
		s.Require().NoError(err)

		s.Assert().Equal("champion-fighter", decoded.CharacterID)
		s.Assert().Equal(19, decoded.Threshold)
		s.Assert().Equal(refs.Conditions.ImprovedCritical().ID, decoded.Ref.ID)
		s.Assert().Equal(refs.Module, decoded.Ref.Module)
		s.Assert().Equal(refs.TypeConditions, decoded.Ref.Type)
	})

	s.Run("deserializes from JSON", func() {
		// First serialize
		data, err := s.condition.ToJSON()
		s.Require().NoError(err)

		// Then deserialize
		newCondition := &ImprovedCriticalCondition{}
		err = newCondition.loadJSON(data)
		s.Require().NoError(err)

		s.Assert().Equal(s.condition.CharacterID, newCondition.CharacterID)
		s.Assert().Equal(s.condition.Threshold, newCondition.Threshold)
	})

	s.Run("defaults threshold to 19 when deserializing 0", func() {
		data := json.RawMessage(
			`{"ref":{"module":"dnd5e","type":"conditions","id":"improved_critical"},` +
				`"character_id":"test","threshold":0}`)

		newCondition := &ImprovedCriticalCondition{}
		err := newCondition.loadJSON(data)
		s.Require().NoError(err)

		s.Assert().Equal(19, newCondition.Threshold)
	})
}

// TestLoader verifies the condition can be loaded via the loader
func (s *ImprovedCriticalTestSuite) TestLoader() {
	s.Run("loads from JSON via LoadJSON", func() {
		data, err := s.condition.ToJSON()
		s.Require().NoError(err)

		loaded, err := LoadJSON(data)
		s.Require().NoError(err)
		s.Require().NotNil(loaded)

		ic, ok := loaded.(*ImprovedCriticalCondition)
		s.Require().True(ok, "loaded condition should be ImprovedCriticalCondition")

		s.Assert().Equal(s.condition.CharacterID, ic.CharacterID)
		s.Assert().Equal(s.condition.Threshold, ic.Threshold)
	})
}

// TestFactory verifies the condition can be created via the factory
func (s *ImprovedCriticalTestSuite) TestFactory() {
	s.Run("creates from ref with config", func() {
		config := json.RawMessage(`{"threshold": 18}`)
		output, err := CreateFromRef(&CreateFromRefInput{
			Ref:         refs.Conditions.ImprovedCritical().String(),
			Config:      config,
			CharacterID: "test-char",
		})
		s.Require().NoError(err)
		s.Require().NotNil(output)

		ic, ok := output.Condition.(*ImprovedCriticalCondition)
		s.Require().True(ok)

		s.Assert().Equal("test-char", ic.CharacterID)
		s.Assert().Equal(18, ic.Threshold)
	})

	s.Run("creates with default threshold when config is empty", func() {
		output, err := CreateFromRef(&CreateFromRefInput{
			Ref:         refs.Conditions.ImprovedCritical().String(),
			Config:      nil,
			CharacterID: "test-char",
		})
		s.Require().NoError(err)
		s.Require().NotNil(output)

		ic, ok := output.Condition.(*ImprovedCriticalCondition)
		s.Require().True(ok)

		s.Assert().Equal(19, ic.Threshold) // Default
	})
}

// TestIntegrationWithAttackChain verifies the condition works in attack resolution
func (s *ImprovedCriticalTestSuite) TestIntegrationWithAttackChain() {
	ctx := context.Background()
	err := s.condition.Apply(ctx, s.bus)
	s.Require().NoError(err)

	s.Run("roll of 19 becomes critical for champion", func() {
		event := combat.AttackChainEvent{
			AttackerID:        "champion-fighter",
			TargetID:          "goblin",
			AttackRoll:        19,
			AttackBonus:       5,
			TargetAC:          15,
			IsNaturalTwenty:   false,
			IsNaturalOne:      false,
			CriticalThreshold: 20,
		}

		chain := events.NewStagedChain[combat.AttackChainEvent](combat.ModifierStages)
		attackChain := combat.AttackChain.On(s.bus)

		modifiedChain, err := attackChain.PublishWithChain(ctx, event, chain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(ctx, event)
		s.Require().NoError(err)

		// With threshold 19, a roll of 19 should be a critical
		s.Assert().Equal(19, finalEvent.CriticalThreshold)
		// The ResolveAttack function would check: attackRoll >= CriticalThreshold
		s.Assert().True(finalEvent.AttackRoll >= finalEvent.CriticalThreshold, "roll of 19 should meet threshold of 19")
	})

	s.Run("roll of 18 is not critical for improved critical", func() {
		event := combat.AttackChainEvent{
			AttackerID:        "champion-fighter",
			TargetID:          "goblin",
			AttackRoll:        18,
			AttackBonus:       5,
			TargetAC:          15,
			IsNaturalTwenty:   false,
			IsNaturalOne:      false,
			CriticalThreshold: 20,
		}

		chain := events.NewStagedChain[combat.AttackChainEvent](combat.ModifierStages)
		attackChain := combat.AttackChain.On(s.bus)

		modifiedChain, err := attackChain.PublishWithChain(ctx, event, chain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(ctx, event)
		s.Require().NoError(err)

		s.Assert().Equal(19, finalEvent.CriticalThreshold)
		s.Assert().False(finalEvent.AttackRoll >= finalEvent.CriticalThreshold, "roll of 18 should not meet threshold of 19")
	})

	s.Run("roll of 20 is still critical", func() {
		event := combat.AttackChainEvent{
			AttackerID:        "champion-fighter",
			TargetID:          "goblin",
			AttackRoll:        20,
			AttackBonus:       5,
			TargetAC:          15,
			IsNaturalTwenty:   true,
			IsNaturalOne:      false,
			CriticalThreshold: 20,
		}

		chain := events.NewStagedChain[combat.AttackChainEvent](combat.ModifierStages)
		attackChain := combat.AttackChain.On(s.bus)

		modifiedChain, err := attackChain.PublishWithChain(ctx, event, chain)
		s.Require().NoError(err)

		finalEvent, err := modifiedChain.Execute(ctx, event)
		s.Require().NoError(err)

		s.Assert().Equal(19, finalEvent.CriticalThreshold)
		s.Assert().True(finalEvent.AttackRoll >= finalEvent.CriticalThreshold, "roll of 20 should always be critical")
	})
}

func TestImprovedCriticalSuite(t *testing.T) {
	suite.Run(t, new(ImprovedCriticalTestSuite))
}
