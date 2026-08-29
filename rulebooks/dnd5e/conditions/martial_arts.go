// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (ma *MartialArtsCondition) Ref() *core.Ref { return refs.Conditions.MartialArts() }

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
		ma.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	ma.subscriptionIDs = append(ma.subscriptionIDs, subID)

	// Subscribe to AttackChain so the ATTACK ROLL uses DEX when it is higher,
	// matching the damage swap above — attack and damage must agree on the
	// governing ability (#709: the swap applied to damage only, so a DEX monk
	// attacked at STR + prof while its damage credited DEX).
	attackChain := dnd5eEvents.AttackChain.On(bus)
	attackSubID, err := attackChain.SubscribeWithChain(ctx, ma.onAttackChain)
	if err != nil {
		// Roll back the damage-chain subscription so a failed Apply leaves no
		// partial state (mirrors DodgingCondition.Apply).
		_ = ma.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to attack chain")
	}
	ma.subscriptionIDs = append(ma.subscriptionIDs, attackSubID)

	return nil
}

// Remove unsubscribes this condition from events
func (ma *MartialArtsCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if ma.bus == nil {
		return nil // Not applied, nothing to remove
	}

	total := len(ma.subscriptionIDs)
	var errs []error
	for _, subID := range ma.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	ma.subscriptionIDs = nil
	ma.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
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

	// Own sheet, looked up in the cast by this character's own ID. A cast that
	// cannot name this monk means no comparison to make — leave the chain
	// untouched rather than erroring, which would discard every other damage
	// component with it. See [member].
	me, ok := member(ctx, ma.CharacterID)
	if !ok {
		return c, nil
	}
	abilityScores := me.AbilityScores()

	// Check if this is an unarmed strike or monk weapon
	isUnarmed, monkWeapon := martialArtsWeaponKind(event.WeaponRef)

	// Only modify if it's an unarmed strike or monk weapon
	if !isUnarmed && monkWeapon == nil {
		return c, nil
	}

	// Add modifier to scale unarmed damage and ensure DEX is used when beneficial
	modifyDamage := func(modCtx context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		dexMod := abilityScores.Modifier(abilities.DEX)
		strMod := abilityScores.Modifier(abilities.STR)

		// For unarmed strikes, we need to replace the weapon damage dice with martial arts dice
		if isUnarmed {
			componentIndex := primaryWeaponComponentIndex(e)
			if componentIndex < 0 {
				return e, nil
			}
			component := &e.Components[componentIndex]
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
			if e.IsCritical && !component.HasProperty(damage.DoesNotCrit) {
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

			// Replace only the component carrying the canonical primary marker.
			component.Dice = martialArtsDice
			e.WeaponDamageDice = martialArtsDice
			component.OriginalDiceRolls = append([]int(nil), newRolls...)
			component.FinalDiceRolls = append([]int(nil), newRolls...)
			component.IsCritical = times == 2

		}

		// If DEX is higher than STR, replace ability modifier for monk weapons and unarmed strikes
		if dexMod > strMod {
			for i := range e.Components {
				component := &e.Components[i]
				if component.Source == dnd5eEvents.DamageSourceAbility {
					// Replace STR modifier value with DEX modifier
					component.FlatBonus = dexMod
					// Update the SourceRef label so combat log shows DEX, not STR
					component.SourceRef = refs.Abilities.Dexterity()
					// Update the ability used in the event
					e.AbilityUsed = abilities.DEX
					break
				}
			}
		}

		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "martial_arts", modifyDamage); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply martial arts for character %s", ma.CharacterID)
	}

	return c, nil
}

// onAttackChain swaps the attack roll's governing ability to DEX for unarmed
// strikes and monk weapons when DEX is higher — the attack-roll mirror of the
// damage swap in onDamageChain, so attack and damage agree on the governing
// ability (#709: the swap applied to damage only, leaving the attack at STR).
func (ma *MartialArtsCondition) onAttackChain(
	ctx context.Context,
	event dnd5eEvents.AttackChainEvent,
	c chain.Chain[dnd5eEvents.AttackChainEvent],
) (chain.Chain[dnd5eEvents.AttackChainEvent], error) {
	// Only modify attacks by this character
	if event.AttackerID != ma.CharacterID {
		return c, nil
	}

	// Own sheet, read off the cast — the damage chain's twin, and it has to
	// read the SAME scores through the SAME channel or attack and damage can
	// disagree about the governing ability (#709).
	me, ok := member(ctx, ma.CharacterID)
	if !ok {
		return c, nil
	}
	abilityScores := me.AbilityScores()

	// Only modify if it's an unarmed strike or monk weapon
	isUnarmed, monkWeapon := martialArtsWeaponKind(event.WeaponRef)
	if !isUnarmed && monkWeapon == nil {
		return c, nil
	}

	// Finesse monk weapons (e.g. shortsword) already attack with the higher of
	// STR/DEX on the base path — adjusting again would double-count DEX.
	if monkWeapon != nil && monkWeapon.HasProperty(weapons.PropertyFinesse) {
		return c, nil
	}

	modifyAttack := func(_ context.Context, e dnd5eEvents.AttackChainEvent) (dnd5eEvents.AttackChainEvent, error) {
		dexMod := abilityScores.Modifier(abilities.DEX)
		strMod := abilityScores.Modifier(abilities.STR)
		if dexMod > strMod {
			// The base bonus used STR (melee, non-finesse); replace it with
			// DEX by applying the difference — same rule the damage chain
			// applies to the ability component.
			e.AttackBonus += dexMod - strMod
		}
		return e, nil
	}

	if err := c.Add(combat.StageFeatures, "martial_arts", modifyAttack); err != nil {
		return c, rpgerr.Wrapf(err, "failed to apply martial arts attack bonus for character %s", ma.CharacterID)
	}

	return c, nil
}

// martialArtsWeaponKind classifies an attack's weapon for Martial Arts
// purposes: whether it is an unarmed strike (nil ref or the unarmed-strike
// ref), and otherwise whether it is a monk weapon (returned so callers can
// inspect properties, e.g. Finesse). A non-monk weapon returns (false, nil).
func martialArtsWeaponKind(weaponRef *core.Ref) (isUnarmed bool, monkWeapon *weapons.Weapon) {
	if weaponRef == nil || weaponRef.ID == refs.Weapons.UnarmedStrike().ID {
		return true, nil
	}
	weapon, err := weapons.GetByID(weaponRef.ID)
	if err != nil || !isMonkWeapon(&weapon) {
		return false, nil
	}
	return false, &weapon
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
