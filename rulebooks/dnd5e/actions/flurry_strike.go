package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// FlurryStrike represents an unarmed strike granted by Flurry of Blows.
// It is a temporary action that removes itself after use or at turn end.
type FlurryStrike struct {
	id             string
	ownerID        string
	uses           int
	bus            events.EventBus
	subscriptionID string
	removed        bool
}

// FlurryStrikeConfig contains configuration for creating a FlurryStrike action
type FlurryStrikeConfig struct {
	ID      string
	OwnerID string
}

// NewFlurryStrike creates a new FlurryStrike action with 1 use
func NewFlurryStrike(config FlurryStrikeConfig) *FlurryStrike {
	return &FlurryStrike{
		id:      config.ID,
		ownerID: config.OwnerID,
		uses:    1,
	}
}

// GetID implements core.Entity
func (f *FlurryStrike) GetID() string {
	return f.id
}

// GetType implements core.Entity
func (f *FlurryStrike) GetType() core.EntityType {
	return EntityTypeAction
}

// CanActivate implements core.Action[ActionInput]
func (f *FlurryStrike) CanActivate(_ context.Context, _ core.Entity, input ActionInput) error {
	if f.removed {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "flurry strike has been removed")
	}

	if f.uses <= 0 {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "flurry strike has no uses remaining")
	}

	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "flurry strike requires a target")
	}

	return nil
}

// Activate implements core.Action[ActionInput]
func (f *FlurryStrike) Activate(ctx context.Context, owner core.Entity, input ActionInput) error {
	if err := f.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Publish the strike request event for the game server to resolve
	if input.Bus != nil {
		topic := dnd5eEvents.FlurryStrikeRequestedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.FlurryStrikeRequestedEvent{
			AttackerID: owner.GetID(),
			TargetID:   input.Target.GetID(),
			ActionID:   f.id,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish flurry strike event")
		}
	}

	// Decrement uses
	f.uses--

	// Publish notification event for UI/logging
	if input.Bus != nil {
		activatedTopic := dnd5eEvents.FlurryStrikeActivatedTopic.On(input.Bus)
		// Ignore error - this is a notification, not critical to the action
		_ = activatedTopic.Publish(ctx, dnd5eEvents.FlurryStrikeActivatedEvent{
			AttackerID:    owner.GetID(),
			TargetID:      input.Target.GetID(),
			ActionID:      f.id,
			UsesRemaining: f.uses,
		})
	}

	// Remove self if no uses remaining
	if f.uses <= 0 && f.bus != nil {
		if err := f.Remove(ctx, f.bus); err != nil {
			return rpgerr.Wrapf(err, "failed to remove flurry strike after use")
		}
	}

	return nil
}

// Apply subscribes to turn end events for automatic cleanup
func (f *FlurryStrike) Apply(ctx context.Context, bus events.EventBus) error {
	if f.bus != nil {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "flurry strike already applied")
	}

	f.bus = bus

	// Subscribe to turn end for cleanup
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(bus)
	subID, err := turnEndTopic.Subscribe(ctx, f.onTurnEnd)
	if err != nil {
		f.bus = nil
		return rpgerr.Wrapf(err, "failed to subscribe to turn end")
	}
	f.subscriptionID = subID

	return nil
}

// Remove unsubscribes from events and marks as removed
func (f *FlurryStrike) Remove(ctx context.Context, bus events.EventBus) error {
	if f.removed {
		return nil // Already removed
	}

	f.removed = true

	if f.subscriptionID != "" {
		if err := bus.Unsubscribe(ctx, f.subscriptionID); err != nil {
			return rpgerr.Wrapf(err, "failed to unsubscribe from turn end")
		}
		f.subscriptionID = ""
	}

	// Publish action removed event so the character can remove it from their list
	removedTopic := dnd5eEvents.ActionRemovedTopic.On(bus)
	err := removedTopic.Publish(ctx, dnd5eEvents.ActionRemovedEvent{
		ActionID: f.id,
		OwnerID:  f.ownerID,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish action removed event")
	}

	return nil
}

// onTurnEnd is called when the turn ends - removes unused strikes
func (f *FlurryStrike) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	// Only remove if this is the owner's turn ending
	if event.CharacterID != f.ownerID {
		return nil
	}

	// Remove self at end of turn
	if !f.removed && f.bus != nil {
		return f.Remove(ctx, f.bus)
	}

	return nil
}

// IsTemporary returns true - flurry strikes are always temporary
func (f *FlurryStrike) IsTemporary() bool {
	return true
}

// UsesRemaining returns the number of uses remaining
func (f *FlurryStrike) UsesRemaining() int {
	return f.uses
}

// ToJSON converts the action to JSON for persistence
func (f *FlurryStrike) ToJSON() (json.RawMessage, error) {
	data := map[string]interface{}{
		"id":       f.id,
		"owner_id": f.ownerID,
		"uses":     f.uses,
		"type":     "flurry_strike",
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal flurry strike: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost (free - uses come from Flurry of Blows bonus action)
func (f *FlurryStrike) ActionType() coreCombat.ActionType {
	return coreCombat.ActionFree
}

// Compile-time check that FlurryStrike implements Action
var _ Action = (*FlurryStrike)(nil)
