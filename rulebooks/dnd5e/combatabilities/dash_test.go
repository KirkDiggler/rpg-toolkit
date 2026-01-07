package combatabilities_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type DashAbilityTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
	dash          *combatabilities.Dash
}

func TestDashAbilityTestSuite(t *testing.T) {
	suite.Run(t, new(DashAbilityTestSuite))
}

func (s *DashAbilityTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-character"}
	s.actionEconomy = combat.NewActionEconomy()
	s.actionEconomy.SetMovement(30) // Standard 30ft speed
	s.dash = combatabilities.NewDash("test-dash")
}

func (s *DashAbilityTestSuite) TestNewDash_Properties() {
	// Assert
	s.Assert().Equal("test-dash", s.dash.GetID())
	s.Assert().Equal(core.EntityType("combat_ability"), s.dash.GetType())
	s.Assert().Equal("Dash", s.dash.Name())
	s.Assert().Equal("Double your movement speed for this turn.", s.dash.Description())
	s.Assert().Equal(coreCombat.ActionStandard, s.dash.ActionType())
	s.Assert().Equal(refs.CombatAbilities.Dash(), s.dash.Ref())
}

func (s *DashAbilityTestSuite) TestCanActivate_Success() {
	// Arrange
	s.Require().True(s.actionEconomy.CanUseAction())

	// Act
	err := s.dash.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *DashAbilityTestSuite) TestCanActivate_NoActionsRemaining() {
	// Arrange - consume the action
	err := s.actionEconomy.UseAction()
	s.Require().NoError(err)

	// Act
	err = s.dash.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().Error(err)
}

func (s *DashAbilityTestSuite) TestActivate_ConsumesAction() {
	// Arrange
	s.Require().Equal(1, s.actionEconomy.ActionsRemaining)

	// Act
	err := s.dash.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Speed:         30,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
}

func (s *DashAbilityTestSuite) TestActivate_AddsMovement() {
	// Arrange - character has 30ft movement set at turn start
	s.Require().Equal(30, s.actionEconomy.MovementRemaining)

	// Act
	err := s.dash.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Speed:         30,
	})

	// Assert
	s.Require().NoError(err)
	// Should now have 60ft (30 initial + 30 from dash)
	s.Assert().Equal(60, s.actionEconomy.MovementRemaining)
}

func (s *DashAbilityTestSuite) TestActivate_AddsMovement_FastCharacter() {
	// Arrange - Monk with 40ft speed
	s.actionEconomy.SetMovement(40)
	s.Require().Equal(40, s.actionEconomy.MovementRemaining)

	// Act
	err := s.dash.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Speed:         40,
	})

	// Assert
	s.Require().NoError(err)
	// Should now have 80ft (40 initial + 40 from dash)
	s.Assert().Equal(80, s.actionEconomy.MovementRemaining)
}

func (s *DashAbilityTestSuite) TestActivate_AddsMovement_AfterPartialMovement() {
	// Arrange - character moved 15ft already (15ft remaining of 30)
	s.Require().Equal(30, s.actionEconomy.MovementRemaining)
	err := s.actionEconomy.UseMovement(15)
	s.Require().NoError(err)
	s.Require().Equal(15, s.actionEconomy.MovementRemaining)

	// Act
	err = s.dash.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Speed:         30,
	})

	// Assert
	s.Require().NoError(err)
	// Should have 45ft (15 remaining + 30 from dash)
	s.Assert().Equal(45, s.actionEconomy.MovementRemaining)
}

func (s *DashAbilityTestSuite) TestActivate_NoActionEconomy() {
	// Act
	err := s.dash.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		Speed: 30,
	})

	// Assert
	s.Require().Error(err)
}

func (s *DashAbilityTestSuite) TestActivate_ZeroSpeed() {
	// Arrange - character with 0 speed (grappled, restrained, etc)
	s.actionEconomy.SetMovement(0)

	// Act
	err := s.dash.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Speed:         0,
	})

	// Assert - still consumes action but adds 0 movement
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
	s.Assert().Equal(0, s.actionEconomy.MovementRemaining)
}

func (s *DashAbilityTestSuite) TestToJSON() {
	// Act
	jsonData, err := s.dash.ToJSON()

	// Assert
	s.Require().NoError(err)
	s.Assert().NotEmpty(jsonData)

	// Verify JSON structure
	var data combatabilities.DashData
	err = json.Unmarshal(jsonData, &data)
	s.Require().NoError(err)
	s.Assert().Equal("test-dash", data.ID)
	s.Assert().Equal(refs.CombatAbilities.Dash(), data.Ref)
}

func (s *DashAbilityTestSuite) TestCombatAbilityInterface() {
	// Act & Assert - verify it can be assigned to the interface
	var ability combatabilities.CombatAbility = s.dash
	s.Assert().NotNil(ability)
	s.Assert().Equal("test-dash", ability.GetID())
	s.Assert().Equal("Dash", ability.Name())
}

// RogueCunningActionTestSuite tests scenarios where Dash is used as a bonus action
type RogueCunningActionTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
}

func TestRogueCunningActionTestSuite(t *testing.T) {
	suite.Run(t, new(RogueCunningActionTestSuite))
}

func (s *RogueCunningActionTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-rogue"}
	s.actionEconomy = combat.NewActionEconomy()
	s.actionEconomy.SetMovement(30)
}

func (s *RogueCunningActionTestSuite) TestBonusDash_DoubleDash() {
	// Arrange - Rogue can Dash as both action and bonus action (Cunning Action)
	standardDash := combatabilities.NewDash("dash-action")
	bonusDash := combatabilities.NewBonusDash("dash-bonus")

	s.Require().Equal(30, s.actionEconomy.MovementRemaining)

	// Act 1 - Use Dash as action
	err := standardDash.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Speed:         30,
	})
	s.Require().NoError(err)
	s.Assert().Equal(60, s.actionEconomy.MovementRemaining)

	// Act 2 - Use Cunning Action: Dash as bonus action
	err = bonusDash.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Speed:         30,
	})
	s.Require().NoError(err)
	s.Assert().Equal(90, s.actionEconomy.MovementRemaining)

	// Assert final state
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
	s.Assert().Equal(0, s.actionEconomy.BonusActionsRemaining)
}
