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
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type MoveTestSuite struct {
	suite.Suite
	ctx             context.Context
	bus             events.EventBus
	owner           *mockOwner
	actionEconomy   *combat.ActionEconomy
	move            *actions.Move
	currentPosition *spatial.Position
	destination     *spatial.Position
}

func TestMoveTestSuite(t *testing.T) {
	suite.Run(t, new(MoveTestSuite))
}

func (s *MoveTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-character"}
	s.actionEconomy = combat.NewActionEconomy()
	// Set 30ft of movement (standard speed)
	s.actionEconomy.SetMovement(30)

	s.move = actions.NewMove(actions.MoveConfig{
		ID:      "test-move-1",
		OwnerID: s.owner.id,
	})

	s.currentPosition = &spatial.Position{X: 0, Y: 0}
	s.destination = &spatial.Position{X: 3, Y: 4} // 5 squares = 25ft diagonal
}

func (s *MoveTestSuite) TestNewMove() {
	s.Run("creates move with correct properties", func() {
		s.Assert().Equal("test-move-1", s.move.GetID())
		s.Assert().Equal(core.EntityType("action"), s.move.GetType())
		s.Assert().Equal(actions.UnlimitedUses, s.move.UsesRemaining())
		s.Assert().False(s.move.IsTemporary())
	})
}

func (s *MoveTestSuite) TestCanActivate_Success() {
	s.Run("succeeds when movement remaining and destination provided", func() {
		err := s.move.CanActivate(s.ctx, s.owner, actions.ActionInput{
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: s.currentPosition,
			Destination:     s.destination,
			MovementCostFt:  15,
		})
		s.Require().NoError(err)
	})
}

func (s *MoveTestSuite) TestCanActivate_NoActionEconomy() {
	s.Run("fails when action economy is nil", func() {
		err := s.move.CanActivate(s.ctx, s.owner, actions.ActionInput{
			CurrentPosition: s.currentPosition,
			Destination:     s.destination,
			MovementCostFt:  15,
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
		s.Assert().Contains(rpgErr.Message, "action economy required")
	})
}

func (s *MoveTestSuite) TestCanActivate_NoDestination() {
	s.Run("fails when destination is nil", func() {
		err := s.move.CanActivate(s.ctx, s.owner, actions.ActionInput{
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: s.currentPosition,
			Destination:     nil,
			MovementCostFt:  15,
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
		s.Assert().Contains(rpgErr.Message, "destination")
	})
}

func (s *MoveTestSuite) TestCanActivate_NoCurrentPosition() {
	s.Run("fails when current position is nil", func() {
		err := s.move.CanActivate(s.ctx, s.owner, actions.ActionInput{
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: nil,
			Destination:     s.destination,
			MovementCostFt:  15,
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
		s.Assert().Contains(rpgErr.Message, "current position")
	})
}

func (s *MoveTestSuite) TestCanActivate_ZeroMovementCost() {
	s.Run("fails when movement cost is zero", func() {
		err := s.move.CanActivate(s.ctx, s.owner, actions.ActionInput{
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: s.currentPosition,
			Destination:     s.destination,
			MovementCostFt:  0,
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
		s.Assert().Contains(rpgErr.Message, "positive")
	})
}

func (s *MoveTestSuite) TestCanActivate_InsufficientMovement() {
	s.Run("fails when insufficient movement remaining", func() {
		err := s.move.CanActivate(s.ctx, s.owner, actions.ActionInput{
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: s.currentPosition,
			Destination:     s.destination,
			MovementCostFt:  35, // More than the 30ft available
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
		s.Assert().Contains(rpgErr.Message, "insufficient movement")
		s.Assert().Contains(rpgErr.Message, "need 35 ft")
		s.Assert().Contains(rpgErr.Message, "have 30 ft")
	})
}

func (s *MoveTestSuite) TestActivate_ConsumesMovement() {
	s.Run("consumes movement from action economy", func() {
		s.Require().Equal(30, s.actionEconomy.MovementRemaining)

		err := s.move.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: s.currentPosition,
			Destination:     s.destination,
			MovementCostFt:  15,
		})

		s.Require().NoError(err)
		s.Assert().Equal(15, s.actionEconomy.MovementRemaining)
	})
}

func (s *MoveTestSuite) TestActivate_PublishesMoveExecutedEvent() {
	s.Run("publishes move executed event", func() {
		var receivedEvent *dnd5eEvents.MoveExecutedEvent
		topic := dnd5eEvents.MoveExecutedTopic.On(s.bus)
		_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.MoveExecutedEvent) error {
			receivedEvent = &event
			return nil
		})
		s.Require().NoError(err)

		err = s.move.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: s.currentPosition,
			Destination:     s.destination,
			MovementCostFt:  15,
		})

		s.Require().NoError(err)
		s.Require().NotNil(receivedEvent)
		s.Assert().Equal(s.owner.id, receivedEvent.EntityID)
		s.Assert().Equal("test-move-1", receivedEvent.ActionID)
		s.Assert().Equal(float64(0), receivedEvent.FromX)
		s.Assert().Equal(float64(0), receivedEvent.FromY)
		s.Assert().Equal(float64(3), receivedEvent.ToX)
		s.Assert().Equal(float64(4), receivedEvent.ToY)
		s.Assert().Equal(15, receivedEvent.DistanceFt)
	})
}

func (s *MoveTestSuite) TestActivate_MultipleMovements() {
	s.Run("can make multiple movements until exhausted", func() {
		// First move: 15ft
		err := s.move.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: s.currentPosition,
			Destination:     &spatial.Position{X: 3, Y: 0},
			MovementCostFt:  15,
		})
		s.Require().NoError(err)
		s.Assert().Equal(15, s.actionEconomy.MovementRemaining)

		// Second move: 10ft
		err = s.move.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: &spatial.Position{X: 3, Y: 0},
			Destination:     &spatial.Position{X: 5, Y: 0},
			MovementCostFt:  10,
		})
		s.Require().NoError(err)
		s.Assert().Equal(5, s.actionEconomy.MovementRemaining)

		// Third move: 10ft should fail (only 5ft remaining)
		err = s.move.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: &spatial.Position{X: 5, Y: 0},
			Destination:     &spatial.Position{X: 7, Y: 0},
			MovementCostFt:  10,
		})
		s.Require().Error(err)
		var rpgErr *rpgerr.Error
		s.Require().True(errors.As(err, &rpgErr))
		s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
	})
}

func (s *MoveTestSuite) TestActivate_NoBus() {
	s.Run("succeeds without bus (no event published)", func() {
		err := s.move.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:             nil,
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: s.currentPosition,
			Destination:     s.destination,
			MovementCostFt:  15,
		})
		s.Require().NoError(err)
		s.Assert().Equal(15, s.actionEconomy.MovementRemaining)
	})
}

func (s *MoveTestSuite) TestActivate_ExactMovement() {
	s.Run("can use exactly remaining movement", func() {
		err := s.move.Activate(s.ctx, s.owner, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   s.actionEconomy,
			CurrentPosition: s.currentPosition,
			Destination:     s.destination,
			MovementCostFt:  30, // Exactly 30ft
		})
		s.Require().NoError(err)
		s.Assert().Equal(0, s.actionEconomy.MovementRemaining)
	})
}

func (s *MoveTestSuite) TestApply_NoOp() {
	s.Run("apply does nothing for permanent action", func() {
		err := s.move.Apply(s.ctx, s.bus)
		s.Require().NoError(err)
	})
}

func (s *MoveTestSuite) TestRemove_NoOp() {
	s.Run("remove does nothing for permanent action", func() {
		err := s.move.Remove(s.ctx, s.bus)
		s.Require().NoError(err)
	})
}

func (s *MoveTestSuite) TestToJSON() {
	s.Run("serializes to JSON correctly", func() {
		jsonData, err := s.move.ToJSON()
		s.Require().NoError(err)
		s.Assert().NotEmpty(jsonData)
		s.Assert().Contains(string(jsonData), "test-move-1")
		s.Assert().Contains(string(jsonData), "move")
	})
}

// Scenario test: Character with Dash uses double movement
func (s *MoveTestSuite) TestScenario_DashDoubleMovement() {
	s.Run("character with Dash can move 60ft", func() {
		// Setup: Character with 30ft speed
		rogue := &mockOwner{id: "merisiel-rogue"}
		economy := combat.NewActionEconomy()

		// Normal movement at turn start
		economy.SetMovement(30)

		// Simulate Dash ability (adds speed again)
		economy.AddMovement(30)

		s.Assert().Equal(60, economy.MovementRemaining)

		move := actions.NewMove(actions.MoveConfig{
			ID:      "move-1",
			OwnerID: rogue.id,
		})

		// Track events
		moveCount := 0
		totalDistance := 0
		topic := dnd5eEvents.MoveExecutedTopic.On(s.bus)
		_, err := topic.Subscribe(s.ctx, func(_ context.Context, event dnd5eEvents.MoveExecutedEvent) error {
			moveCount++
			totalDistance += event.DistanceFt
			return nil
		})
		s.Require().NoError(err)

		// First move: 30ft
		err = move.Activate(s.ctx, rogue, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   economy,
			CurrentPosition: &spatial.Position{X: 0, Y: 0},
			Destination:     &spatial.Position{X: 6, Y: 0},
			MovementCostFt:  30,
		})
		s.Require().NoError(err)

		// Second move: 25ft
		err = move.Activate(s.ctx, rogue, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   economy,
			CurrentPosition: &spatial.Position{X: 6, Y: 0},
			Destination:     &spatial.Position{X: 11, Y: 0},
			MovementCostFt:  25,
		})
		s.Require().NoError(err)

		// Verify
		s.Assert().Equal(2, moveCount)
		s.Assert().Equal(55, totalDistance)
		s.Assert().Equal(5, economy.MovementRemaining)
	})
}

// Scenario test: Fighter interleaves attacks and movement
func (s *MoveTestSuite) TestScenario_FighterInterleavedCombat() {
	s.Run("fighter can interleave attacks and movement", func() {
		// This tests that Strike and Move actions can be used in any order

		fighter := &mockOwner{id: "seelah-fighter"}
		goblin1 := &mockTarget{id: "goblin-1"}
		goblin2 := &mockTarget{id: "goblin-2"}
		economy := combat.NewActionEconomy()
		economy.SetMovement(30)
		economy.SetAttacks(2) // Extra Attack

		move := actions.NewMove(actions.MoveConfig{
			ID:      "move-1",
			OwnerID: fighter.id,
		})

		strike := actions.NewStrike(actions.StrikeConfig{
			ID:       "strike-1",
			OwnerID:  fighter.id,
			WeaponID: "longsword",
		})

		// Attack goblin 1
		err := strike.Activate(s.ctx, fighter, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: economy,
			Target:        goblin1,
		})
		s.Require().NoError(err)
		s.Assert().Equal(1, economy.AttacksRemaining)

		// Move 15ft to goblin 2
		err = move.Activate(s.ctx, fighter, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   economy,
			CurrentPosition: &spatial.Position{X: 0, Y: 0},
			Destination:     &spatial.Position{X: 3, Y: 0},
			MovementCostFt:  15,
		})
		s.Require().NoError(err)
		s.Assert().Equal(15, economy.MovementRemaining)

		// Attack goblin 2 with second attack
		err = strike.Activate(s.ctx, fighter, actions.ActionInput{
			Bus:           s.bus,
			ActionEconomy: economy,
			Target:        goblin2,
		})
		s.Require().NoError(err)
		s.Assert().Equal(0, economy.AttacksRemaining)

		// Move 10ft to safety
		err = move.Activate(s.ctx, fighter, actions.ActionInput{
			Bus:             s.bus,
			ActionEconomy:   economy,
			CurrentPosition: &spatial.Position{X: 3, Y: 0},
			Destination:     &spatial.Position{X: 5, Y: 0},
			MovementCostFt:  10,
		})
		s.Require().NoError(err)
		s.Assert().Equal(5, economy.MovementRemaining)
	})
}
