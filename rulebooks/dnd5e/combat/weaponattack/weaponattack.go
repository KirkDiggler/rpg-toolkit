// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package weaponattack turns a catalog weapon plus whoever holds it into the
// inert [actions.Definition] the resolution machine reads.
//
// # Why this is one package and not two methods
//
// A weapon line on a stat block is the WIELDER's number, not the weapon's
// (rpg-project#448). "Shortbow +4, 1d6+2" is DEX 14 and a +2 proficiency
// bonus standing behind a 1d6 piercing weapon with an 80/320 range — and it
// is the same arithmetic whether the wielder is a character who bought the
// bow or a skeleton that was authored holding one. Two copies of it is two
// places for the goblin's scimitar to grow a one-foot reach.
//
// So the assembly lives here, over [Wielder] — three questions and no
// fourth — and `character` and `monster` both call it. Neither package
// imports the other, and this one imports neither.
//
// # What it does not do
//
// It compiles STATIC evidence only. Situational effects — advantage, a bless
// die, a rage bonus — continue to arrive through resolution chains, and
// nothing here reaches for a bus, a context, or a die.
//
// It also does not find the weapon. A caller that holds a slot resolves the
// slot; a caller that holds an authored ref resolves the ref. This package is
// handed the weapon it is to compile.
package weaponattack

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

const (
	defaultMeleeReach = 5
	reachWeaponReach  = 10
)

// Wielder is everything the assembly asks of whoever holds the weapon.
//
// THREE QUESTIONS, AND THEY ARE ALL ABOUT THE WIELDER. An ability modifier,
// a proficiency bonus, and whether this particular weapon is one the wielder
// is trained with. Nothing here asks what KIND of thing is holding the
// weapon, which is the whole point: a character answers from its sheet and a
// monster answers from its stat block, and the arithmetic in between cannot
// tell them apart.
//
// IsProficientWith IS THE SEAT THE PLACEMENT SWITCH WILL SIT IN
// (rpg-project#448 decision 7). Proficiency is the creature's, not the
// weapon's, so it is asked as a question rather than assumed — a monster
// answers YES today, and the reserved `proficient: false` placement field
// lands as one field feeding this one answer, not as a second assembly.
//
// The method names match [character.Character]'s existing ones so that type
// satisfies this interface without a shim; the character path's behaviour is
// pinned by its own tests and does not change.
type Wielder interface {
	// GetAbilityModifier reports the wielder's modifier for one ability.
	GetAbilityModifier(ability abilities.Ability) int

	// ProficiencyBonus reports the wielder's proficiency bonus.
	ProficiencyBonus() int

	// IsProficientWith reports whether the wielder is trained with this
	// weapon. False leaves the proficiency bonus off the attack roll;
	// damage keeps its ability modifier, which never came from proficiency.
	IsProficientWith(weapon *weapons.Weapon) bool
}

// Input names the wielder, the weapon, the grip, and the optional price.
type Input struct {
	// Wielder is whoever holds the weapon. REQUIRED.
	Wielder Wielder

	// Weapon is the catalog entry to compile. REQUIRED, and already
	// resolved: this package does not look weapons up.
	Weapon *weapons.Weapon

	// TwoHanded picks the versatile grip. Ignored by a weapon that is not
	// versatile, and already true for a weapon that is two-handed anyway.
	TwoHanded bool

	// Cost is the price the caller's action-economy policy chose, cloned
	// onto the definition. Nil means the definition declares no price.
	Cost *combat.SpendProfile

	// OffHandWeaponRef is what the wielder holds in the OTHER hand, for the
	// weapon context. Nil for a wielder with no second hand to read — which
	// is every monster today, and a character swinging an unarmed strike.
	OffHandWeaponRef *core.Ref

	// AlwaysProficient adds the proficiency bonus without asking the
	// wielder. Everyone is proficient with an unarmed strike, and that is
	// the rule rather than a training the sheet records.
	AlwaysProficient bool
}

// Assemble derives an inert shared attack definition from a weapon and its
// wielder's numbers.
func Assemble(in *Input) (combatActions.Definition, error) {
	if in == nil {
		return combatActions.Definition{}, rpgerr.New(rpgerr.CodeNil, "no weapon attack input")
	}
	if in.Wielder == nil {
		return combatActions.Definition{}, rpgerr.New(rpgerr.CodeNil, "no wielder to assemble an attack for")
	}
	if in.Weapon == nil {
		return combatActions.Definition{}, rpgerr.New(rpgerr.CodeNil, "no weapon to assemble an attack from")
	}

	weapon := in.Weapon
	weaponRef := refs.Weapons.ByID(string(weapon.ID))
	if weaponRef == nil {
		return combatActions.Definition{}, rpgerr.Newf(
			rpgerr.CodeInvalidArgument, "no ref for weapon %q", weapon.ID)
	}

	delivery, err := DeliveryFor(weapon)
	if err != nil {
		return combatActions.Definition{}, err
	}

	ability := AbilityFor(in.Wielder, weapon)
	modifier := in.Wielder.GetAbilityModifier(ability)
	attackBonus := modifier
	if in.AlwaysProficient || in.Wielder.IsProficientWith(weapon) {
		attackBonus += in.Wielder.ProficiencyBonus()
	}

	pools, err := weapon.DamageForGrip(in.TwoHanded)
	if err != nil {
		return combatActions.Definition{}, rpgerr.Wrap(err, "cannot compile weapon damage")
	}

	definition := combatActions.Definition{
		Ref:  *weaponRef,
		Name: weapon.Name,
		Cost: combatActions.CloneSpendProfile(in.Cost),
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    delivery,
			AttackBonus: attackBonus,
			Ability: &combatActions.AbilityContribution{
				Ability:  ability,
				Modifier: modifier,
			},
			Weapon: &combatActions.WeaponContext{
				Ref:              CopyRef(weaponRef),
				TwoHanded:        in.TwoHanded || weapon.HasProperty(weapons.PropertyTwoHanded),
				OffHandWeaponRef: CopyRef(in.OffHandWeaponRef),
			},
			Damage: copyDamagePools(pools),
		},
	}
	if err := definition.Validate(); err != nil {
		return combatActions.Definition{}, rpgerr.Wrap(err, "assembled attack is invalid")
	}

	return definition, nil
}

// DeliveryFor reports how a weapon reaches its target: a ranged weapon's two
// bands, or a melee weapon's reach in feet.
func DeliveryFor(weapon *weapons.Weapon) (combatActions.AttackDelivery, error) {
	if weapon == nil {
		return combatActions.AttackDelivery{}, rpgerr.New(
			rpgerr.CodeNil, "no weapon to read delivery from")
	}
	if weapon.IsRanged() {
		if weapon.Range == nil {
			return combatActions.AttackDelivery{}, rpgerr.Newf(
				rpgerr.CodeInvalidArgument, "ranged weapon %q has no range", weapon.ID)
		}
		return combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{
			NormalFeet: weapon.Range.Normal,
			LongFeet:   weapon.Range.Long,
		}}, nil
	}
	if !weapon.IsMelee() {
		return combatActions.AttackDelivery{}, rpgerr.Newf(
			rpgerr.CodeInvalidArgument, "weapon %q has unknown category %q", weapon.ID, weapon.Category)
	}

	reach := defaultMeleeReach
	if weapon.HasProperty(weapons.PropertyReach) {
		reach = reachWeaponReach
	}
	return combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: reach}}, nil
}

// AbilityFor reports which ability this wielder attacks with using this
// weapon: finesse takes the better of STR and DEX, a ranged weapon takes DEX,
// and everything else takes STR.
func AbilityFor(wielder Wielder, weapon *weapons.Weapon) abilities.Ability {
	if weapon.HasProperty(weapons.PropertyFinesse) {
		if wielder.GetAbilityModifier(abilities.DEX) > wielder.GetAbilityModifier(abilities.STR) {
			return abilities.DEX
		}
		return abilities.STR
	}
	if weapon.IsRanged() {
		return abilities.DEX
	}
	return abilities.STR
}

// CopyRef returns a copy of a ref, so a caller cannot reach back through a
// compiled definition and edit a package-level singleton.
func CopyRef(ref *core.Ref) *core.Ref {
	if ref == nil {
		return nil
	}
	copied := *ref
	return &copied
}

func copyDamagePools(pools []damage.Damage) []damage.Damage {
	copied := make([]damage.Damage, len(pools))
	for index, pool := range pools {
		copied[index] = pool
		copied[index].Properties = append([]damage.Property(nil), pool.Properties...)
	}
	return copied
}
