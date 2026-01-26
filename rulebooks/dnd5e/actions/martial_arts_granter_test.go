// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

type MartialArtsGranterTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func (s *MartialArtsGranterTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func TestMartialArtsGranterSuite(t *testing.T) {
	suite.Run(t, new(MartialArtsGranterTestSuite))
}

func (s *MartialArtsGranterTestSuite) TestGrantsForUnarmedStrike() {
	// Unarmed strike from Attack action should grant bonus strike
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		IsUnarmed:     true,
		SourceAbility: "attack",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.True(result.Granted)
	s.NotNil(result.Action)
	s.Equal("unarmed strike or monk weapon attack", result.Reason)
}

func (s *MartialArtsGranterTestSuite) TestGrantsForShortsword() {
	// Shortsword is explicitly a monk weapon
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		WeaponID:      weapons.Shortsword,
		IsUnarmed:     false,
		SourceAbility: "attack",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.True(result.Granted)
	s.NotNil(result.Action)
}

func (s *MartialArtsGranterTestSuite) TestGrantsForQuarterstaff() {
	// Quarterstaff is a simple melee weapon without Heavy or Two-Handed
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		WeaponID:      weapons.Quarterstaff,
		IsUnarmed:     false,
		SourceAbility: "attack",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.True(result.Granted)
	s.NotNil(result.Action)
}

func (s *MartialArtsGranterTestSuite) TestDeniesForLongsword() {
	// Longsword is a martial weapon, not a monk weapon
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		WeaponID:      weapons.Longsword,
		IsUnarmed:     false,
		SourceAbility: "attack",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.False(result.Granted)
	s.Nil(result.Action)
	s.Equal("weapon is not a monk weapon", result.Reason)
}

func (s *MartialArtsGranterTestSuite) TestDeniesForGreataxe() {
	// Greataxe is a martial weapon with Heavy property
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		WeaponID:      weapons.Greataxe,
		IsUnarmed:     false,
		SourceAbility: "attack",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.False(result.Granted)
	s.Nil(result.Action)
	s.Equal("weapon is not a monk weapon", result.Reason)
}

func (s *MartialArtsGranterTestSuite) TestDeniesForFlurrySource() {
	// Flurry of Blows attacks don't grant additional bonus strikes
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		IsUnarmed:     true,
		SourceAbility: "flurry",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.False(result.Granted)
	s.Nil(result.Action)
	s.Contains(result.Reason, "expected Attack action")
}

func (s *MartialArtsGranterTestSuite) TestDeniesForOffHandSource() {
	// Off-hand attacks don't grant martial arts bonus strikes
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		IsUnarmed:     true,
		SourceAbility: "off_hand",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.False(result.Granted)
	s.Nil(result.Action)
	s.Contains(result.Reason, "expected Attack action")
}

func (s *MartialArtsGranterTestSuite) TestDeniesIfAlreadyGranted() {
	// Only one martial arts bonus strike per turn
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:            "monk-1",
		IsUnarmed:              true,
		SourceAbility:          "attack",
		AlreadyGrantedThisTurn: true,
		EventBus:               s.bus,
	})

	s.Require().NoError(err)
	s.False(result.Granted)
	s.Nil(result.Action)
	s.Equal("martial arts bonus strike already granted this turn", result.Reason)
}

func (s *MartialArtsGranterTestSuite) TestGrantsWithEmptySourceAbility() {
	// Empty source ability defaults to allowing (backwards compatibility)
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		IsUnarmed:     true,
		SourceAbility: "",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.True(result.Granted)
	s.NotNil(result.Action)
}

func (s *MartialArtsGranterTestSuite) TestGrantsForDagger() {
	// Dagger is a simple melee weapon with Light and Finesse, no Heavy/Two-Handed
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		WeaponID:      weapons.Dagger,
		IsUnarmed:     false,
		SourceAbility: "attack",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.True(result.Granted)
	s.NotNil(result.Action)
}

func (s *MartialArtsGranterTestSuite) TestActionHasCorrectProperties() {
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		IsUnarmed:     true,
		SourceAbility: "attack",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.Require().True(result.Granted)
	s.Require().NotNil(result.Action)

	// Verify action properties
	s.Equal("monk-1-martial-arts-bonus-strike", result.Action.GetID())
	s.True(result.Action.IsTemporary())
	s.Equal(1, result.Action.UsesRemaining())
}

func (s *MartialArtsGranterTestSuite) TestWorksWithoutEventBus() {
	// Should work (for testing) without an event bus
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		IsUnarmed:     true,
		SourceAbility: "attack",
		EventBus:      nil, // No event bus
	})

	s.Require().NoError(err)
	s.True(result.Granted)
	s.NotNil(result.Action)
}

func (s *MartialArtsGranterTestSuite) TestPublishesActionGrantedEvent() {
	// Subscribe to ActionGrantedTopic
	var receivedEvent *dnd5eEvents.ActionGrantedEvent
	topic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.ActionGrantedEvent) error {
		receivedEvent = &event
		return nil
	})
	s.Require().NoError(err)

	// Grant the action
	result, err := actions.CheckAndGrantMartialArtsBonusStrike(s.ctx, &actions.MartialArtsGranterInput{
		CharacterID:   "monk-1",
		IsUnarmed:     true,
		SourceAbility: "attack",
		EventBus:      s.bus,
	})

	s.Require().NoError(err)
	s.True(result.Granted)
	s.Require().NotNil(receivedEvent, "Should publish ActionGrantedEvent")
	s.Equal("monk-1", receivedEvent.CharacterID)
	s.Equal("martial_arts", receivedEvent.Source)
	s.NotNil(receivedEvent.Action)
	s.Equal(result.Action.GetID(), receivedEvent.Action.GetID())
}
