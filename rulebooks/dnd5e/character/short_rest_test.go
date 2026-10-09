package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"

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
	return attachRestFighter(t, restFighterData(t))
}

func restFighterData(t *testing.T) *Data {
	t.Helper()
	secondWind, err := json.Marshal(features.SecondWindData{
		Ref:         refs.Features.SecondWind(),
		ID:          "second-wind-short-rest",
		Name:        "Second Wind",
		CharacterID: "short-rest-fighter",
		Uses:        0,
		MaxUses:     1,
	})
	require.NoError(t, err)

	return &Data{Levels: syntheticLevels(classes.Fighter, 2),
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
	}
}

func attachRestFighter(t *testing.T, data *Data) *Character {
	t.Helper()
	ctx := context.Background()
	char, err := Load(ctx, data)
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

	require.NoError(t, restErr(char.LongRest(ctx)))

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

	require.NoError(t, restErr(char.LongRest(ctx)))
	require.Equal(t, 1, pool.Current())
}

// A short rest is an hour, and an hour ends a fight's conditions: a prone,
// blessed fighter stands up unblessed. The legacy unconscious shell is the one
// effect left on the long-rest path, and it stays.
func TestShortRestEndsAFightsConditions(t *testing.T) {
	ctx := context.Background()
	data := restFighterData(t)
	data.Conditions = []json.RawMessage{
		json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"prone"},"member_id":"short-rest-fighter"}`),
		json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"blessed"},"member_id":"short-rest-fighter","source_id":"cleric-1","source_ref":{"module":"dnd5e","type":"spells","id":"bless"}}`),
		json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"unconscious"},"member_id":"short-rest-fighter","successes":0,"failures":0,"stabilized":false,"dead":false}`),
	}
	char := attachRestFighter(t, data)
	heldNow := func() []string {
		var out []string
		for _, condition := range authored(char) {
			out = append(out, condition.Ref().String())
		}
		return out
	}
	require.Subset(t, heldNow(), []string{
		refs.Conditions.Prone().String(), refs.Conditions.Blessed().String(), refs.Conditions.Unconscious().String(),
	}, "all three are on the sheet before the rest")

	_, err := char.ShortRest(ctx, &ShortRestInput{})
	require.NoError(t, err)

	held := heldNow()
	require.NotContains(t, held, refs.Conditions.Prone().String())
	require.NotContains(t, held, refs.Conditions.Blessed().String())
	require.Contains(t, held, refs.Conditions.Unconscious().String(), "long-rest-only stays through a short rest")
}

// A rest reports which resources it refilled, each named by the feature that
// reports its pool: a short rest refills spent Second Wind and leaves spent
// Rage unlisted; a long rest lists Rage. A pool that was already full is not
// listed, because nothing rose.
func TestRestsReportWhatRefilled(t *testing.T) {
	ctx := context.Background()
	data := restFighterData(t)
	rage, err := features.LoadJSON(mustJSON(t, features.RageData{Ref: refs.Features.Rage(), ID: "rage-rest", Name: "Rage"}))
	require.NoError(t, err)
	rageBlob, err := rage.ToJSON()
	require.NoError(t, err)
	data.Features = append(data.Features, rageBlob)
	char := attachRestFighter(t, data)

	short, err := char.ShortRest(ctx, &ShortRestInput{})
	require.NoError(t, err)
	require.Equal(t, []string{refs.Features.SecondWind().String()}, refilledStrings(short.Refilled))

	long, err := char.LongRest(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{refs.Features.Rage().String()}, refilledStrings(long.Refilled),
		"Second Wind was refilled by the short rest, so only Rage rose")
}

// A pool no feature reports is named by its key.
func TestARestNamesAnUnreportedPoolByItsKey(t *testing.T) {
	char := restFighter(t)

	long, err := char.LongRest(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{
		refs.Features.SecondWind().String(),
		"dnd5e:resources:rage_charges",
	}, refilledStrings(long.Refilled))
}

func refilledStrings(in []*core.Ref) []string {
	var out []string
	for _, ref := range in {
		out = append(out, ref.String())
	}
	return out
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

// Hit dice are counted on their own and never listed as refilled, even by the
// long rest that returns them.
func TestARestNeverListsHitDice(t *testing.T) {
	ctx := context.Background()
	char := restFighter(t)
	_, err := char.ShortRest(ctx, &ShortRestInput{HitDice: 2, Roller: &mockHitDiceRoller{rolls: []int{1, 1}}})
	require.NoError(t, err)

	long, err := char.LongRest(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, char.GetResource(resources.HitDice).Current(), "the long rest did return a die")
	require.NotContains(t, refilledStrings(long.Refilled), "dnd5e:resources:hit_dice")
}

// The list is sorted by ref, not by the resource keys the pools are held
// under. Here the two orders disagree: by key, second_wind comes last; by ref,
// its feature ref (dnd5e:features:...) comes before every dnd5e:resources:
// pool. So a list left in key order fails, on every run.
func TestRefilledIsSortedByRef(t *testing.T) {
	char := restFighter(t)
	for _, key := range []coreResources.ResourceKey{"c_pool", "b_pool", "a_pool"} {
		pool := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID: string(key), Maximum: 1, CharacterID: char.GetID(), ResetType: coreResources.ResetLongRest,
		})
		require.NoError(t, pool.Use(1))
		char.AddResource(key, pool)
	}

	long, err := char.LongRest(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{
		refs.Features.SecondWind().String(),
		"dnd5e:resources:a_pool",
		"dnd5e:resources:b_pool",
		"dnd5e:resources:c_pool",
		"dnd5e:resources:rage_charges",
	}, refilledStrings(long.Refilled))
}

// A pool that rose during the rest for another reason is not a refill: the
// rest's own reset kind has to have fired for it.
func TestAPoolThatRoseForAnotherReasonIsNotARefill(t *testing.T) {
	ctx := context.Background()
	char := restFighter(t)
	rage := char.GetResource(resources.RageCharges)
	require.Equal(t, 0, rage.Current())

	// Something else on the bus hands back a rage charge while the short
	// rest is under way. Rage resets on a long rest; a short rest did not
	// refill it.
	_, err := dnd5eEvents.RestTopic.On(char.bus).Subscribe(ctx,
		func(_ context.Context, _ dnd5eEvents.RestEvent) error {
			rage.Restore(1)
			return nil
		})
	require.NoError(t, err)

	short, err := char.ShortRest(ctx, &ShortRestInput{})
	require.NoError(t, err)
	require.Equal(t, 1, rage.Current(), "the pool did rise")
	require.Equal(t, []string{refs.Features.SecondWind().String()}, refilledStrings(short.Refilled))
}

// The reset-kind check covers a pool a feature reports when the sheet also
// holds it: Rage reports the sheet's long-rest rage charges, so a charge
// handed back during a short rest is not a refill, under the feature's name
// as under the key.
func TestAFeatureReportedPoolKeepsItsResetKind(t *testing.T) {
	ctx := context.Background()
	data := restFighterData(t)
	rageFeature, err := features.LoadJSON(mustJSON(t, features.RageData{Ref: refs.Features.Rage(), ID: "rage-kind", Name: "Rage"}))
	require.NoError(t, err)
	blob, err := rageFeature.ToJSON()
	require.NoError(t, err)
	data.Features = append(data.Features, blob)
	char := attachRestFighter(t, data)
	rage := char.GetResource(resources.RageCharges)

	_, err = dnd5eEvents.RestTopic.On(char.bus).Subscribe(ctx,
		func(_ context.Context, _ dnd5eEvents.RestEvent) error {
			rage.Restore(1)
			return nil
		})
	require.NoError(t, err)

	short, err := char.ShortRest(ctx, &ShortRestInput{})
	require.NoError(t, err)
	require.Equal(t, 1, rage.Current(), "the pool did rise")
	require.NotContains(t, refilledStrings(short.Refilled), refs.Features.Rage().String())
	require.Equal(t, []string{refs.Features.SecondWind().String()}, refilledStrings(short.Refilled))
}
