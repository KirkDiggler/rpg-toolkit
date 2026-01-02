package actions_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

// mockActionHolder implements actions.ActionHolder for testing
type mockActionHolder struct {
	actions    []actions.Action
	addError   error
	addCalled  bool
	addedIndex int
}

func newMockActionHolder() *mockActionHolder {
	return &mockActionHolder{
		actions: []actions.Action{},
	}
}

func (m *mockActionHolder) AddAction(action actions.Action) error {
	m.addCalled = true
	if m.addError != nil {
		return m.addError
	}
	m.addedIndex = len(m.actions)
	m.actions = append(m.actions, action)
	return nil
}

func (m *mockActionHolder) RemoveAction(actionID string) error {
	for i, a := range m.actions {
		if a.GetID() == actionID {
			m.actions = append(m.actions[:i], m.actions[i+1:]...)
			return nil
		}
	}
	return rpgerr.Newf(rpgerr.CodeNotFound, "action %s not found", actionID)
}

func (m *mockActionHolder) GetActions() []actions.Action {
	return m.actions
}

func (m *mockActionHolder) GetAction(actionID string) actions.Action {
	for _, a := range m.actions {
		if a.GetID() == actionID {
			return a
		}
	}
	return nil
}

type TwoWeaponGranterTestSuite struct {
	suite.Suite
	ctx          context.Context
	bus          events.EventBus
	actionHolder *mockActionHolder
}

func TestTwoWeaponGranterTestSuite(t *testing.T) {
	suite.Run(t, new(TwoWeaponGranterTestSuite))
}

func (s *TwoWeaponGranterTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.actionHolder = newMockActionHolder()
}

func (s *TwoWeaponGranterTestSuite) TestGrantsOffHandStrike_DualLightWeapons() {
	// Arrange - dual-wielding shortswords (both light)
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().True(output.Granted)
	s.Assert().NotNil(output.Action)
	s.Assert().Equal("dual-wielding light weapons", output.Reason)
	s.Assert().True(s.actionHolder.addCalled)
	s.Assert().Len(s.actionHolder.GetActions(), 1)
}

func (s *TwoWeaponGranterTestSuite) TestGrantsOffHandStrike_DaggerAndShortsword() {
	// Arrange - dagger and shortsword (both light)
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-rogue",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Dagger},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().True(output.Granted)
	s.Assert().NotNil(output.Action)
}

func (s *TwoWeaponGranterTestSuite) TestDenied_OffHandAttack() {
	// Arrange - this is an off-hand attack, not main-hand
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandOff, // Off-hand attack
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().False(output.Granted)
	s.Assert().Nil(output.Action)
	s.Assert().Equal("not a main-hand attack", output.Reason)
	s.Assert().False(s.actionHolder.addCalled)
}

func (s *TwoWeaponGranterTestSuite) TestDenied_NoMainHandWeapon() {
	// Arrange - no main-hand weapon (unarmed?)
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: nil, // No main-hand weapon
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().False(output.Granted)
	s.Assert().Equal("no main-hand weapon", output.Reason)
}

func (s *TwoWeaponGranterTestSuite) TestDenied_NoOffHandWeapon() {
	// Arrange - no off-hand weapon (shield or empty)
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  nil, // No off-hand weapon
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().False(output.Granted)
	s.Assert().Equal("no off-hand weapon", output.Reason)
}

func (s *TwoWeaponGranterTestSuite) TestDenied_MainHandNotLight() {
	// Arrange - longsword is not light
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Longsword}, // Not light
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().False(output.Granted)
	s.Assert().Equal("main-hand weapon is not light", output.Reason)
}

func (s *TwoWeaponGranterTestSuite) TestDenied_OffHandNotLight() {
	// Arrange - shortsword is light, but longsword is not
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Longsword}, // Not light
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().False(output.Granted)
	s.Assert().Equal("off-hand weapon is not light", output.Reason)
}

func (s *TwoWeaponGranterTestSuite) TestDenied_MainHandWeaponNotFound() {
	// Arrange - invalid weapon ID
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: "invalid-weapon"},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().False(output.Granted)
	s.Assert().Equal("main-hand weapon not found", output.Reason)
}

func (s *TwoWeaponGranterTestSuite) TestDenied_OffHandWeaponNotFound() {
	// Arrange - invalid weapon ID for off-hand
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: "invalid-weapon"},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().False(output.Granted)
	s.Assert().Equal("off-hand weapon not found", output.Reason)
}

func (s *TwoWeaponGranterTestSuite) TestGrantsWithoutEventBus() {
	// Arrange - no event bus (action won't subscribe to turn-end)
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       nil, // No event bus
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().True(output.Granted)
	s.Assert().NotNil(output.Action)
}

func (s *TwoWeaponGranterTestSuite) TestGrantsWithoutActionHolder() {
	// Arrange - no action holder (action won't be added to character)
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   nil, // No action holder
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().True(output.Granted)
	s.Assert().NotNil(output.Action)
}

func (s *TwoWeaponGranterTestSuite) TestActionHolderAddError_RollsBackEventSubscription() {
	// Arrange - action holder will fail
	s.actionHolder.addError = rpgerr.New(rpgerr.CodeInternal, "mock add error")
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().Error(err)
	s.Assert().Nil(output)
	s.Assert().Contains(err.Error(), "failed to add off-hand strike")
}

func (s *TwoWeaponGranterTestSuite) TestEmptyAttackHand_TreatedAsMainHand() {
	// Arrange - empty attack hand should be treated as main-hand
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     "", // Empty - should be treated as main-hand
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().True(output.Granted)
}

func (s *TwoWeaponGranterTestSuite) TestGrantedActionHasCorrectWeapon() {
	// Arrange
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Dagger}, // Different weapon
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().True(output.Granted)
	// The action should use the off-hand weapon (dagger), not main-hand
	s.Assert().Equal(weapons.Dagger, output.Action.GetWeaponID())
}

func (s *TwoWeaponGranterTestSuite) TestGrantedActionID() {
	// Arrange
	input := &actions.TwoWeaponGranterInput{
		CharacterID:    "test-fighter",
		AttackHand:     actions.AttackHandMain,
		MainHandWeapon: &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		OffHandWeapon:  &actions.EquippedWeaponInfo{WeaponID: weapons.Shortsword},
		ActionHolder:   s.actionHolder,
		EventBus:       s.bus,
	}

	// Act
	output, err := actions.CheckAndGrantOffHandStrike(s.ctx, input)

	// Assert
	s.Require().NoError(err)
	s.Require().NotNil(output)
	s.Assert().Equal("test-fighter-off-hand-strike", output.Action.GetID())
}
