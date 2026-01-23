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

type AttackAbilityTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
	attack        *combatabilities.Attack
}

func TestAttackAbilityTestSuite(t *testing.T) {
	suite.Run(t, new(AttackAbilityTestSuite))
}

func (s *AttackAbilityTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-character"}
	s.actionEconomy = combat.NewActionEconomy()
	s.attack = combatabilities.NewAttack("test-attack")
}

func (s *AttackAbilityTestSuite) TestNewAttack_Properties() {
	// Assert
	s.Assert().Equal("test-attack", s.attack.GetID())
	s.Assert().Equal(core.EntityType("combat_ability"), s.attack.GetType())
	s.Assert().Equal("Attack", s.attack.Name())
	s.Assert().Equal("Make weapon attacks against enemies.", s.attack.Description())
	s.Assert().Equal(coreCombat.ActionStandard, s.attack.ActionType())
	s.Assert().Equal(refs.CombatAbilities.Attack(), s.attack.Ref())
}

func (s *AttackAbilityTestSuite) TestCanActivate_Success() {
	// Arrange
	s.Require().True(s.actionEconomy.CanUseAction())

	// Act
	err := s.attack.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *AttackAbilityTestSuite) TestCanActivate_NoActionsRemaining() {
	// Arrange - consume the action
	err := s.actionEconomy.UseAction()
	s.Require().NoError(err)

	// Act
	err = s.attack.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().Error(err)
}

func (s *AttackAbilityTestSuite) TestActivate_ConsumesAction() {
	// Arrange
	s.Require().Equal(1, s.actionEconomy.ActionsRemaining)

	// Act
	err := s.attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
}

func (s *AttackAbilityTestSuite) TestActivate_SetsAttacksRemaining_NoExtraAttack() {
	// Arrange - normal character with no Extra Attack
	s.Require().Equal(0, s.actionEconomy.AttacksRemaining)

	// Act
	err := s.attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		ExtraAttacks:  0, // Normal character (1 attack)
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(1, s.actionEconomy.AttacksRemaining)
}

func (s *AttackAbilityTestSuite) TestActivate_SetsAttacksRemaining_ExtraAttack() {
	// Arrange - Fighter with Extra Attack (level 5+)
	s.Require().Equal(0, s.actionEconomy.AttacksRemaining)

	// Act
	err := s.attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		ExtraAttacks:  1, // Extra Attack feature (2 attacks total)
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(2, s.actionEconomy.AttacksRemaining)
}

func (s *AttackAbilityTestSuite) TestActivate_SetsAttacksRemaining_ExtraAttack2() {
	// Arrange - Fighter with Extra Attack (2) at level 11
	s.Require().Equal(0, s.actionEconomy.AttacksRemaining)

	// Act
	err := s.attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		ExtraAttacks:  2, // Extra Attack (2) feature (3 attacks total)
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(3, s.actionEconomy.AttacksRemaining)
}

func (s *AttackAbilityTestSuite) TestActivate_SetsAttacksRemaining_ExtraAttack3() {
	// Arrange - Fighter with Extra Attack (3) at level 20
	s.Require().Equal(0, s.actionEconomy.AttacksRemaining)

	// Act
	err := s.attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		ExtraAttacks:  3, // Extra Attack (3) feature (4 attacks total)
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(4, s.actionEconomy.AttacksRemaining)
}

func (s *AttackAbilityTestSuite) TestActivate_NoActionEconomy() {
	// Act
	err := s.attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{})

	// Assert
	s.Require().Error(err)
}

func (s *AttackAbilityTestSuite) TestToJSON() {
	// Act
	jsonData, err := s.attack.ToJSON()

	// Assert
	s.Require().NoError(err)
	s.Assert().NotEmpty(jsonData)

	// Verify JSON structure
	var data combatabilities.AttackData
	err = json.Unmarshal(jsonData, &data)
	s.Require().NoError(err)
	s.Assert().Equal("test-attack", data.ID)
	s.Assert().Equal(refs.CombatAbilities.Attack(), data.Ref)
}

func (s *AttackAbilityTestSuite) TestCombatAbilityInterface() {
	// Act & Assert - verify it can be assigned to the interface
	var ability combatabilities.CombatAbility = s.attack
	s.Assert().NotNil(ability)
	s.Assert().Equal("test-attack", ability.GetID())
	s.Assert().Equal("Attack", ability.Name())
}

// FighterCombatFlowTestSuite tests realistic combat scenarios
type FighterCombatFlowTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
}

func TestFighterCombatFlowTestSuite(t *testing.T) {
	suite.Run(t, new(FighterCombatFlowTestSuite))
}

func (s *FighterCombatFlowTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-fighter"}
	s.actionEconomy = combat.NewActionEconomy()
	s.actionEconomy.SetMovement(30) // Fighter with 30ft speed
}

func (s *FighterCombatFlowTestSuite) TestFighterTurn_AttackThenStrike() {
	// Arrange
	attack := combatabilities.NewAttack("attack-ability")
	s.Require().Equal(1, s.actionEconomy.ActionsRemaining)
	s.Require().Equal(0, s.actionEconomy.AttacksRemaining)

	// Act 1 - Use Attack ability (level 5 fighter with Extra Attack)
	err := attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		Bus:           s.bus,
		ActionEconomy: s.actionEconomy,
		ExtraAttacks:  1, // Fighter has Extra Attack
	})

	// Assert after Attack ability
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
	s.Assert().Equal(2, s.actionEconomy.AttacksRemaining)

	// Act 2 - First strike
	err = s.actionEconomy.UseAttack()
	s.Require().NoError(err)
	s.Assert().Equal(1, s.actionEconomy.AttacksRemaining)

	// Act 3 - Second strike
	err = s.actionEconomy.UseAttack()
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.AttacksRemaining)

	// Act 4 - No more attacks
	s.Assert().False(s.actionEconomy.CanUseAttack())
}

func (s *FighterCombatFlowTestSuite) TestFighterTurn_ActionSurge() {
	// Arrange - Fighter with Action Surge (two actions this turn)
	attack := combatabilities.NewAttack("attack-ability")
	s.actionEconomy.GrantExtraAction()
	s.Require().Equal(2, s.actionEconomy.ActionsRemaining)

	// Act 1 - First Attack ability
	err := attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		Bus:           s.bus,
		ActionEconomy: s.actionEconomy,
		ExtraAttacks:  1,
	})
	s.Require().NoError(err)
	s.Assert().Equal(1, s.actionEconomy.ActionsRemaining)
	s.Assert().Equal(2, s.actionEconomy.AttacksRemaining)

	// Use both attacks
	_ = s.actionEconomy.UseAttack()
	_ = s.actionEconomy.UseAttack()
	s.Assert().Equal(0, s.actionEconomy.AttacksRemaining)

	// Act 2 - Second Attack ability (Action Surge!)
	err = attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		Bus:           s.bus,
		ActionEconomy: s.actionEconomy,
		ExtraAttacks:  1,
	})
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
	s.Assert().Equal(2, s.actionEconomy.AttacksRemaining)
}
