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
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// UnconsciousData is the JSON structure for persisting unconscious condition state
type UnconsciousData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
	Successes   int       `json:"successes"`
	Failures    int       `json:"failures"`
	Stabilized  bool      `json:"stabilized"`
	Dead        bool      `json:"dead"`
}

// UnconsciousCondition represents an unconscious character making death saves.
// It subscribes to turn start, damage, and healing events to automate
// death saving throws per D&D 5e rules.
type UnconsciousCondition struct {
	CharacterID     string
	Roller          dice.Roller
	deathSaveState  *saves.DeathSaveState
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure UnconsciousCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*UnconsciousCondition)(nil)

// IsApplied returns true if this condition is currently applied
func (c *UnconsciousCondition) IsApplied() bool {
	return c.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (c *UnconsciousCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if c.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "unconscious condition already applied")
	}

	if c.deathSaveState == nil {
		c.deathSaveState = &saves.DeathSaveState{}
	}

	c.bus = bus

	// Subscribe to turn start events to auto-roll death saves
	turnStarts := dnd5eEvents.TurnStartTopic.On(bus)
	subID1, err := turnStarts.Subscribe(ctx, c.onTurnStart)
	if err != nil {
		c.bus = nil
		return err
	}
	c.subscriptionIDs = append(c.subscriptionIDs, subID1)

	// Subscribe to damage events to add automatic failures
	damages := dnd5eEvents.DamageReceivedTopic.On(bus)
	subID2, err := damages.Subscribe(ctx, c.onDamageReceived)
	if err != nil {
		_ = c.Remove(ctx, bus)
		return err
	}
	c.subscriptionIDs = append(c.subscriptionIDs, subID2)

	// Subscribe to healing events to wake up
	healings := dnd5eEvents.HealingReceivedTopic.On(bus)
	subID3, err := healings.Subscribe(ctx, c.onHealingReceived)
	if err != nil {
		_ = c.Remove(ctx, bus)
		return err
	}
	c.subscriptionIDs = append(c.subscriptionIDs, subID3)

	return nil
}

// Remove unsubscribes this condition from events using error-collection pattern
func (c *UnconsciousCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if c.bus == nil {
		return nil
	}

	var errs []error
	for _, subID := range c.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, err)
		}
	}

	c.subscriptionIDs = nil
	c.bus = nil

	if len(errs) > 0 {
		return rpgerr.Wrapf(errs[0], "failed to unsubscribe %d handlers", len(errs))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (c *UnconsciousCondition) ToJSON() (json.RawMessage, error) {
	data := UnconsciousData{
		Ref:         refs.Conditions.Unconscious(),
		CharacterID: c.CharacterID,
	}
	if c.deathSaveState != nil {
		data.Successes = c.deathSaveState.Successes
		data.Failures = c.deathSaveState.Failures
		data.Stabilized = c.deathSaveState.Stabilized
		data.Dead = c.deathSaveState.Dead
	}
	return json.Marshal(data)
}

// loadJSON loads unconscious condition state from JSON
func (c *UnconsciousCondition) loadJSON(data json.RawMessage) error {
	var ud UnconsciousData
	if err := json.Unmarshal(data, &ud); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal unconscious data")
	}

	c.CharacterID = ud.CharacterID
	c.deathSaveState = &saves.DeathSaveState{
		Successes:  ud.Successes,
		Failures:   ud.Failures,
		Stabilized: ud.Stabilized,
		Dead:       ud.Dead,
	}

	return nil
}

// onTurnStart handles turn start events to auto-roll death saves
func (c *UnconsciousCondition) onTurnStart(ctx context.Context, event dnd5eEvents.TurnStartEvent) error {
	if event.CharacterID != c.CharacterID {
		return nil
	}
	if c.deathSaveState.Stabilized || c.deathSaveState.Dead {
		return nil
	}

	result, err := saves.MakeDeathSave(ctx, &saves.DeathSaveInput{
		Roller: c.Roller,
		State:  c.deathSaveState,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to make death save for character %s", c.CharacterID)
	}

	// Update internal state from result
	c.deathSaveState = result.State

	// Publish death save rolled event
	rolledEvent := dnd5eEvents.DeathSaveRolledEvent{
		CharacterID:           c.CharacterID,
		Roll:                  result.Roll,
		IsSuccess:             result.Roll >= 10,
		IsCriticalFail:        result.IsCriticalFail,
		IsCriticalSuccess:     result.IsCriticalSuccess,
		Successes:             result.State.Successes,
		Failures:              result.State.Failures,
		Stabilized:            result.State.Stabilized,
		Dead:                  result.State.Dead,
		RegainedConsciousness: result.RegainedConsciousness,
		HPRestored:            result.HPRestored,
	}

	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(c.bus)
	if err := rolledTopic.Publish(ctx, rolledEvent); err != nil {
		return rpgerr.Wrapf(err, "failed to publish death save rolled event for character %s", c.CharacterID)
	}

	// Handle outcomes
	if result.State.Dead {
		diedTopic := dnd5eEvents.CharacterDiedTopic.On(c.bus)
		if err := diedTopic.Publish(ctx, dnd5eEvents.CharacterDiedEvent{
			CharacterID: c.CharacterID,
		}); err != nil {
			return rpgerr.Wrapf(err, "failed to publish character died event for character %s", c.CharacterID)
		}
		return nil
	}

	if result.State.Stabilized {
		stabilizedTopic := dnd5eEvents.CharacterStabilizedTopic.On(c.bus)
		if err := stabilizedTopic.Publish(ctx, dnd5eEvents.CharacterStabilizedEvent{
			CharacterID: c.CharacterID,
		}); err != nil {
			return rpgerr.Wrapf(err, "failed to publish character stabilized event for character %s", c.CharacterID)
		}
		return nil
	}

	if result.RegainedConsciousness {
		// Capture bus before Remove nils it
		bus := c.bus

		// Remove self first (unsubscribe from events before publishing healing
		// to avoid re-triggering onHealingReceived)
		if err := c.Remove(ctx, bus); err != nil {
			return rpgerr.Wrapf(err, "failed to remove unconscious condition for character %s", c.CharacterID)
		}

		// Publish condition removal
		removals := dnd5eEvents.ConditionRemovedTopic.On(bus)
		if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
			CharacterID:  c.CharacterID,
			ConditionRef: refs.Conditions.Unconscious().String(),
			Reason:       "nat_20",
		}); err != nil {
			return rpgerr.Wrapf(err, "failed to publish condition removed event for character %s", c.CharacterID)
		}

		// Publish healing event for 1 HP
		healingTopic := dnd5eEvents.HealingReceivedTopic.On(bus)
		if err := healingTopic.Publish(ctx, dnd5eEvents.HealingReceivedEvent{
			TargetID: c.CharacterID,
			Amount:   1,
			Source:   "death_save_nat_20",
		}); err != nil {
			return rpgerr.Wrapf(err, "failed to publish healing event for character %s", c.CharacterID)
		}

		return nil
	}

	return nil
}

// onDamageReceived handles damage events to add automatic death save failures
func (c *UnconsciousCondition) onDamageReceived(ctx context.Context, event dnd5eEvents.DamageReceivedEvent) error {
	if event.TargetID != c.CharacterID {
		return nil
	}
	if c.deathSaveState.Dead {
		return nil
	}

	// Per 5e RAW, a stabilized-but-unconscious creature taking damage
	// loses stabilization and gains a death save failure
	if c.deathSaveState.Stabilized {
		c.deathSaveState.Stabilized = false
	}

	result, err := saves.TakeDamageWhileUnconscious(ctx, &saves.DamageWhileUnconsciousInput{
		State:      c.deathSaveState,
		IsCritical: event.IsCritical,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to process damage while unconscious for character %s", c.CharacterID)
	}

	// Update internal state
	c.deathSaveState = result.State

	// Publish death save rolled event (roll=0 indicates damage, not a roll)
	rolledEvent := dnd5eEvents.DeathSaveRolledEvent{
		CharacterID: c.CharacterID,
		Roll:        0,
		Successes:   result.State.Successes,
		Failures:    result.State.Failures,
		Dead:        result.State.Dead,
	}

	rolledTopic := dnd5eEvents.DeathSaveRolledTopic.On(c.bus)
	if err := rolledTopic.Publish(ctx, rolledEvent); err != nil {
		return rpgerr.Wrapf(err, "failed to publish death save rolled event for character %s", c.CharacterID)
	}

	if result.State.Dead {
		diedTopic := dnd5eEvents.CharacterDiedTopic.On(c.bus)
		if err := diedTopic.Publish(ctx, dnd5eEvents.CharacterDiedEvent{
			CharacterID: c.CharacterID,
		}); err != nil {
			return rpgerr.Wrapf(err, "failed to publish character died event for character %s", c.CharacterID)
		}
	}

	return nil
}

// onHealingReceived handles healing events to wake up unconscious characters
func (c *UnconsciousCondition) onHealingReceived(ctx context.Context, event dnd5eEvents.HealingReceivedEvent) error {
	if event.TargetID != c.CharacterID {
		return nil
	}
	if c.deathSaveState.Dead {
		return nil
	}

	// Reset death save state
	c.deathSaveState.Successes = 0
	c.deathSaveState.Failures = 0
	c.deathSaveState.Stabilized = false

	// Capture bus before Remove nils it
	bus := c.bus

	// Remove self first
	if err := c.Remove(ctx, bus); err != nil {
		return rpgerr.Wrapf(err, "failed to remove unconscious condition for character %s", c.CharacterID)
	}

	// Publish condition removal
	removals := dnd5eEvents.ConditionRemovedTopic.On(bus)
	if err := removals.Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		CharacterID:  c.CharacterID,
		ConditionRef: refs.Conditions.Unconscious().String(),
		Reason:       "healed",
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish condition removed event for character %s", c.CharacterID)
	}

	return nil
}
