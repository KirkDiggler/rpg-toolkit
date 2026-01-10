// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// DisengagingConditionData is the serializable form of the disengaging condition.
// This is stored by the game server as an opaque JSON blob.
type DisengagingConditionData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// DisengagingCondition prevents opportunity attacks against the character
// until the end of their current turn. This condition is applied when
// a character uses the Disengage combat ability and automatically
// removes itself when the character's turn ends.
type DisengagingCondition struct {
	CharacterID     string
	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure DisengagingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*DisengagingCondition)(nil)

// NewDisengagingCondition creates a new Disengaging condition for the specified character.
// The condition will prevent opportunity attacks when the character moves and will
// automatically remove itself at the end of the character's turn.
func NewDisengagingCondition(characterID string) *DisengagingCondition {
	return &DisengagingCondition{
		CharacterID: characterID,
	}
}

// IsApplied returns true if this condition is currently applied.
func (d *DisengagingCondition) IsApplied() bool {
	return d.bus != nil
}

// Apply subscribes this condition to MovementChain and TurnEnd events.
// MovementChain subscription adds OA prevention when this character moves.
// TurnEnd subscription removes the condition when the character's turn ends.
func (d *DisengagingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if d.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "disengaging condition already applied")
	}
	d.bus = bus

	// Subscribe to MovementChain to prevent opportunity attacks
	movementChain := dnd5eEvents.MovementChain.On(bus)
	subID1, err := movementChain.SubscribeWithChain(ctx, d.onMovementChain)
	if err != nil {
		d.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to movement chain")
	}
	d.subscriptionIDs = append(d.subscriptionIDs, subID1)

	// Subscribe to TurnEnd to remove condition when turn ends
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(bus)
	subID2, err := turnEndTopic.Subscribe(ctx, d.onTurnEnd)
	if err != nil {
		// Rollback: unsubscribe from previous subscriptions
		_ = d.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to turn end topic")
	}
	d.subscriptionIDs = append(d.subscriptionIDs, subID2)

	return nil
}

// Remove unsubscribes this condition from all events.
func (d *DisengagingCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if d.bus == nil {
		return nil // Not applied, nothing to remove
	}

	for _, subID := range d.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe from event")
		}
	}

	d.subscriptionIDs = nil
	d.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (d *DisengagingCondition) ToJSON() (json.RawMessage, error) {
	data := DisengagingConditionData{
		Ref:         refs.Conditions.Disengaging(),
		CharacterID: d.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads disengaging condition state from JSON.
func (d *DisengagingCondition) loadJSON(data json.RawMessage) error {
	var disengagingData DisengagingConditionData
	if err := json.Unmarshal(data, &disengagingData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal disengaging data")
	}

	d.CharacterID = disengagingData.CharacterID
	return nil
}

// onMovementChain handles movement events to add OA prevention for this character.
// When the moving entity is this character, adds an OA prevention source to the event.
func (d *DisengagingCondition) onMovementChain(
	_ context.Context,
	event *dnd5eEvents.MovementChainEvent,
	c chain.Chain[*dnd5eEvents.MovementChainEvent],
) (chain.Chain[*dnd5eEvents.MovementChainEvent], error) {
	// Only apply to this character's movement
	if event.EntityID != d.CharacterID {
		return c, nil
	}

	// Add OA prevention at the conditions stage
	modifyMovement := func(_ context.Context, e *dnd5eEvents.MovementChainEvent) (*dnd5eEvents.MovementChainEvent, error) {
		e.OAPreventionSources = append(e.OAPreventionSources, dnd5eEvents.MovementModifierSource{
			Name:       "Disengaging",
			SourceType: "condition",
			SourceRef:  refs.Conditions.Disengaging(),
			EntityID:   d.CharacterID,
		})
		return e, nil
	}

	if err := c.Add(combat.StageConditions, "disengaging", modifyMovement); err != nil {
		return c, rpgerr.Wrapf(err, "failed to add disengaging modifier for character %s", d.CharacterID)
	}

	return c, nil
}

// onTurnEnd handles turn end events to remove this condition when the character's turn ends.
func (d *DisengagingCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	// Only remove on this character's turn end
	if event.CharacterID != d.CharacterID {
		return nil
	}

	if d.bus == nil {
		return nil
	}

	// Publish condition removed event
	removals := dnd5eEvents.ConditionRemovedTopic.On(d.bus)
	err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		CharacterID:  d.CharacterID,
		ConditionRef: refs.Conditions.Disengaging().String(),
		Reason:       "turn_end",
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish disengaging removal for character %s", d.CharacterID)
	}

	// Actually remove the condition (unsubscribe from events)
	return d.Remove(ctx, d.bus)
}
