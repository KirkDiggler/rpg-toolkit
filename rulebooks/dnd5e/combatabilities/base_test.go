package combatabilities_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

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

type BaseCombatAbilityTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
	base          *combatabilities.BaseCombatAbility
}

func TestBaseCombatAbilityTestSuite(t *testing.T) {
	suite.Run(t, new(BaseCombatAbilityTestSuite))
}

func (s *BaseCombatAbilityTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-character"}
	s.actionEconomy = combat.NewActionEconomy()
	s.base = combatabilities.NewBaseCombatAbility(combatabilities.BaseCombatAbilityConfig{
		ID:          "test-ability",
		Name:        "Test Ability",
		Description: "A test combat ability",
		ActionType:  coreCombat.ActionStandard,
		Ref:         refs.CombatAbilities.Attack(),
	})
}

func (s *BaseCombatAbilityTestSuite) TestNewBaseCombatAbility() {
	// Assert
	s.Assert().Equal("test-ability", s.base.GetID())
	s.Assert().Equal(core.EntityType("combat_ability"), s.base.GetType())
	s.Assert().Equal("Test Ability", s.base.Name())
	s.Assert().Equal("A test combat ability", s.base.Description())
	s.Assert().Equal(coreCombat.ActionStandard, s.base.ActionType())
	s.Assert().Equal(refs.CombatAbilities.Attack(), s.base.Ref())
}

func (s *BaseCombatAbilityTestSuite) TestCanActivate_Success_StandardAction() {
	// Arrange
	s.Require().True(s.actionEconomy.CanUseAction())

	// Act
	err := s.base.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *BaseCombatAbilityTestSuite) TestCanActivate_NoActionEconomy() {
	// Act
	err := s.base.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
}

func (s *BaseCombatAbilityTestSuite) TestCanActivate_NoActionsRemaining() {
	// Arrange - consume the action
	err := s.actionEconomy.UseAction()
	s.Require().NoError(err)
	s.Require().False(s.actionEconomy.CanUseAction())

	// Act
	err = s.base.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *BaseCombatAbilityTestSuite) TestCanActivate_BonusAction() {
	// Arrange
	bonusAbility := combatabilities.NewBaseCombatAbility(combatabilities.BaseCombatAbilityConfig{
		ID:          "bonus-ability",
		Name:        "Bonus Ability",
		Description: "Uses a bonus action",
		ActionType:  coreCombat.ActionBonus,
		Ref:         refs.CombatAbilities.Dash(),
	})

	// Act
	err := bonusAbility.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *BaseCombatAbilityTestSuite) TestCanActivate_NoBonusActionsRemaining() {
	// Arrange
	bonusAbility := combatabilities.NewBaseCombatAbility(combatabilities.BaseCombatAbilityConfig{
		ID:          "bonus-ability",
		Name:        "Bonus Ability",
		Description: "Uses a bonus action",
		ActionType:  coreCombat.ActionBonus,
		Ref:         refs.CombatAbilities.Dash(),
	})
	err := s.actionEconomy.UseBonusAction()
	s.Require().NoError(err)

	// Act
	err = bonusAbility.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *BaseCombatAbilityTestSuite) TestCanActivate_Reaction() {
	// Arrange
	reactionAbility := combatabilities.NewBaseCombatAbility(combatabilities.BaseCombatAbilityConfig{
		ID:          "reaction-ability",
		Name:        "Reaction Ability",
		Description: "Uses a reaction",
		ActionType:  coreCombat.ActionReaction,
		Ref:         refs.CombatAbilities.Attack(),
	})

	// Act
	err := reactionAbility.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *BaseCombatAbilityTestSuite) TestCanActivate_NoReactionsRemaining() {
	// Arrange
	reactionAbility := combatabilities.NewBaseCombatAbility(combatabilities.BaseCombatAbilityConfig{
		ID:          "reaction-ability",
		Name:        "Reaction Ability",
		Description: "Uses a reaction",
		ActionType:  coreCombat.ActionReaction,
		Ref:         refs.CombatAbilities.Attack(),
	})
	err := s.actionEconomy.UseReaction()
	s.Require().NoError(err)

	// Act
	err = reactionAbility.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeResourceExhausted, rpgErr.Code)
}

func (s *BaseCombatAbilityTestSuite) TestCanActivate_FreeAction() {
	// Arrange
	freeAbility := combatabilities.NewBaseCombatAbility(combatabilities.BaseCombatAbilityConfig{
		ID:          "free-ability",
		Name:        "Free Ability",
		Description: "Free action",
		ActionType:  coreCombat.ActionFree,
		Ref:         refs.CombatAbilities.Attack(),
	})

	// Consume all actions to prove free doesn't need them
	err := s.actionEconomy.UseAction()
	s.Require().NoError(err)
	err = s.actionEconomy.UseBonusAction()
	s.Require().NoError(err)
	err = s.actionEconomy.UseReaction()
	s.Require().NoError(err)

	// Act
	err = freeAbility.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *BaseCombatAbilityTestSuite) TestActivate_ConsumesStandardAction() {
	// Arrange
	s.Require().Equal(1, s.actionEconomy.ActionsRemaining)

	// Act
	err := s.base.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
}

func (s *BaseCombatAbilityTestSuite) TestActivate_ConsumesBonusAction() {
	// Arrange
	bonusAbility := combatabilities.NewBaseCombatAbility(combatabilities.BaseCombatAbilityConfig{
		ID:          "bonus-ability",
		Name:        "Bonus Ability",
		Description: "Uses a bonus action",
		ActionType:  coreCombat.ActionBonus,
		Ref:         refs.CombatAbilities.Dash(),
	})
	s.Require().Equal(1, s.actionEconomy.BonusActionsRemaining)

	// Act
	err := bonusAbility.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.BonusActionsRemaining)
}

func (s *BaseCombatAbilityTestSuite) TestActivate_ConsumesReaction() {
	// Arrange
	reactionAbility := combatabilities.NewBaseCombatAbility(combatabilities.BaseCombatAbilityConfig{
		ID:          "reaction-ability",
		Name:        "Reaction Ability",
		Description: "Uses a reaction",
		ActionType:  coreCombat.ActionReaction,
		Ref:         refs.CombatAbilities.Attack(),
	})
	s.Require().Equal(1, s.actionEconomy.ReactionsRemaining)

	// Act
	err := reactionAbility.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.ReactionsRemaining)
}

func (s *BaseCombatAbilityTestSuite) TestActivate_FreeDoesNotConsume() {
	// Arrange
	freeAbility := combatabilities.NewBaseCombatAbility(combatabilities.BaseCombatAbilityConfig{
		ID:          "free-ability",
		Name:        "Free Ability",
		Description: "Free action",
		ActionType:  coreCombat.ActionFree,
		Ref:         refs.CombatAbilities.Attack(),
	})
	s.Require().Equal(1, s.actionEconomy.ActionsRemaining)
	s.Require().Equal(1, s.actionEconomy.BonusActionsRemaining)
	s.Require().Equal(1, s.actionEconomy.ReactionsRemaining)

	// Act
	err := freeAbility.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(1, s.actionEconomy.ActionsRemaining)
	s.Assert().Equal(1, s.actionEconomy.BonusActionsRemaining)
	s.Assert().Equal(1, s.actionEconomy.ReactionsRemaining)
}

func (s *BaseCombatAbilityTestSuite) TestActivate_NoActionEconomy() {
	// Act
	err := s.base.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{})

	// Assert
	s.Require().Error(err)
	var rpgErr *rpgerr.Error
	s.Require().True(errors.As(err, &rpgErr))
	s.Assert().Equal(rpgerr.CodeInvalidArgument, rpgErr.Code)
}

func (s *BaseCombatAbilityTestSuite) TestApply_NoOp() {
	// Act
	err := s.base.Apply(s.ctx, s.bus)

	// Assert
	s.Require().NoError(err)
}

func (s *BaseCombatAbilityTestSuite) TestRemove_NoOp() {
	// Act
	err := s.base.Remove(s.ctx, s.bus)

	// Assert
	s.Require().NoError(err)
}

func (s *BaseCombatAbilityTestSuite) TestToJSON() {
	// Act
	jsonData, err := s.base.ToJSON()

	// Assert
	s.Require().NoError(err)
	s.Assert().NotEmpty(jsonData)

	// Verify JSON structure
	var data combatabilities.BaseCombatAbilityData
	err = json.Unmarshal(jsonData, &data)
	s.Require().NoError(err)
	s.Assert().Equal("test-ability", data.ID)
	s.Assert().Equal("Test Ability", data.Name)
	s.Assert().Equal("A test combat ability", data.Description)
	s.Assert().Equal(coreCombat.ActionStandard, data.ActionType)
	s.Assert().Equal(refs.CombatAbilities.Attack(), data.Ref)
}

func (s *BaseCombatAbilityTestSuite) TestLoadJSON_UnknownType() {
	// Arrange
	jsonData := []byte(`{"ref": {"module": "dnd5e", "type": "combat_abilities", "id": "unknown"}}`)

	// Act
	ability, err := combatabilities.LoadJSON(jsonData)

	// Assert
	s.Require().Error(err)
	s.Assert().Nil(ability)
	s.Assert().Contains(err.Error(), "unknown combat ability type")
}

func (s *BaseCombatAbilityTestSuite) TestLoadJSON_InvalidJSON() {
	// Arrange
	jsonData := []byte(`{invalid json}`)

	// Act
	ability, err := combatabilities.LoadJSON(jsonData)

	// Assert
	s.Require().Error(err)
	s.Assert().Nil(ability)
}

// TestCombatAbilityInterface verifies that BaseCombatAbility can be used as CombatAbility
// when wrapped by a concrete implementation
type testCombatAbility struct {
	*combatabilities.BaseCombatAbility
}

func (s *BaseCombatAbilityTestSuite) TestCombatAbilityInterface() {
	// Arrange - create a concrete ability that embeds BaseCombatAbility
	concreteAbility := &testCombatAbility{
		BaseCombatAbility: s.base,
	}

	// Act & Assert - verify it can be assigned to the interface
	var ability combatabilities.CombatAbility = concreteAbility
	s.Assert().NotNil(ability)
	s.Assert().Equal("test-ability", ability.GetID())
	s.Assert().Equal("Test Ability", ability.Name())
}
