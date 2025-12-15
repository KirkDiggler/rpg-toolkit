package features_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/stretchr/testify/suite"
)

// mockResourceAccessor implements features.ResourceAccessor for testing
type mockResourceAccessor struct {
	id        string
	resources map[coreResources.ResourceKey]*combat.RecoverableResource
}

func (m *mockResourceAccessor) GetID() string {
	return m.id
}

func (m *mockResourceAccessor) GetType() core.EntityType {
	return "character"
}

func (m *mockResourceAccessor) GetResource(key coreResources.ResourceKey) *combat.RecoverableResource {
	if m.resources == nil {
		return combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:      "",
			Maximum: 0,
		})
	}
	if r, ok := m.resources[key]; ok {
		return r
	}
	return combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:      "",
		Maximum: 0,
	})
}

func (m *mockResourceAccessor) AddResource(key coreResources.ResourceKey, resource *combat.RecoverableResource) {
	if m.resources == nil {
		m.resources = make(map[coreResources.ResourceKey]*combat.RecoverableResource)
	}
	m.resources[key] = resource
}

type FlurryOfBlowsTestSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	accessor *mockResourceAccessor
	feature  features.Feature
}

func TestFlurryOfBlowsTestSuite(t *testing.T) {
	suite.Run(t, new(FlurryOfBlowsTestSuite))
}

func (s *FlurryOfBlowsTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	// Create a mock character with Ki resource
	s.accessor = &mockResourceAccessor{
		id: "test-monk",
	}

	// Add Ki resource (3 Ki points for level 3 monk)
	kiResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          string(resources.Ki),
		Maximum:     3,
		CharacterID: s.accessor.id,
		ResetType:   coreResources.ResetShortRest,
	})
	s.accessor.AddResource(resources.Ki, kiResource)

	// Create the Flurry of Blows feature via factory
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.FlurryOfBlows().String(),
		Config:      json.RawMessage(`{}`),
		CharacterID: s.accessor.id,
	})
	s.Require().NoError(err)
	s.feature = output.Feature
}

func (s *FlurryOfBlowsTestSuite) TestCanActivate_WithKi() {
	// Arrange - accessor has Ki

	// Act
	err := s.feature.CanActivate(s.ctx, s.accessor, features.FeatureInput{})

	// Assert
	s.Require().NoError(err)
}

func (s *FlurryOfBlowsTestSuite) TestCanActivate_WithoutKi() {
	// Arrange - consume all Ki
	ki := s.accessor.GetResource(resources.Ki)
	err := ki.Use(3)
	s.Require().NoError(err)

	// Act
	err = s.feature.CanActivate(s.ctx, s.accessor, features.FeatureInput{})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *FlurryOfBlowsTestSuite) TestActivate_ConsumesKi() {
	// Arrange
	ki := s.accessor.GetResource(resources.Ki)
	initialKi := ki.Current()
	s.Require().Equal(3, initialKi)

	// Act
	err := s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus: s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(2, ki.Current(), "Should consume 1 Ki point")
}

func (s *FlurryOfBlowsTestSuite) TestActivate_PublishesEvent() {
	// Arrange
	var receivedEvent *dnd5eEvents.FlurryOfBlowsActivatedEvent
	topic := dnd5eEvents.FlurryOfBlowsActivatedTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.FlurryOfBlowsActivatedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act
	err = s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus: s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	s.Assert().Equal(s.accessor.GetID(), receivedEvent.CharacterID)
	s.Assert().Equal(2, receivedEvent.UnarmedStrikes)
	s.Assert().Equal(refs.Features.FlurryOfBlows().ID, receivedEvent.Source)
}

func (s *FlurryOfBlowsTestSuite) TestActivate_FailsWhenNoKi() {
	// Arrange - consume all Ki
	ki := s.accessor.GetResource(resources.Ki)
	err := ki.Use(3)
	s.Require().NoError(err)

	// Act
	err = s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus: s.bus,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *FlurryOfBlowsTestSuite) TestToJSON() {
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
	s.Assert().Equal(s.accessor.GetID(), data["character_id"])
}

func (s *FlurryOfBlowsTestSuite) TestLoadJSON() {
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

func (s *FlurryOfBlowsTestSuite) TestRoundTrip() {
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

func (s *FlurryOfBlowsTestSuite) TestCreateFromRef() {
	// Arrange
	config := json.RawMessage(`{}`)

	// Act
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.FlurryOfBlows().String(),
		Config:      config,
		CharacterID: "test-char",
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Require().NotNil(output.Feature)
	s.Assert().Equal(refs.Features.FlurryOfBlows().ID, output.Feature.GetID())
}
