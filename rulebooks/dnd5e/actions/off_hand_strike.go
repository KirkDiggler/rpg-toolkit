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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// OffHandStrike represents an off-hand attack granted by two-weapon fighting.
// It is a temporary action that removes itself when ActionEconomy.OffHandAttacksRemaining
// reaches 0 or at turn end.
// Unlike normal attacks, off-hand strikes don't add ability modifier to damage
// (unless the character has the Two-Weapon Fighting fighting style).
type OffHandStrike struct {
	id             string
	ownerID        string
	weaponID       weapons.WeaponID // The off-hand weapon to attack with
	bus            events.EventBus
	subscriptionID string
	removed        bool
}

// OffHandStrikeConfig contains configuration for creating an OffHandStrike action
type OffHandStrikeConfig struct {
	ID       string
	OwnerID  string
	WeaponID weapons.WeaponID // The off-hand weapon
}

// NewOffHandStrike creates a new OffHandStrike action.
// Capacity is tracked via ActionEconomy.OffHandAttacksRemaining, not internally.
func NewOffHandStrike(config OffHandStrikeConfig) *OffHandStrike {
	return &OffHandStrike{
		id:       config.ID,
		ownerID:  config.OwnerID,
		weaponID: config.WeaponID,
	}
}

// GetID implements core.Entity
func (o *OffHandStrike) GetID() string {
	return o.id
}

// GetType implements core.Entity
func (o *OffHandStrike) GetType() core.EntityType {
	return EntityTypeAction
}

// GetWeaponID returns the weapon ID for this off-hand strike
func (o *OffHandStrike) GetWeaponID() weapons.WeaponID {
	return o.weaponID
}

// CanActivate implements core.Action[ActionInput]
func (o *OffHandStrike) CanActivate(_ context.Context, _ core.Entity, input ActionInput) error {
	if o.removed {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "off-hand strike has been removed")
	}

	if input.ActionEconomy == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action economy required")
	}

	if !input.ActionEconomy.CanUseOffHandAttack() {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no off-hand attacks remaining")
	}

	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "off-hand strike requires a target")
	}

	return nil
}

// Activate implements core.Action[ActionInput]
func (o *OffHandStrike) Activate(ctx context.Context, owner core.Entity, input ActionInput) error {
	if err := o.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Consume from ActionEconomy
	if err := input.ActionEconomy.UseOffHandAttack(); err != nil {
		return rpgerr.Wrapf(err, "failed to use off-hand attack")
	}

	// Publish the strike request event for the game server to resolve
	if input.Bus != nil {
		topic := dnd5eEvents.OffHandStrikeRequestedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.OffHandStrikeRequestedEvent{
			AttackerID: owner.GetID(),
			TargetID:   input.Target.GetID(),
			WeaponID:   string(o.weaponID),
			ActionID:   o.id,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish off-hand strike event")
		}
	}

	// Publish notification event for UI/logging
	if input.Bus != nil {
		activatedTopic := dnd5eEvents.OffHandStrikeActivatedTopic.On(input.Bus)
		// Ignore error - this is a notification, not critical to the action
		_ = activatedTopic.Publish(ctx, dnd5eEvents.OffHandStrikeActivatedEvent{
			AttackerID:    owner.GetID(),
			TargetID:      input.Target.GetID(),
			WeaponID:      string(o.weaponID),
			ActionID:      o.id,
			UsesRemaining: input.ActionEconomy.OffHandAttacksRemaining,
		})
	}

	// Remove self if no off-hand attacks remaining
	if input.ActionEconomy.OffHandAttacksRemaining <= 0 && o.bus != nil {
		if err := o.Remove(ctx, o.bus); err != nil {
			return rpgerr.Wrapf(err, "failed to remove off-hand strike after use")
		}
	}

	return nil
}

// Apply subscribes to turn end events for automatic cleanup
func (o *OffHandStrike) Apply(ctx context.Context, bus events.EventBus) error {
	if o.bus != nil {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "off-hand strike already applied")
	}

	o.bus = bus

	// Subscribe to turn end for cleanup
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(bus)
	subID, err := turnEndTopic.Subscribe(ctx, o.onTurnEnd)
	if err != nil {
		o.bus = nil
		return rpgerr.Wrapf(err, "failed to subscribe to turn end")
	}
	o.subscriptionID = subID

	return nil
}

// Remove unsubscribes from events and marks as removed
func (o *OffHandStrike) Remove(ctx context.Context, bus events.EventBus) error {
	if o.removed {
		return nil // Already removed
	}

	o.removed = true

	if o.subscriptionID != "" {
		if err := bus.Unsubscribe(ctx, o.subscriptionID); err != nil {
			return rpgerr.Wrapf(err, "failed to unsubscribe from turn end")
		}
		o.subscriptionID = ""
	}

	// Publish action removed event so the character can remove it from their list
	removedTopic := dnd5eEvents.ActionRemovedTopic.On(bus)
	err := removedTopic.Publish(ctx, dnd5eEvents.ActionRemovedEvent{
		ActionID: o.id,
		OwnerID:  o.ownerID,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish action removed event")
	}

	return nil
}

// onTurnEnd is called when the turn ends - removes unused strikes
func (o *OffHandStrike) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	// Only remove if this is the owner's turn ending
	if event.CharacterID != o.ownerID {
		return nil
	}

	// Remove self at end of turn
	if !o.removed && o.bus != nil {
		return o.Remove(ctx, o.bus)
	}

	return nil
}

// IsTemporary returns true - off-hand strikes are always temporary
func (o *OffHandStrike) IsTemporary() bool {
	return true
}

// UsesRemaining returns UnlimitedUses since capacity is tracked via ActionEconomy.
// The actual remaining count is in ActionEconomy.OffHandAttacksRemaining.
func (o *OffHandStrike) UsesRemaining() int {
	return UnlimitedUses
}

// ToJSON converts the action to JSON for persistence.
// Note: Uses are not serialized as capacity is tracked via ActionEconomy.
func (o *OffHandStrike) ToJSON() (json.RawMessage, error) {
	data := map[string]interface{}{
		"id":        o.id,
		"owner_id":  o.ownerID,
		"weapon_id": o.weaponID,
		"type":      "off_hand_strike",
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal off-hand strike: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost (bonus action for off-hand attacks)
func (o *OffHandStrike) ActionType() coreCombat.ActionType {
	return coreCombat.ActionBonus
}

// Compile-time check that OffHandStrike implements Action
var _ Action = (*OffHandStrike)(nil)
