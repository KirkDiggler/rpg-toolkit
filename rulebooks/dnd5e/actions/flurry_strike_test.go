package actions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/stretchr/testify/suite"
)

// mockTarget implements core.Entity for testing
type mockTarget struct {
	id string
}

func (m *mockTarget) GetID() string {
	return m.id
}

func (m *mockTarget) GetType() core.EntityType {
	return "target"
}

// mockOwner implements core.Entity for testing
type mockOwner struct {
	id string
}

func (m *mockOwner) GetID() string {
	return m.id
}

func (m *mockOwner) GetType() core.EntityType {
	return "character"
}

type FlurryStrikeTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	target        *mockTarget
	strike        *actions.FlurryStrike
	actionEconomy *combat.ActionEconomy
}

func TestFlurryStrikeTestSuite(t *testing.T) {
	suite.Run(t, new(FlurryStrikeTestSuite))
}

func (s *FlurryStrikeTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-monk"}
	s.target = &mockTarget{id: "goblin-1"}
	s.strike = actions.NewFlurryStrike(actions.FlurryStrikeConfig{
		ID:      "test-strike-1",
		OwnerID: s.owner.id,
	})
	// ActionEconomy tracks flurry strike capacity
	s.actionEconomy = combat.NewActionEconomy()
	s.actionEconomy.SetFlurryStrikes(2) // Flurry of Blows grants 2 strikes
}

func (s *FlurryStrikeTestSuite) TestNewFlurryStrike() {
	// Assert
	s.Assert().Equal("test-strike-1", s.strike.GetID())
	s.Assert().Equal(core.EntityType("action"), s.strike.GetType())
	s.Assert().Equal(actions.UnlimitedUses, s.strike.UsesRemaining()) // Capacity tracked via ActionEconomy
	s.Assert().True(s.strike.IsTemporary())
}

func (s *FlurryStrikeTestSuite) TestCanActivate_Success() {
	// Arrange - apply to bus first
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Act
	err = s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Target:        s.target,
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *FlurryStrikeTestSuite) TestCanActivate_NoTarget() {
	// Act
	err := s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Target:        nil,
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
}

func (s *FlurryStrikeTestSuite) TestCanActivate_NoActionEconomy() {
	// Act
	err := s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Target:        s.target,
		ActionEconomy: nil,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
}

func (s *FlurryStrikeTestSuite) TestCanActivate_NoFlurryStrikesRemaining() {
	// Arrange - exhaust flurry strikes
	s.actionEconomy.SetFlurryStrikes(0)

	// Act
	err := s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Target:        s.target,
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *FlurryStrikeTestSuite) TestCanActivate_AlreadyRemoved() {
	// Arrange - apply and remove
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	err = s.strike.Remove(s.ctx, s.bus)
	s.Require().NoError(err)

	// Act
	err = s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Target:        s.target,
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
}

func (s *FlurryStrikeTestSuite) TestActivate_PublishesEvent() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var receivedEvent *dnd5eEvents.FlurryStrikeRequestedEvent
	topic := dnd5eEvents.FlurryStrikeRequestedTopic.On(s.bus)
	_, err = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.FlurryStrikeRequestedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act
	err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:           s.bus,
		Target:        s.target,
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	s.Assert().Equal(s.owner.id, receivedEvent.AttackerID)
	s.Assert().Equal(s.target.id, receivedEvent.TargetID)
	s.Assert().Equal("test-strike-1", receivedEvent.ActionID)
}

func (s *FlurryStrikeTestSuite) TestActivate_ConsumesFromActionEconomy() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Require().Equal(2, s.actionEconomy.FlurryStrikesRemaining)

	// Act
	err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:           s.bus,
		Target:        s.target,
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(1, s.actionEconomy.FlurryStrikesRemaining)
}

func (s *FlurryStrikeTestSuite) TestActivate_PublishesActivatedNotification() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var activatedEvent *dnd5eEvents.FlurryStrikeActivatedEvent
	activatedTopic := dnd5eEvents.FlurryStrikeActivatedTopic.On(s.bus)
	_, err = activatedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.FlurryStrikeActivatedEvent) error {
		activatedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act
	err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:           s.bus,
		Target:        s.target,
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(activatedEvent, "Should publish FlurryStrikeActivatedEvent")
	s.Assert().Equal(s.owner.id, activatedEvent.AttackerID)
	s.Assert().Equal(s.target.id, activatedEvent.TargetID)
	s.Assert().Equal("test-strike-1", activatedEvent.ActionID)
	s.Assert().Equal(1, activatedEvent.UsesRemaining, "Should have 1 use remaining after activation (started with 2)")
}

func (s *FlurryStrikeTestSuite) TestActivate_RemovesSelfWhenNoUsesRemaining() {
	// Arrange - set up with only 1 flurry strike so it exhausts after one use
	s.actionEconomy.SetFlurryStrikes(1)
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var removedEvent *dnd5eEvents.ActionRemovedEvent
	removedTopic := dnd5eEvents.ActionRemovedTopic.On(s.bus)
	_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ActionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act
	err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:           s.bus,
		Target:        s.target,
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(removedEvent, "Should remove self when no flurry strikes remaining")
	s.Assert().Equal("test-strike-1", removedEvent.ActionID)
	s.Assert().Equal(s.owner.id, removedEvent.OwnerID)
	s.Assert().Equal(0, s.actionEconomy.FlurryStrikesRemaining)
}

func (s *FlurryStrikeTestSuite) TestApply_SubscribesToTurnEnd() {
	// Arrange - not applied yet

	// Act
	err := s.strike.Apply(s.ctx, s.bus)

	// Assert
	s.Require().NoError(err)
}

func (s *FlurryStrikeTestSuite) TestApply_FailsIfAlreadyApplied() {
	// Arrange - apply once
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Act - apply again
	err = s.strike.Apply(s.ctx, s.bus)

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeAlreadyExists, rpgErr.Code)
}

func (s *FlurryStrikeTestSuite) TestRemove_PublishesActionRemovedEvent() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var removedEvent *dnd5eEvents.ActionRemovedEvent
	removedTopic := dnd5eEvents.ActionRemovedTopic.On(s.bus)
	_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ActionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act
	err = s.strike.Remove(s.ctx, s.bus)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(removedEvent)
	s.Assert().Equal("test-strike-1", removedEvent.ActionID)
	s.Assert().Equal(s.owner.id, removedEvent.OwnerID)
}

func (s *FlurryStrikeTestSuite) TestRemove_IdempotentIfAlreadyRemoved() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	err = s.strike.Remove(s.ctx, s.bus)
	s.Require().NoError(err)

	// Act - remove again
	err = s.strike.Remove(s.ctx, s.bus)

	// Assert - should not error
	s.Require().NoError(err)
}

func (s *FlurryStrikeTestSuite) TestTurnEnd_RemovesUnusedStrike() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var removedEvent *dnd5eEvents.ActionRemovedEvent
	removedTopic := dnd5eEvents.ActionRemovedTopic.On(s.bus)
	_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ActionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act - publish turn end for the owner
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: s.owner.id,
	})
	s.Require().NoError(err)

	// Assert
	s.Require().NotNil(removedEvent)
	s.Assert().Equal("test-strike-1", removedEvent.ActionID)
}

func (s *FlurryStrikeTestSuite) TestTurnEnd_IgnoresOtherCharacterTurnEnd() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var removedEvent *dnd5eEvents.ActionRemovedEvent
	removedTopic := dnd5eEvents.ActionRemovedTopic.On(s.bus)
	_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ActionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act - publish turn end for a different character
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(s.bus)
	err = turnEndTopic.Publish(s.ctx, dnd5eEvents.TurnEndEvent{
		CharacterID: "other-character",
	})
	s.Require().NoError(err)

	// Assert - should not have been removed
	s.Assert().Nil(removedEvent)
	// Action economy still has flurry strikes available
	s.Assert().Equal(2, s.actionEconomy.FlurryStrikesRemaining)
}

func (s *FlurryStrikeTestSuite) TestToJSON() {
	// Act
	jsonData, err := s.strike.ToJSON()

	// Assert
	s.Require().NoError(err)
	s.Assert().NotEmpty(jsonData)
	s.Assert().Contains(string(jsonData), "test-strike-1")
	s.Assert().Contains(string(jsonData), "flurry_strike")
}
