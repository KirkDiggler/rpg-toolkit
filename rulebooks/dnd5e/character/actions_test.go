package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// mockAction implements actions.Action for testing
type mockAction struct {
	id          string
	temporary   bool
	uses        int
	applied     bool
	removed     bool
	activations int
	applyErr    error // If set, Apply returns this error
}

func (m *mockAction) GetID() string {
	return m.id
}

func (m *mockAction) GetType() core.EntityType {
	return actions.EntityTypeAction
}

func (m *mockAction) CanActivate(_ context.Context, _ core.Entity, _ actions.ActionInput) error {
	return nil
}

func (m *mockAction) Activate(_ context.Context, _ core.Entity, _ actions.ActionInput) error {
	m.activations++
	if m.uses > 0 {
		m.uses--
	}
	return nil
}

func (m *mockAction) Apply(_ context.Context, _ events.EventBus) error {
	if m.applyErr != nil {
		return m.applyErr
	}
	m.applied = true
	return nil
}

func (m *mockAction) Remove(_ context.Context, _ events.EventBus) error {
	m.removed = true
	return nil
}

func (m *mockAction) IsTemporary() bool {
	return m.temporary
}

func (m *mockAction) UsesRemaining() int {
	return m.uses
}

func (m *mockAction) ToJSON() (json.RawMessage, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.id,
		"temporary": m.temporary,
		"uses":      m.uses,
	})
}

func (m *mockAction) ActionType() coreCombat.ActionType {
	return coreCombat.ActionBonus
}

// ActionHolderTestSuite tests the ActionHolder implementation on Character
type ActionHolderTestSuite struct {
	suite.Suite
	character *Character
}

func (s *ActionHolderTestSuite) SetupTest() {
	s.character = &Character{
		id:   "test-char",
		name: "Test Character",
	}
}

func (s *ActionHolderTestSuite) TestAddAction() {
	action := &mockAction{id: "test-action-1", uses: 1}

	err := s.character.AddAction(action)

	s.Require().NoError(err)
	s.Assert().Len(s.character.actions, 1)
	s.Assert().Equal("test-action-1", s.character.actions[0].GetID())
}

func (s *ActionHolderTestSuite) TestAddActionNilReturnsError() {
	err := s.character.AddAction(nil)

	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "cannot be nil")
}

func (s *ActionHolderTestSuite) TestAddMultipleActions() {
	action1 := &mockAction{id: "action-1"}
	action2 := &mockAction{id: "action-2"}

	s.Require().NoError(s.character.AddAction(action1))
	s.Require().NoError(s.character.AddAction(action2))

	s.Assert().Len(s.character.actions, 2)
}

func (s *ActionHolderTestSuite) TestGetActions() {
	action1 := &mockAction{id: "action-1"}
	action2 := &mockAction{id: "action-2"}
	s.Require().NoError(s.character.AddAction(action1))
	s.Require().NoError(s.character.AddAction(action2))

	result := s.character.GetActions()

	s.Assert().Len(result, 2)
}

func (s *ActionHolderTestSuite) TestGetActionsReturnsEmptySliceWhenNone() {
	result := s.character.GetActions()

	s.Assert().Nil(result)
}

func (s *ActionHolderTestSuite) TestGetAction() {
	action := &mockAction{id: "target-action", uses: 3}
	s.Require().NoError(s.character.AddAction(&mockAction{id: "other-action"}))
	s.Require().NoError(s.character.AddAction(action))

	result := s.character.GetAction("target-action")

	s.Require().NotNil(result)
	s.Assert().Equal("target-action", result.GetID())
	s.Assert().Equal(3, result.UsesRemaining())
}

func (s *ActionHolderTestSuite) TestGetActionReturnsNilWhenNotFound() {
	s.Require().NoError(s.character.AddAction(&mockAction{id: "other-action"}))

	result := s.character.GetAction("nonexistent")

	s.Assert().Nil(result)
}

func (s *ActionHolderTestSuite) TestRemoveAction() {
	action1 := &mockAction{id: "keep-action"}
	action2 := &mockAction{id: "remove-action"}
	s.Require().NoError(s.character.AddAction(action1))
	s.Require().NoError(s.character.AddAction(action2))

	err := s.character.RemoveAction("remove-action")

	s.Require().NoError(err)
	s.Assert().Len(s.character.actions, 1)
	s.Assert().Equal("keep-action", s.character.actions[0].GetID())
}

func (s *ActionHolderTestSuite) TestRemoveActionNotFoundReturnsError() {
	s.Require().NoError(s.character.AddAction(&mockAction{id: "existing"}))

	err := s.character.RemoveAction("nonexistent")

	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "not found")
}

func (s *ActionHolderTestSuite) TestRemoveMiddleAction() {
	s.Require().NoError(s.character.AddAction(&mockAction{id: "first"}))
	s.Require().NoError(s.character.AddAction(&mockAction{id: "middle"}))
	s.Require().NoError(s.character.AddAction(&mockAction{id: "last"}))

	err := s.character.RemoveAction("middle")

	s.Require().NoError(err)
	s.Assert().Len(s.character.actions, 2)
	s.Assert().Equal("first", s.character.actions[0].GetID())
	s.Assert().Equal("last", s.character.actions[1].GetID())
}

func (s *ActionHolderTestSuite) TestCharacterImplementsActionHolder() {
	// Compile-time check is in character.go, this is a runtime verification
	var holder actions.ActionHolder = s.character
	s.Assert().NotNil(holder)
}

func TestActionHolderSuite(t *testing.T) {
	suite.Run(t, new(ActionHolderTestSuite))
}

// ActionLifecycleTestSuite tests action Apply/Remove through the event system
type ActionLifecycleTestSuite struct {
	suite.Suite
	character *Character
	bus       events.EventBus
	ctx       context.Context
}

func (s *ActionLifecycleTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.character = &Character{
		id:   "test-char",
		name: "Test Character",
		bus:  s.bus,
	}
	err := s.character.subscribeToEvents(s.ctx)
	s.Require().NoError(err)
}

func (s *ActionLifecycleTestSuite) TestActionGrantedCallsApply() {
	action := &mockAction{id: "temp-action", temporary: true}

	// Publish ActionGrantedEvent
	topic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
	err := topic.Publish(s.ctx, dnd5eEvents.ActionGrantedEvent{
		CharacterID: "test-char",
		Action:      action,
		Source:      "test",
	})

	s.Require().NoError(err)
	s.Assert().True(action.applied, "Apply should be called on granted action")
	s.Assert().Len(s.character.actions, 1)
}

func (s *ActionLifecycleTestSuite) TestActionGrantedToleratesAlreadyApplied() {
	// Simulate a granter that already called Apply (returns AlreadyExists on second call)
	action := &mockAction{
		id:        "pre-applied-action",
		temporary: true,
		applyErr:  rpgerr.New(rpgerr.CodeAlreadyExists, "already applied"),
	}

	// Publish ActionGrantedEvent - should not error despite Apply returning AlreadyExists
	topic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
	err := topic.Publish(s.ctx, dnd5eEvents.ActionGrantedEvent{
		CharacterID: "test-char",
		Action:      action,
		Source:      "test",
	})

	s.Require().NoError(err)
	s.Assert().Len(s.character.actions, 1, "action should still be added")
}

func (s *ActionLifecycleTestSuite) TestActionGrantedIgnoresOtherCharacters() {
	action := &mockAction{id: "other-action", temporary: true}

	topic := dnd5eEvents.ActionGrantedTopic.On(s.bus)
	err := topic.Publish(s.ctx, dnd5eEvents.ActionGrantedEvent{
		CharacterID: "other-char",
		Action:      action,
		Source:      "test",
	})

	s.Require().NoError(err)
	s.Assert().False(action.applied, "should not apply action for other character")
	s.Assert().Empty(s.character.actions)
}

func (s *ActionLifecycleTestSuite) TestCleanupRemovesTemporaryActions() {
	tempAction := &mockAction{id: "temp-action", temporary: true}
	permAction := &mockAction{id: "perm-action", temporary: false}

	s.Require().NoError(s.character.AddAction(tempAction))
	s.Require().NoError(s.character.AddAction(permAction))
	s.Assert().Len(s.character.actions, 2)

	err := s.character.Cleanup(s.ctx)

	s.Require().NoError(err)
	s.Assert().True(tempAction.removed, "temporary action should be removed")
	s.Assert().False(permAction.removed, "permanent action should NOT be removed")
	s.Assert().Len(s.character.actions, 1, "only permanent action should remain")
	s.Assert().Equal("perm-action", s.character.actions[0].GetID())
}

func (s *ActionLifecycleTestSuite) TestCleanupKeepsAllPermanentActions() {
	perm1 := &mockAction{id: "strike-1", temporary: false}
	perm2 := &mockAction{id: "strike-2", temporary: false}

	s.Require().NoError(s.character.AddAction(perm1))
	s.Require().NoError(s.character.AddAction(perm2))

	err := s.character.Cleanup(s.ctx)

	s.Require().NoError(err)
	s.Assert().Len(s.character.actions, 2, "all permanent actions should remain")
	s.Assert().False(perm1.removed)
	s.Assert().False(perm2.removed)
}

func (s *ActionLifecycleTestSuite) TestCleanupRemovesAllTemporaryActions() {
	temp1 := &mockAction{id: "flurry-1", temporary: true}
	temp2 := &mockAction{id: "offhand-1", temporary: true}

	s.Require().NoError(s.character.AddAction(temp1))
	s.Require().NoError(s.character.AddAction(temp2))

	err := s.character.Cleanup(s.ctx)

	s.Require().NoError(err)
	s.Assert().True(temp1.removed)
	s.Assert().True(temp2.removed)
	s.Assert().Empty(s.character.actions, "no actions should remain")
}

func TestActionLifecycleSuite(t *testing.T) {
	suite.Run(t, new(ActionLifecycleTestSuite))
}
