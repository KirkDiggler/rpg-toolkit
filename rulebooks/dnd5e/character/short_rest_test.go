package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
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
		classID:      classes.Fighter,
		levels:       syntheticLevels(classes.Fighter, 2),
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
		_, err := s.character.ShortRest(s.ctx, &ShortRestInput{})

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
		_, err := s.character.ShortRest(s.ctx, &ShortRestInput{})

		// Assert
		s.Require().NoError(err)
		s.Equal(0, rageResource.Current(), "rage should NOT be restored by short rest")
	})

	s.Run("does NOT restore HP", func() {
		// Arrange: Character is at half HP
		s.character.hitPoints = 10
		s.character.maxHitPoints = 20

		// Act
		_, err := s.character.ShortRest(s.ctx, &ShortRestInput{})

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
		_, err := s.character.ShortRest(s.ctx, &ShortRestInput{})

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
		_, err = s.character.ShortRest(s.ctx, &ShortRestInput{})

		// Assert
		s.Require().NoError(err)
		s.True(receivedEvent, "RestEvent should be published")
		s.Equal(coreResources.ResetShortRest, receivedRestType, "RestEvent should have RestType ShortRest")
	})

	s.Run("returns error when bus is nil", func() {
		// Arrange: Character with no bus
		s.character.bus = nil

		// Act
		_, err := s.character.ShortRest(s.ctx, &ShortRestInput{})

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
		_, err := s.character.ShortRest(s.ctx, &ShortRestInput{})

		// Assert
		s.Require().NoError(err)
		s.Equal(1, secondWindResource.Current(), "second wind should be restored")
		s.Equal(1, actionSurgeResource.Current(), "action surge should be restored")
	})
}

func TestShortRestSuite(t *testing.T) {
	suite.Run(t, new(ShortRestTestSuite))
}

// restFighter is a loaded, attached level-2 fighter with CON 14 (+2): its
// Second Wind spent, its Rage charges spent, both hit dice still to spend.
// Rage on a fighter is only a long-rest pool here, standing for every pool
// whose own reset kind is the long rest.
func restFighter(t *testing.T) *Character {
	t.Helper()
	ctx := context.Background()
	secondWind, err := json.Marshal(features.SecondWindData{
		Ref:         refs.Features.SecondWind(),
		ID:          "second-wind-short-rest",
		Name:        "Second Wind",
		CharacterID: "short-rest-fighter",
		Uses:        0,
		MaxUses:     1,
	})
	require.NoError(t, err)

	char, err := Load(ctx, &Data{Levels: syntheticLevels(classes.Fighter, 2),
		ID:               "short-rest-fighter",
		PlayerID:         "short-rest-player",
		Name:             "Short Rest Fighter",
		Level:            2,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 12, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 10,
		},
		HitPoints:    3,
		MaxHitPoints: 24,
		Resources: map[coreResources.ResourceKey]RecoverableResourceData{
			resources.HitDice:     {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest},
			resources.RageCharges: {Current: 0, Maximum: 2, ResetType: coreResources.ResetLongRest},
		},
		Features: []json.RawMessage{secondWind},
	})
	require.NoError(t, err)
	require.NoError(t, Attach(ctx, char, events.NewEventBus()))
	t.Cleanup(func() { require.NoError(t, char.Cleanup(ctx)) })
	return char
}

func secondWindUses(t *testing.T, char *Character) int {
	t.Helper()
	var data features.SecondWindData
	require.NoError(t, json.Unmarshal(featureByRef(t, mustToData(t, char).Features, refs.Features.SecondWind()), &data))
	return data.Uses
}

// Two hit dice for a level-2 fighter with CON +2 heal both rolls plus 4 and
// spend exactly two; the short-rest resource refills and the long-rest one
// does not.
func TestShortRestSpendsHitDiceAndRefillsByResetKind(t *testing.T) {
	ctx := context.Background()
	char := restFighter(t)
	roller := &mockHitDiceRoller{rolls: []int{3, 8}}

	out, err := char.ShortRest(ctx, &ShortRestInput{HitDice: 2, Roller: roller})
	require.NoError(t, err)

	require.Equal(t, 1, roller.calls, "the roller it was handed threw the dice")
	require.Equal(t, 2, out.HitDiceSpent)
	require.Equal(t, 15, out.Requested, "3 + 8 + 2*2")
	require.Equal(t, 15, out.Healed, "all of it lands")
	require.Equal(t, 18, char.GetHitPoints(), "3 + 15")
	require.Equal(t, 0, char.GetResource(resources.HitDice).Current(), "exactly two dice spent")
	require.Equal(t, 1, secondWindUses(t, char), "Second Wind resets on a short rest")
	require.Equal(t, 0, char.GetResource(resources.RageCharges).Current(), "Rage resets on a long rest only")
	require.True(t, char.IsDirty())
}

func TestShortRestNeverHealsPastMaximum(t *testing.T) {
	char := restFighter(t)

	out, err := char.ShortRest(context.Background(), &ShortRestInput{
		HitDice: 2, Roller: &mockHitDiceRoller{rolls: []int{10, 10}},
	})
	require.NoError(t, err)
	require.Equal(t, 24, out.Requested, "10 + 10 + 2*2 asked")
	require.Equal(t, 21, out.Healed, "3 to 24 is what landed")
	require.Equal(t, 24, char.GetHitPoints(), "capped at maximum")
}

// Asking for more dice than remain refuses and spends nothing — not a die,
// not a pool, not a resource the rest would otherwise have refilled.
func TestShortRestRefusingTheCountSpendsNothing(t *testing.T) {
	char := restFighter(t)
	markSaved(char)
	roller := &mockHitDiceRoller{rolls: []int{5}}

	out, err := char.ShortRest(context.Background(), &ShortRestInput{HitDice: 3, Roller: roller})
	require.Error(t, err)
	require.Nil(t, out)
	require.Zero(t, roller.calls)
	require.Equal(t, 2, char.GetResource(resources.HitDice).Current())
	require.Equal(t, 3, char.GetHitPoints())
	require.Equal(t, 0, secondWindUses(t, char), "a refused rest refills nothing")
	require.False(t, char.IsDirty())
}

// A long rest refills both kinds and returns half the hit dice.
func TestLongRestRefillsBothKindsAndHalfTheHitDice(t *testing.T) {
	ctx := context.Background()
	char := restFighter(t)
	_, err := char.ShortRest(ctx, &ShortRestInput{HitDice: 2, Roller: &mockHitDiceRoller{rolls: []int{1, 1}}})
	require.NoError(t, err)
	require.Equal(t, 0, char.GetResource(resources.HitDice).Current())

	require.NoError(t, char.LongRest(ctx))

	require.Equal(t, 1, secondWindUses(t, char))
	require.Equal(t, 2, char.GetResource(resources.RageCharges).Current())
	require.Equal(t, 1, char.GetResource(resources.HitDice).Current(), "half of two")
	require.Equal(t, 24, char.GetHitPoints())
}

// Half of one hit die is none; the long rest returns at least one.
func TestLongRestReturnsAtLeastOneHitDie(t *testing.T) {
	ctx := context.Background()
	char := restFighter(t)
	pool := resources.NewHitDiceResource(resources.HitDiceResourceConfig{CharacterID: char.GetID(), Level: 1})
	require.NoError(t, pool.Use(1))
	char.AddResource(resources.HitDice, pool)

	require.NoError(t, char.LongRest(ctx))
	require.Equal(t, 1, pool.Current())
}
