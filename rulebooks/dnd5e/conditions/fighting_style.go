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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
)

// FightingStyleData is the JSON structure for persisting fighting style condition state
type FightingStyleData struct {
	Ref         core.Ref                     `json:"ref"`
	Name        string                       `json:"name"`
	CharacterID string                       `json:"character_id"`
	Style       fightingstyles.FightingStyle `json:"style"`
}

// FightingStyleCondition represents a passive fighting style that modifies combat mechanics.
// Unlike Rage, fighting styles are always active and don't need activation.
type FightingStyleCondition struct {
	CharacterID     string
	Style           fightingstyles.FightingStyle
	subscriptionIDs []string
	bus             events.EventBus
	roller          dice.Roller

	// Track current attack weapon for Archery
	currentAttackIsMelee bool
}

// Ensure FightingStyleCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*FightingStyleCondition)(nil)

// diceRegex matches dice notation like "1d8", "2d6", etc.
var diceRegex = regexp.MustCompile(`(\d+)[dD](\d+)`)

// IsApplied returns true if this condition is currently applied
func (f *FightingStyleCondition) IsApplied() bool {
	return f.bus != nil
}

// Apply subscribes this condition to relevant combat events based on the fighting style
func (f *FightingStyleCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if f.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "fighting style condition already applied")
	}
	f.bus = bus

	switch f.Style {
	case fightingstyles.Archery:
		// Subscribe to AttackEvent to track weapon type
		attackTopic := dnd5eEvents.AttackTopic.On(bus)
		subID1, err := attackTopic.Subscribe(ctx, f.onAttackEvent)
		if err != nil {
			return rpgerr.Wrap(err, "failed to subscribe to attack events")
		}
		f.subscriptionIDs = append(f.subscriptionIDs, subID1)

		// Subscribe to AttackChain to add +2 bonus for ranged attacks
		attackChain := combat.AttackChain.On(bus)
		subID2, err := attackChain.SubscribeWithChain(ctx, f.onAttackChain)
		if err != nil {
			// Rollback
			_ = f.Remove(ctx, bus)
			return rpgerr.Wrap(err, "failed to subscribe to attack chain")
		}
		f.subscriptionIDs = append(f.subscriptionIDs, subID2)

	case fightingstyles.GreatWeaponFighting:
		// Subscribe to DamageChain to reroll 1s and 2s
		damageChain := combat.DamageChain.On(bus)
		subID, err := damageChain.SubscribeWithChain(ctx, f.onDamageChain)
		if err != nil {
			return rpgerr.Wrap(err, "failed to subscribe to damage chain")
		}
		f.subscriptionIDs = append(f.subscriptionIDs, subID)

	default:
		// Other fighting styles not yet implemented
		return rpgerr.Newf(rpgerr.CodeNotAllowed, "fighting style %s not yet implemented", f.Style)
	}

	return nil
}

// Remove unsubscribes this condition from events
func (f *FightingStyleCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if f.bus == nil {
		return nil // Not applied, nothing to remove
	}

	for _, subID := range f.subscriptionIDs {
		err := bus.Unsubscribe(ctx, subID)
		if err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe from event")
		}
	}

	f.subscriptionIDs = nil
	f.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (f *FightingStyleCondition) ToJSON() (json.RawMessage, error) {
	data := FightingStyleData{
		Ref: core.Ref{
			Module: "dnd5e",
			Type:   "conditions",
			Value:  "fighting_style",
		},
		Name:        fightingstyles.Name(f.Style),
		CharacterID: f.CharacterID,
		Style:       f.Style,
	}
	return json.Marshal(data)
}

// loadJSON loads fighting style condition state from JSON
//
//nolint:unused // Used by loader.go
func (f *FightingStyleCondition) loadJSON(data json.RawMessage) error {
	var fsData FightingStyleData
	if err := json.Unmarshal(data, &fsData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal fighting style data")
	}

	f.CharacterID = fsData.CharacterID
	f.Style = fsData.Style

	return nil
}

// onAttackEvent tracks whether the current attack is melee or ranged (for Archery)
func (f *FightingStyleCondition) onAttackEvent(_ context.Context, event dnd5eEvents.AttackEvent) error {
	// Only track attacks by this character
	if event.AttackerID != f.CharacterID {
		return nil
	}

	// Track whether this attack is melee or ranged
	f.currentAttackIsMelee = event.IsMelee
	return nil
}

// onAttackChain adds +2 to attack rolls for ranged weapons (Archery fighting style)
func (f *FightingStyleCondition) onAttackChain(
	_ context.Context,
	event combat.AttackChainEvent,
	c chain.Chain[combat.AttackChainEvent],
) (chain.Chain[combat.AttackChainEvent], error) {
	// Only modify attacks by this character
	if event.AttackerID != f.CharacterID {
		return c, nil
	}

	// Only apply to ranged attacks (tracked from AttackEvent)
	if f.currentAttackIsMelee {
		return c, nil
	}

	// Add +2 to attack bonus at StageFeatures
	modifyAttack := func(_ context.Context, e combat.AttackChainEvent) (combat.AttackChainEvent, error) {
		e.AttackBonus += 2
		return e, nil
	}

	err := c.Add(dnd5e.StageFeatures, "archery", modifyAttack)
	if err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply archery bonus for character %s", f.CharacterID)
	}

	return c, nil
}

// onDamageChain rerolls 1s and 2s on weapon damage dice (Great Weapon Fighting)
func (f *FightingStyleCondition) onDamageChain(
	_ context.Context,
	event *combat.DamageChainEvent,
	c chain.Chain[*combat.DamageChainEvent],
) (chain.Chain[*combat.DamageChainEvent], error) {
	// Only modify damage for attacks by this character
	if event.AttackerID != f.CharacterID {
		return c, nil
	}

	// Get weapon to check properties
	// We need to parse weapon from the attack - for now we'll get it from the weapon component
	// The weapon component should be the first component in the damage
	if len(event.Components) == 0 {
		return c, nil // No weapon component
	}

	weaponComponent := &event.Components[0]
	if weaponComponent.Source != combat.DamageSourceWeapon {
		return c, nil // First component isn't weapon damage
	}

	// Check if weapon has TwoHanded or Versatile property by checking WeaponDamage
	// We need to get the actual weapon to check properties
	// For now, we'll implement a simplified version that checks based on damage dice

	// Parse weapon info to get weapon ID from the event
	// Since we don't have direct weapon access, we'll need to work with what we have
	// The DamageChainEvent has WeaponDamage string but not weapon properties

	// For proper implementation, we need to track the weapon being used
	// Let's add a modifier that rerolls at the StageFeatures stage

	modifyDamage := func(modCtx context.Context, e *combat.DamageChainEvent) (*combat.DamageChainEvent, error) {
		// Find weapon component
		for i := range e.Components {
			component := &e.Components[i]
			if component.Source != combat.DamageSourceWeapon {
				continue
			}

			// Reroll 1s and 2s in the weapon damage
			roller := f.roller
			if roller == nil {
				roller = dice.NewRoller()
			}

			// Parse die size from weapon damage notation
			dieSize, err := parseWeaponDieSize(e.WeaponDamage)
			if err != nil {
				return e, rpgerr.Wrapf(err, "failed to parse weapon damage: %s", e.WeaponDamage)
			}

			// Reroll 1s and 2s
			newRolls := make([]int, len(component.OriginalDiceRolls))
			copy(newRolls, component.OriginalDiceRolls)

			for idx, roll := range component.OriginalDiceRolls {
				if roll == 1 || roll == 2 {
					// Reroll this die
					newRoll, rollErr := roller.Roll(modCtx, dieSize)
					if rollErr != nil {
						return e, rpgerr.Wrap(rollErr, "failed to reroll die")
					}

					// Track reroll
					component.Rerolls = append(component.Rerolls, combat.RerollEvent{
						DieIndex: idx,
						Before:   roll,
						After:    newRoll,
						Reason:   "great_weapon_fighting",
					})

					newRolls[idx] = newRoll
				}
			}

			// Update final rolls
			component.FinalDiceRolls = newRolls
		}

		return e, nil
	}

	err := c.Add(dnd5e.StageFeatures, "great_weapon_fighting", modifyDamage)
	if err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply great weapon fighting for character %s", f.CharacterID)
	}

	return c, nil
}

// parseWeaponDieSize extracts the die size from weapon damage notation
// Examples: "1d8" -> 8, "2d6" -> 6
func parseWeaponDieSize(notation string) (int, error) {
	matches := diceRegex.FindStringSubmatch(notation)
	if len(matches) < 3 {
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid weapon damage notation: %s", notation)
	}

	dieSize, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, rpgerr.Wrapf(err, "invalid die size in notation: %s", notation)
	}

	return dieSize, nil
}

// FightingStyleConditionConfig configures a fighting style condition
type FightingStyleConditionConfig struct {
	CharacterID string
	Style       fightingstyles.FightingStyle
	Roller      dice.Roller // optional, uses default if nil
}

// NewFightingStyleCondition creates a fighting style condition from config
func NewFightingStyleCondition(cfg FightingStyleConditionConfig) *FightingStyleCondition {
	return &FightingStyleCondition{
		CharacterID: cfg.CharacterID,
		Style:       cfg.Style,
		roller:      cfg.Roller,
	}
}
