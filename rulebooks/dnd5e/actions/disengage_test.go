package actions_test

// Wave 2.11e (#666) — Disengage action tests.
//
// Disengage consumes 1 action and applies the DisengagingCondition to the
// owner. The condition suppresses opportunity attacks for the rest of the
// owner's turn (auto-removes on TurnEnd). The toolkit-as-product framing
// wants the rule application (Apply'ing the condition) toolkit-side; the
// game server (rpg-api) gets a DisengageActivatedEvent for telemetry.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

type DisengageTestSuite struct {
	suite.Suite
	ctx           context.Context
	bus           events.EventBus
	owner         *mockOwner
	actionEconomy *combat.ActionEconomy
	disengage     *actions.Disengage
}

func TestDisengageTestSuite(t *testing.T) {
	suite.Run(t, new(DisengageTestSuite))
}

func (s *DisengageTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.owner = &mockOwner{id: "test-rogue"}
	s.actionEconomy = combat.NewActionEconomy()

	s.disengage = actions.NewDisengage(actions.DisengageConfig{
		ID:      "test-disengage-1",
		OwnerID: s.owner.id,
	})
}

func (s *DisengageTestSuite) TestNewDisengage() {
	s.Equal("test-disengage-1", s.disengage.GetID())
	s.Equal(actions.EntityTypeAction, s.disengage.GetType())
}

func (s *DisengageTestSuite) TestCanActivate_NoActionEconomy_Errors() {
	err := s.disengage.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Bus: s.bus,
	})
	s.Require().Error(err)
	s.True(rpgerr.GetCode(err) == rpgerr.CodeInvalidArgument,
		"expected InvalidArgument, got %v", err)
}

func (s *DisengageTestSuite) TestCanActivate_NoActionsRemaining_Errors() {
	// Burn the action first.
	s.Require().NoError(s.actionEconomy.UseAction())

	err := s.disengage.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Bus:           s.bus,
		ActionEconomy: s.actionEconomy,
	})
	s.Require().Error(err)
	s.True(rpgerr.GetCode(err) == rpgerr.CodeResourceExhausted,
		"expected ResourceExhausted, got %v", err)
}

func (s *DisengageTestSuite) TestCanActivate_WithActionAvailable_OK() {
	err := s.disengage.CanActivate(s.ctx, s.owner, actions.ActionInput{
		Bus:           s.bus,
		ActionEconomy: s.actionEconomy,
	})
	s.Require().NoError(err)
}

func (s *DisengageTestSuite) TestActivate_ConsumesAction() {
	s.Equal(1, s.actionEconomy.ActionsRemaining)

	err := s.disengage.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:           s.bus,
		ActionEconomy: s.actionEconomy,
	})
	s.Require().NoError(err)

	s.Equal(0, s.actionEconomy.ActionsRemaining,
		"Disengage must consume 1 action from the economy")
}

func (s *DisengageTestSuite) TestActivate_PublishesDisengageActivatedEvent() {
	// Subscribe to capture the activation event.
	var captured *dnd5eEvents.DisengageActivatedEvent
	topic := dnd5eEvents.DisengageActivatedTopic.On(s.bus)
	_, err := topic.Subscribe(s.ctx, func(_ context.Context, evt dnd5eEvents.DisengageActivatedEvent) error {
		captured = &evt
		return nil
	})
	s.Require().NoError(err)

	err = s.disengage.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:           s.bus,
		ActionEconomy: s.actionEconomy,
	})
	s.Require().NoError(err)

	s.Require().NotNil(captured, "DisengageActivatedEvent must fire for game-server telemetry")
	s.Equal(s.owner.id, captured.CharacterID)
}

// TestActivate_AppliesDisengagingCondition_OASuppressed is the load-bearing
// goal-shaped test: after Activate, the owner's movement must produce an
// OAPreventionSources entry in the MovementChain event. This proves the
// DisengagingCondition was applied toolkit-side (Q1=(a) director signoff)
// and is actively suppressing OAs.
//
// NOTE: this test does NOT re-test the DisengagingCondition's internals
// (already covered in conditions/disengaging_test.go). It only verifies
// that Disengage.Activate wires the condition to the bus so the suppression
// mechanism activates.
func (s *DisengageTestSuite) TestActivate_AppliesDisengagingCondition_OASuppressed() {
	err := s.disengage.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus:           s.bus,
		ActionEconomy: s.actionEconomy,
	})
	s.Require().NoError(err)

	// Now publish a MovementChainEvent for the owner moving. The applied
	// DisengagingCondition should subscribe to MovementChain and add an
	// OAPreventionSources entry at StageConditions.
	movementChain := dnd5eEvents.MovementChain.On(s.bus)
	chainBuilder := events.NewStagedChain[*dnd5eEvents.MovementChainEvent](combat.ModifierStages)
	event := &dnd5eEvents.MovementChainEvent{
		EntityID:            s.owner.id,
		FromPosition:        dnd5eEvents.Position{X: 0, Y: 0},
		ToPosition:          dnd5eEvents.Position{X: 1, Y: 0},
		OAPreventionSources: []dnd5eEvents.MovementModifierSource{},
	}

	modifiedChain, err := movementChain.PublishWithChain(s.ctx, event, chainBuilder)
	s.Require().NoError(err)

	finalEvent, err := modifiedChain.Execute(s.ctx, event)
	s.Require().NoError(err)

	// The condition added a prevention source.
	s.Require().NotEmpty(finalEvent.OAPreventionSources,
		"DisengagingCondition must add OAPreventionSources after Disengage.Activate")
	s.True(finalEvent.IsOAPrevented(),
		"IsOAPrevented must be true so OpportunityAttackCondition predicate skips")
	s.Equal("Disengaging", finalEvent.OAPreventionSources[0].Name)
}

func (s *DisengageTestSuite) TestActivate_FailsWithoutEconomy() {
	err := s.disengage.Activate(s.ctx, s.owner, actions.ActionInput{
		Bus: s.bus,
	})
	s.Require().Error(err,
		"Activate must surface the CanActivate failure when no ActionEconomy supplied")
}

func (s *DisengageTestSuite) TestActionType_Action() {
	// Regular Disengage is a full Action cost. Bonus-action Disengage is
	// reachable via the monk's Step of the Wind feature, not via this
	// constructor (Q3 director signoff: full-action constructor only).
	s.Equal(coreCombat.ActionStandard, s.disengage.ActionType())
}

func (s *DisengageTestSuite) TestCapacityType_None() {
	// Disengage consumes only the action; no specialized capacity.
	s.Equal(combat.CapacityNone, s.disengage.CapacityType())
}

func (s *DisengageTestSuite) TestIsTemporary_False() {
	// Disengage is a permanent ability; activating it doesn't change
	// the ability's availability for future turns.
	s.False(s.disengage.IsTemporary())
}

func (s *DisengageTestSuite) TestUsesRemaining_Unlimited() {
	s.Equal(actions.UnlimitedUses, s.disengage.UsesRemaining())
}

func (s *DisengageTestSuite) TestToJSON_RoundTrip() {
	data, err := s.disengage.ToJSON()
	s.Require().NoError(err)
	s.NotEmpty(data, "ToJSON must produce serializable bytes")
}
