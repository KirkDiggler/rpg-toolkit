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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// MartialArtsData is the JSON structure for persisting martial arts condition
// state. No monk level is stored; a blob saved with the old "monk_level" key
// loads and the copy is ignored.
type MartialArtsData struct {
	Ref      *core.Ref `json:"ref"`
	MemberID string    `json:"member_id"`
}

// MartialArtsCondition represents the Monk's Martial Arts feature: unarmed
// strikes and monk weapons may use Dexterity, and an unarmed strike deals the
// Martial Arts die.
//
// Both are settled at assembly, through [MartialArtsCondition.WeaponAttackOverride],
// before any rule is asked and before any die is rolled — the attack's
// ability and its damage die are the ones the swing uses, and no roll is made
// and then discarded. The condition subscribes to nothing. The Martial Arts
// die scales with monk level, which the sheet hands the override with each
// swing; the condition stores none.
type MartialArtsCondition struct {
	MemberID string
	bus      events.EventBus
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
// primary pool becomes the Martial Arts die for the monk level the sheet's
// level record answers at this swing. Any other weapon gets no override.
//
// An unarmed strike with no level record handed in, or from a holder with no
// monk levels, is an error: the die cannot be answered, and zero is never read
// as level one.
func (ma *MartialArtsCondition) WeaponAttackOverride(
	in *weaponattack.OverrideInput,
) (*weaponattack.OverrideOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "martial arts: no attack assembly input")
	}
	unarmed, monk := in.ItemID == "", false
	if !unarmed {
		unarmed, monk = martialArtsWeaponID(in.ItemID)
	}
	switch {
	case unarmed:
		die, err := martialArtsDie(in.Levels)
		if err != nil {
			return nil, err
		}
		return &weaponattack.OverrideOutput{
			Override: &weaponattack.Override{Dice: die, Ability: abilities.DEX},
		}, nil
	case monk:
		return &weaponattack.OverrideOutput{Override: &weaponattack.Override{Ability: abilities.DEX}}, nil
	default:
		return &weaponattack.OverrideOutput{}, nil
	}
}

// martialArtsWeaponID classifies a catalogue weapon ID for Martial Arts: the
// unarmed strike, or a monk weapon. An ID the catalogue does not know is
// neither. Attack assembly and AssessAction share it.
func martialArtsWeaponID(id string) (unarmed, monk bool) {
	weapon, err := weapons.GetByID(weapons.WeaponID(id))
	if err != nil {
		return false, false
	}
	if weapon.ID == weapons.UnarmedStrike {
		return true, false
	}
	return false, isMonkWeapon(&weapon)
}

// ToJSON converts the condition to JSON for persistence
func (ma *MartialArtsCondition) ToJSON() (json.RawMessage, error) {
	data := MartialArtsData{
		Ref:      refs.Conditions.MartialArts(),
		MemberID: ma.MemberID,
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

// martialArtsDie returns the unarmed strike's Martial Arts die for the monk
// level the holder's level record answers. No level record, or no monk
// levels, is an error.
func martialArtsDie(levels classes.LevelHolder) (string, error) {
	if levels == nil {
		return "", rpgerr.New(rpgerr.CodeInvalidArgument,
			"martial arts: no level record handed to the attack assembly")
	}
	level := levels.ClassLevel(classes.Monk)
	if level < 1 {
		return "", rpgerr.New(rpgerr.CodePrerequisiteNotMet,
			"martial arts: the holder has no monk levels, so the Martial Arts die cannot be answered")
	}
	return martialArtsDieAt(level), nil
}

// martialArtsDieAt is the Martial Arts die at a monk level of at least one:
// 1d4, 1d6 from 5th, 1d8 from 11th, 1d10 from 17th.
func martialArtsDieAt(level int) string {
	switch {
	case level >= 17:
		return "1d10"
	case level >= 11:
		return "1d8"
	case level >= 5:
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

// MartialArtsInput provides configuration for creating a martial arts
// condition. It takes no level: the die is read from the sheet's level record
// at each swing.
type MartialArtsInput struct {
	MemberID string
}

// NewMartialArtsCondition creates a new martial arts condition
func NewMartialArtsCondition(input MartialArtsInput) *MartialArtsCondition {
	return &MartialArtsCondition{
		MemberID: input.MemberID,
	}
}
