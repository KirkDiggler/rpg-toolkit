package character

import (
	"context"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

// LongRestTestSuite tests the Character.LongRest() functionality
type LongRestTestSuite struct {
	suite.Suite
	ctx       context.Context
	bus       events.EventBus
	character *Character
}

func (s *LongRestTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.createFreshCharacter()
}

func (s *LongRestTestSuite) SetupSubTest() {
	// Reset to fresh state for each subtest
	if s.character != nil {
		_ = s.character.Cleanup(s.ctx)
	}
	s.bus = events.NewEventBus()
	s.createFreshCharacter()
}

func (s *LongRestTestSuite) createFreshCharacter() {
	// Create a level 4 Barbarian with 14 CON
	s.character = &Character{
		id:           "test-barbarian",
		level:        4,
		hitDice:      12, // d12
		hitPoints:    20, // Half HP (40 max)
		maxHitPoints: 40,
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

func (s *LongRestTestSuite) TearDownTest() {
	if s.character != nil {
		_ = s.character.Cleanup(s.ctx)
	}
}

func (s *LongRestTestSuite) TestLongRest() {
	s.Run("restores HP to maximum", func() {
		// Arrange: Character is at half HP
		s.character.hitPoints = 20
		s.character.maxHitPoints = 40

		// Act
		err := s.character.LongRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		s.Equal(40, s.character.GetHitPoints(), "HP should be restored to maximum")
	})

	s.Run("clears death save state", func() {
		// Arrange: Character has death save failures
		s.character.deathSaveState = &saves.DeathSaveState{
			Successes: 1,
			Failures:  2,
		}

		// Act
		err := s.character.LongRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		state := s.character.GetDeathSaveState()
		s.Equal(0, state.Successes, "successes should be cleared")
		s.Equal(0, state.Failures, "failures should be cleared")
	})

	s.Run("publishes RestEvent that triggers resource recovery", func() {
		// Arrange: Add a rage resource with 0 uses remaining
		rageResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          "rage",
			Maximum:     3,
			CharacterID: s.character.id,
			ResetType:   coreResources.ResetLongRest,
		})
		_ = rageResource.Use(3) // Deplete all uses
		s.Require().Equal(0, rageResource.Current(), "rage should be depleted")

		// Apply resource so it subscribes to RestTopic
		err := rageResource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		s.character.AddResource("rage", rageResource)

		// Act
		err = s.character.LongRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		s.Equal(3, rageResource.Current(), "rage uses should be restored via RestEvent")
	})

	s.Run("recovers hit dice (half level, minimum 1) via RestEvent", func() {
		// Arrange: Add hit dice resource with 0 remaining
		hitDiceResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          string(resources.HitDice),
			Maximum:     4, // Level 4 character
			CharacterID: s.character.id,
			ResetType:   coreResources.ResetLongRest,
			RecoveryFunc: func(r *combat.RecoverableResource) {
				// D&D 5e: Recover half (minimum 1) on long rest
				amount := r.Maximum() / 2
				if amount < 1 {
					amount = 1
				}
				r.Restore(amount)
			},
		})
		_ = hitDiceResource.Use(4) // Deplete all hit dice
		s.Require().Equal(0, hitDiceResource.Current(), "hit dice should be depleted")

		// Apply resource so it subscribes to RestTopic
		err := hitDiceResource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
		s.character.AddResource(resources.HitDice, hitDiceResource)

		// Act
		err = s.character.LongRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		// Should recover 2 hit dice (half of 4)
		s.Equal(2, hitDiceResource.Current(), "hit dice should recover half (2 of 4)")
	})

	s.Run("returns error when bus is nil", func() {
		// Arrange: Character with no bus
		s.character.bus = nil

		// Act
		err := s.character.LongRest(s.ctx)

		// Assert
		s.Error(err)
		s.Contains(err.Error(), "no event bus")
	})

	s.Run("works when character is already at full HP", func() {
		// Arrange: Character already at full HP
		s.character.hitPoints = 40
		s.character.maxHitPoints = 40

		// Act
		err := s.character.LongRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		s.Equal(40, s.character.GetHitPoints(), "HP should remain at maximum")
	})

	s.Run("works when death save state is nil", func() {
		// Arrange: No death save state
		s.character.deathSaveState = nil

		// Act
		err := s.character.LongRest(s.ctx)

		// Assert
		s.Require().NoError(err)
		// Should not panic, state should be cleared (or nil is fine)
	})
}

func TestLongRestSuite(t *testing.T) {
	suite.Run(t, new(LongRestTestSuite))
}
