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

type DodgeAbilityTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
	dodge         *combatabilities.Dodge
}

func TestDodgeAbilityTestSuite(t *testing.T) {
	suite.Run(t, new(DodgeAbilityTestSuite))
}

func (s *DodgeAbilityTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-character"}
	s.actionEconomy = combat.NewActionEconomy()
	s.dodge = combatabilities.NewDodge("test-dodge")
}

func (s *DodgeAbilityTestSuite) TestNewDodge_Properties() {
	// Assert
	s.Assert().Equal("test-dodge", s.dodge.GetID())
	s.Assert().Equal(core.EntityType("combat_ability"), s.dodge.GetType())
	s.Assert().Equal("Dodge", s.dodge.Name())
	s.Assert().Contains(s.dodge.Description(), "disadvantage")
	s.Assert().Equal(coreCombat.ActionStandard, s.dodge.ActionType())
	s.Assert().Equal(refs.CombatAbilities.Dodge(), s.dodge.Ref())
}

func (s *DodgeAbilityTestSuite) TestCanActivate_Success() {
	// Arrange
	s.Require().True(s.actionEconomy.CanUseAction())

	// Act
	err := s.dodge.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
}

func (s *DodgeAbilityTestSuite) TestCanActivate_NoActionsRemaining() {
	// Arrange - consume the action
	err := s.actionEconomy.UseAction()
	s.Require().NoError(err)

	// Act
	err = s.dodge.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert
	s.Require().Error(err)
}

func (s *DodgeAbilityTestSuite) TestCanActivate_RequiresEventBus() {
	// Act
	err := s.dodge.CanActivate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           nil,
	})

	// Assert - should fail because Bus is required
	s.Require().Error(err)
}

func (s *DodgeAbilityTestSuite) TestActivate_ConsumesAction() {
	// Arrange
	s.Require().Equal(1, s.actionEconomy.ActionsRemaining)

	// Act
	err := s.dodge.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.ActionsRemaining)
}

func (s *DodgeAbilityTestSuite) TestActivate_PublishesDodgeActivatedEvent() {
	// Arrange - subscribe to the event
	eventReceived := false
	var receivedEvent dnd5eEvents.DodgeActivatedEvent

	_, err := dnd5eEvents.DodgeActivatedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(ctx context.Context, event dnd5eEvents.DodgeActivatedEvent) error {
			eventReceived = true
			receivedEvent = event
			return nil
		},
	)
	s.Require().NoError(err)

	// Act
	err = s.dodge.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().True(eventReceived, "DodgeActivatedEvent should be published")
	s.Assert().Equal(s.owner.GetID(), receivedEvent.CharacterID)
}

func (s *DodgeAbilityTestSuite) TestActivate_NoActionEconomy() {
	// Act
	err := s.dodge.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		Bus: s.bus,
	})

	// Assert
	s.Require().Error(err)
}

func (s *DodgeAbilityTestSuite) TestActivate_NoEventBus() {
	// Act
	err := s.dodge.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
	})

	// Assert - should fail because Bus is required to publish event
	s.Require().Error(err)
}

func (s *DodgeAbilityTestSuite) TestToJSON() {
	// Act
	jsonData, err := s.dodge.ToJSON()

	// Assert
	s.Require().NoError(err)
	s.Assert().NotEmpty(jsonData)

	// Verify JSON structure
	var data combatabilities.DodgeData
	err = json.Unmarshal(jsonData, &data)
	s.Require().NoError(err)
	s.Assert().Equal("test-dodge", data.ID)
	s.Assert().Equal(refs.CombatAbilities.Dodge(), data.Ref)
}

func (s *DodgeAbilityTestSuite) TestCombatAbilityInterface() {
	// Act & Assert - verify it can be assigned to the interface
	var ability combatabilities.CombatAbility = s.dodge
	s.Assert().NotNil(ability)
	s.Assert().Equal("test-dodge", ability.GetID())
	s.Assert().Equal("Dodge", ability.Name())
}

// BonusDodgeTestSuite tests scenarios where Dodge is used as a bonus action (Patient Defense)
type BonusDodgeTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
}

func TestBonusDodgeTestSuite(t *testing.T) {
	suite.Run(t, new(BonusDodgeTestSuite))
}

func (s *BonusDodgeTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-monk"}
	s.actionEconomy = combat.NewActionEconomy()
}

func (s *BonusDodgeTestSuite) TestBonusDodge_ConsumesBonusAction() {
	// Arrange - Monk with Patient Defense (Dodge as bonus action)
	bonusDodge := combatabilities.NewBonusDodge("dodge-bonus")
	s.Require().Equal(1, s.actionEconomy.BonusActionsRemaining)

	// Act
	err := bonusDodge.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().Equal(0, s.actionEconomy.BonusActionsRemaining)
	s.Assert().Equal(1, s.actionEconomy.ActionsRemaining) // Standard action still available
}

func (s *BonusDodgeTestSuite) TestBonusDodge_PublishesEvent() {
	// Arrange
	bonusDodge := combatabilities.NewBonusDodge("dodge-bonus")
	eventReceived := false

	_, err := dnd5eEvents.DodgeActivatedTopic.On(s.bus).Subscribe(
		s.ctx,
		func(ctx context.Context, event dnd5eEvents.DodgeActivatedEvent) error {
			eventReceived = true
			return nil
		},
	)
	s.Require().NoError(err)

	// Act
	err = bonusDodge.Activate(s.ctx, s.owner, combatabilities.CombatAbilityInput{
		ActionEconomy: s.actionEconomy,
		Bus:           s.bus,
	})

	// Assert
	s.Require().NoError(err)
	s.Assert().True(eventReceived)
}
