// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// MartialArtsData is the JSON structure for persisting martial arts condition state
type MartialArtsData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	MonkLevel int       `json:"monk_level"`
}

// MartialArtsCondition represents the Monk's Martial Arts feature: unarmed
// strikes and monk weapons may use Dexterity, and an unarmed strike deals the
// Martial Arts die.
//
// Both are settled at assembly, through [MartialArtsCondition.WeaponAttackOverride],
// before any rule is asked and before any die is rolled — the attack's
// ability and its damage die are the ones the swing uses, and no roll is made
// and then discarded. The condition subscribes to nothing.
type MartialArtsCondition struct {
	MemberID  string
	MonkLevel int
	bus       events.EventBus
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

// Apply marks the condition active. Martial Arts acts at attack assembly and
// installs no subscriber.
func (ma *MartialArtsCondition) Apply(_ context.Context, bus events.EventBus) error {
	if ma.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "martial arts condition already applied")
	}
	ma.bus = bus
	return nil
}

// Remove marks the condition inactive.
func (ma *MartialArtsCondition) Remove(_ context.Context, _ events.EventBus) error {
	ma.bus = nil
	return nil
}

// WeaponAttackOverride settles Martial Arts at attack assembly for an unarmed
// strike (an empty hand, or the bonus unarmed strike) or a monk weapon:
// Dexterity is offered as the attack's ability, which assembly takes only when
// its modifier is higher than the weapon's own ability, and an unarmed strike's
// primary pool becomes the Martial Arts die for this monk's level. Any other
// weapon gets no override.
func (ma *MartialArtsCondition) WeaponAttackOverride(_, itemID string) *weaponattack.Override {
	if itemID == "" {
		return &weaponattack.Override{Dice: ma.getMartialArtsDice(), Ability: abilities.DEX}
	}
	weapon, err := weapons.GetByID(weapons.WeaponID(itemID))
	if err != nil {
		return nil
	}
	switch {
	case weapon.ID == weapons.UnarmedStrike:
		return &weaponattack.Override{Dice: ma.getMartialArtsDice(), Ability: abilities.DEX}
	case isMonkWeapon(&weapon):
		return &weaponattack.Override{Ability: abilities.DEX}
	default:
		return nil
	}
}

// ToJSON converts the condition to JSON for persistence
func (ma *MartialArtsCondition) ToJSON() (json.RawMessage, error) {
	data := MartialArtsData{
		Ref:       refs.Conditions.MartialArts(),
		MemberID:  ma.MemberID,
		MonkLevel: ma.MonkLevel,
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

	ma.MemberID = maData.MemberID
	ma.MonkLevel = maData.MonkLevel

	return nil
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

// IsMartialArtsWeapon reports whether a weapon ref qualifies for Martial Arts:
// an unarmed strike, a shortsword, or a simple melee weapon without Heavy or
// Two-Handed. It is the shared static weapon predicate used by both the
// condition folds and the character action-cost compiler.
func IsMartialArtsWeapon(weaponRef *core.Ref) bool {
	isUnarmed, monkWeapon := martialArtsWeaponKind(weaponRef)
	return isUnarmed || monkWeapon != nil
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
	MemberID  string
	MonkLevel int
}

// NewMartialArtsCondition creates a new martial arts condition
func NewMartialArtsCondition(input MartialArtsInput) *MartialArtsCondition {
	return &MartialArtsCondition{
		MemberID:  input.MemberID,
		MonkLevel: input.MonkLevel,
	}
}
