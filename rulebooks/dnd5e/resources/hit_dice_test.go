package resources

import (
	"context"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

type HitDiceResourceTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestHitDiceResourceSuite(t *testing.T) {
	suite.Run(t, new(HitDiceResourceTestSuite))
}

func (s *HitDiceResourceTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func (s *HitDiceResourceTestSuite) SetupSubTest() {
	s.bus = events.NewEventBus()
}

func (s *HitDiceResourceTestSuite) TestNewHitDiceResource() {
	s.Run("creates resource with correct configuration", func() {
		resource := NewHitDiceResource(HitDiceResourceConfig{
			CharacterID: "char-1",
			Level:       5,
		})

		s.Equal(5, resource.Maximum())
		s.Equal(5, resource.Current())
		s.Equal(string(HitDice), resource.ID())
		s.Equal(coreResources.ResetLongRest, resource.ResetType)
	})

	s.Run("recovers half on long rest (rounded down, min 1)", func() {
		// Level 8 character - should recover 4 on long rest
		resource := NewHitDiceResource(HitDiceResourceConfig{
			CharacterID: "char-1",
			Level:       8,
		})

		err := resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use all hit dice
		err = resource.Use(8)
		s.Require().NoError(err)
		s.Equal(0, resource.Current())

		// Publish long rest event
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetLongRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// Should recover half (4)
		s.Equal(4, resource.Current(), "should recover half of maximum")
	})

	s.Run("recovers minimum of 1 for level 1 character", func() {
		// Level 1 character - half of 1 = 0, but min 1
		resource := NewHitDiceResource(HitDiceResourceConfig{
			CharacterID: "char-1",
			Level:       1,
		})

		err := resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use the one hit die
		err = resource.Use(1)
		s.Require().NoError(err)
		s.Equal(0, resource.Current())

		// Publish long rest event
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetLongRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// Should recover 1 (minimum)
		s.Equal(1, resource.Current(), "should recover minimum of 1")
	})

	s.Run("recovers odd levels correctly", func() {
		// Level 5 character - half of 5 = 2 (rounded down)
		resource := NewHitDiceResource(HitDiceResourceConfig{
			CharacterID: "char-1",
			Level:       5,
		})

		err := resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use all hit dice
		err = resource.Use(5)
		s.Require().NoError(err)
		s.Equal(0, resource.Current())

		// Publish long rest event
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetLongRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// Should recover 2 (half of 5 rounded down)
		s.Equal(2, resource.Current(), "should recover 2 for level 5")
	})

	s.Run("does not recover on short rest", func() {
		resource := NewHitDiceResource(HitDiceResourceConfig{
			CharacterID: "char-1",
			Level:       4,
		})

		err := resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use all hit dice
		err = resource.Use(4)
		s.Require().NoError(err)
		s.Equal(0, resource.Current())

		// Publish short rest event
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetShortRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// Should NOT recover - hit dice only recover on long rest
		s.Equal(0, resource.Current(), "should not recover on short rest")
	})

	s.Run("does not exceed maximum when partially spent", func() {
		// Level 6 character - recovers 3 per long rest
		resource := NewHitDiceResource(HitDiceResourceConfig{
			CharacterID: "char-1",
			Level:       6,
		})

		err := resource.Apply(s.ctx, s.bus)
		s.Require().NoError(err)

		// Use only 2 hit dice (leaving 4)
		err = resource.Use(2)
		s.Require().NoError(err)
		s.Equal(4, resource.Current())

		// Publish long rest event
		rests := dnd5eEvents.RestTopic.On(s.bus)
		err = rests.Publish(s.ctx, dnd5eEvents.RestEvent{
			RestType:    coreResources.ResetLongRest,
			CharacterID: "char-1",
		})
		s.Require().NoError(err)

		// 4 + 3 = 7, but max is 6, so should cap at 6
		s.Equal(6, resource.Current(), "should not exceed maximum")
	})
}
