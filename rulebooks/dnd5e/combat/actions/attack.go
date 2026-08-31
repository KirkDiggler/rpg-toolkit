package actions

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

const (
	dnd5eModule   = "dnd5e"
	conditionType = "conditions"
)

// AttackCategory distinguishes attack-roll rules without coupling a profile to
// the content that produced it.
type AttackCategory string

const (
	// AttackCategoryWeapon identifies a weapon, natural, or unarmed attack.
	AttackCategoryWeapon AttackCategory = "weapon"
	// AttackCategorySpell identifies a spell attack.
	AttackCategorySpell AttackCategory = "spell"
)

// AttackProfile declares everything the attack-roll resolution machine needs.
type AttackProfile struct {
	Category    AttackCategory       `json:"category"`
	Delivery    AttackDelivery       `json:"delivery"`
	AttackBonus int                  `json:"attack_bonus"`
	Ability     *AbilityContribution `json:"ability,omitempty"`
	Weapon      *WeaponContext       `json:"weapon,omitempty"`
	// TwoWeaponBonus identifies the bonus attack granted after attacking with
	// one of two light melee weapons. It is action semantics, not merely the
	// physical equipment slot holding the weapon.
	TwoWeaponBonus bool                   `json:"two_weapon_bonus,omitempty"`
	Damage         []damage.Damage        `json:"damage,omitempty"`
	OnHit          []ConditionApplication `json:"on_hit,omitempty"`
}

// Validate reports whether the profile declares a supported category,
// delivery, outcome, and internally consistent optional evidence.
func (p AttackProfile) Validate() error {
	switch p.Category {
	case AttackCategoryWeapon:
	case AttackCategorySpell:
		if p.Weapon != nil {
			return fmt.Errorf("spell attack must not carry weapon context")
		}
	default:
		return fmt.Errorf("unknown attack category %q", p.Category)
	}

	if err := p.Delivery.Validate(); err != nil {
		return err
	}
	if p.TwoWeaponBonus {
		if p.Category != AttackCategoryWeapon {
			return fmt.Errorf("two-weapon bonus attack must be a weapon attack")
		}
		if p.Weapon == nil {
			return fmt.Errorf("two-weapon bonus attack requires weapon context")
		}
		if !p.Delivery.IsMelee() {
			return fmt.Errorf("two-weapon bonus attack requires melee delivery")
		}
	}
	if len(p.Damage) == 0 && len(p.OnHit) == 0 {
		return fmt.Errorf("attack must declare damage or an on-hit condition")
	}
	if len(p.Damage) > 0 {
		if err := damage.Validate(p.Damage); err != nil {
			return fmt.Errorf("damage declaration is invalid: %w", err)
		}
	}

	abilityMarkers := 0
	for _, pool := range p.Damage {
		if pool.HasProperty(damage.AddsAttackAbilityModifier) {
			abilityMarkers++
		}
	}
	if p.Ability == nil {
		if abilityMarkers != 0 {
			return fmt.Errorf("attack with no ability evidence must have no ability-marked damage pool")
		}
	} else {
		if !isAbility(p.Ability.Ability) {
			return fmt.Errorf("attack ability %q is not a D&D 5e ability", p.Ability.Ability)
		}
		if abilityMarkers != 1 {
			return fmt.Errorf("attack with ability evidence must have exactly one ability-marked damage pool")
		}
	}

	for index, application := range p.OnHit {
		if err := application.Validate(); err != nil {
			return fmt.Errorf("on-hit condition %d is invalid: %w", index, err)
		}
	}

	return nil
}

// Clone returns a deep copy of all mutable evidence, damage, and condition
// declarations in the profile.
func (p AttackProfile) Clone() AttackProfile {
	clone := p
	clone.Delivery = p.Delivery.clone()
	if p.Ability != nil {
		ability := *p.Ability
		clone.Ability = &ability
	}
	if p.Weapon != nil {
		weapon := *p.Weapon
		weapon.Ref = cloneRef(p.Weapon.Ref)
		weapon.OffHandWeaponRef = cloneRef(p.Weapon.OffHandWeaponRef)
		clone.Weapon = &weapon
	}
	if p.Damage != nil {
		clone.Damage = make([]damage.Damage, len(p.Damage))
		copy(clone.Damage, p.Damage)
		for index := range p.Damage {
			clone.Damage[index].Properties = append([]damage.Property(nil), p.Damage[index].Properties...)
		}
	}
	if p.OnHit != nil {
		clone.OnHit = make([]ConditionApplication, len(p.OnHit))
		for index, application := range p.OnHit {
			clone.OnHit[index] = application.Clone()
		}
	}
	return clone
}

// AbilityContribution records the ability and static modifier selected by a
// producer when it assembled an attack.
type AbilityContribution struct {
	Ability  abilities.Ability `json:"ability"`
	Modifier int               `json:"modifier"`
}

// WeaponContext records optional wielded-weapon evidence used by attack rules.
type WeaponContext struct {
	Ref              *core.Ref `json:"ref,omitempty"`
	TwoHanded        bool      `json:"two_handed"`
	OffHandWeaponRef *core.Ref `json:"off_hand_weapon_ref,omitempty"`
}

// AttackDelivery is an exactly-one union of melee and ranged delivery.
type AttackDelivery struct {
	Melee  *MeleeDelivery  `json:"melee,omitempty"`
	Ranged *RangedDelivery `json:"ranged,omitempty"`
}

// MeleeDelivery declares a melee attack's reach in feet.
type MeleeDelivery struct {
	ReachFeet int `json:"reach_feet"`
}

// RangedDelivery declares a ranged attack's normal and optional long range in
// feet. A zero long range means no separate long-range bracket exists.
type RangedDelivery struct {
	NormalFeet int `json:"normal_feet"`
	LongFeet   int `json:"long_feet,omitempty"`
}

// Validate reports whether exactly one delivery arm is populated and its
// distances form a legal positive range.
func (d AttackDelivery) Validate() error {
	if (d.Melee == nil) == (d.Ranged == nil) {
		return fmt.Errorf("attack must declare exactly one delivery")
	}
	if d.Melee != nil {
		if d.Melee.ReachFeet <= 0 {
			return fmt.Errorf("melee delivery requires a positive reach")
		}
		return nil
	}
	if d.Ranged.NormalFeet <= 0 {
		return fmt.Errorf("ranged delivery requires a positive normal range")
	}
	if d.Ranged.LongFeet != 0 && d.Ranged.LongFeet < d.Ranged.NormalFeet {
		return fmt.Errorf("ranged delivery long range must be zero or at least normal range")
	}
	return nil
}

// IsMelee reports whether the populated delivery arm is melee.
func (d AttackDelivery) IsMelee() bool {
	return d.Melee != nil
}

// MaxRangeFeet returns the maximum legal distance for the populated delivery.
func (d AttackDelivery) MaxRangeFeet() int {
	if d.Melee != nil {
		return d.Melee.ReachFeet
	}
	if d.Ranged == nil {
		return 0
	}
	if d.Ranged.LongFeet > 0 {
		return d.Ranged.LongFeet
	}
	return d.Ranged.NormalFeet
}

// NormalRangeFeet returns the distance before ranged disadvantage applies, or
// melee reach for a melee delivery.
func (d AttackDelivery) NormalRangeFeet() int {
	if d.Melee != nil {
		return d.Melee.ReachFeet
	}
	if d.Ranged == nil {
		return 0
	}
	return d.Ranged.NormalFeet
}

func (d AttackDelivery) clone() AttackDelivery {
	clone := d
	if d.Melee != nil {
		melee := *d.Melee
		clone.Melee = &melee
	}
	if d.Ranged != nil {
		ranged := *d.Ranged
		clone.Ranged = &ranged
	}
	return clone
}

// ConditionApplication declares one condition to build and apply after an
// attack hits. Parameters remain opaque to this package.
type ConditionApplication struct {
	Ref        core.Ref        `json:"ref"`
	Parameters json.RawMessage `json:"parameters,omitempty"`
	Save       *saves.SaveGate `json:"save,omitempty"`
}

// Validate reports whether the declaration names a D&D 5e condition and uses
// negation-on-success semantics for any save gate.
func (a ConditionApplication) Validate() error {
	if err := a.Ref.IsValid(); err != nil {
		return fmt.Errorf("condition ref is invalid: %w", err)
	}
	if a.Ref.Module != dnd5eModule || a.Ref.Type != conditionType {
		return fmt.Errorf("condition ref must use %s:%s, got %s", dnd5eModule, conditionType, a.Ref.String())
	}
	if len(a.Parameters) > 0 && !json.Valid(a.Parameters) {
		return fmt.Errorf("condition parameters must be valid JSON")
	}
	if a.Save != nil {
		if a.Save.OnSuccess != saves.Negated {
			return fmt.Errorf("condition save must negate the condition on success")
		}
		if err := a.Save.Validate(); err != nil {
			return fmt.Errorf("condition save is invalid: %w", err)
		}
	}
	return nil
}

// Clone returns a deep copy of the declaration's opaque parameters and save
// ability list.
func (a ConditionApplication) Clone() ConditionApplication {
	clone := a
	clone.Parameters = append(json.RawMessage(nil), a.Parameters...)
	if a.Save != nil {
		save := *a.Save
		save.Abilities = append([]abilities.Ability(nil), a.Save.Abilities...)
		clone.Save = &save
	}
	return clone
}

func cloneRef(ref *core.Ref) *core.Ref {
	if ref == nil {
		return nil
	}
	clone := *ref
	return &clone
}

func isAbility(ability abilities.Ability) bool {
	switch ability {
	case abilities.STR, abilities.DEX, abilities.CON, abilities.INT, abilities.WIS, abilities.CHA:
		return true
	default:
		return false
	}
}
