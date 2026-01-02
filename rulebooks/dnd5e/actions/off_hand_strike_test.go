package actions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

// offHandTarget implements core.Entity for testing
type offHandTarget struct {
	id string
}

func (m *offHandTarget) GetID() string {
	return m.id
}

func (m *offHandTarget) GetType() core.EntityType {
	return "target"
}

// offHandOwner implements core.Entity for testing
type offHandOwner struct {
	id string
}

func (m *offHandOwner) GetID() string {
	return m.id
}

func (m *offHandOwner) GetType() core.EntityType {
	return "character"
}

type OffHandStrikeTestSuite struct {
	suite.Suite
	ctx    context.Context
	bus    events.EventBus
	owner  *offHandOwner
	target *offHandTarget
	strike *actions.OffHandStrike
}

func TestOffHandStrikeTestSuite(t *testing.T) {
	suite.Run(t, new(OffHandStrikeTestSuite))
}

func (s *OffHandStrikeTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &offHandOwner{id: "test-fighter"}
	s.target = &offHandTarget{id: "goblin-1"}
	s.strike = actions.NewOffHandStrike(actions.OffHandStrikeConfig{
		ID:       "test-off-hand-strike-1",
		OwnerID:  s.owner.id,
		WeaponID: weapons.Shortsword,
	})
}

func (s *OffHandStrikeTestSuite) TestNewOffHandStrike() {
	// Assert
	s.Assert().Equal("test-off-hand-strike-1", s.strike.GetID())
	s.Assert().Equal(core.EntityType("action"), s.strike.GetType())
	s.Assert().Equal(1, s.strike.UsesRemaining())
	s.Assert().True(s.strike.IsTemporary())
	s.Assert().Equal(weapons.Shortsword, s.strike.GetWeaponID())
}

func (s *OffHandStrikeTestSuite) TestActionType_IsBonusAction() {
	// Act
	actionType := s.strike.ActionType()

	// Assert
	s.Assert().Equal(coreCombat.ActionBonus, actionType)
}

func (s *OffHandStrikeTestSuite) TestCanActivate_Success() {
	// Arrange - apply to bus first
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Act
	err = s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Target: s.target,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *OffHandStrikeTestSuite) TestCanActivate_NoTarget() {
	// Act
	err := s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Target: nil,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
}

func (s *OffHandStrikeTestSuite) TestCanActivate_AlreadyRemoved() {
	// Arrange - apply and remove
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	err = s.strike.Remove(s.ctx, s.bus)
	s.Require().NoError(err)

	// Act
	err = s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Target: s.target,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
}

func (s *OffHandStrikeTestSuite) TestCanActivate_NoUsesRemaining() {
	// Arrange - apply and use it
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:    s.bus,
		Target: s.target,
	})
	s.Require().NoError(err)

	// Create a new strike to test exhaustion without removal
	// (the original gets removed after use, so make a fresh one)
	strike2 := actions.NewOffHandStrike(actions.OffHandStrikeConfig{
		ID:       "test-off-hand-strike-2",
		OwnerID:  s.owner.id,
		WeaponID: weapons.Shortsword,
	})
	// Don't apply to bus so it won't auto-remove

	// Manually exhaust it by activating without bus (won't trigger removal)
	err = strike2.Activate(s.ctx, s.owner, actions.ActionInput{
		Target: s.target,
		// No bus - won't auto-remove
	})
	s.Require().NoError(err)
	s.Assert().Equal(0, strike2.UsesRemaining())

	// Act
	err = strike2.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Target: s.target,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *OffHandStrikeTestSuite) TestActivate_PublishesRequestEvent() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var receivedEvent *dnd5eEvents.OffHandStrikeRequestedEvent
	topic := dnd5eEvents.OffHandStrikeRequestedTopic.On(s.bus)
	_, err = topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.OffHandStrikeRequestedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act
	err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:    s.bus,
		Target: s.target,
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(receivedEvent)
	s.Assert().Equal(s.owner.id, receivedEvent.AttackerID)
	s.Assert().Equal(s.target.id, receivedEvent.TargetID)
	s.Assert().Equal(string(weapons.Shortsword), receivedEvent.WeaponID)
	s.Assert().Equal("test-off-hand-strike-1", receivedEvent.ActionID)
}

func (s *OffHandStrikeTestSuite) TestActivate_ConsumesUse() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.Require().Equal(1, s.strike.UsesRemaining())

	// Act
	err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:    s.bus,
		Target: s.target,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.strike.UsesRemaining())
}

func (s *OffHandStrikeTestSuite) TestActivate_PublishesActivatedNotification() {
	// Arrange
	err := s.strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var activatedEvent *dnd5eEvents.OffHandStrikeActivatedEvent
	activatedTopic := dnd5eEvents.OffHandStrikeActivatedTopic.On(s.bus)
	_, err = activatedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.OffHandStrikeActivatedEvent) error {
		activatedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Act
	err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:    s.bus,
		Target: s.target,
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(activatedEvent, "Should publish OffHandStrikeActivatedEvent")
	s.Assert().Equal(s.owner.id, activatedEvent.AttackerID)
	s.Assert().Equal(s.target.id, activatedEvent.TargetID)
	s.Assert().Equal(string(weapons.Shortsword), activatedEvent.WeaponID)
	s.Assert().Equal("test-off-hand-strike-1", activatedEvent.ActionID)
	s.Assert().Equal(0, activatedEvent.UsesRemaining, "Should have 0 uses remaining after activation")
}

func (s *OffHandStrikeTestSuite) TestActivate_RemovesSelfWhenNoUsesRemaining() {
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
	err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:    s.bus,
		Target: s.target,
	})

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(removedEvent)
	s.Assert().Equal("test-off-hand-strike-1", removedEvent.ActionID)
	s.Assert().Equal(s.owner.id, removedEvent.OwnerID)
}

func (s *OffHandStrikeTestSuite) TestApply_SubscribesToTurnEnd() {
	// Arrange - not applied yet

	// Act
	err := s.strike.Apply(s.ctx, s.bus)

	// Assert
	s.Require().NoError(err)
}

func (s *OffHandStrikeTestSuite) TestApply_FailsIfAlreadyApplied() {
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

func (s *OffHandStrikeTestSuite) TestRemove_PublishesActionRemovedEvent() {
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
	s.Assert().Equal("test-off-hand-strike-1", removedEvent.ActionID)
	s.Assert().Equal(s.owner.id, removedEvent.OwnerID)
}

func (s *OffHandStrikeTestSuite) TestRemove_IdempotentIfAlreadyRemoved() {
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

func (s *OffHandStrikeTestSuite) TestTurnEnd_RemovesUnusedStrike() {
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
	s.Assert().Equal("test-off-hand-strike-1", removedEvent.ActionID)
}

func (s *OffHandStrikeTestSuite) TestTurnEnd_IgnoresOtherCharacterTurnEnd() {
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
	s.Assert().Equal(1, s.strike.UsesRemaining())
}

func (s *OffHandStrikeTestSuite) TestToJSON() {
	// Act
	jsonData, err := s.strike.ToJSON()

	// Assert
	s.Require().NoError(err)
	s.Assert().NotEmpty(jsonData)
	s.Assert().Contains(string(jsonData), "test-off-hand-strike-1")
	s.Assert().Contains(string(jsonData), "off_hand_strike")
	s.Assert().Contains(string(jsonData), string(weapons.Shortsword))
}
