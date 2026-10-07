// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//nolint:dupl // Immunity and Vulnerability implement same interface with similar structure but different behavior
package monstertraits

import (
	"context"
	"encoding/json"
	"slices"

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

// Ref returns the canonical ref this trait names itself by — the same ref its
// ToJSON embeds and its loader routes on.
func (i *immunityCondition) Ref() *core.Ref { return refs.MonsterTraits.Immunity() }

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

	// Answer on the incoming fold: the immunity is this target's own answer
	// to the damage dealt to it.
	incoming := dnd5eEvents.IncomingDamageChain.On(bus)
	subID, err := incoming.SubscribeWithChain(ctx, i.onIncomingDamage)
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

// onIncomingDamage answers with a immunity multiplier when the damage dealt
// to this trait's owner includes its damage type: nothing of the type gets through.
func (i *immunityCondition) onIncomingDamage(
	_ context.Context,
	event *dnd5eEvents.IncomingDamageEvent,
	c chain.Chain[*dnd5eEvents.IncomingDamageEvent],
) (chain.Chain[*dnd5eEvents.IncomingDamageEvent], error) {
	if event.TargetID() != i.ownerID || !slices.Contains(event.DealtTypes(), i.damageType) {
		return c, nil
	}

	answer := func(_ context.Context, e *dnd5eEvents.IncomingDamageEvent) (*dnd5eEvents.IncomingDamageEvent, error) {
		e.Multipliers = append(e.Multipliers, dnd5eEvents.DamageMultiplier{
			Category: dnd5eEvents.DamageSourceMonsterTrait,
			Source: dnd5eEvents.RollSource{
				Ref:  refs.MonsterTraits.Immunity(),
				Name: "Immunity",
			},
			DamageType: i.damageType,
			Factor:     dnd5eEvents.DamageFactorImmunity,
		})
		return e, nil
	}

	// Keyed by damage type: one owner may hold this trait for several types,
	// and one hit may deal more than one of them.
	if err := c.Add(combat.StageFinal, "immunity:"+string(i.damageType), answer); err != nil {
		return c, rpgerr.Wrapf(err, "error applying immunity for owner %s", i.ownerID)
	}

	return c, nil
}
