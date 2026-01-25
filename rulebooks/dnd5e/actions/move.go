package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// Move represents a movement action that consumes movement from the action economy.
// Movement in D&D 5e is free - you don't need to use your action to move.
// However, you are limited by your MovementRemaining for the turn.
// This action publishes a MoveExecutedEvent for the game layer to handle
// the actual position change and any movement-triggered effects (like AoO).
type Move struct {
	id      string
	ownerID string
}

// MoveConfig contains configuration for creating a Move action
type MoveConfig struct {
	ID      string
	OwnerID string
}

// NewMove creates a new Move action
func NewMove(config MoveConfig) *Move {
	return &Move{
		id:      config.ID,
		ownerID: config.OwnerID,
	}
}

// GetID implements core.Entity
func (m *Move) GetID() string {
	return m.id
}

// GetType implements core.Entity
func (m *Move) GetType() core.EntityType {
	return EntityTypeAction
}

// CanActivate implements core.Action[ActionInput]
// Move can be activated when there is enough movement remaining for the distance.
func (m *Move) CanActivate(_ context.Context, _ core.Entity, input ActionInput) error {
	if input.ActionEconomy == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action economy required")
	}

	if input.Destination == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "move requires a destination")
	}

	if input.CurrentPosition == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "move requires current position")
	}

	if input.MovementCostFt <= 0 {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "movement cost must be positive")
	}

	if !input.ActionEconomy.CanUseMovement(input.MovementCostFt) {
		return rpgerr.New(rpgerr.CodeResourceExhausted,
			fmt.Sprintf("insufficient movement: need %d ft, have %d ft",
				input.MovementCostFt, input.ActionEconomy.MovementRemaining))
	}

	return nil
}

// Activate implements core.Action[ActionInput]
// Move consumes the movement cost and publishes a MoveExecutedEvent for the game
// layer to handle the actual position change.
func (m *Move) Activate(ctx context.Context, owner core.Entity, input ActionInput) error {
	if err := m.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Store positions before consuming movement (for event)
	fromX := input.CurrentPosition.X
	fromY := input.CurrentPosition.Y
	toX := input.Destination.X
	toY := input.Destination.Y
	distanceFt := input.MovementCostFt

	// Consume the movement
	if err := input.ActionEconomy.UseMovement(distanceFt); err != nil {
		return rpgerr.Wrapf(err, "failed to use movement")
	}

	// Publish the move executed event for the game server to handle
	if input.Bus != nil {
		topic := dnd5eEvents.MoveExecutedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.MoveExecutedEvent{
			EntityID:   owner.GetID(),
			ActionID:   m.id,
			FromX:      fromX,
			FromY:      fromY,
			ToX:        toX,
			ToY:        toY,
			DistanceFt: distanceFt,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish move executed event")
		}
	}

	return nil
}

// Apply implements Action - Move is a permanent action and does not need
// to subscribe to any events.
func (m *Move) Apply(_ context.Context, _ events.EventBus) error {
	// Move is permanent and doesn't need event subscriptions
	return nil
}

// Remove implements Action - Move is a permanent action and does not need
// to unsubscribe from any events.
func (m *Move) Remove(_ context.Context, _ events.EventBus) error {
	// Move is permanent and doesn't need cleanup
	return nil
}

// IsTemporary returns false - Move is a permanent action
func (m *Move) IsTemporary() bool {
	return false
}

// UsesRemaining returns UnlimitedUses - Move can be used as long as movement remains
func (m *Move) UsesRemaining() int {
	return UnlimitedUses
}

// ToJSON converts the action to JSON for persistence
func (m *Move) ToJSON() (json.RawMessage, error) {
	data := map[string]interface{}{
		"id":       m.id,
		"owner_id": m.ownerID,
		"type":     "move",
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal move: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost (free - movement is not an action)
func (m *Move) ActionType() coreCombat.ActionType {
	return coreCombat.ActionFree
}

// CapacityType returns that Move consumes movement capacity
func (m *Move) CapacityType() combat.CapacityType {
	return combat.CapacityMovement
}

// Compile-time check that Move implements Action
var _ Action = (*Move)(nil)
