package character

import (
	"context"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

// ShortRestTestSuite tests the Character.ShortRest() functionality
type ShortRestTestSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	character *Character
}

func (s *ShortRestTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.createFreshCharacter()
}

func (s *ShortRestTestSuite) SetupSubTest() {
	// Reset to fresh state for each subtest
	if s.character != nil {
		_ = s.character.Cleanup(s.ctx)
	}
	s.bus = events.NewEventBus()
	s.createFreshCharacter()
}

func (s *ShortRestTestSuite) createFreshCharacter() {
	// Create a level 2 Fighter with 14 CON
	s.character = &Character{
		id:           "test-fighter",
		level:        2,
		hitDice:      10, // d10
		hitPoints:    10, // Half HP (20 max)
		maxHitPoints: 20,
		abilityScores: shared.AbilityScores{
			abilities.CON: 14, // +2 modifier
		},
		bus:       s.bus,
		resources: make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}

	// Subscribe to events
	err := s.character.subscribeToEvents(s.ctx)
	s.Require().NoError(err)
}

func (s *ShortRestTestSuite) TearDownTest() {
	if s.character != nil {
		_ = s.character.Cleanup(s.ctx)
	}
}

func (s *ShortRestTestSuite) TestShortRest() {
	s.Run("restores resources that reset on short rest", func() {
		// Arrange: Add Second Wind resource with 0 uses remaining
		secondWindResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          "second-wind",
			Maximum:     1,
			CharacterID: s.character.id,
			ResetType:   coreResources.ResetShortRest,
		})
		_ = secondWindResource.Use(1) // Deplete all uses
		s.Require().Equal(0, secondWindResource.Current(), "second wind should be depleted")

		s.character.AddResource("second-wind", secondWindResource)

		// Act
		err := s.character.ShortRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		s.Equal(1, secondWindResource.Current(), "second wind uses should be restored")
	})

	s.Run("does NOT restore resources that reset on long rest", func() {
		// Arrange: Add Rage resource (long rest only) with 0 uses remaining
		rageResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          "rage",
			Maximum:     2,
			CharacterID: s.character.id,
			ResetType:   coreResources.ResetLongRest,
		})
		_ = rageResource.Use(2) // Deplete all uses
		s.Require().Equal(0, rageResource.Current(), "rage should be depleted")

		s.character.AddResource("rage", rageResource)

		// Act
		err := s.character.ShortRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		s.Equal(0, rageResource.Current(), "rage should NOT be restored by short rest")
	})

	s.Run("does NOT restore HP", func() {
		// Arrange: Character is at half HP
		s.character.hitPoints = 10
		s.character.maxHitPoints = 20

		// Act
		err := s.character.ShortRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		s.Equal(10, s.character.GetHitPoints(), "HP should NOT be restored by short rest")
	})

	s.Run("does NOT clear death save state", func() {
		// Arrange: Character has death save failures
		s.character.deathSaveState = &saves.DeathSaveState{
			Successes: 1,
			Failures:  2,
		}

		// Act
		err := s.character.ShortRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		state := s.character.GetDeathSaveState()
		s.Equal(1, state.Successes, "successes should NOT be cleared by short rest")
		s.Equal(2, state.Failures, "failures should NOT be cleared by short rest")
	})

	s.Run("publishes RestEvent for conditions to react", func() {
		// Arrange: Subscribe to rest events to verify publication
		var receivedEvent bool
		var receivedRestType coreResources.ResetType
		restTopic := dnd5eEvents.RestTopic.On(s.bus)
		_, err := restTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.RestEvent) error {
			receivedEvent = true
			receivedRestType = event.RestType
			return nil
		})
		s.Require().NoError(err)

		// Act
		err = s.character.ShortRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		s.True(receivedEvent, "RestEvent should be published")
		s.Equal(coreResources.ResetShortRest, receivedRestType, "RestEvent should have RestType ShortRest")
	})

	s.Run("returns error when bus is nil", func() {
		// Arrange: Character with no bus
		s.character.bus = nil

		// Act
		err := s.character.ShortRest(s.ctx)

		// Assert
		s.Error(err)
		s.Contains(err.Error(), "no event bus")
	})

	s.Run("works with multiple short rest resources", func() {
		// Arrange: Add multiple short rest resources
		secondWindResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          "second-wind",
			Maximum:     1,
			CharacterID: s.character.id,
			ResetType:   coreResources.ResetShortRest,
		})
		_ = secondWindResource.Use(1)

		actionSurgeResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          "action-surge",
			Maximum:     1,
			CharacterID: s.character.id,
			ResetType:   coreResources.ResetShortRest,
		})
		_ = actionSurgeResource.Use(1)

		s.character.AddResource("second-wind", secondWindResource)
		s.character.AddResource("action-surge", actionSurgeResource)

		// Act
		err := s.character.ShortRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		s.Equal(1, secondWindResource.Current(), "second wind should be restored")
		s.Equal(1, actionSurgeResource.Current(), "action surge should be restored")
	})
}

func TestShortRestSuite(t *testing.T) {
	suite.Run(t, new(ShortRestTestSuite))
}
