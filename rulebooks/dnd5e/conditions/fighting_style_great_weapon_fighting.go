// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// FightingStyleGreatWeaponFightingData is the JSON structure for persisting GWF condition state
type FightingStyleGreatWeaponFightingData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
}

// FightingStyleGreatWeaponFightingCondition allows rerolling 1s and 2s on weapon damage dice.
type FightingStyleGreatWeaponFightingCondition struct {
	MemberID        string
	roller          dice.Roller
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure FightingStyleGreatWeaponFightingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleGreatWeaponFightingCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (f *FightingStyleGreatWeaponFightingCondition) Ref() *core.Ref {
	return refs.Conditions.FightingStyleGreatWeaponFighting()
}

// NewFightingStyleGreatWeaponFightingCondition creates a new Great Weapon Fighting condition.
func NewFightingStyleGreatWeaponFightingCondition(
	characterID string, roller dice.Roller,
) *FightingStyleGreatWeaponFightingCondition {
	return &FightingStyleGreatWeaponFightingCondition{
		MemberID: characterID,
		roller:   roller,
	}
}

// IsApplied returns true if this condition is currently applied.
func (f *FightingStyleGreatWeaponFightingCondition) IsApplied() bool {
	return f.bus != nil
}

// Apply subscribes this condition to damage chain events.
func (f *FightingStyleGreatWeaponFightingCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "great weapon fighting style already applied")
	}
	f.bus = bus

	// Subscribe to DamageChain to reroll 1s and 2s
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID, err := damageChain.SubscribeWithChain(ctx, f.onDamageChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	f.subscriptionIDs = append(f.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events.
func (f *FightingStyleGreatWeaponFightingCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if f.bus == nil {
		return nil
	}

	total := len(f.subscriptionIDs)
	var errs []error
	for _, subID := range f.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	f.subscriptionIDs = nil
	f.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (f *FightingStyleGreatWeaponFightingCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleGreatWeaponFightingData{
		Ref:      refs.Conditions.FightingStyleGreatWeaponFighting(),
		MemberID: f.MemberID,
	}
	return json.Marshal(data)
}

// loadJSON loads GWF condition state from JSON.
func (f *FightingStyleGreatWeaponFightingCondition) loadJSON(data json.RawMessage) error {
	var gwfData FightingStyleGreatWeaponFightingData
	if err := json.Unmarshal(data, &gwfData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal great weapon fighting data")
	}

	f.MemberID = gwfData.MemberID
	return nil
}

// onDamageChain rerolls 1s and 2s on the marked primary weapon component's
// dice trace. It operates on a staged copy of the current final faces —
// eligibility, the recorded Before, the replacement, and the subtotal delta
// all use the face as it stands NOW, so an earlier rule's rerolls stay ordered
// and valid — and publishes the staged finals, rerolls, and subtotal back to
// component.Roll.Dice only after every required reroll succeeds, so a roller
// failure cannot leak partial mutations. Original faces are never rewritten,
// and every ordered reroll is sourced to this condition's canonical ref and
// display name.
func (f *FightingStyleGreatWeaponFightingCondition) onDamageChain(
	_ context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	// Only modify damage for attacks by this character
	if event.AttackerID != f.MemberID {
		return c, nil
	}

	if primaryWeaponComponentIndex(event) < 0 {
		return c, nil
	}

	// Add modifier that rerolls at StageFeatures
	modifyDamage := func(modCtx context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		componentIndex := primaryWeaponComponentIndex(e)
		if componentIndex < 0 {
			return e, nil
		}
		trace := e.Components[componentIndex].Roll.Dice
		if trace == nil {
			return e, nil
		}

		roller := f.roller
		if roller == nil {
			roller = dice.NewRoller()
		}

		// Stage everything locally. The caller-owned trace is only published
		// back once every required reroll has succeeded — a failure partway
		// through leaves it exactly as the caller built it.
		stagedFinals := slices.Clone(trace.FinalRolls)
		var stagedRerolls []dnd5eEvents.DiceReroll
		stagedSubtotal := trace.Subtotal

		for idx, current := range stagedFinals {
			if current != 1 && current != 2 {
				continue
			}
			newRoll, rollErr := roller.Roll(modCtx, trace.DieSize)
			if rollErr != nil {
				return e, rpgerr.Wrap(rollErr, "failed to reroll die")
			}

			stagedRerolls = append(stagedRerolls, dnd5eEvents.DiceReroll{
				DieIndex: idx,
				Before:   current,
				After:    newRoll,
				Source: dnd5eEvents.RollSource{
					Ref:  refs.Conditions.FightingStyleGreatWeaponFighting(),
					Name: gwfDisplayName,
				},
			})
			stagedFinals[idx] = newRoll

			// A kept die contributes to the subtotal; a dropped one does not.
			if len(trace.KeptIndices) == 0 || slices.Contains(trace.KeptIndices, idx) {
				stagedSubtotal += newRoll - current
			}
		}

		if len(stagedRerolls) > 0 {
			trace.FinalRolls = stagedFinals
			trace.Rerolls = append(trace.Rerolls, stagedRerolls...)
			trace.Subtotal = stagedSubtotal
		}

		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "great_weapon_fighting", modifyDamage); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply great weapon fighting for character %s", f.MemberID)
	}

	return c, nil
}

// gwfDisplayName is the canonical display name this condition sources its
// rerolls with — the same rulebook-owned label the conditions display catalog
// carries for the condition's ref.
const gwfDisplayName = "Great Weapon Fighting"

// primaryWeaponComponentIndex finds the weapon component carrying the exact
// canonical ability marker. Type and position cannot identify the primary
// component when one attack carries multiple same-type weapon pools.
func primaryWeaponComponentIndex(event *dnd5eEvents.DamageChainEvent) int {
	for i := range event.Components {
		component := &event.Components[i]
		if component.Source == dnd5eEvents.DamageSourceWeapon &&
			component.HasProperty(damage.AddsAttackAbilityModifier) {
			return i
		}
	}
	return -1
}
