package features_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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

// Wave 2.11e (#666) — Step of the Wind tests below this line verify the
// toolkit-side condition application added in this wave. Per director
// signoff (#666 Q1=(a)), Activate with action="disengage" must apply
// DisengagingCondition directly to input.Bus in addition to publishing
// the existing telemetry event. The "dash" branch keeps event-only
// behavior because Dash is not part of Wave 2.11e's OA-suppression goal.

type StepOfTheWindTestSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	accessor *mockResourceAccessor
	feature  features.Feature
}

func TestStepOfTheWindTestSuite(t *testing.T) {
	suite.Run(t, new(StepOfTheWindTestSuite))
}

func (s *StepOfTheWindTestSuite) SetupTest() {
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

	// Create the Step of the Wind feature via factory
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.StepOfTheWind().String(),
		Config:      json.RawMessage(`{}`),
		CharacterID: s.accessor.id,
	})
	s.Require().NoError(err)
	s.feature = output.Feature
}

func (s *StepOfTheWindTestSuite) TestCanActivate_WithKi() {
	// Arrange - accessor has Ki

	// Act
	err := s.feature.CanActivate(s.ctx, s.accessor, features.FeatureInput{})

	// Assert
	s.Require().NoError(err)
}

func (s *StepOfTheWindTestSuite) TestCanActivate_WithoutKi() {
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

func (s *StepOfTheWindTestSuite) TestActivate_ConsumesKi() {
	// Arrange
	ki := s.accessor.GetResource(resources.Ki)
	initialKi := ki.Current()
	s.Require().Equal(3, initialKi)

	// Act
	err := s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus:    s.bus,
		Action: "disengage",
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(2, ki.Current(), "Should consume 1 Ki point")
}

//nolint:dupl // Test functions intentionally similar - different action parameter
func (s *StepOfTheWindTestSuite) TestActivate_PublishesEvent_Disengage() {
	// Arrange
	var receivedEvent *dnd5eEvents.StepOfTheWindActivatedEvent
	topic := dnd5eEvents.StepOfTheWindActivatedTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.StepOfTheWindActivatedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act
	err = s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus:    s.bus,
		Action: "disengage",
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	s.Assert().Equal(s.accessor.GetID(), receivedEvent.CharacterID)
	s.Assert().Equal("disengage", receivedEvent.Action)
	s.Assert().Equal(refs.Features.StepOfTheWind().ID, receivedEvent.Source)
}

//nolint:dupl // Test functions intentionally similar - different action parameter
func (s *StepOfTheWindTestSuite) TestActivate_PublishesEvent_Dash() {
	// Arrange
	var receivedEvent *dnd5eEvents.StepOfTheWindActivatedEvent
	topic := dnd5eEvents.StepOfTheWindActivatedTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.StepOfTheWindActivatedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act
	err = s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus:    s.bus,
		Action: "dash",
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	s.Assert().Equal(s.accessor.GetID(), receivedEvent.CharacterID)
	s.Assert().Equal("dash", receivedEvent.Action)
	s.Assert().Equal(refs.Features.StepOfTheWind().ID, receivedEvent.Source)
}

func (s *StepOfTheWindTestSuite) TestActivate_DefaultsToDisengage() {
	// Arrange
	var receivedEvent *dnd5eEvents.StepOfTheWindActivatedEvent
	topic := dnd5eEvents.StepOfTheWindActivatedTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.StepOfTheWindActivatedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act - no action specified
	err = s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus: s.bus,
	})

	// Assert - should default to "disengage"
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	s.Assert().Equal("disengage", receivedEvent.Action)
}

func (s *StepOfTheWindTestSuite) TestActivate_InvalidAction() {
	// Act - invalid action
	err := s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus:    s.bus,
		Action: "jump", // Invalid action
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
}

func (s *StepOfTheWindTestSuite) TestActivate_FailsWhenNoKi() {
	// Arrange - consume all Ki
	ki := s.accessor.GetResource(resources.Ki)
	err := ki.Use(3)
	s.Require().NoError(err)

	// Act
	err = s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus:    s.bus,
		Action: "disengage",
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

// TestActivate_DisengageBranch_AppliesDisengagingCondition is the
// load-bearing wave-2.11e (#666) test: when Activate runs with
// action="disengage", the DisengagingCondition must be applied to the
// owner on input.Bus so the next MovementChain for the owner emits an
// OAPreventionSources entry. Without this, monks using Step of the Wind
// would take OAs when moving past enemies — breaking the playable goal.
func (s *StepOfTheWindTestSuite) TestActivate_DisengageBranch_AppliesDisengagingCondition() {
	err := s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus:    s.bus,
		Action: "disengage",
	})
	s.Require().NoError(err)

	// Now drive a MovementChain for the owner and verify the applied
	// DisengagingCondition adds an OAPreventionSources entry.
	movementChain := dnd5eEvents.MovementChain.On(s.bus)
	chainBuilder := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	event := &dnd5eEvents.MovementChainEvent{
		EntityID:            s.accessor.GetID(),
		FromPosition:        dnd5eEvents.Position{X: 0, Y: 0},
		ToPosition:          dnd5eEvents.Position{X: 1, Y: 0},
		OAPreventionSources: []dnd5eEvents.MovementModifierSource{},
	}

	modifiedChain, err := movementChain.PublishWithChain(s.ctx, event, chainBuilder)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Require().NotEmpty(finalEvent.OAPreventionSources,
		"DisengagingCondition must add OAPreventionSources after Step of the Wind disengage activation")
	s.True(finalEvent.IsOAPrevented(),
		"IsOAPrevented must be true so OpportunityAttackCondition predicate skips")
	s.Equal("Disengaging", finalEvent.OAPreventionSources[0].Name)
}

// TestActivate_DashBranch_DoesNotApplyDisengagingCondition verifies the
// "dash" branch keeps event-only behavior — no DisengagingCondition is
// applied because Dash itself doesn't suppress OAs in 5e rules. This
// preserves the wave-scope split (Q2 director signoff).
func (s *StepOfTheWindTestSuite) TestActivate_DashBranch_DoesNotApplyDisengagingCondition() {
	err := s.feature.Activate(s.ctx, s.accessor, features.FeatureInput{
		Bus:    s.bus,
		Action: "dash",
	})
	s.Require().NoError(err)

	// Movement should NOT see any OAPreventionSources added.
	movementChain := dnd5eEvents.MovementChain.On(s.bus)
	chainBuilder := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	event := &dnd5eEvents.MovementChainEvent{
		EntityID:            s.accessor.GetID(),
		FromPosition:        dnd5eEvents.Position{X: 0, Y: 0},
		ToPosition:          dnd5eEvents.Position{X: 1, Y: 0},
		OAPreventionSources: []dnd5eEvents.MovementModifierSource{},
	}

	modifiedChain, err := movementChain.PublishWithChain(s.ctx, event, chainBuilder)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Empty(finalEvent.OAPreventionSources,
		"Dash branch must NOT apply DisengagingCondition — only disengage branch suppresses OAs")
	s.False(finalEvent.IsOAPrevented())
}

func (s *StepOfTheWindTestSuite) TestToJSON() {
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

func (s *StepOfTheWindTestSuite) TestLoadJSON() {
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

func (s *StepOfTheWindTestSuite) TestRoundTrip() {
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

func (s *StepOfTheWindTestSuite) TestCreateFromRef() {
	// Arrange
	config := json.RawMessage(`{}`)

	// Act
	output, err := features.CreateFromRef(&features.CreateFromRefInput{
		Ref:         refs.Features.StepOfTheWind().String(),
		Config:      config,
		CharacterID: "test-char",
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Require().NotNil(output.Feature)
	s.Assert().Equal(refs.Features.StepOfTheWind().ID, output.Feature.GetID())
}
