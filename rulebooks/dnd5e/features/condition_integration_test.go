// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/class"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/race"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

// ConditionIntegrationTestSuite demonstrates the flow of:
// 1. Character loads with features
// 2. Feature activation publishes condition event
// 3. Character receives event and stores condition JSON
type ConditionIntegrationTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *ConditionIntegrationTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestConditionIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ConditionIntegrationTestSuite))
}

func (s *ConditionIntegrationTestSuite) TestRageFeaturePublishesConditionAndCharacterStoresIt() {
	// Create a test character (barbarian)
	charData := character.Data{
		ID:       "test-barbarian",
		PlayerID: "player1",
		Name:     "Ragnar",
		Level:    5,
		ClassID:  "barbarian",
		RaceID:   "human",
		AbilityScores: shared.AbilityScores{
			constants.STR: 16,
			constants.DEX: 14,
			constants.CON: 16,
			constants.INT: 10,
			constants.WIS: 12,
			constants.CHA: 8,
		},
		HitPoints:    45,
		MaxHitPoints: 45,
	}

	// Create minimal race/class data for loading
	raceData := &race.Data{
		ID:   "human",
		Name: "Human",
	}

	classData := &class.Data{
		ID:      "barbarian",
		Name:    "Barbarian",
		HitDice: 12,
	}

	backgroundData := &shared.Background{
		ID:   "soldier",
		Name: "Soldier",
	}

	// Load the character
	char, err := character.LoadCharacterFromData(charData, raceData, classData, backgroundData)
	s.Require().NoError(err)

	// Subscribe the character to condition events
	err = char.ApplyToEventBus(s.ctx, s.bus)
	s.Require().NoError(err)

	// Create a rage feature
	rage := features.NewRage(features.RageConfig{
		ID:    "rage-feature-1",
		Level: 5,
		Uses:  3,
		Bus:   s.bus,
	})

	// Verify character has no conditions initially
	s.Empty(char.GetConditions())
	s.False(char.HasCondition("dnd5e:conditions:raging"))

	// Create a simple entity for the character
	mockChar := &testEntity{id: "test-barbarian"}

	// Activate rage - this should publish a ConditionAppliedEvent
	err = rage.Activate(s.ctx, mockChar, features.FeatureInput{})
	s.Require().NoError(err)

	// Character should now have the raging condition
	conditions := char.GetConditions()
	s.Require().Len(conditions, 1, "Character should have 1 condition after rage activation")

	// Verify the condition data
	var conditionData map[string]interface{}
	err = json.Unmarshal(conditions[0], &conditionData)
	s.Require().NoError(err)

	s.Equal("dnd5e:conditions:raging", conditionData["ref"])
	s.Equal("raging", conditionData["type"])
	s.Equal("rage-feature-1", conditionData["source"]) // Source is the rage feature ID
	s.Equal(float64(5), conditionData["level"])        // JSON unmarshals numbers as float64
	s.Equal(float64(2), conditionData["damage_bonus"]) // Level 5 barbarian gets +2 damage

	// Verify HasCondition works
	s.True(char.HasCondition("dnd5e:conditions:raging"))

	// In a real game, the condition would end via:
	// 1. OnTurnEnd detecting no combat activity
	// 2. 10 rounds passing (1 minute)
	// 3. Character falling unconscious
	// For testing, we'll simulate the condition ending
	removals := character.ConditionRemovedTopic.On(s.bus)
	err = removals.Publish(s.ctx, character.ConditionRemovedEvent{
		CharacterID:  "test-barbarian",
		ConditionRef: "dnd5e:conditions:raging",
		Reason:       "no_combat_activity",
	})
	s.Require().NoError(err)

	// Character should no longer have the condition
	s.Empty(char.GetConditions())
	s.False(char.HasCondition("dnd5e:conditions:raging"))
}

// TestMultipleConditionsCanBeTracked is skipped until we have more condition implementations
// func (s *ConditionIntegrationTestSuite) TestMultipleConditionsCanBeTracked() {
// 	// TODO: Implement when we have poisoned, blessed, etc conditions
// }

// testEntity is a simple entity implementation for testing
type testEntity struct {
	id string
}

func (e *testEntity) GetID() string            { return e.id }
func (e *testEntity) GetType() core.EntityType { return "character" }
