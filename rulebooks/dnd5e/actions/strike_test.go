package actions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type StrikeTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	target        *mockTarget
	actionEconomy *combat.ActionEconomy
	strike        *actions.Strike
}

func TestStrikeTestSuite(t *testing.T) {
	suite.Run(t, new(StrikeTestSuite))
}

func (s *StrikeTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-fighter"}
	s.target = &mockTarget{id: "goblin-1"}
	s.actionEconomy = combat.NewActionEconomy()
	// Grant 2 attacks (like a fighter with Extra Attack)
	s.actionEconomy.SetAttacks(2)

	s.strike = actions.NewStrike(actions.StrikeConfig{
		ID:       "test-strike-1",
		OwnerID:  s.owner.id,
		WeaponID: weapons.Longsword,
	})
}

func (s *StrikeTestSuite) TestNewStrike() {
	s.Run("creates strike with correct properties", func() {
		s.Assert().Equal("test-strike-1", s.strike.GetID())
		s.Assert().Equal(core.EntityType("action"), s.strike.GetType())
		s.Assert().Equal(weapons.Longsword, s.strike.GetWeaponID())
		s.Assert().Equal(actions.UnlimitedUses, s.strike.UsesRemaining())
		s.Assert().False(s.strike.IsTemporary())
	})
}

func (s *StrikeTestSuite) TestCanActivate_Success() {
	s.Run("succeeds when attacks remaining and target provided", func() {
		err := s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
			ActionEconomy: s.actionEconomy,
			Target:        s.target,
		})
		s.Require().NoError(err)
	})
}

func (s *StrikeTestSuite) TestCanActivate_NoActionEconomy() {
	s.Run("fails when action economy is nil", func() {
		err := s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
			Target: s.target,
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
		s.Assert().Contains(rpgErr.Message, "action economy required")
	})
}

func (s *StrikeTestSuite) TestCanActivate_NoAttacksRemaining() {
	s.Run("fails when no attacks remaining", func() {
		// Use all attacks
		s.actionEconomy.SetAttacks(0)

		err := s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
			ActionEconomy: s.actionEconomy,
			Target:        s.target,
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
		s.Assert().Contains(rpgErr.Message, "no attacks remaining")
	})
}

func (s *StrikeTestSuite) TestCanActivate_NoTarget() {
	s.Run("fails when target is nil", func() {
		err := s.strike.CanActivate(s.ctx, s.owner, actions.ActionInput{
			ActionEconomy: s.actionEconomy,
			Target:        nil,
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
		s.Assert().Contains(rpgErr.Message, "strike requires a target")
	})
}

func (s *StrikeTestSuite) TestActivate_ConsumesAttack() {
	s.Run("consumes one attack from action economy", func() {
		s.Require().Equal(2, s.actionEconomy.AttacksRemaining)

		err := s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
			Target:        s.target,
		})

		s.Require().NoError(err)
		s.Assert().Equal(1, s.actionEconomy.AttacksRemaining)
	})
}

func (s *StrikeTestSuite) TestActivate_PublishesStrikeExecutedEvent() {
	s.Run("publishes strike executed event", func() {
		var receivedEvent *dnd5eEvents.StrikeExecutedEvent
		topic := dnd5eEvents.StrikeExecutedTopic.On(s.bus)
		_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.StrikeExecutedEvent) error {
			receivedEvent = &event
			return nil
		})
		s.Require().NoError(err)

		err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
			Target:        s.target,
		})

		s.Require().NoError(err)
		s.Require().NotNil(receivedEvent)
		s.Assert().Equal(s.owner.id, receivedEvent.AttackerID)
		s.Assert().Equal(s.target.id, receivedEvent.TargetID)
		s.Assert().Equal(string(weapons.Longsword), receivedEvent.WeaponID)
		s.Assert().Equal("test-strike-1", receivedEvent.ActionID)
	})
}

func (s *StrikeTestSuite) TestActivate_MultipleAttacks() {
	s.Run("can make multiple attacks until exhausted", func() {
		// First attack
		err := s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
			Target:        s.target,
		})
		s.Require().NoError(err)
		s.Assert().Equal(1, s.actionEconomy.AttacksRemaining)

		// Second attack
		err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
			Target:        s.target,
		})
		s.Require().NoError(err)
		s.Assert().Equal(0, s.actionEconomy.AttacksRemaining)

		// Third attack should fail
		err = s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: s.actionEconomy,
			Target:        s.target,
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
	})
}

func (s *StrikeTestSuite) TestActivate_NoBus() {
	s.Run("succeeds without bus (no event published)", func() {
		err := s.strike.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:           nil,
			ActionEconomy: s.actionEconomy,
			Target:        s.target,
		})
		s.Require().NoError(err)
		s.Assert().Equal(1, s.actionEconomy.AttacksRemaining)
	})
}

func (s *StrikeTestSuite) TestApply_NoOp() {
	s.Run("apply does nothing for permanent action", func() {
		err := s.strike.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
	})
}

func (s *StrikeTestSuite) TestRemove_NoOp() {
	s.Run("remove does nothing for permanent action", func() {
		err := s.strike.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
	})
}

func (s *StrikeTestSuite) TestToJSON() {
	s.Run("serializes to JSON correctly", func() {
		jsonData, err := s.strike.ToJSON()
		s.Require().NoError(err)
		s.Assert().NotEmpty(jsonData)
		s.Assert().Contains(string(jsonData), "test-strike-1")
		s.Assert().Contains(string(jsonData), "strike")
		s.Assert().Contains(string(jsonData), "longsword")
	})
}

// Scenario test: Fighter with Extra Attack makes 2 attacks
func (s *StrikeTestSuite) TestScenario_FighterExtraAttack() {
	s.Run("fighter with Extra Attack can make 2 attacks", func() {
		// Setup: Fighter with Extra Attack (2 attacks)
		fighter := &mockOwner{id: "valeros-fighter"}
		goblin := &mockTarget{id: "goblin-chief"}
		economy := combat.NewActionEconomy()

		// Simulate Attack ability being used (grants 2 attacks with Extra Attack)
		economy.SetAttacks(2)

		strike := actions.NewStrike(actions.StrikeConfig{
			ID:       "strike-1",
			OwnerID:  fighter.id,
			WeaponID: weapons.Greatsword,
		})

		// Track events
		attackCount := 0
		topic := dnd5eEvents.StrikeExecutedTopic.On(s.bus)
		_, err := topic.Subscribe(s.ctx, func(_ context.Context, _ dnd5eEvents.StrikeExecutedEvent) error {
			attackCount++
			return nil
		})
		s.Require().NoError(err)

		// First attack
		err = strike.Activate(s.ctx, fighter, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: economy,
			Target:        goblin,
		})
		s.Require().NoError(err)

		// Second attack
		err = strike.Activate(s.ctx, fighter, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: economy,
			Target:        goblin,
		})
		s.Require().NoError(err)

		// Verify
		s.Assert().Equal(2, attackCount)
		s.Assert().Equal(0, economy.AttacksRemaining)

		// Third attack should fail
		err = strike.Activate(s.ctx, fighter, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: economy,
			Target:        goblin,
		})
		s.Require().Error(err)
		s.Assert().Equal(2, attackCount) // No additional attack
	})
}
