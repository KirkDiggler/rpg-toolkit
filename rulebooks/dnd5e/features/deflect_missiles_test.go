package features_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type DeflectMissilesTestSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	accessor *mockResourceAccessor
	feature  features.Feature
}

func TestDeflectMissilesTestSuite(t *testing.T) {
	suite.Run(t, new(DeflectMissilesTestSuite))
}

func (s *DeflectMissilesTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	// Create a mock character
	s.accessor = &mockResourceAccessor{
		id: "test-monk",
	}

	// Create the Deflect Missiles feature via factory
	// Level 3 monk with +3 DEX modifier
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.DeflectMissiles().String(),
		Config:      json.RawMessage(`{"monk_level": 3, "dex_modifier": 3}`),
		CharacterID: s.accessor.id,
	})
	s.Require().NoError(err)
	s.feature = output.Feature
}

func (s *DeflectMissilesTestSuite) TestCreateFromRef() {
	// Arrange
	config := json.RawMessage(`{"monk_level": 5, "dex_modifier": 4}`)

	// Act
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.DeflectMissiles().String(),
		Config:      config,
		CharacterID: "test-char",
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Require().NotNil(output.Feature)
	s.Assert().Equal(refs.Features.DeflectMissiles().ID, output.Feature.GetID())
}

func (s *DeflectMissilesTestSuite) TestCreateFromRef_DefaultValues() {
	// Arrange - empty config should use defaults
	config := json.RawMessage(`{}`)

	// Act
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.DeflectMissiles().String(),
		Config:      config,
		CharacterID: "test-char",
	})

	// Assert - should succeed with default values
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Require().NotNil(output.Feature)
}

func (s *DeflectMissilesTestSuite) TestApply_SubscribesToEvents() {
	// Arrange
	busEffect, ok := s.feature.(events.BusEffect)
	s.Require().True(ok, "DeflectMissiles should implement events.BusEffect")

	// Act
	err := busEffect.Apply(s.ctx, s.bus)

	// Assert
	s.Require().NoError(err)
	s.Assert().True(busEffect.IsApplied())
}

func (s *DeflectMissilesTestSuite) TestApply_AlreadyApplied() {
	// Arrange
	busEffect, ok := s.feature.(events.BusEffect)
	s.Require().True(ok)
	err := busEffect.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Act - try to apply again
	err = busEffect.Apply(s.ctx, s.bus)

	// Assert - should fail
	s.Require().Error(err)
}

func (s *DeflectMissilesTestSuite) TestRemove_UnsubscribesFromEvents() {
	// Arrange
	busEffect, ok := s.feature.(events.BusEffect)
	s.Require().True(ok)
	err := busEffect.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Act
	err = busEffect.Remove(s.ctx, s.bus)

	// Assert
	s.Require().NoError(err)
	s.Assert().False(busEffect.IsApplied())
}

func (s *DeflectMissilesTestSuite) TestOnDamageReceived_PublishesDeflectEvent() {
	// Arrange
	busEffect, ok := s.feature.(events.BusEffect)
	s.Require().True(ok)
	err := busEffect.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var receivedEvent *dnd5eEvents.DeflectMissilesTriggerEvent
	topic := dnd5eEvents.DeflectMissilesTriggerTopic.On(s.bus)
	_, err = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeflectMissilesTriggerEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act - publish damage received event
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   s.accessor.id,
		SourceID:   "enemy-archer",
		Amount:     10,
		DamageType: "piercing",
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	s.Assert().Equal(s.accessor.id, receivedEvent.CharacterID)
	s.Assert().Equal(10, receivedEvent.OriginalDamage)
	s.Assert().Greater(receivedEvent.Reduction, 0, "Reduction should be > 0")
	// With monk level 3 + DEX 3, minimum reduction is 1 (1d10) + 3 + 3 = 7
	s.Assert().GreaterOrEqual(receivedEvent.Reduction, 7, "Minimum reduction should be 7")
	// Maximum reduction is 10 (1d10) + 3 + 3 = 16
	s.Assert().LessOrEqual(receivedEvent.Reduction, 16, "Maximum reduction should be 16")
	s.Assert().Equal(refs.Features.DeflectMissiles().ID, receivedEvent.Source)
}

func (s *DeflectMissilesTestSuite) TestOnDamageReceived_DamageReducedToZero() {
	// Arrange
	busEffect, ok := s.feature.(events.BusEffect)
	s.Require().True(ok)
	err := busEffect.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var receivedEvent *dnd5eEvents.DeflectMissilesTriggerEvent
	topic := dnd5eEvents.DeflectMissilesTriggerTopic.On(s.bus)
	_, err = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeflectMissilesTriggerEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act - publish damage received event with low damage (likely to be reduced to 0)
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   s.accessor.id,
		SourceID:   "enemy-archer",
		Amount:     5, // Low damage, likely to be reduced to 0
		DamageType: "piercing",
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	// With min reduction of 7 and damage of 5, should be reduced to 0
	s.Assert().True(receivedEvent.DamageReducedTo0, "Damage should be reduced to 0")
}

func (s *DeflectMissilesTestSuite) TestOnDamageReceived_IgnoresOtherCharacters() {
	// Arrange
	busEffect, ok := s.feature.(events.BusEffect)
	s.Require().True(ok)
	err := busEffect.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var receivedEvent *dnd5eEvents.DeflectMissilesTriggerEvent
	topic := dnd5eEvents.DeflectMissilesTriggerTopic.On(s.bus)
	_, err = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeflectMissilesTriggerEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act - publish damage to a different character
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   "other-character",
		SourceID:   "enemy-archer",
		Amount:     10,
		DamageType: "piercing",
	})

	// Assert - should not trigger deflect
	s.Require().NoError(err)
	s.Assert().Nil(receivedEvent, "Should not trigger deflect for other characters")
}

func (s *DeflectMissilesTestSuite) TestActivate_PublishesThrowEvent() {
	// Arrange
	var receivedEvent *dnd5eEvents.DeflectMissilesThrowEvent
	topic := dnd5eEvents.DeflectMissilesThrowTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeflectMissilesThrowEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act - activate the catch-and-throw
	err = s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus: s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	s.Assert().Equal(s.accessor.id, receivedEvent.CharacterID)
	s.Assert().Equal(refs.Features.DeflectMissiles().ID, receivedEvent.Source)
}

func (s *DeflectMissilesTestSuite) TestCanActivate() {
	// Act
	err := s.feature.CanActivate(s.ctx, s.accessor, features.FeatureInput{})

	// Assert - for now, CanActivate always returns nil
	// The game server is responsible for checking Ki and reaction availability
	s.Require().NoError(err)
}

func (s *DeflectMissilesTestSuite) TestToJSON() {
	// Act
	jsonData, err := s.feature.ToJSON()

	// Assert - just verify it's valid JSON
	s.Require().NoError(err)
	s.Assert().NotEmpty(jsonData)

	// Verify it can be parsed back
	var data map[string]interface{}
	err = json.Unmarshal(jsonData, &data)
	s.Require().NoError(err)
	s.Assert().Contains(data, "ref")
	s.Assert().Contains(data, "character_id")
	s.Assert().Contains(data, "monk_level")
	s.Assert().Contains(data, "dex_modifier")
	s.Assert().Equal(s.accessor.GetID(), data["character_id"])
	s.Assert().Equal(float64(3), data["monk_level"])   // JSON numbers are float64
	s.Assert().Equal(float64(3), data["dex_modifier"]) // JSON numbers are float64
}

func (s *DeflectMissilesTestSuite) TestLoadJSON() {
	// Arrange
	originalJSON, err := s.feature.ToJSON()
	s.Require().NoError(err)

	// Act - use the loader to load it back
	loaded, err := features.LoadJSON(originalJSON)

	// Assert
	s.Require().NoError(err)
	s.Assert().NotNil(loaded)
	s.Assert().Equal(s.feature.GetID(), loaded.GetID())
}

func (s *DeflectMissilesTestSuite) TestRoundTrip() {
	// Arrange - serialize
	jsonData, err := s.feature.ToJSON()
	s.Require().NoError(err)

	// Act - deserialize via LoadJSON
	loaded, err := features.LoadJSON(jsonData)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(loaded)
	s.Assert().Equal(s.feature.GetID(), loaded.GetID())
}
