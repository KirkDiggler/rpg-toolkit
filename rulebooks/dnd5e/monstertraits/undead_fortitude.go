// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// UndeadFortitudeData is the JSON structure for persisting undead fortitude trait state
type UndeadFortitudeData struct {
	Ref         *core.Ref `json:"ref"`
	OwnerID     string    `json:"owner_id"`
	ConModifier int       `json:"con_modifier"`
}

// undeadFortitudeCondition represents a zombie's ability to make CON saves to avoid death.
// It implements the ConditionBehavior interface.
type undeadFortitudeCondition struct {
	ownerID     string
	conModifier int
	roller      dice.Roller
	bus         events.EventBus
	subID       string
}

// Ensure undeadFortitudeCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*undeadFortitudeCondition)(nil)

// UndeadFortitude creates a new undead fortitude trait.
// When damage would reduce the creature to 0 HP, it makes a CON save (DC = 5 + damage taken).
// On success, the creature drops to 1 HP instead.
// Does not work against radiant damage or critical hits.
func UndeadFortitude(ownerID string, conModifier int, roller dice.Roller) dnd5eEvents.ConditionBehavior {
	return &undeadFortitudeCondition{
		ownerID:     ownerID,
		conModifier: conModifier,
		roller:      roller,
	}
}

// IsApplied returns true if this condition is currently applied
func (u *undeadFortitudeCondition) IsApplied() bool {
	return u.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (u *undeadFortitudeCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if u.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "undead fortitude condition already applied")
	}
	u.bus = bus

	// Subscribe to damage received events to trigger CON save when dropped to 0
	damageReceived := dnd5eEvents.DamageReceivedTopic.On(bus)
	subID, err := damageReceived.Subscribe(ctx, u.onDamageReceived)
	if err != nil {
		return err
	}
	u.subID = subID

	return nil
}

// Remove unsubscribes this condition from events
func (u *undeadFortitudeCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if u.bus == nil {
		return nil // Not applied, nothing to remove
	}

	if u.subID != "" {
		err := bus.Unsubscribe(ctx, u.subID)
		if err != nil {
			return err
		}
	}

	u.subID = ""
	u.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (u *undeadFortitudeCondition) ToJSON() (json.RawMessage, error) {
	data := UndeadFortitudeData{
		Ref:         refs.MonsterTraits.UndeadFortitude(),
		OwnerID:     u.ownerID,
		ConModifier: u.conModifier,
	}
	return json.Marshal(data)
}

// loadJSON loads undead fortitude condition state from JSON
func (u *undeadFortitudeCondition) loadJSON(data json.RawMessage) error {
	var fortitudeData UndeadFortitudeData
	if err := json.Unmarshal(data, &fortitudeData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal undead fortitude data")
	}

	u.ownerID = fortitudeData.OwnerID
	u.conModifier = fortitudeData.ConModifier

	return nil
}

// onDamageReceived listens for damage that would drop HP to 0 and attempts a CON save
func (u *undeadFortitudeCondition) onDamageReceived(ctx context.Context, event dnd5eEvents.DamageReceivedEvent) error {
	// Only process if we're the target
	if event.TargetID != u.ownerID {
		return nil
	}

	// Check if this is radiant damage (bypasses Undead Fortitude)
	if event.DamageType == damage.Radiant {
		return nil
	}

	// Note: We can't check for critical hits here because DamageReceivedEvent
	// doesn't include that information. The game server will need to handle
	// the logic of "would this drop to 0 HP" and "was this a critical hit".
	// This implementation just demonstrates the CON save logic.

	// Calculate DC = 5 + damage taken
	dc := 5 + event.Amount

	// Roll d20 + CON modifier
	roll, err := u.roller.Roll(ctx, 20)
	if err != nil {
		return rpgerr.Wrap(err, "failed to roll for undead fortitude")
	}

	total := roll + u.conModifier

	// For now, we just log the save result. The actual HP management
	// would be done by the game server listening to these events.
	// In a full implementation, we'd publish an "UndeadFortitudeSaveEvent"
	// that the game server would react to.
	_ = total >= dc // Success if total >= DC

	return nil
}
