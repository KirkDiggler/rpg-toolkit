// Package attack defines reusable D&D 5e attack metadata.
package attack

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Category identifies the rule source for an attack.
type Category string

const (
	// CategoryNatural identifies an attack made without equipment.
	CategoryNatural Category = "natural"
	// CategoryEquipmentWeapon identifies an attack made with an equipment weapon.
	CategoryEquipmentWeapon Category = "equipment_weapon"
)

// Definition describes an attack independently of its attacker and combat resolution.
type Definition struct {
	ActionID        string
	DisplayName     string
	Category        Category
	Bonus           BonusRule
	Targeting       Targeting
	EquipmentWeapon *weapons.Weapon
	Damage          damage.DamageSpec
}

// Validate verifies that the definition has all of the static metadata needed to resolve an attack.
func (d *Definition) Validate() error {
	if d == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "attack definition is required")
	}
	if d.ActionID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "attack action ID is required")
	}
	if d.Category != CategoryNatural && d.Category != CategoryEquipmentWeapon {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "unknown attack category")
	}
	if err := d.Bonus.Validate(); err != nil {
		return err
	}
	if err := d.Targeting.Validate(); err != nil {
		return err
	}
	if err := d.Damage.Validate(); err != nil {
		return err
	}
	if d.Category == CategoryEquipmentWeapon && d.EquipmentWeapon == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "equipment weapon is required for equipment weapon attacks")
	}

	return nil
}

type bonusRuleKind string

const (
	bonusRuleFixed   bonusRuleKind = "fixed"
	bonusRuleDerived bonusRuleKind = "derived"
)

// BonusRule describes whether an attack roll uses a fixed or dynamically derived bonus.
type BonusRule struct {
	Fixed int
	kind  bonusRuleKind
}

// FixedBonus returns a rule that always uses bonus for the attack roll.
func FixedBonus(bonus int) BonusRule {
	return BonusRule{Fixed: bonus, kind: bonusRuleFixed}
}

// DerivedBonus returns a rule that derives the attack bonus during resolution.
func DerivedBonus() BonusRule {
	return BonusRule{kind: bonusRuleDerived}
}

// Validate verifies that the bonus rule was created by a supported constructor.
func (r BonusRule) Validate() error {
	if r.kind != bonusRuleFixed && r.kind != bonusRuleDerived {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "unknown attack bonus rule")
	}
	return nil
}

// Targeting describes the target range supported by an attack.
type Targeting struct {
	Mode        TargetingMode
	Reach       int
	RangeNormal int
	RangeLong   int
}

// TargetingMode identifies the targeting range model used by an attack.
type TargetingMode string

const (
	// TargetingMeleeReach identifies a melee attack with a reach measured in hexes.
	TargetingMeleeReach TargetingMode = "melee_reach"
	// TargetingRanged identifies a ranged attack with normal and long ranges measured in hexes.
	TargetingRanged TargetingMode = "ranged"
)

// MeleeReach returns targeting metadata for a melee attack with the specified reach in hexes.
func MeleeReach(reach int) Targeting {
	return Targeting{Mode: TargetingMeleeReach, Reach: reach}
}

// Ranged returns targeting metadata for a ranged attack with normal and long ranges in hexes.
func Ranged(normalRange, longRange int) Targeting {
	return Targeting{Mode: TargetingRanged, RangeNormal: normalRange, RangeLong: longRange}
}

// Validate verifies that targeting uses a supported model and valid distances.
func (t Targeting) Validate() error {
	switch t.Mode {
	case TargetingMeleeReach:
		if t.Reach <= 0 {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "attack reach must be positive")
		}
	case TargetingRanged:
		if t.RangeNormal <= 0 {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "attack normal range must be positive")
		}
		if t.RangeLong < t.RangeNormal {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "attack long range must be at least the normal range")
		}
	default:
		return rpgerr.New(rpgerr.CodeInvalidArgument, "unknown attack targeting mode")
	}
	return nil
}
