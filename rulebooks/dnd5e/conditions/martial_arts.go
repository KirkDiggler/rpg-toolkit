// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// MartialArtsData is the JSON structure for persisting martial arts condition state
type MartialArtsData struct {
	Ref         *core.Ref `json:"ref"`
	CharacterID string    `json:"character_id"`
	MonkLevel   int       `json:"monk_level"`
}

// MartialArtsCondition represents the Monk's Martial Arts feature.
// Allows DEX for unarmed strikes and monk weapons, and scales unarmed damage.
type MartialArtsCondition struct {
	CharacterID     string
	MonkLevel       int
	subscriptionIDs []string
	bus             events.EventBus
	roller          dice.Roller
}

// Ensure MartialArtsCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*MartialArtsCondition)(nil)

// IsApplied returns true if this condition is currently applied
func (ma *MartialArtsCondition) IsApplied() bool {
	return ma.bus != nil
}

// Apply subscribes this condition to attack and damage chain events
func (ma *MartialArtsCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if ma.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "martial arts condition already applied")
	}
	ma.bus = bus

	// Subscribe to DamageChain to modify unarmed strike damage and ensure DEX is used
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID, err := damageChain.SubscribeWithChain(ctx, ma.onDamageChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	ma.subscriptionIDs = append(ma.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events
func (ma *MartialArtsCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if ma.bus == nil {
		return nil // Not applied, nothing to remove
	}

	for _, subID := range ma.subscriptionIDs {
		err := bus.Unsubscribe(ctx, subID)
		if err != nil {
			return rpgerr.Wrap(err, "failed to unsubscribe from event")
		}
	}

	ma.subscriptionIDs = nil
	ma.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (ma *MartialArtsCondition) ToJSON() (json.RawMessage, error) {
	data := MartialArtsData{
		Ref:         refs.Conditions.MartialArts(),
		CharacterID: ma.CharacterID,
		MonkLevel:   ma.MonkLevel,
	}
	return json.Marshal(data)
}

// loadJSON loads martial arts condition state from JSON
//
//nolint:unused // Used by loader.go
func (ma *MartialArtsCondition) loadJSON(data json.RawMessage) error {
	var maData MartialArtsData
	if err := json.Unmarshal(data, &maData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal martial arts data")
	}

	ma.CharacterID = maData.CharacterID
	ma.MonkLevel = maData.MonkLevel

	return nil
}

// onDamageChain modifies damage to scale unarmed strike damage and use DEX when appropriate
func (ma *MartialArtsCondition) onDamageChain(
	ctx context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	// Only modify damage for attacks by this character
	if event.AttackerID != ma.CharacterID {
		return c, nil
	}

	// Get character registry to check weapon and ability scores
	registry, ok := gamectx.Characters(ctx)
	if !ok {
		return c, nil
	}

	// Get ability scores for DEX vs STR comparison
	abilityScores := registry.GetCharacterAbilityScores(ma.CharacterID)
	if abilityScores == nil {
		return c, nil
	}

	// Check if this is an unarmed strike or monk weapon
	isUnarmed := event.WeaponRef == nil || event.WeaponRef == refs.Weapons.UnarmedStrike()
	isMonkWeaponAttack := false

	if !isUnarmed && event.WeaponRef != nil {
		// Try to get the weapon to check if it's a monk weapon
		weapon, err := weapons.GetByID(event.WeaponRef.ID)
		if err == nil {
			isMonkWeaponAttack = isMonkWeapon(&weapon)
		}
	}

	// Only modify if it's an unarmed strike or monk weapon
	if !isUnarmed && !isMonkWeaponAttack {
		return c, nil
	}

	// Add modifier to scale unarmed damage and ensure DEX is used when beneficial
	modifyDamage := func(modCtx context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		dexMod := abilityScores.Modifier(abilities.DEX)
		strMod := abilityScores.Modifier(abilities.STR)

		// For unarmed strikes, we need to replace the weapon damage dice with martial arts dice
		if isUnarmed {
			martialArtsDice := ma.getMartialArtsDice()

			// Re-roll weapon damage with martial arts dice
			roller := ma.roller
			if roller == nil {
				roller = dice.NewRoller()
			}

			// Parse martial arts dice notation
			pool, err := dice.ParseNotation(martialArtsDice)
			if err != nil {
				return e, rpgerr.Wrapf(err, "failed to parse martial arts dice: %s", martialArtsDice)
			}

			// Roll the dice (double for crits)
			times := 1
			if e.IsCritical {
				times = 2
			}

			var newRolls []int
			for i := 0; i < times; i++ {
				result := pool.RollContext(modCtx, roller)
				if result.Error() != nil {
					return e, rpgerr.Wrap(result.Error(), "failed to roll martial arts damage")
				}
				// Flatten the roll groups
				for _, group := range result.Rolls() {
					newRolls = append(newRolls, group...)
				}
			}

			// Find and update the weapon component with new rolls
			for i := range e.Components {
				component := &e.Components[i]
				if component.Source == dnd5eEvents.DamageSourceWeapon {
					component.OriginalDiceRolls = newRolls
					component.FinalDiceRolls = newRolls
					break
				}
			}

			// Update the weapon damage notation in the event for reference
			e.WeaponDamage = martialArtsDice
		}

		// If DEX is higher than STR, replace ability modifier for monk weapons and unarmed strikes
		if dexMod > strMod {
			for i := range e.Components {
				component := &e.Components[i]
				if component.Source == dnd5eEvents.DamageSourceAbility {
					// Replace STR modifier with DEX modifier
					component.FlatBonus = dexMod
					// Update the ability used in the event
					e.AbilityUsed = abilities.DEX
					break
				}
			}
		}

		return e, nil
	}

	err := c.Add(combat.StageFeatures, "martial_arts", modifyDamage)
	if err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply martial arts for character %s", ma.CharacterID)
	}

	return c, nil
}

// getMartialArtsDice returns the damage dice for unarmed strikes based on monk level
func (ma *MartialArtsCondition) getMartialArtsDice() string {
	switch {
	case ma.MonkLevel >= 17:
		return "1d10"
	case ma.MonkLevel >= 11:
		return "1d8"
	case ma.MonkLevel >= 5:
		return "1d6"
	default:
		return "1d4"
	}
}

// isMonkWeapon checks if a weapon is a monk weapon
// Monk weapons are shortswords and simple melee weapons without Heavy or Two-Handed properties
func isMonkWeapon(weapon *weapons.Weapon) bool {
	// Shortsword is explicitly a monk weapon
	if weapon.ID == weapons.Shortsword {
		return true
	}

	// Must be a simple melee weapon
	if weapon.Category != weapons.CategorySimpleMelee {
		return false
	}

	// Cannot have Heavy property
	if weapon.HasProperty(weapons.PropertyHeavy) {
		return false
	}

	// Cannot have Two-Handed property
	if weapon.HasProperty(weapons.PropertyTwoHanded) {
		return false
	}

	return true
}

// MartialArtsInput provides configuration for creating a martial arts condition
type MartialArtsInput struct {
	CharacterID string
	MonkLevel   int
	Roller      dice.Roller // optional, uses default if nil
}

// NewMartialArtsCondition creates a new martial arts condition
func NewMartialArtsCondition(input MartialArtsInput) *MartialArtsCondition {
	return &MartialArtsCondition{
		CharacterID: input.CharacterID,
		MonkLevel:   input.MonkLevel,
		roller:      input.Roller,
	}
}
