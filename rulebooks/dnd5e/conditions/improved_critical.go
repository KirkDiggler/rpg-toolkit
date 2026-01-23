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

// ImprovedCriticalData is the JSON structure for persisting improved critical condition state
type ImprovedCriticalData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
	Threshold   int       `json:"threshold"` // Critical threshold (19 for Champion level 3)
}

// ImprovedCriticalCondition represents the Champion's Improved Critical feature.
// Your weapon attacks score a critical hit on a roll of 19 or 20.
type ImprovedCriticalCondition struct {
	CharacterID     string
	Threshold       int // Critical threshold (19 for Champion level 3, 18 for Superior Critical)
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure ImprovedCriticalCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*ImprovedCriticalCondition)(nil)

// IsApplied returns true if this condition is currently applied
func (ic *ImprovedCriticalCondition) IsApplied() bool {
	return ic.bus != nil
}

// Apply subscribes this condition to attack chain events
func (ic *ImprovedCriticalCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if ic.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "improved critical condition already applied")
	}
	ic.bus = bus

	// Subscribe to AttackChain to modify critical threshold
	attackChain := dnd5eEvents.AttackChain.On(bus)
	subID, err := attackChain.SubscribeWithChain(ctx, ic.onAttackChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	ic.subscriptionIDs = append(ic.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events
func (ic *ImprovedCriticalCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if ic.bus == nil {
		return nil // Not applied, nothing to remove
	}

	for _, subID := range ic.subscriptionIDs {
		err := bus.Unsubscribe(ctx, subID)
		if err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe from event")
		}
	}

	ic.subscriptionIDs = nil
	ic.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (ic *ImprovedCriticalCondition) ToJSON() (json.RawMessage, error) {
	data := ImprovedCriticalData{
		Ref:         refs.Conditions.ImprovedCritical(),
		CharacterID: ic.CharacterID,
		Threshold:   ic.Threshold,
	}
	return json.Marshal(data)
}

// loadJSON loads improved critical condition state from JSON
//
//nolint:unused // Used by loader.go
func (ic *ImprovedCriticalCondition) loadJSON(data json.RawMessage) error {
	var icData ImprovedCriticalData
	if err := json.Unmarshal(data, &icData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal improved critical data")
	}

	ic.CharacterID = icData.CharacterID
	ic.Threshold = icData.Threshold

	// Default to 19 if not specified
	if ic.Threshold == 0 {
		ic.Threshold = 19
	}

	return nil
}

// onAttackChain modifies the critical threshold for attacks by this character
func (ic *ImprovedCriticalCondition) onAttackChain(
	_ context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	// Only modify attacks by this character
	if event.AttackerID != ic.CharacterID {
		return c, nil
	}

	// Modify critical threshold at StageFeatures
	modifyThreshold := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		// Only lower the threshold, never raise it
		if ic.Threshold < e.CriticalThreshold {
			e.CriticalThreshold = ic.Threshold
		}
		return e, nil
	}

	err := c.Add(combat.StageFeatures, "improved_critical", modifyThreshold)
	if err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply improved critical for character %s", ic.CharacterID)
	}

	return c, nil
}

// ImprovedCriticalInput provides configuration for creating an improved critical condition
type ImprovedCriticalInput struct {
	CharacterID string
	Threshold   int // Critical threshold (default 19)
}

// NewImprovedCriticalCondition creates a new improved critical condition
func NewImprovedCriticalCondition(input ImprovedCriticalInput) *ImprovedCriticalCondition {
	threshold := input.Threshold
	if threshold == 0 {
		threshold = 19 // Default for Champion level 3
	}

	return &ImprovedCriticalCondition{
		CharacterID: input.CharacterID,
		Threshold:   threshold,
	}
}
