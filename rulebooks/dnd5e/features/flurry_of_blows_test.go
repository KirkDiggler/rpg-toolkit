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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/stretchr/testify/suite"
)

// mockResourceAccessor implements coreResources.ResourceAccessor for testing (used by other test files)
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

// IsResourceAvailable implements coreResources.ResourceAccessor
func (m *mockResourceAccessor) IsResourceAvailable(key coreResources.ResourceKey) bool {
	if m.resources == nil {
		return false
	}
	r, ok := m.resources[key]
	if !ok {
		return false
	}
	return r.IsAvailable()
}

// UseResource implements coreResources.ResourceAccessor
func (m *mockResourceAccessor) UseResource(key coreResources.ResourceKey, amount int) error {
	if m.resources == nil {
		return rpgerr.Newf(rpgerr.CodeNotFound, "resource %s not found", key)
	}
	r, ok := m.resources[key]
	if !ok {
		return rpgerr.Newf(rpgerr.CodeNotFound, "resource %s not found", key)
	}
	return r.Use(amount)
}

// GetResource is a test helper to access internal state (not part of ResourceAccessor interface)
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

// mockMonkCharacter extends mockResourceAccessor with ActionHolder interface
type mockMonkCharacter struct {
	mockResourceAccessor
	actions []actions.Action
}

// AddAction implements actions.ActionHolder
func (m *mockMonkCharacter) AddAction(action actions.Action) error {
	m.actions = append(m.actions, action)
	return nil
}

// RemoveAction implements actions.ActionHolder
func (m *mockMonkCharacter) RemoveAction(actionID string) error {
	for i, a := range m.actions {
		if a.GetID() == actionID {
			m.actions = append(m.actions[:i], m.actions[i+1:]...)
			return nil
		}
	}
	return rpgerr.Newf(rpgerr.CodeNotFound, "action %s not found", actionID)
}

// GetActions implements actions.ActionHolder
func (m *mockMonkCharacter) GetActions() []actions.Action {
	return m.actions
}

// GetAction implements actions.ActionHolder
func (m *mockMonkCharacter) GetAction(actionID string) actions.Action {
	for _, a := range m.actions {
		if a.GetID() == actionID {
			return a
		}
	}
	return nil
}

type FlurryOfBlowsTestSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	character *mockMonkCharacter
	feature   features.Feature
}

func TestFlurryOfBlowsTestSuite(t *testing.T) {
	suite.Run(t, new(FlurryOfBlowsTestSuite))
}

func (s *FlurryOfBlowsTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	// Create a mock character with Ki resource
	s.character = &mockMonkCharacter{
		mockResourceAccessor: mockResourceAccessor{
			id: "test-monk",
		},
	}

	// Subscribe to ActionGrantedEvent to add actions to the mock character
	// This simulates what Character.subscribeToEvents() does
	topic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
	_, _ = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ActionGrantedEvent) error {
		if event.CharacterID != s.character.GetID() {
			return nil
		}
		if action, ok := event.Action.(actions.Action); ok {
			return s.character.AddAction(action)
		}
		return nil
	})

	// Add Ki resource (3 Ki points for level 3 monk)
	kiResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
		ID:          string(resources.Ki),
		Maximum:     3,
		CharacterID: s.character.GetID(),
		ResetType:   coreResources.ResetShortRest,
	})
	s.character.AddResource(resources.Ki, kiResource)

	// Create the Flurry of Blows feature via factory
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.FlurryOfBlows().String(),
		Config:      json.RawMessage(`{}`),
		CharacterID: s.character.GetID(),
	})
	s.Require().NoError(err)
	s.feature = output.Feature
}

func (s *FlurryOfBlowsTestSuite) TestCanActivate_WithKi() {
	// Arrange - character has Ki

	// Act
	err := s.feature.CanActivate(s.ctx, s.character, features.FeatureInput{})

	// Assert
	s.Require().NoError(err)
}

func (s *FlurryOfBlowsTestSuite) TestCanActivate_WithoutKi() {
	// Arrange - consume all Ki
	ki := s.character.GetResource(resources.Ki)
	err := ki.Use(3)
	s.Require().NoError(err)

	// Act
	err = s.feature.CanActivate(s.ctx, s.character, features.FeatureInput{})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *FlurryOfBlowsTestSuite) TestActivate_ConsumesKi() {
	// Arrange
	ki := s.character.GetResource(resources.Ki)
	initialKi := ki.Current()
	s.Require().Equal(3, initialKi)

	// Act
	err := s.feature.Activate(s.ctx, s.character, features.FeatureInput{
		Bus: s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(2, ki.Current(), "Should consume 1 Ki point")
}

func (s *FlurryOfBlowsTestSuite) TestActivate_GrantsFlurryStrikeActions() {
	// Arrange - character should have no actions initially
	s.Require().Empty(s.character.GetActions())

	// Act
	err := s.feature.Activate(s.ctx, s.character, features.FeatureInput{
		Bus: s.bus,
	})

	// Assert
	s.Require().NoError(err)
	grantedActions := s.character.GetActions()
	s.Require().Len(grantedActions, 2, "Should grant 2 FlurryStrike actions")

	// Verify they're FlurryStrike actions with correct IDs
	s.Assert().Equal("test-monk-flurry-strike-1", grantedActions[0].GetID())
	s.Assert().Equal("test-monk-flurry-strike-2", grantedActions[1].GetID())
}

func (s *FlurryOfBlowsTestSuite) TestActivate_FailsWhenNoKi() {
	// Arrange - consume all Ki
	ki := s.character.GetResource(resources.Ki)
	err := ki.Use(3)
	s.Require().NoError(err)

	// Act
	err = s.feature.Activate(s.ctx, s.character, features.FeatureInput{
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
	s.Assert().Equal(s.character.GetID(), data["character_id"])
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
