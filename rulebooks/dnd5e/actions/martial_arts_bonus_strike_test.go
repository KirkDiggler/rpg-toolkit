// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/stretchr/testify/suite"
)

// martialArtsTarget implements core.Entity for testing
type martialArtsTarget struct {
	id string
}

func (m *martialArtsTarget) GetID() string {
	return m.id
}

func (m *martialArtsTarget) GetType() core.EntityType {
	return "target"
}

// martialArtsOwner implements core.Entity for testing
type martialArtsOwner struct {
	id string
}

func (m *martialArtsOwner) GetID() string {
	return m.id
}

func (m *martialArtsOwner) GetType() core.EntityType {
	return "character"
}

type MartialArtsBonusStrikeTestSuite struct {
	suite.Suite
	ctx    context.Context
	bus    events.EventBus
	owner  *martialArtsOwner
	target *martialArtsTarget
}

func (s *MartialArtsBonusStrikeTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &martialArtsOwner{id: "monk-1"}
	s.target = &martialArtsTarget{id: "goblin-1"}
}

func TestMartialArtsBonusStrikeSuite(t *testing.T) {
	suite.Run(t, new(MartialArtsBonusStrikeTestSuite))
}

func (s *MartialArtsBonusStrikeTestSuite) TestNewMartialArtsBonusStrike() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	s.Equal("strike-1", strike.GetID())
	s.Equal(core.EntityType("action"), strike.GetType())
	s.True(strike.IsTemporary())
	s.Equal(1, strike.UsesRemaining())
}

func (s *MartialArtsBonusStrikeTestSuite) TestActionType() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	s.Equal(coreCombat.ActionBonus, strike.ActionType())
}

func (s *MartialArtsBonusStrikeTestSuite) TestCapacityType() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	s.Equal(combat.CapacityNone, strike.CapacityType())
}

func (s *MartialArtsBonusStrikeTestSuite) TestCanActivate_RequiresActionEconomy() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	err := strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		ActionEconomy: nil,
		Target:        s.target,
	})

	s.Require().Error(err)
	s.Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
	s.Contains(err.Error(), "action economy required")
}

func (s *MartialArtsBonusStrikeTestSuite) TestCanActivate_RequiresTarget() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	economy := combat.NewActionEconomy()

	err := strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		ActionEconomy: economy,
		Target:        nil,
	})

	s.Require().Error(err)
	s.Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
	s.Contains(err.Error(), "requires a target")
}

func (s *MartialArtsBonusStrikeTestSuite) TestCanActivate_RequiresBonusAction() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	economy := combat.NewActionEconomy()
	// Use up the bonus action
	_ = economy.UseBonusAction()

	err := strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		ActionEconomy: economy,
		Target:        s.target,
	})

	s.Require().Error(err)
	s.Equal(rpgerr.CodeResourceExhausted, rpgerr.GetCode(err))
	s.Contains(err.Error(), "no bonus action available")
}

func (s *MartialArtsBonusStrikeTestSuite) TestCanActivate_Success() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	economy := combat.NewActionEconomy()

	err := strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		ActionEconomy: economy,
		Target:        s.target,
	})

	s.NoError(err)
}

func (s *MartialArtsBonusStrikeTestSuite) TestActivate_ConsumesBonusAction() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	economy := combat.NewActionEconomy()
	s.True(economy.CanUseBonusAction())

	err := strike.Activate(s.ctx, s.owner, actions.ActionInput{
		ActionEconomy: economy,
		Target:        s.target,
		Bus:           s.bus,
	})

	s.NoError(err)
	s.False(economy.CanUseBonusAction())
}

func (s *MartialArtsBonusStrikeTestSuite) TestActivate_MarksAsUsed() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	economy := combat.NewActionEconomy()
	s.Equal(1, strike.UsesRemaining())

	err := strike.Activate(s.ctx, s.owner, actions.ActionInput{
		ActionEconomy: economy,
		Target:        s.target,
		Bus:           s.bus,
	})

	s.NoError(err)
	s.Equal(0, strike.UsesRemaining())
}

func (s *MartialArtsBonusStrikeTestSuite) TestActivate_CannotUseAgain() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	// Apply to bus so it can remove itself
	err := strike.Apply(s.ctx, s.bus)
	s.Require().NoError(err)

	economy := combat.NewActionEconomy()

	// First activation should work
	err = strike.Activate(s.ctx, s.owner, actions.ActionInput{
		ActionEconomy: economy,
		Target:        s.target,
		Bus:           s.bus,
	})
	s.NoError(err)

	// Reset economy for second attempt
	economy2 := combat.NewActionEconomy()

	// Second activation should fail
	// After Activate(), the action removes itself, so error is "has been removed" (InvalidArgument)
	// rather than "already used" (ResourceExhausted)
	err = strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
		ActionEconomy: economy2,
		Target:        s.target,
	})
	s.Error(err)
	// Action removes itself after use, so it's "removed" not just "used"
	s.Equal(rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
	s.Contains(err.Error(), "removed")
}

func (s *MartialArtsBonusStrikeTestSuite) TestApplyAndRemove() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	// Apply
	err := strike.Apply(s.ctx, s.bus)
	s.NoError(err)

	// Apply again should fail
	err = strike.Apply(s.ctx, s.bus)
	s.Error(err)
	s.Equal(rpgerr.CodeAlreadyExists, rpgerr.GetCode(err))

	// Remove
	err = strike.Remove(s.ctx, s.bus)
	s.NoError(err)

	// Remove again should be no-op
	err = strike.Remove(s.ctx, s.bus)
	s.NoError(err)
}

func (s *MartialArtsBonusStrikeTestSuite) TestToJSON() {
	strike := actions.NewMartialArtsBonusStrike(actions.MartialArtsBonusStrikeConfig{
		ID:      "strike-1",
		OwnerID: "monk-1",
	})

	data, err := strike.ToJSON()
	s.Require().NoError(err)
	s.Contains(string(data), "strike-1")
	s.Contains(string(data), "monk-1")
	s.Contains(string(data), "martial_arts_bonus_strike")
}
