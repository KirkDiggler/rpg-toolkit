package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/require"
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

	s.Run("recovers hit dice (half level, minimum 1)", func() {
		// Arrange: Add hit dice resource with 0 remaining
		hitDiceResource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          string(resources.HitDice),
			Maximum:     4, // Level 4 character
			CharacterID: s.character.id,
			ResetType:   coreResources.ResetLongRest,
			// Note: RecoveryFunc not needed - LongRest handles hit dice specially
		})
		_ = hitDiceResource.Use(4) // Deplete all hit dice
		s.Require().Equal(0, hitDiceResource.Current(), "hit dice should be depleted")

		// Add resource to character (no event subscription needed - LongRest handles directly)
		s.character.AddResource(resources.HitDice, hitDiceResource)

		// Act
		err := s.character.LongRest(s.ctx)

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

func TestLongRestPersistsCompleteRecoveryOnAttachedSheet(t *testing.T) {
	ctx := context.Background()
	secondWind, err := json.Marshal(features.SecondWindData{
		Ref:         refs.Features.SecondWind(),
		ID:          "second-wind-rest",
		Name:        "Second Wind",
		Level:       4,
		CharacterID: "rest-fighter",
		Uses:        0,
		MaxUses:     1,
	})
	require.NoError(t, err)

	shortRestPool := coreResources.ResourceKey("test-short-rest-pool")
	data := &Data{
		ID:               "rest-fighter",
		PlayerID:         "rest-player",
		Name:             "Rest Fighter",
		Level:            4,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:      11,
		MaxHitPoints:   36,
		ArmorClass:     16,
		DeathSaveState: &saves.DeathSaveState{Successes: 1, Failures: 2},
		SpellSlots:     map[int]SpellSlotData{1: {Max: 3, Used: 2}},
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			shortRestPool:     {Current: 0, Maximum: 2, ResetType: coreResources.ResetShortRest},
			resources.HitDice: {Current: 0, Maximum: 4, ResetType: coreResources.ResetLongRest},
		},
		Features: []json.RawMessage{secondWind},
	}

	char, err := Load(ctx, data)
	require.NoError(t, err)
	bus := events.NewEventBus()
	require.NoError(t, Attach(ctx, char, bus))
	t.Cleanup(func() { require.NoError(t, char.Cleanup(ctx)) })

	require.NoError(t, char.LongRest(ctx))
	got := char.ToData()
	require.Equal(t, 36, got.HitPoints)
	require.Equal(t, 36, got.MaxHitPoints)
	if got.DeathSaveState != nil {
		require.Zero(t, got.DeathSaveState.Successes)
		require.Zero(t, got.DeathSaveState.Failures)
	}
	require.Equal(t, 2, got.Resources[shortRestPool].Current)
	require.Equal(t, 2, got.Resources[resources.HitDice].Current,
		"a level-four character recovers half its hit dice, not all four")

	var restoredSecondWind features.SecondWindData
	require.NoError(t, json.Unmarshal(featureByRef(t, got.Features, refs.Features.SecondWind()), &restoredSecondWind))
	require.Equal(t, 1, restoredSecondWind.Uses)
	require.Equal(t, 1, restoredSecondWind.MaxUses)

	require.Equal(t, 0, got.SpellSlots[1].Used)
}

func featureByRef(t *testing.T, blobs []json.RawMessage, want *core.Ref) json.RawMessage {
	t.Helper()
	for _, raw := range blobs {
		var envelope struct {
			Ref core.Ref `json:"ref"`
		}
		require.NoError(t, json.Unmarshal(raw, &envelope))
		if envelope.Ref.Equals(want) {
			return raw
		}
	}
	t.Fatalf("feature %s not found", want.String())
	return nil
}
