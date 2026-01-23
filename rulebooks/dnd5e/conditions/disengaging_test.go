// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type DisengagingConditionTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *DisengagingConditionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestDisengagingConditionSuite(t *testing.T) {
	suite.Run(t, new(DisengagingConditionTestSuite))
}

func (s *DisengagingConditionTestSuite) TestNewDisengagingCondition() {
	disengaging := conditions.NewDisengagingCondition("rogue-1")

	s.NotNil(disengaging)
	s.False(disengaging.IsApplied())
}

func (s *DisengagingConditionTestSuite) TestApplyAndRemove() {
	disengaging := conditions.NewDisengagingCondition("rogue-1")

	// Apply should succeed
	err := disengaging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(disengaging.IsApplied())

	// Applying again should fail
	err = disengaging.Apply(s.ctx, s.bus)
	s.Error(err)

	// Remove should succeed
	err = disengaging.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(disengaging.IsApplied())
}

func (s *DisengagingConditionTestSuite) TestPreventsOpportunityAttacks() {
	disengaging := conditions.NewDisengagingCondition("rogue-1")

	err := disengaging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = disengaging.Remove(s.ctx, s.bus) }()

	// Create a movement chain event for the disengaging character
	movementEvent := &dnd5eEvents.MovementChainEvent{
		EntityID:            "rogue-1",
		EntityType:          "character",
		FromPosition:        dnd5eEvents.Position{X: 5, Y: 5},
		ToPosition:          dnd5eEvents.Position{X: 6, Y: 5},
		ThreateningEntities: []string{"goblin-1", "goblin-2"},
		OAPreventionSources: make([]dnd5eEvents.MovementModifierSource, 0),
	}

	// Execute through movement chain
	movementChain := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	movements := dnd5eEvents.MovementChain.On(s.bus)
	modifiedChain, err := movements.PublishWithChain(s.ctx, movementEvent, movementChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, movementEvent)
	s.Require().NoError(err)

	// Should have OA prevention added
	s.Len(finalEvent.OAPreventionSources, 1)
	s.True(finalEvent.IsOAPrevented())
	s.Equal("Disengaging", finalEvent.OAPreventionSources[0].Name)
	s.Equal("condition", finalEvent.OAPreventionSources[0].SourceType)
	s.Equal(refs.Conditions.Disengaging(), finalEvent.OAPreventionSources[0].SourceRef)
	s.Equal("rogue-1", finalEvent.OAPreventionSources[0].EntityID)
}

func (s *DisengagingConditionTestSuite) TestDoesNotAffectOtherCharactersMovement() {
	disengaging := conditions.NewDisengagingCondition("rogue-1")

	err := disengaging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	defer func() { _ = disengaging.Remove(s.ctx, s.bus) }()

	// Create a movement chain event for a DIFFERENT character
	movementEvent := &dnd5eEvents.MovementChainEvent{
		EntityID:            "fighter-1", // Not the disengaging character
		EntityType:          "character",
		FromPosition:        dnd5eEvents.Position{X: 5, Y: 5},
		ToPosition:          dnd5eEvents.Position{X: 6, Y: 5},
		ThreateningEntities: []string{"goblin-1"},
		OAPreventionSources: make([]dnd5eEvents.MovementModifierSource, 0),
	}

	// Execute through movement chain
	movementChain := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	movements := dnd5eEvents.MovementChain.On(s.bus)
	modifiedChain, err := movements.PublishWithChain(s.ctx, movementEvent, movementChain)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, movementEvent)
	s.Require().NoError(err)

	// Should NOT have OA prevention - disengaging doesn't apply to other characters
	s.Empty(finalEvent.OAPreventionSources)
	s.False(finalEvent.IsOAPrevented())
}

func (s *DisengagingConditionTestSuite) TestRemovedOnTurnEnd() {
	disengaging := conditions.NewDisengagingCondition("rogue-1")

	err := disengaging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(disengaging.IsApplied())

	// Track condition removal
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removedTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish turn end for the disengaging character
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: "rogue-1",
		Round:       1,
	})
	s.Require().NoError(err)

	// Condition should be removed
	s.False(disengaging.IsApplied())
	s.NotNil(removedEvent)
	s.Equal("rogue-1", removedEvent.CharacterID)
	s.Equal(refs.Conditions.Disengaging().String(), removedEvent.ConditionRef)
	s.Equal("turn_end", removedEvent.Reason)
}

func (s *DisengagingConditionTestSuite) TestNotRemovedOnOtherCharactersTurnEnd() {
	disengaging := conditions.NewDisengagingCondition("rogue-1")

	err := disengaging.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(disengaging.IsApplied())

	// Publish turn end for a DIFFERENT character
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: "fighter-1", // Not the disengaging character
		Round:       1,
	})
	s.Require().NoError(err)

	// Condition should still be applied
	s.True(disengaging.IsApplied())
}

func (s *DisengagingConditionTestSuite) TestToJSON() {
	disengaging := conditions.NewDisengagingCondition("rogue-1")

	jsonData, err := disengaging.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(jsonData), refs.Conditions.Disengaging().ID)
	s.Contains(string(jsonData), "rogue-1")
}

func (s *DisengagingConditionTestSuite) TestJSONRoundTrip() {
	// Create and serialize a condition
	original := conditions.NewDisengagingCondition("rogue-1")

	jsonData, err := original.ToJSON()
	s.Require().NoError(err)

	// Load it back using the loader
	loaded, err := conditions.LoadJSON(jsonData)
	s.Require().NoError(err)

	// Verify we can use it
	err = loaded.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(loaded.IsApplied())

	// Serialize the loaded condition and compare
	reserializedData, err := loaded.ToJSON()
	s.Require().NoError(err)

	// Compare the JSON structures
	var originalMap, reserializedMap map[string]interface{}
	err = json.Unmarshal(jsonData, &originalMap)
	s.Require().NoError(err)
	err = json.Unmarshal(reserializedData, &reserializedMap)
	s.Require().NoError(err)

	s.Equal(originalMap["character_id"], reserializedMap["character_id"])
}

func (s *DisengagingConditionTestSuite) TestCreateFromRef() {
	// Test factory creation
	output, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref:         refs.Conditions.Disengaging().String(),
		CharacterID: "rogue-1",
	})
	s.Require().NoError(err)
	s.NotNil(output.Condition)

	// Apply and verify it works
	err = output.Condition.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(output.Condition.IsApplied())
}
