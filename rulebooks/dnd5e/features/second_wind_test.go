package features

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
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

	// Activate second wind
	err = s.secondWind.Activate(s.ctx, owner, FeatureInput{Bus: s.bus})
	s.NoError(err)

	// Check that event was published
	s.NotNil(receivedEvent)
	s.Equal("fighter-1", receivedEvent.TargetID)
	s.Equal("second_wind", receivedEvent.Source)

	// Healing should be 1d10 + level (3)
	// Roll should be between 1 and 10
	s.GreaterOrEqual(receivedEvent.Roll, 1, "Roll should be at least 1")
	s.LessOrEqual(receivedEvent.Roll, 10, "Roll should be at most 10")

	// Modifier should be fighter level
	s.Equal(3, receivedEvent.Modifier, "Modifier should be fighter level")

	// Total should be roll + modifier
	s.Equal(receivedEvent.Roll+receivedEvent.Modifier, receivedEvent.Amount)
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

			s.NotNil(receivedEvent)
			s.Equal(tc.expectedModifier, receivedEvent.Modifier,
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
