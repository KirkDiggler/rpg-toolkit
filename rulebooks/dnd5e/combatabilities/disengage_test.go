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
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type DisengageAbilityTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
	disengage     *combatabilities.Disengage
}

func TestDisengageAbilityTestSuite(t *testing.T) {
	suite.Run(t, new(DisengageAbilityTestSuite))
}

func (s *DisengageAbilityTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-character"}
	s.actionEconomy = combat.NewActionEconomy()
	s.disengage = combatabilities.NewDisengage("test-disengage")
}

func (s *DisengageAbilityTestSuite) TestNewDisengage_Properties() {
	// Assert
	s.Assert().Equal("test-disengage", s.disengage.GetID())
	s.Assert().Equal(core.EntityType("combat_ability"), s.disengage.GetType())
	s.Assert().Equal("Disengage", s.disengage.Name())
	s.Assert().Contains(s.disengage.Description(), "opportunity attacks")
	s.Assert().Equal(coreCombat.ActionStandard, s.disengage.ActionType())
	s.Assert().Equal(refs.CombatAbilities.Disengage(), s.disengage.Ref())
}

func (s *DisengageAbilityTestSuite) TestCanActivate_Success() {
	// Arrange
	s.Require().True(s.actionEconomy.CanUseAction())

	// Act
	err := s.disengage.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *DisengageAbilityTestSuite) TestCanActivate_NoActionsRemaining() {
	// Arrange - consume the action
	err := s.actionEconomy.UseAction()
	s.Require().NoError(err)

	// Act
	err = s.disengage.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().Error(err)
}

func (s *DisengageAbilityTestSuite) TestCanActivate_RequiresEventBus() {
	// Act
	err := s.disengage.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           nil,
	})

	// Assert - should fail because Bus is required
	s.Require().Error(err)
}

func (s *DisengageAbilityTestSuite) TestActivate_ConsumesAction() {
	// Arrange
	s.Require().Equal(1, s.actionEconomy.ActionsRemaining)

	// Act
	err := s.disengage.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
}

func (s *DisengageAbilityTestSuite) TestActivate_PublishesDisengageActivatedEvent() {
	// Arrange - subscribe to the event
	eventReceived := false
	var receivedEvent dnd5eEvents.DisengageActivatedEvent

	_, err := dnd5eEvents.DisengageActivatedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(ctx context.Context, event dnd5eEvents.DisengageActivatedEvent) error {
			eventReceived = true
			receivedEvent = event
			return nil
		},
	)
	s.Require().NoError(err)

	// Act
	err = s.disengage.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().True(eventReceived, "DisengageActivatedEvent should be published")
	s.Assert().Equal(s.owner.GetID(), receivedEvent.CharacterID)
}

func (s *DisengageAbilityTestSuite) TestActivate_NoActionEconomy() {
	// Act
	err := s.disengage.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		Bus: s.bus,
	})

	// Assert
	s.Require().Error(err)
}

func (s *DisengageAbilityTestSuite) TestActivate_NoEventBus() {
	// Act
	err := s.disengage.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert - should fail because Bus is required to publish event
	s.Require().Error(err)
}

func (s *DisengageAbilityTestSuite) TestToJSON() {
	// Act
	jsonData, err := s.disengage.ToJSON()

	// Assert
	s.Require().NoError(err)
	s.Assert().NotEmpty(jsonData)

	// Verify JSON structure
	var data combatabilities.DisengageData
	err = json.Unmarshal(jsonData, &data)
	s.Require().NoError(err)
	s.Assert().Equal("test-disengage", data.ID)
	s.Assert().Equal(refs.CombatAbilities.Disengage(), data.Ref)
}

func (s *DisengageAbilityTestSuite) TestCombatAbilityInterface() {
	// Act & Assert - verify it can be assigned to the interface
	var ability combatabilities.CombatAbility = s.disengage
	s.Assert().NotNil(ability)
	s.Assert().Equal("test-disengage", ability.GetID())
	s.Assert().Equal("Disengage", ability.Name())
}

// BonusDisengageTestSuite tests scenarios where Disengage is used as a bonus action
type BonusDisengageTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
}

func TestBonusDisengageTestSuite(t *testing.T) {
	suite.Run(t, new(BonusDisengageTestSuite))
}

func (s *BonusDisengageTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-rogue"}
	s.actionEconomy = combat.NewActionEconomy()
}

func (s *BonusDisengageTestSuite) TestBonusDisengage_ConsumesBonusAction() {
	// Arrange - Rogue with Cunning Action (Disengage as bonus action)
	bonusDisengage := combatabilities.NewBonusDisengage("disengage-bonus")
	s.Require().Equal(1, s.actionEconomy.BonusActionsRemaining)

	// Act
	err := bonusDisengage.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.BonusActionsRemaining)
	s.Assert().Equal(1, s.actionEconomy.ActionsRemaining) // Standard action still available
}

func (s *BonusDisengageTestSuite) TestBonusDisengage_PublishesEvent() {
	// Arrange
	bonusDisengage := combatabilities.NewBonusDisengage("disengage-bonus")
	eventReceived := false

	_, err := dnd5eEvents.DisengageActivatedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(ctx context.Context, event dnd5eEvents.DisengageActivatedEvent) error {
			eventReceived = true
			return nil
		},
	)
	s.Require().NoError(err)

	// Act
	err = bonusDisengage.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().True(eventReceived)
}

// RogueCunningActionDisengageTestSuite tests Rogue-specific Disengage scenarios
type RogueCunningActionDisengageTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
}

func TestRogueCunningActionDisengageTestSuite(t *testing.T) {
	suite.Run(t, new(RogueCunningActionDisengageTestSuite))
}

func (s *RogueCunningActionDisengageTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-rogue"}
	s.actionEconomy = combat.NewActionEconomy()
	s.actionEconomy.SetMovement(30)
}

func (s *RogueCunningActionDisengageTestSuite) TestRogue_DisengageThenAttack() {
	// Arrange - Rogue wants to hit-and-run
	bonusDisengage := combatabilities.NewBonusDisengage("cunning-disengage")
	attack := combatabilities.NewAttack("attack")

	// Act 1 - Use Cunning Action: Disengage (bonus action)
	err := bonusDisengage.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})
	s.Require().NoError(err)

	// Act 2 - Use Attack (standard action)
	err = attack.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		ExtraAttacks:  0, // Rogue has no Extra Attack
	})
	s.Require().NoError(err)

	// Assert - Rogue can still move (to escape)
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
	s.Assert().Equal(0, s.actionEconomy.BonusActionsRemaining)
	s.Assert().Equal(30, s.actionEconomy.MovementRemaining)
	s.Assert().Equal(1, s.actionEconomy.AttacksRemaining)
}
