// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	mock_dice "github.com/KirkDiggler/rpg-toolkit/dice/mock"
	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// UnconsciousConditionTestSuite tests the UnconsciousCondition behavior
type UnconsciousConditionTestSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	ctx        context.Context
	bus        events.EventBus
	mockRoller *mock_dice.MockRoller
}

func TestUnconsciousConditionTestSuite(t *testing.T) {
	suite.Run(t, new(UnconsciousConditionTestSuite))
}

func (s *UnconsciousConditionTestSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.mockRoller = mock_dice.NewMockRoller(s.ctrl)
}

func (s *UnconsciousConditionTestSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *UnconsciousConditionTestSuite) newCondition(characterID string) *UnconsciousCondition {
	return &UnconsciousCondition{
		CharacterID:    characterID,
		Roller:         s.mockRoller,
		deathSaveState: &saves.DeathSaveState{},
	}
}

func (s *UnconsciousConditionTestSuite) TestApply_SubscribesToEvents() {
	uc := s.newCondition("char-1")

	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Should have 3 subscriptions: TurnStart, DamageReceived, HealingReceived
	s.Len(uc.subscriptionIDs, 3)
	s.True(uc.IsApplied())
}

func (s *UnconsciousConditionTestSuite) TestApply_AlreadyApplied_ReturnsError() {
	uc := s.newCondition("char-1")

	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	err = uc.Apply(s.ctx, s.bus)
	s.Require().Error(err)
	s.Contains(err.Error(), "already applied")
}

func (s *UnconsciousConditionTestSuite) TestRemove_Unsubscribes() {
	uc := s.newCondition("char-1")

	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(uc.IsApplied())

	err = uc.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(uc.IsApplied())
	s.Nil(uc.subscriptionIDs)
}

func (s *UnconsciousConditionTestSuite) TestOnTurnStart_RollsDeathSave() {
	uc := s.newCondition("char-1")
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track death save rolled event
	var rolledEvent *dnd5eEvents.DeathSaveRolledEvent
	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(s.bus)
	_, err = rolledTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeathSaveRolledEvent) error {
		rolledEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Mock: roll a 15 (success)
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)

	// Publish turn start
	turnStartTopic := dnd5eEvents.TurnStartTopic.On(s.bus)
	err = turnStartTopic.Publish(s.ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: "char-1",
		Round:       1,
	})
	s.Require().NoError(err)

	s.Require().NotNil(rolledEvent)
	s.Equal("char-1", rolledEvent.CharacterID)
	s.Equal(15, rolledEvent.Roll)
	s.True(rolledEvent.IsSuccess)
	s.Equal(1, rolledEvent.Successes)
	s.Equal(0, rolledEvent.Failures)
}

func (s *UnconsciousConditionTestSuite) TestOnTurnStart_IgnoresOtherCharacters() {
	uc := s.newCondition("char-1")
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Track death save rolled event
	var rolledEvent *dnd5eEvents.DeathSaveRolledEvent
	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(s.bus)
	_, err = rolledTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeathSaveRolledEvent) error {
		rolledEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish turn start for a different character - no roller mock expected
	turnStartTopic := dnd5eEvents.TurnStartTopic.On(s.bus)
	err = turnStartTopic.Publish(s.ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: "char-2",
		Round:       1,
	})
	s.Require().NoError(err)

	s.Nil(rolledEvent, "should not roll death save for other characters")
}

func (s *UnconsciousConditionTestSuite) TestOnTurnStart_CriticalFail_TwoFailures() {
	uc := s.newCondition("char-1")
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var rolledEvent *dnd5eEvents.DeathSaveRolledEvent
	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(s.bus)
	_, err = rolledTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeathSaveRolledEvent) error {
		rolledEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Mock: roll a 1 (critical fail)
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(1, nil)

	turnStartTopic := dnd5eEvents.TurnStartTopic.On(s.bus)
	err = turnStartTopic.Publish(s.ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: "char-1",
		Round:       1,
	})
	s.Require().NoError(err)

	s.Require().NotNil(rolledEvent)
	s.True(rolledEvent.IsCriticalFail)
	s.Equal(2, rolledEvent.Failures)
	s.Equal(0, rolledEvent.Successes)
}

func (s *UnconsciousConditionTestSuite) TestOnTurnStart_CriticalSuccess_RegainsConsciousness() {
	uc := s.newCondition("char-1")
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var rolledEvent *dnd5eEvents.DeathSaveRolledEvent
	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(s.bus)
	_, err = rolledTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeathSaveRolledEvent) error {
		rolledEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Track condition removal
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removedTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Track healing event
	var healingEvent *dnd5eEvents.HealingReceivedEvent
	healingTopic := dnd5eEvents.HealingReceivedTopic.On(s.bus)
	_, err = healingTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
		healingEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Mock: roll a 20 (critical success)
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(20, nil)

	turnStartTopic := dnd5eEvents.TurnStartTopic.On(s.bus)
	err = turnStartTopic.Publish(s.ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: "char-1",
		Round:       1,
	})
	s.Require().NoError(err)

	s.Require().NotNil(rolledEvent)
	s.True(rolledEvent.IsSuccess)
	s.True(rolledEvent.IsCriticalSuccess)
	s.True(rolledEvent.RegainedConsciousness)
	s.Equal(1, rolledEvent.HPRestored)

	// Condition should have published removal
	s.Require().NotNil(removedEvent)
	s.Equal("char-1", removedEvent.CharacterID)
	s.Equal("dnd5e:conditions:unconscious", removedEvent.ConditionRef)

	// Healing event should have been published
	s.Require().NotNil(healingEvent)
	s.Equal("char-1", healingEvent.TargetID)
	s.Equal(1, healingEvent.Amount)

	// Condition should be removed
	s.False(uc.IsApplied())
}

func (s *UnconsciousConditionTestSuite) TestOnTurnStart_ThreeFailures_Dies() {
	uc := s.newCondition("char-1")
	// Start with 2 failures already
	uc.deathSaveState.Failures = 2

	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var diedEvent *dnd5eEvents.CharacterDiedEvent
	diedTopic := dnd5eEvents.CharacterDiedTopic.On(s.bus)
	_, err = diedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.CharacterDiedEvent) error {
		diedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Mock: roll a 5 (failure, bringing total to 3)
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(5, nil)

	turnStartTopic := dnd5eEvents.TurnStartTopic.On(s.bus)
	err = turnStartTopic.Publish(s.ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: "char-1",
		Round:       1,
	})
	s.Require().NoError(err)

	s.Require().NotNil(diedEvent)
	s.Equal("char-1", diedEvent.CharacterID)
}

func (s *UnconsciousConditionTestSuite) TestOnTurnStart_ThreeSuccesses_Stabilizes() {
	uc := s.newCondition("char-1")
	// Start with 2 successes already
	uc.deathSaveState.Successes = 2

	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var stabilizedEvent *dnd5eEvents.CharacterStabilizedEvent
	stabilizedTopic := dnd5eEvents.CharacterStabilizedTopic.On(s.bus)
	_, err = stabilizedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.CharacterStabilizedEvent) error {
		stabilizedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Mock: roll a 15 (success, bringing total to 3)
	s.mockRoller.EXPECT().Roll(s.ctx, 20).Return(15, nil)

	turnStartTopic := dnd5eEvents.TurnStartTopic.On(s.bus)
	err = turnStartTopic.Publish(s.ctx, dnd5eEvents.TurnStartEvent{
		CharacterID: "char-1",
		Round:       1,
	})
	s.Require().NoError(err)

	s.Require().NotNil(stabilizedEvent)
	s.Equal("char-1", stabilizedEvent.CharacterID)
}

func (s *UnconsciousConditionTestSuite) TestOnDamageReceived_AddsFailure() {
	uc := s.newCondition("char-1")
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var rolledEvent *dnd5eEvents.DeathSaveRolledEvent
	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(s.bus)
	_, err = rolledTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeathSaveRolledEvent) error {
		rolledEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish damage event
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   "char-1",
		SourceID:   "goblin-1",
		Amount:     5,
		IsCritical: false,
	})
	s.Require().NoError(err)

	s.Require().NotNil(rolledEvent)
	s.Equal("char-1", rolledEvent.CharacterID)
	s.Equal(0, rolledEvent.Roll, "damage events should have roll=0")
	s.Equal(1, rolledEvent.Failures)
}

func (s *UnconsciousConditionTestSuite) TestOnDamageReceived_CriticalHit_AddsTwoFailures() {
	uc := s.newCondition("char-1")
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var rolledEvent *dnd5eEvents.DeathSaveRolledEvent
	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(s.bus)
	_, err = rolledTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeathSaveRolledEvent) error {
		rolledEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish critical damage event
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   "char-1",
		SourceID:   "goblin-1",
		Amount:     10,
		IsCritical: true,
	})
	s.Require().NoError(err)

	s.Require().NotNil(rolledEvent)
	s.Equal(2, rolledEvent.Failures)
}

func (s *UnconsciousConditionTestSuite) TestOnDamageReceived_IgnoresOtherCharacters() {
	uc := s.newCondition("char-1")
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var rolledEvent *dnd5eEvents.DeathSaveRolledEvent
	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(s.bus)
	_, err = rolledTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeathSaveRolledEvent) error {
		rolledEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish damage for different character
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   "char-2",
		SourceID:   "goblin-1",
		Amount:     5,
		IsCritical: false,
	})
	s.Require().NoError(err)

	s.Nil(rolledEvent, "should not process damage for other characters")
}

func (s *UnconsciousConditionTestSuite) TestOnHealingReceived_RemovesCondition() {
	uc := s.newCondition("char-1")
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(uc.IsApplied())

	// Track condition removal
	var removedEvent *dnd5eEvents.ConditionRemovedEvent
	removedTopic := dnd5eEvents.ConditionRemovedTopic.On(s.bus)
	_, err = removedTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
		removedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish healing event
	healingTopic := dnd5eEvents.HealingReceivedTopic.On(s.bus)
	err = healingTopic.Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: "char-1",
		Amount:   5,
		Source:   "cure_wounds",
	})
	s.Require().NoError(err)

	// Condition should be removed
	s.False(uc.IsApplied())
	s.Require().NotNil(removedEvent)
	s.Equal("char-1", removedEvent.CharacterID)
	s.Equal("dnd5e:conditions:unconscious", removedEvent.ConditionRef)
	s.Equal("healed", removedEvent.Reason)
}

func (s *UnconsciousConditionTestSuite) TestOnHealingReceived_IgnoresOtherCharacters() {
	uc := s.newCondition("char-1")
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	// Publish healing for different character
	healingTopic := dnd5eEvents.HealingReceivedTopic.On(s.bus)
	err = healingTopic.Publish(s.ctx, dnd5eEvents.HealingReceivedEvent{
		TargetID: "char-2",
		Amount:   5,
		Source:   "cure_wounds",
	})
	s.Require().NoError(err)

	// Condition should still be applied
	s.True(uc.IsApplied())
}

func (s *UnconsciousConditionTestSuite) TestToJSON_RoundTrip() {
	uc := &UnconsciousCondition{
		CharacterID:    "char-1",
		deathSaveState: &saves.DeathSaveState{Successes: 1, Failures: 2},
	}

	data, err := uc.ToJSON()
	s.Require().NoError(err)

	// Load from JSON
	uc2 := &UnconsciousCondition{}
	err = uc2.loadJSON(data)
	s.Require().NoError(err)

	s.Equal("char-1", uc2.CharacterID)
	s.Equal(1, uc2.deathSaveState.Successes)
	s.Equal(2, uc2.deathSaveState.Failures)
}

func (s *UnconsciousConditionTestSuite) TestOnDamageReceived_StabilizedCreature_LosesStabilization() {
	uc := s.newCondition("char-1")
	// Start stabilized (3 successes)
	uc.deathSaveState.Successes = 3
	uc.deathSaveState.Stabilized = true

	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	var rolledEvent *dnd5eEvents.DeathSaveRolledEvent
	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(s.bus)
	_, err = rolledTopic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.DeathSaveRolledEvent) error {
		rolledEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Publish damage event to stabilized creature
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(s.bus)
	err = damageTopic.Publish(s.ctx, dnd5eEvents.DamageReceivedEvent{
		TargetID:   "char-1",
		SourceID:   "goblin-1",
		Amount:     5,
		IsCritical: false,
	})
	s.Require().NoError(err)

	// Should have processed the damage
	s.Require().NotNil(rolledEvent)
	s.Equal("char-1", rolledEvent.CharacterID)
	s.Equal(1, rolledEvent.Failures, "should gain a death save failure")

	// Stabilization should be lost
	s.False(uc.deathSaveState.Stabilized, "stabilization should be reset")
}

func (s *UnconsciousConditionTestSuite) TestIsApplied_ReflectsBusState() {
	uc := s.newCondition("char-1")

	// Not applied yet
	s.False(uc.IsApplied())

	// Apply
	err := uc.Apply(s.ctx, s.bus)
	s.Require().NoError(err)
	s.True(uc.IsApplied())

	// Remove
	err = uc.Remove(s.ctx, s.bus)
	s.Require().NoError(err)
	s.False(uc.IsApplied())
}
