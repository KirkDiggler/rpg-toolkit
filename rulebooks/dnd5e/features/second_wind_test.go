package features

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

type SecondWindTestSuite struct {
	suite.Suite
	bus        events.EventBus
	secondWind *SecondWind
	ctx        context.Context
}

// newSecondWindForTest creates a Second Wind feature for testing
func newSecondWindForTest(id string, level int, characterID string) *SecondWind {
	return &SecondWind{
		id:          id,
		name:        "Second Wind",
		level:       level,
		characterID: characterID,
		resource: combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          refs.Features.SecondWind().ID,
			Maximum:     1,
			CharacterID: characterID,
			ResetType:   coreResources.ResetShortRest,
		}),
	}
}

func (s *SecondWindTestSuite) SetupTest() {
	s.bus = events.NewEventBus()
	s.secondWind = newSecondWindForTest("second-wind-feature", 3, "fighter-1") // Level 3 fighter
	s.ctx = context.Background()
}

type deadSecondWindOwner struct {
	id             string
	hitPoints      int
	deathSaveState saves.DeathSaveState
	dirty          bool
}

func (o *deadSecondWindOwner) GetID() string            { return o.id }
func (o *deadSecondWindOwner) GetType() core.EntityType { return "character" }
func (o *deadSecondWindOwner) LifeState() combat.LifeState {
	return combat.ClassifyLifeState(combat.LifeStateInput{
		Kind:       combat.CombatantKindCharacter,
		Down:       o.hitPoints <= 0,
		Stabilized: o.deathSaveState.Stabilized,
		Dead:       o.deathSaveState.Dead,
	})
}

type countingSecondWindRoller struct {
	calls int
}

func (r *countingSecondWindRoller) Roll(_ context.Context, _ int) (int, error) {
	r.calls++
	return 10, nil
}

func (r *countingSecondWindRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	r.calls++
	return make([]int, count), nil
}

var _ dice.Roller = (*countingSecondWindRoller)(nil)

// fixedSecondWindRoller always rolls the same face, keeping Second Wind's
// published calculation exact without touching process-global randomness.
type fixedSecondWindRoller struct {
	face int
}

func (r *fixedSecondWindRoller) Roll(_ context.Context, _ int) (int, error) {
	return r.face, nil
}

func (r *fixedSecondWindRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	faces := make([]int, count)
	for i := range faces {
		faces[i] = r.face
	}
	return faces, nil
}

var _ dice.Roller = (*fixedSecondWindRoller)(nil)

func (s *SecondWindTestSuite) TestDeadOwnerIsRejectedBeforeRollOrSpend() {
	owner := &deadSecondWindOwner{
		id:             "fighter-1",
		deathSaveState: saves.DeathSaveState{Failures: 3, Dead: true},
	}
	roller := &countingSecondWindRoller{}
	published := 0
	_, err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			published++
			owner.hitPoints += event.Amount
			owner.deathSaveState = saves.DeathSaveState{}
			owner.dirty = true
			return nil
		},
	)
	s.Require().NoError(err)

	err = s.secondWind.Activate(s.ctx, owner, FeatureInput{Bus: s.bus, Roller: roller})

	s.Require().Error(err)
	s.Equal(rpgerr.CodeInvalidState, rpgerr.GetCode(err))
	s.Zero(roller.calls)
	s.Equal(1, s.secondWind.resource.Current())
	s.Zero(owner.hitPoints)
	s.Equal(saves.DeathSaveState{Failures: 3, Dead: true}, owner.deathSaveState)
	s.False(owner.dirty)
	s.Zero(published)
}

func (s *SecondWindTestSuite) TestCanActivate() {
	owner := &StubEntity{id: "fighter-1"}

	// Should be able to activate with uses available
	err := s.secondWind.CanActivate(s.ctx, owner, FeatureInput{})
	s.NoError(err)

	// Use second wind
	err = s.secondWind.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.NoError(err)

	// Should not be able to activate with no uses
	err = s.secondWind.CanActivate(s.ctx, owner, FeatureInput{})
	s.Error(err)
	s.Contains(err.Error(), "no second wind uses remaining")
}

func (s *SecondWindTestSuite) TestActivatePublishesHealingEvent() {
	owner := &StubEntity{id: "fighter-1"}

	// Track if healing event was published
	var receivedEvent *dnd5eEvents.HealingReceivedEvent
	topic := dnd5eEvents.HealingReceivedTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.NoError(err)

	// Activate second wind with no scoped roller: the production default rolls.
	err = s.secondWind.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.NoError(err)

	// Check that event was published
	s.NotNil(receivedEvent)
	s.Equal("fighter-1", receivedEvent.TargetID)
	s.Equal("second_wind", receivedEvent.Source)
	s.True(receivedEvent.SourceRef.Equals(refs.Features.SecondWind()))
	s.Equal("Second Wind", receivedEvent.SourceName)

	// The sourced calculation owns the requested healing: 1d10 + level (3).
	s.Require().NotNil(receivedEvent.Calculation, "Second Wind publishes a roll calculation")
	s.Equal(receivedEvent.Calculation.Total, receivedEvent.Amount)
	s.Require().Len(receivedEvent.Calculation.Components, 2)

	trace := receivedEvent.Calculation.Components[0].Dice
	s.Require().NotNil(trace)
	s.Equal("1d10", trace.Notation)
	s.Equal(10, trace.DieSize)
	s.GreaterOrEqual(trace.Subtotal, 1, "the d10 subtotal should be at least 1")
	s.LessOrEqual(trace.Subtotal, 10, "the d10 subtotal should be at most 10")
	s.Equal(trace.Subtotal, receivedEvent.Calculation.Components[0].Dice.Subtotal)
	for _, face := range trace.OriginalRolls {
		s.GreaterOrEqual(face, 1)
		s.LessOrEqual(face, 10)
	}

	s.Require().NotNil(receivedEvent.Calculation.Components[1].Modifier)
	s.Equal(3, *receivedEvent.Calculation.Components[1].Modifier,
		"Modifier should be fighter level")
	s.Equal(trace.Subtotal+3, receivedEvent.Calculation.Total)
}

func (s *SecondWindTestSuite) TestActivatePublishesSourcedRollCalculation() {
	owner := &StubEntity{id: "fighter-1"}
	sw := newSecondWindForTest("second-wind-feature", 1, "fighter-1")
	roller := &fixedSecondWindRoller{face: 6}

	var receivedEvent *dnd5eEvents.HealingReceivedEvent
	topic := dnd5eEvents.HealingReceivedTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	err = sw.Activate(s.ctx, owner, FeatureInput{Bus: s.bus, Roller: roller})
	s.Require().NoError(err)

	s.Require().NotNil(receivedEvent)
	s.Equal("fighter-1", receivedEvent.TargetID)
	s.Equal(0, sw.resource.Current(), "the spend precedes the roll")

	// One calculation, carrying the exact scoped roll and the sourced modifier.
	s.Require().NotNil(receivedEvent.Calculation)
	calculation := receivedEvent.Calculation
	s.Equal(7, calculation.Total)
	s.Equal(7, receivedEvent.Amount, "the calculation total owns the requested healing")
	s.Require().Len(calculation.Components, 2)

	dice := calculation.Components[0]
	s.True(dice.Source.Ref.Equals(refs.Features.SecondWind()))
	s.Equal("Second Wind", dice.Source.Name)
	s.Require().NotNil(dice.Dice)
	s.Equal("1d10", dice.Dice.Notation)
	s.Equal(10, dice.Dice.DieSize)
	s.Equal([]int{6}, dice.Dice.OriginalRolls)
	s.Equal([]int{6}, dice.Dice.FinalRolls)
	s.Empty(dice.Dice.Rerolls)
	s.Empty(dice.Dice.KeptIndices)
	s.Equal(6, dice.Dice.Subtotal)
	s.Nil(dice.Modifier)

	modifier := calculation.Components[1]
	s.True(modifier.Source.Ref.Equals(refs.Classes.Fighter()))
	s.Equal("Fighter", modifier.Source.Name)
	s.Equal("Fighter level", modifier.Source.Label)
	s.Require().NotNil(modifier.Modifier)
	s.Equal(1, *modifier.Modifier)
	s.Nil(modifier.Dice)

	// The roll-bearing scalar path is retired for Second Wind: the calculation
	// is the only representation of the roll.
	s.Zero(receivedEvent.Roll)
	s.Zero(receivedEvent.Modifier)
}

// TestActivatePublishesOwnRefsNotSharedSingletons proves the published healing
// event never carries the package-singleton refs: the event graph is mutable
// and published to strangers, so a receiver mutating a published ref must not
// corrupt refs.Features.SecondWind() or refs.Classes.Fighter() for everyone else.
func (s *SecondWindTestSuite) TestActivatePublishesOwnRefsNotSharedSingletons() {
	owner := &StubEntity{id: "fighter-1"}
	sw := newSecondWindForTest("second-wind-feature", 1, "fighter-1")

	var receivedEvent *dnd5eEvents.HealingReceivedEvent
	_, err := dnd5eEvents.HealingReceivedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			receivedEvent = &event
			return nil
		},
	)
	s.Require().NoError(err)

	err = sw.Activate(s.ctx, owner, FeatureInput{Bus: s.bus, Roller: &fixedSecondWindRoller{face: 6}})
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	s.Require().NotNil(receivedEvent.SourceRef, "Second Wind publishes a source ref")
	s.Require().NotNil(receivedEvent.Calculation)
	s.Require().Len(receivedEvent.Calculation.Components, 2)

	// The published refs are fresh copies, not the shared singletons.
	s.Require().NotSame(refs.Features.SecondWind(), receivedEvent.SourceRef,
		"the top-level SourceRef must be a fresh copy")
	s.Require().NotSame(refs.Features.SecondWind(), receivedEvent.Calculation.Components[0].Source.Ref,
		"the dice component source ref must be a fresh copy")
	s.Require().NotSame(refs.Classes.Fighter(), receivedEvent.Calculation.Components[1].Source.Ref,
		"the modifier component source ref must be a fresh copy")

	// A hostile receiver scribbles on every published ref in the event graph.
	receivedEvent.SourceRef.ID = "corrupted"
	receivedEvent.Calculation.Components[0].Source.Ref.ID = "corrupted"
	receivedEvent.Calculation.Components[1].Source.Ref.ID = "corrupted"

	s.Equal("second_wind", refs.Features.SecondWind().ID,
		"mutating the published top-level SourceRef must not corrupt the shared ref")
	s.Equal("fighter", refs.Classes.Fighter().ID,
		"mutating the published modifier source ref must not corrupt the shared ref")
	s.Equal(core.Ref{Module: refs.Module, Type: refs.TypeFeatures, ID: "second_wind"},
		*refs.Features.SecondWind(), "the singleton keeps its full identity")
	s.Equal(core.Ref{Module: refs.Module, Type: refs.TypeClasses, ID: "fighter"},
		*refs.Classes.Fighter(), "the singleton keeps its full identity")
}

func (s *SecondWindTestSuite) TestHealingScalesWithLevel() {
	testCases := []struct {
		level            int
		expectedModifier int
	}{
		{1, 1},
		{3, 3},
		{5, 5},
		{10, 10},
		{20, 20},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Level %d", tc.level), func() {
			sw := newSecondWindForTest("test-sw", tc.level, "fighter-1")
			owner := &StubEntity{id: "fighter-1"}

			var receivedEvent *dnd5eEvents.HealingReceivedEvent
			topic := dnd5eEvents.HealingReceivedTopic.On(s.bus)
			_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
				receivedEvent = &event
				return nil
			})
			s.NoError(err)

			err = sw.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
			s.NoError(err)

			s.Require().NotNil(receivedEvent)
			s.Require().NotNil(receivedEvent.Calculation)
			s.Require().Len(receivedEvent.Calculation.Components, 2)
			s.Require().NotNil(receivedEvent.Calculation.Components[1].Modifier)
			s.Equal(tc.expectedModifier, *receivedEvent.Calculation.Components[1].Modifier,
				"Level %d should have modifier %d", tc.level, tc.expectedModifier)
		})
	}
}

func (s *SecondWindTestSuite) TestLoadJSON() {
	jsonData := []byte(`{
		"ref": {"value": "second_wind"},
		"id": "loaded-second-wind",
		"name": "Second Wind",
		"level": 5,
		"character_id": "fighter-99",
		"uses": 0,
		"max_uses": 1
	}`)

	sw := &SecondWind{}
	err := sw.loadJSON(jsonData)
	s.NoError(err)

	s.Equal("loaded-second-wind", sw.id)
	s.Equal("Second Wind", sw.name)
	s.Equal(5, sw.level)
	s.Equal("fighter-99", sw.characterID)
	s.Equal(0, sw.resource.Current())
	s.Equal(1, sw.resource.Maximum())
}

func (s *SecondWindTestSuite) TestToJSON() {
	jsonData, err := s.secondWind.ToJSON()
	s.NoError(err)

	// Load it back
	loaded := &SecondWind{}
	err = loaded.loadJSON(jsonData)
	s.NoError(err)

	s.Equal(s.secondWind.id, loaded.id)
	s.Equal(s.secondWind.name, loaded.name)
	s.Equal(s.secondWind.level, loaded.level)
}

func (s *SecondWindTestSuite) TestAutomaticShortRestRecovery() {
	owner := &StubEntity{id: "fighter-1"}

	// Apply the resource to the event bus for automatic recovery
	err := s.secondWind.Apply(s.ctx, s.bus)
	s.NoError(err)

	// Use second wind
	err = s.secondWind.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.NoError(err)

	// Should have 0 uses left
	s.Equal(0, s.secondWind.resource.Current())

	// Publish a short rest event for the same character
	rests := dnd5eEvents.RestTopic.On(s.bus)
	err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
		CharacterID: "fighter-1",
		RestType:    coreResources.ResetShortRest,
	})
	s.NoError(err)

	// Should automatically have 1 use again
	s.Equal(1, s.secondWind.resource.Current())

	// Should be able to activate again
	err = s.secondWind.CanActivate(s.ctx, owner, FeatureInput{})
	s.NoError(err)
}

func (s *SecondWindTestSuite) TestAutomaticLongRestRecovery() {
	owner := &StubEntity{id: "fighter-1"}

	// Apply the resource to the event bus for automatic recovery
	err := s.secondWind.Apply(s.ctx, s.bus)
	s.NoError(err)

	// Use second wind
	err = s.secondWind.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.NoError(err)

	// Should have 0 uses left
	s.Equal(0, s.secondWind.resource.Current())

	// Publish a long rest event for the same character
	// Long rest should also restore short rest resources
	rests := dnd5eEvents.RestTopic.On(s.bus)
	err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
		CharacterID: "fighter-1",
		RestType:    coreResources.ResetLongRest,
	})
	s.NoError(err)

	// Should automatically have 1 use again
	s.Equal(1, s.secondWind.resource.Current())
}

func (s *SecondWindTestSuite) TestNoRecoveryForDifferentCharacter() {
	owner := &StubEntity{id: "fighter-1"}

	// Apply the resource to the event bus for automatic recovery
	err := s.secondWind.Apply(s.ctx, s.bus)
	s.NoError(err)

	// Use second wind
	err = s.secondWind.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.NoError(err)

	// Should have 0 uses left
	s.Equal(0, s.secondWind.resource.Current())

	// Publish a short rest event for a DIFFERENT character
	rests := dnd5eEvents.RestTopic.On(s.bus)
	err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
		CharacterID: "fighter-2", // Different character!
		RestType:    coreResources.ResetShortRest,
	})
	s.NoError(err)

	// Should still have 0 uses (no recovery for other character)
	s.Equal(0, s.secondWind.resource.Current())
}

func (s *SecondWindTestSuite) TestApplyRemove() {
	// Test that Apply/Remove work correctly
	err := s.secondWind.Apply(s.ctx, s.bus)
	s.NoError(err)
	s.True(s.secondWind.resource.IsApplied())

	err = s.secondWind.Remove(s.ctx, s.bus)
	s.NoError(err)
	s.False(s.secondWind.resource.IsApplied())

	// After removal, rest events should not restore
	owner := &StubEntity{id: "fighter-1"}
	err = s.secondWind.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.NoError(err)
	s.Equal(0, s.secondWind.resource.Current())

	// Publish rest event
	rests := dnd5eEvents.RestTopic.On(s.bus)
	err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
		CharacterID: "fighter-1",
		RestType:    coreResources.ResetShortRest,
	})
	s.NoError(err)

	// Should still be 0 (not restored because removed)
	s.Equal(0, s.secondWind.resource.Current())
}

func TestSecondWindTestSuite(t *testing.T) {
	suite.Run(t, new(SecondWindTestSuite))
}
