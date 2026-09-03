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

// UnconsciousData is the legacy JSON structure for persisted unconscious
// condition state. The counters remain loadable for migration compatibility,
// but Character.Data.DeathSaveState is authoritative.
type UnconsciousData struct {
	Ref        *core.Ref `json:"ref"`
	MemberID   string    `json:"member_id"`
	Successes  int       `json:"successes"`
	Failures   int       `json:"failures"`
	Stabilized bool      `json:"stabilized"`
	Dead       bool      `json:"dead"`
}

// UnconsciousCondition is a compatibility shell for legacy persisted blobs.
// It deliberately owns no death-save behavior: its turn, damage, and healing
// handlers are inert and therefore cannot maintain a second progress ledger
// beside Character.Data.DeathSaveState. Long-rest removal remains so an old
// condition can age out normally after loading.
type UnconsciousCondition struct {
	CharacterID string

	// Roller remains for Go source compatibility with the historical
	// constructor and struct surface. It is never invoked.
	Roller dice.Roller

	deathSaveState  *saves.DeathSaveState
	subscriptionIDs []string
	bus             events.EventBus
}

var _ dnd5eEvents.ConditionBehavior = (*UnconsciousCondition)(nil)

// Ref returns the canonical ref embedded by the legacy persistence format.
func (c *UnconsciousCondition) Ref() *core.Ref { return refs.Conditions.Unconscious() }

// NewUnconsciousCondition constructs the compatibility condition. Death-save
// execution belongs to character.MakeDeathSave rather than this condition.
func NewUnconsciousCondition(characterID string, roller dice.Roller) *UnconsciousCondition {
	return &UnconsciousCondition{
		CharacterID: characterID,
		Roller:      roller,
	}
}

// IsApplied reports whether the compatibility shell is attached to a bus.
func (c *UnconsciousCondition) IsApplied() bool { return c.bus != nil }

// Apply keeps the legacy subscription shape for attach/rollback compatibility,
// but all three death-save handlers are inert. Only long-rest cleanup can
// mutate the condition.
func (c *UnconsciousCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if c.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "unconscious condition already applied")
	}
	if c.deathSaveState == nil {
		c.deathSaveState = &saves.DeathSaveState{}
	}

	c.bus = bus

	turnSubID, err := dnd5eEvents.TurnStartTopic.On(bus).Subscribe(ctx, c.onTurnStart)
	if err != nil {
		c.bus = nil
		return err
	}
	c.subscriptionIDs = append(c.subscriptionIDs, turnSubID)

	damageSubID, err := dnd5eEvents.DamageReceivedTopic.On(bus).Subscribe(ctx, c.onDamageReceived)
	if err != nil {
		_ = c.Remove(ctx, bus)
		return err
	}
	c.subscriptionIDs = append(c.subscriptionIDs, damageSubID)

	healingSubID, err := dnd5eEvents.HealingReceivedTopic.On(bus).Subscribe(ctx, c.onHealingReceived)
	if err != nil {
		_ = c.Remove(ctx, bus)
		return err
	}
	c.subscriptionIDs = append(c.subscriptionIDs, healingSubID)

	restSubID, err := subscribeRemoveOnLongRest(ctx, bus, c.CharacterID, c.Ref(), c.Remove)
	if err != nil {
		_ = c.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to long rest")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, restSubID)
	return nil
}

func (c *UnconsciousCondition) onTurnStart(context.Context, dnd5eEvents.TurnStartEvent) error {
	return nil
}

func (c *UnconsciousCondition) onDamageReceived(context.Context, dnd5eEvents.DamageReceivedEvent) error {
	return nil
}

func (c *UnconsciousCondition) onHealingReceived(context.Context, dnd5eEvents.HealingReceivedEvent) error {
	return nil
}

// Remove unsubscribes the compatibility shell.
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

// ToJSON retains the legacy schema so loading and writing an old blob is
// lossless until its normal removal.
func (c *UnconsciousCondition) ToJSON() (json.RawMessage, error) {
	data := UnconsciousData{
		Ref:      refs.Conditions.Unconscious(),
		MemberID: c.CharacterID,
	}
	if c.deathSaveState != nil {
		data.Successes = c.deathSaveState.Successes
		data.Failures = c.deathSaveState.Failures
		data.Stabilized = c.deathSaveState.Stabilized
		data.Dead = c.deathSaveState.Dead
	}
	return json.Marshal(data)
}

// loadJSON restores the legacy blob without promoting its counters to an
// active rules ledger.
func (c *UnconsciousCondition) loadJSON(data json.RawMessage) error {
	var ud UnconsciousData
	if err := json.Unmarshal(data, &ud); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal unconscious data")
	}

	c.CharacterID = ud.MemberID
	c.deathSaveState = &saves.DeathSaveState{
		Successes:  ud.Successes,
		Failures:   ud.Failures,
		Stabilized: ud.Stabilized,
		Dead:       ud.Dead,
	}
	return nil
}
