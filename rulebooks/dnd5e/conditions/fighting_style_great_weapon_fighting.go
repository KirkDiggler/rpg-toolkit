// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// gwfDiceRegex matches dice notation like "1d8", "2d6", etc.
var gwfDiceRegex = regexp.MustCompile(`(\d+)[dD](\d+)`)

// FightingStyleGreatWeaponFightingData is the JSON structure for persisting GWF condition state
type FightingStyleGreatWeaponFightingData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
}

// FightingStyleGreatWeaponFightingCondition allows rerolling 1s and 2s on weapon damage dice.
type FightingStyleGreatWeaponFightingCondition struct {
	CharacterID     string
	roller          dice.Roller
	subscriptionIDs []string
	bus             events.EventBus
}

// Ensure FightingStyleGreatWeaponFightingCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleGreatWeaponFightingCondition)(nil)

// NewFightingStyleGreatWeaponFightingCondition creates a new Great Weapon Fighting condition.
func NewFightingStyleGreatWeaponFightingCondition(
	characterID string, roller dice.Roller,
) *FightingStyleGreatWeaponFightingCondition {
	return &FightingStyleGreatWeaponFightingCondition{
		CharacterID: characterID,
		roller:      roller,
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

	for _, subID := range f.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe from event")
		}
	}

	f.subscriptionIDs = nil
	f.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence.
func (f *FightingStyleGreatWeaponFightingCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleGreatWeaponFightingData{
		Ref:         refs.Conditions.FightingStyleGreatWeaponFighting(),
		CharacterID: f.CharacterID,
	}
	return json.Marshal(data)
}

// loadJSON loads GWF condition state from JSON.
func (f *FightingStyleGreatWeaponFightingCondition) loadJSON(data json.RawMessage) error {
	var gwfData FightingStyleGreatWeaponFightingData
	if err := json.Unmarshal(data, &gwfData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal great weapon fighting data")
	}

	f.CharacterID = gwfData.CharacterID
	return nil
}

// onDamageChain rerolls 1s and 2s on weapon damage dice.
func (f *FightingStyleGreatWeaponFightingCondition) onDamageChain(
	_ context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	// Only modify damage for attacks by this character
	if event.AttackerID != f.CharacterID {
		return c, nil
	}

	// Check for weapon component
	if len(event.Components) == 0 {
		return c, nil
	}

	weaponComponent := &event.Components[0]
	if weaponComponent.Source != dnd5eEvents.DamageSourceWeapon {
		return c, nil
	}

	// Add modifier that rerolls at StageFeatures
	modifyDamage := func(modCtx context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		for i := range e.Components {
			component := &e.Components[i]
			if component.Source != dnd5eEvents.DamageSourceWeapon {
				continue
			}

			// Get roller
			roller := f.roller
			if roller == nil {
				roller = dice.NewRoller()
			}

			// Parse die size from weapon damage notation
			dieSize, err := parseGWFWeaponDieSize(e.WeaponDamage)
			if err != nil {
				return e, rpgerr.Wrapf(err, "failed to parse weapon damage: %s", e.WeaponDamage)
			}

			// Reroll 1s and 2s
			newRolls := make([]int, len(component.OriginalDiceRolls))
			copy(newRolls, component.OriginalDiceRolls)

			for idx, roll := range component.OriginalDiceRolls {
				if roll == 1 || roll == 2 {
					newRoll, rollErr := roller.Roll(modCtx, dieSize)
					if rollErr != nil {
						return e, rpgerr.Wrap(rollErr, "failed to reroll die")
					}

					component.Rerolls = append(component.Rerolls, dnd5eEvents.RerollEvent{
						DieIndex: idx,
						Before:   roll,
						After:    newRoll,
						Reason:   "great_weapon_fighting",
					})

					newRolls[idx] = newRoll
				}
			}

			component.FinalDiceRolls = newRolls
		}

		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "great_weapon_fighting", modifyDamage); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply great weapon fighting for character %s", f.CharacterID)
	}

	return c, nil
}

// parseGWFWeaponDieSize extracts the die size from weapon damage notation
func parseGWFWeaponDieSize(notation string) (int, error) {
	matches := gwfDiceRegex.FindStringSubmatch(notation)
	if len(matches) < 3 {
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid weapon damage notation: %s", notation)
	}

	dieSize, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, rpgerr.Wrapf(err, "invalid die size in notation: %s", notation)
	}

	return dieSize, nil
}
