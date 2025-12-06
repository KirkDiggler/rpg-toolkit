package features

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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type ActionSurgeTestSuite struct {
	suite.Suite
	actionSurge   *ActionSurge
	bus           events.EventBus
	actionEconomy *combat.ActionEconomy
	owner         core.Entity
}

func TestActionSurgeSuite(t *testing.T) {
	suite.Run(t, new(ActionSurgeTestSuite))
}

func (s *ActionSurgeTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.actionEconomy = combat.NewActionEconomy()
	s.owner = &mockEntity{id: "fighter-1", entityType: "character"}

	// Create action surge via factory
	output, err := CreateFromRef(&CreateFromRefInput{
		Ref:         refs.Features.ActionSurge().String(),
		CharacterID: "fighter-1",
		Config:      nil, // Use defaults
	})
	s.Require().NoError(err)
	s.Require().NotNil(output)

	var ok bool
	s.actionSurge, ok = output.Feature.(*ActionSurge)
	s.Require().True(ok, "feature should be *ActionSurge")
	s.Require().NotNil(s.actionSurge)
}

func (s *ActionSurgeTestSuite) SetupSubTest() {
	// Reset action economy and recreate action surge for each subtest
	s.actionEconomy = combat.NewActionEconomy()

	// Create fresh action surge
	output, err := CreateFromRef(&CreateFromRefInput{
		Ref:         refs.Features.ActionSurge().String(),
		CharacterID: "fighter-1",
		Config:      nil,
	})
	s.Require().NoError(err)
	s.Require().NotNil(output)

	var ok bool
	s.actionSurge, ok = output.Feature.(*ActionSurge)
	s.Require().True(ok, "feature should be *ActionSurge")
	s.Require().NotNil(s.actionSurge)
}

func (s *ActionSurgeTestSuite) TestGetID() {
	s.Run("returns correct ID", func() {
		s.Equal(refs.Features.ActionSurge().ID, s.actionSurge.GetID())
	})
}

func (s *ActionSurgeTestSuite) TestGetType() {
	s.Run("returns feature type", func() {
		s.Equal(EntityTypeFeature, s.actionSurge.GetType())
	})
}

func (s *ActionSurgeTestSuite) TestCanActivate() {
	s.Run("succeeds when resource available and action economy provided", func() {
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}
		err := s.actionSurge.CanActivate(context.Background(), s.owner, input)
		s.NoError(err)
	})

	s.Run("fails when no action economy provided", func() {
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: nil,
		}
		err := s.actionSurge.CanActivate(context.Background(), s.owner, input)
		s.Error(err)
		var rpgErr *rpgerr.Error
		s.True(errors.As(err, &rpgErr))
		s.Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
		s.Contains(err.Error(), "action economy is required")
	})

	s.Run("fails when resource exhausted", func() {
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}

		// Use up the action surge
		err := s.actionSurge.Activate(context.Background(), s.owner, input)
		s.Require().NoError(err)

		// Try to activate again
		err = s.actionSurge.CanActivate(context.Background(), s.owner, input)
		s.Error(err)
		s.True(rpgerr.IsResourceExhausted(err))
		s.Contains(err.Error(), "no action surge uses remaining")
	})
}

func (s *ActionSurgeTestSuite) TestActivate() {
	s.Run("grants extra action on success", func() {
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}

		// Initial state: 1 action
		s.Equal(1, s.actionEconomy.ActionsRemaining)

		// Activate action surge
		err := s.actionSurge.Activate(context.Background(), s.owner, input)
		s.NoError(err)

		// Should now have 2 actions
		s.Equal(2, s.actionEconomy.ActionsRemaining)
	})

	s.Run("consumes resource on activation", func() {
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}

		// Should be available initially
		s.True(s.actionSurge.resource.IsAvailable())

		// Activate
		err := s.actionSurge.Activate(context.Background(), s.owner, input)
		s.NoError(err)

		// Should no longer be available
		s.False(s.actionSurge.resource.IsAvailable())
	})

	s.Run("fails when resource exhausted", func() {
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}

		// Use action surge once
		err := s.actionSurge.Activate(context.Background(), s.owner, input)
		s.Require().NoError(err)

		// Reset action economy for second attempt
		s.actionEconomy = combat.NewActionEconomy()
		input.ActionEconomy = s.actionEconomy

		// Try to use again - should fail
		err = s.actionSurge.Activate(context.Background(), s.owner, input)
		s.Error(err)
		s.True(rpgerr.IsResourceExhausted(err))

		// Action economy should not have been modified
		s.Equal(1, s.actionEconomy.ActionsRemaining)
	})

	s.Run("grants action even when actions depleted", func() {
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}

		// Use up the normal action
		err := s.actionEconomy.UseAction()
		s.Require().NoError(err)
		s.Equal(0, s.actionEconomy.ActionsRemaining)

		// Activate action surge
		err = s.actionSurge.Activate(context.Background(), s.owner, input)
		s.NoError(err)

		// Should now have 1 action available
		s.Equal(1, s.actionEconomy.ActionsRemaining)
	})
}

func (s *ActionSurgeTestSuite) TestApplyRemove() {
	s.Run("apply and remove work with event bus", func() {
		ctx := context.Background()

		// Apply should succeed
		err := s.actionSurge.Apply(ctx, s.bus)
		s.NoError(err)

		// Remove should succeed
		err = s.actionSurge.Remove(ctx, s.bus)
		s.NoError(err)
	})
}

func (s *ActionSurgeTestSuite) TestSerialization() {
	s.Run("serializes and deserializes correctly", func() {
		// Use the action surge
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}
		err := s.actionSurge.Activate(context.Background(), s.owner, input)
		s.Require().NoError(err)

		// Serialize
		data, err := s.actionSurge.ToJSON()
		s.Require().NoError(err)
		s.NotNil(data)

		// Deserialize into new instance
		loaded, err := LoadJSON(data)
		s.Require().NoError(err)
		s.Require().NotNil(loaded)

		loadedActionSurge, ok := loaded.(*ActionSurge)
		s.Require().True(ok)

		// Verify state
		s.Equal(s.actionSurge.id, loadedActionSurge.id)
		s.Equal(s.actionSurge.name, loadedActionSurge.name)
		s.Equal(s.actionSurge.characterID, loadedActionSurge.characterID)
		s.Equal(s.actionSurge.resource.Current(), loadedActionSurge.resource.Current())
		s.Equal(s.actionSurge.resource.Maximum(), loadedActionSurge.resource.Maximum())

		// Should not be available (was used before serialization)
		s.False(loadedActionSurge.resource.IsAvailable())
	})

	s.Run("serialized data contains correct ref", func() {
		data, err := s.actionSurge.ToJSON()
		s.Require().NoError(err)

		var parsed ActionSurgeData
		err = json.Unmarshal(data, &parsed)
		s.Require().NoError(err)

		s.Equal(refs.Module, parsed.Ref.Module)
		s.Equal(refs.TypeFeatures, parsed.Ref.Type)
		s.Equal(refs.Features.ActionSurge().ID, parsed.Ref.ID)
	})
}

func (s *ActionSurgeTestSuite) TestShortRestRecovery() {
	s.Run("recovers uses on short rest", func() {
		ctx := context.Background()

		// Apply to event bus
		err := s.actionSurge.Apply(ctx, s.bus)
		s.Require().NoError(err)

		// Use action surge
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}
		err = s.actionSurge.Activate(ctx, s.owner, input)
		s.Require().NoError(err)
		s.False(s.actionSurge.resource.IsAvailable())

		// Publish short rest event
		topic := dnd5eEvents.RestTopic.On(s.bus)
		err = topic.Publish(ctx, dnd5eEvents.RestEvent{
			CharacterID: "fighter-1",
			RestType:    coreResources.ResetShortRest,
		})
		s.Require().NoError(err)

		// Should be available again
		s.True(s.actionSurge.resource.IsAvailable())
	})

	s.Run("recovers uses on long rest", func() {
		ctx := context.Background()

		// Apply to event bus
		err := s.actionSurge.Apply(ctx, s.bus)
		s.Require().NoError(err)

		// Use action surge
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}
		err = s.actionSurge.Activate(ctx, s.owner, input)
		s.Require().NoError(err)
		s.False(s.actionSurge.resource.IsAvailable())

		// Publish long rest event
		topic := dnd5eEvents.RestTopic.On(s.bus)
		err = topic.Publish(ctx, dnd5eEvents.RestEvent{
			CharacterID: "fighter-1",
			RestType:    coreResources.ResetLongRest,
		})
		s.Require().NoError(err)

		// Should be available again
		s.True(s.actionSurge.resource.IsAvailable())
	})

	s.Run("does not recover for other characters", func() {
		ctx := context.Background()

		// Apply to event bus
		err := s.actionSurge.Apply(ctx, s.bus)
		s.Require().NoError(err)

		// Use action surge
		input := FeatureInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
		}
		err = s.actionSurge.Activate(ctx, s.owner, input)
		s.Require().NoError(err)
		s.False(s.actionSurge.resource.IsAvailable())

		// Publish rest event for different character
		topic := dnd5eEvents.RestTopic.On(s.bus)
		err = topic.Publish(ctx, dnd5eEvents.RestEvent{
			CharacterID: "other-character",
			RestType:    coreResources.ResetShortRest,
		})
		s.Require().NoError(err)

		// Should still not be available
		s.False(s.actionSurge.resource.IsAvailable())
	})
}

// mockEntity is a simple mock for testing
type mockEntity struct {
	id         string
	entityType core.EntityType
}

func (m *mockEntity) GetID() string {
	return m.id
}

func (m *mockEntity) GetType() core.EntityType {
	return m.entityType
}
