// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//nolint:dupl // Immunity and Vulnerability implement same interface with similar structure but different behavior
package monstertraits

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ImmunityData is the JSON structure for persisting immunity trait state
type ImmunityData struct {
	Ref        *core.Ref   `json:"ref"`
	OwnerID    string      `json:"owner_id"`
	DamageType damage.Type `json:"damage_type"`
}

// immunityCondition represents a monster's immunity to a specific damage type.
// It implements the ConditionBehavior interface.
type immunityCondition struct {
	ownerID    string
	damageType damage.Type
	bus        events.EventBus
	subID      string
}

// Ensure immunityCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*immunityCondition)(nil)

// Immunity creates a new immunity trait that reduces damage of the specified type to 0
func Immunity(ownerID string, damageType damage.Type) dnd5eEvents.ConditionBehavior {
	return &immunityCondition{
		ownerID:    ownerID,
		damageType: damageType,
	}
}

// ImmunityJSON creates the JSON representation of an immunity trait.
// This is used by factory functions to add trait data before a bus is available.
func ImmunityJSON(ownerID string, damageType damage.Type) (json.RawMessage, error) {
	data := ImmunityData{
		Ref:        refs.MonsterTraits.Immunity(),
		OwnerID:    ownerID,
		DamageType: damageType,
	}
	return json.Marshal(data)
}

// MustImmunityJSON creates the JSON representation of an immunity trait.
// It panics if JSON marshaling fails (which should never happen with valid inputs).
// Use this in factory functions where errors indicate programming bugs, not runtime issues.
func MustImmunityJSON(ownerID string, damageType damage.Type) json.RawMessage {
	data, err := ImmunityJSON(ownerID, damageType)
	if err != nil {
		panic("monstertraits: failed to marshal immunity JSON: " + err.Error())
	}
	return data
}

// IsApplied returns true if this condition is currently applied
func (i *immunityCondition) IsApplied() bool {
	return i.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (i *immunityCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if i.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "immunity condition already applied")
	}
	i.bus = bus

	// Subscribe to damage chain to reduce immune damage to 0
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID, err := damageChain.SubscribeWithChain(ctx, i.onDamageChain)
	if err != nil {
		return err
	}
	i.subID = subID

	return nil
}

// Remove unsubscribes this condition from events
func (i *immunityCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if i.bus == nil {
		return nil // Not applied, nothing to remove
	}

	if i.subID != "" {
		err := bus.Unsubscribe(ctx, i.subID)
		if err != nil {
			return err
		}
	}

	i.subID = ""
	i.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (i *immunityCondition) ToJSON() (json.RawMessage, error) {
	data := ImmunityData{
		Ref:        refs.MonsterTraits.Immunity(),
		OwnerID:    i.ownerID,
		DamageType: i.damageType,
	}
	return json.Marshal(data)
}

// loadJSON loads immunity condition state from JSON
func (i *immunityCondition) loadJSON(data json.RawMessage) error {
	var immunityData ImmunityData
	if err := json.Unmarshal(data, &immunityData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal immunity data")
	}

	i.ownerID = immunityData.OwnerID
	i.damageType = immunityData.DamageType

	return nil
}

// onDamageChain reduces damage to 0 if the damage type matches the immunity
func (i *immunityCondition) onDamageChain(
	_ context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	// Only process if we're the target
	if event.TargetID != i.ownerID {
		return c, nil
	}

	// Modify damage components to reduce immune damage to 0
	modifyDamage := func(_ context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		for idx := range e.Components {
			// If this component matches our immune damage type, reduce to 0
			if e.Components[idx].DamageType == i.damageType {
				// Zero out all dice rolls
				for j := range e.Components[idx].FinalDiceRolls {
					e.Components[idx].FinalDiceRolls[j] = 0
				}
				// Zero out flat bonus
				e.Components[idx].FlatBonus = 0
			}
		}
		return e, nil
	}

	// Add to chain - process in final stage (for resistance/vulnerability/immunity)
	err := c.Add(combat.StageFinal, "immunity", modifyDamage)
	if err != nil {
		return c, rpgerr.Wrapf(err, "error applying immunity for owner %s", i.ownerID)
	}

	return c, nil
}
