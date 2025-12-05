package combat

import (
	"context"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type RecoverableResourceTestSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	resource *RecoverableResource
}

func TestRecoverableResourceSuite(t *testing.T) {
	suite.Run(t, new(RecoverableResourceTestSuite))
}

func (s *RecoverableResourceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()

	// Create a test resource that recovers on short rest
	s.resource = NewRecoverableResource(RecoverableResourceConfig{
		ID:          "test-resource",
		Maximum:     3,
		CharacterID: "char-1",
		ResetType:   coreResources.ResetShortRest,
	})
}

func (s *RecoverableResourceTestSuite) SetupSubTest() {
	// Reset to clean state for each subtest
	s.resource = NewRecoverableResource(RecoverableResourceConfig{
		ID:          "test-resource",
		Maximum:     3,
		CharacterID: "char-1",
		ResetType:   coreResources.ResetShortRest,
	})
}

func (s *RecoverableResourceTestSuite) TestNewRecoverableResource() {
	s.Run("creates with config values", func() {
		config := RecoverableResourceConfig{
			ID:          "spell-slots",
			Maximum:     4,
			CharacterID: "wizard-1",
			ResetType:   coreResources.ResetLongRest,
		}

		resource := NewRecoverableResource(config)

		s.Require().NotNil(resource)
		s.Equal("spell-slots", resource.ID())
		s.Equal(4, resource.Maximum())
		s.Equal(4, resource.Current(), "should start at full")
		s.Equal("wizard-1", resource.CharacterID)
		s.Equal(coreResources.ResetLongRest, resource.ResetType)
		s.False(resource.IsApplied(), "should not be applied initially")
	})
}

func (s *RecoverableResourceTestSuite) TestApply() {
	s.Run("subscribes to rest events", func() {
		err := s.resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		s.True(s.resource.IsApplied())
		s.NotEmpty(s.resource.subscriptionID)
	})

	s.Run("returns error when already applied", func() {
		err := s.resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Try to apply again
		err = s.resource.Apply(s.ctx, s.bus)
		s.Error(err)
		s.Contains(err.Error(), "already applied")
	})
}

func (s *RecoverableResourceTestSuite) TestRemove() {
	s.Run("unsubscribes from rest events", func() {
		// First apply
		err := s.resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		s.True(s.resource.IsApplied())

		// Then remove
		err = s.resource.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
		s.False(s.resource.IsApplied())
		s.Empty(s.resource.subscriptionID)
	})

	s.Run("does nothing when not applied", func() {
		// Try to remove without applying first
		err := s.resource.Remove(s.ctx, s.bus)
		s.NoError(err)
		s.False(s.resource.IsApplied())
	})
}

func (s *RecoverableResourceTestSuite) TestIsApplied() {
	s.Run("returns false initially", func() {
		s.False(s.resource.IsApplied())
	})

	s.Run("returns true after apply", func() {
		err := s.resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		s.True(s.resource.IsApplied())
	})

	s.Run("returns false after remove", func() {
		err := s.resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		err = s.resource.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
		s.False(s.resource.IsApplied())
	})
}

func (s *RecoverableResourceTestSuite) TestOnRest_MatchingEvent() {
	s.Run("restores resource on matching rest type and character", func() {
		// Apply to subscribe to events
		err := s.resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use some of the resource
		err = s.resource.Use(2)
		s.Require().NoError(err)
		s.Equal(1, s.resource.Current())

		// Publish matching rest event
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetShortRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// Resource should be restored
		s.Equal(3, s.resource.Current(), "resource should be restored to maximum")
	})
}

func (s *RecoverableResourceTestSuite) TestOnRest_NonMatchingCharacterID() {
	s.Run("does not restore for different character", func() {
		// Apply to subscribe to events
		err := s.resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use some of the resource
		err = s.resource.Use(2)
		s.Require().NoError(err)
		s.Equal(1, s.resource.Current())

		// Publish rest event for different character
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetShortRest,
			CharacterID: "char-2", // Different character
		})
		s.Require().NoError(err)

		// Resource should NOT be restored
		s.Equal(1, s.resource.Current(), "resource should not be restored for different character")
	})
}

func (s *RecoverableResourceTestSuite) TestOnRest_LongRestRestoresShortRestResource() {
	s.Run("long rest restores short rest resource (D&D 5e rule)", func() {
		// Apply to subscribe to events
		err := s.resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use some of the resource (short rest resource)
		err = s.resource.Use(2)
		s.Require().NoError(err)
		s.Equal(1, s.resource.Current())

		// Publish long rest event - should restore short rest resource too
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetLongRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// Resource SHOULD be restored (long rest satisfies short rest)
		s.Equal(3, s.resource.Current(), "long rest should restore short rest resources (D&D 5e PHB p. 186)")
	})
}

func (s *RecoverableResourceTestSuite) TestOnRest_ShortRestDoesNotRestoreLongRest() {
	s.Run("short rest does not restore long rest resource", func() {
		// Create a long rest resource
		longRestResource := NewRecoverableResource(RecoverableResourceConfig{
			ID:          "long-rest-resource",
			Maximum:     5,
			CharacterID: "char-1",
			ResetType:   coreResources.ResetLongRest,
		})

		// Apply to subscribe to events
		err := longRestResource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use some of the resource
		err = longRestResource.Use(3)
		s.Require().NoError(err)
		s.Equal(2, longRestResource.Current())

		// Publish short rest event
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetShortRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// Resource should NOT be restored
		s.Equal(2, longRestResource.Current(), "long rest resource should not restore on short rest")
	})
}

func (s *RecoverableResourceTestSuite) TestOnRest_MultipleResources() {
	s.Run("only restores matching resources", func() {
		// Create multiple resources with different reset types
		shortRestResource := NewRecoverableResource(RecoverableResourceConfig{
			ID:          "short-rest-resource",
			Maximum:     3,
			CharacterID: "char-1",
			ResetType:   coreResources.ResetShortRest,
		})

		longRestResource := NewRecoverableResource(RecoverableResourceConfig{
			ID:          "long-rest-resource",
			Maximum:     5,
			CharacterID: "char-1",
			ResetType:   coreResources.ResetLongRest,
		})

		// Apply both
		err := shortRestResource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		err = longRestResource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use both resources
		err = shortRestResource.Use(2)
		s.Require().NoError(err)
		err = longRestResource.Use(3)
		s.Require().NoError(err)

		s.Equal(1, shortRestResource.Current())
		s.Equal(2, longRestResource.Current())

		// Publish short rest event
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetShortRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// Only short rest resource should be restored
		s.Equal(3, shortRestResource.Current(), "short rest resource should be restored")
		s.Equal(2, longRestResource.Current(), "long rest resource should not be restored")
	})
}

func (s *RecoverableResourceTestSuite) TestOnRest_NotApplied() {
	s.Run("does not restore if not applied", func() {
		// Don't apply (no subscription)

		// Use some of the resource
		err := s.resource.Use(2)
		s.Require().NoError(err)
		s.Equal(1, s.resource.Current())

		// Publish matching rest event
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetShortRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// Resource should NOT be restored (not subscribed)
		s.Equal(1, s.resource.Current(), "resource should not restore when not applied")
	})
}

func (s *RecoverableResourceTestSuite) TestResourceFunctionality() {
	s.Run("retains composed resource functionality", func() {
		// Test that we can still use the wrapper methods for Resource
		s.Equal(3, s.resource.Maximum())
		s.Equal(3, s.resource.Current())
		s.True(s.resource.IsFull())

		// Use resource
		err := s.resource.Use(2)
		s.Require().NoError(err)
		s.Equal(1, s.resource.Current())
		s.False(s.resource.IsFull())
		s.True(s.resource.IsAvailable())

		// Restore manually
		s.resource.Restore(1)
		s.Equal(2, s.resource.Current())

		// RestoreToFull
		s.resource.RestoreToFull()
		s.Equal(3, s.resource.Current())
		s.True(s.resource.IsFull())
	})
}
