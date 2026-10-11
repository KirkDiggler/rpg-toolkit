// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

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

// Ensure undeadFortitudeCondition can receive a runtime roller at attach, as
// Sneak Attack does.
var _ RollerBinder = (*undeadFortitudeCondition)(nil)

// BindRoller binds the roller this trait makes its Constitution save with, so a
// trait restored from persisted JSON — whose loader has no roller to give it —
// rolls the interaction's dice. A nil roller leaves the current one alone.
func (u *undeadFortitudeCondition) BindRoller(roller dice.Roller) {
	if roller == nil {
		return
	}
	u.roller = roller
}

// Ref returns the canonical ref this trait names itself by — the same ref its
// ToJSON embeds and its loader routes on.
func (u *undeadFortitudeCondition) Ref() *core.Ref { return refs.MonsterTraits.UndeadFortitude() }

// UndeadFortitude creates a new undead fortitude trait. A trait built in
// process is built with its roller; a loaded one is bound at attach through
// [RollerBinder].
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

	// A trait nobody bound a roller to cannot save. Rolling anyway would mean a
	// process-global default the interaction's roll trace never sees.
	if u.roller == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "undead fortitude has no roller bound")
	}

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

// undeadFortitudeDescriptor is the one entry for [undeadFortitudeCondition]; see [descriptors].
var undeadFortitudeDescriptor = descriptor{
	ref:           refs.MonsterTraits.UndeadFortitude(),
	display:       Display{Name: "Undead Fortitude"},
	load:          loaderOf[undeadFortitudeCondition](),
	ownAction:     notBearing,
	againstHolder: notBearing,
}
