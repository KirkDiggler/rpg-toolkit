// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

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

// AssembleAttackInput identifies the equipped weapon and grip to compile, plus
// the optional price chosen by the caller's action-economy policy.
type AssembleAttackInput struct {
	Slot      InventorySlot
	TwoHanded bool
	Cost      *combat.SpendProfile
}

// AssembleAttack derives an inert shared attack definition from a character's
// sheet and equipped weapon. It compiles static evidence only; situational
// effects continue to contribute through resolution chains.
func AssembleAttack(c *Character, in *AssembleAttackInput) (combatActions.Definition, error) {
	if c == nil {
		return combatActions.Definition{}, rpgerr.New(rpgerr.CodeNil, "no character to assemble an attack for")
	}
	if in == nil {
		return combatActions.Definition{}, rpgerr.New(rpgerr.CodeNil, "no attack input")
	}

	weapon, unarmed, err := equippedWeapon(c, in.Slot)
	if err != nil {
		return combatActions.Definition{}, err
	}

	weaponRef := refs.Weapons.ByID(string(weapon.ID))
	if weaponRef == nil {
		return combatActions.Definition{}, rpgerr.Newf(
			rpgerr.CodeInvalidArgument, "no ref for weapon %q", weapon.ID)
	}

	delivery, err := deliveryForWeapon(weapon)
	if err != nil {
		return combatActions.Definition{}, err
	}

	ability := attackAbility(c, weapon)
	modifier := c.GetAbilityModifier(ability)
	attackBonus := modifier
	if unarmed || c.IsProficientWith(weapon) {
		attackBonus += c.ProficiencyBonus()
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
				Ref:              copyRef(weaponRef),
				TwoHanded:        in.TwoHanded || weapon.HasProperty(weapons.PropertyTwoHanded),
				OffHandWeaponRef: copyRef(otherHandWeaponRef(c, in.Slot)),
			},
			Damage: copyDamagePools(pools),
		},
	}
	if err := definition.Validate(); err != nil {
		return combatActions.Definition{}, rpgerr.Wrap(err, "assembled attack is invalid")
	}

	return definition, nil
}

func deliveryForWeapon(weapon *weapons.Weapon) (combatActions.AttackDelivery, error) {
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

func otherHandWeaponRef(c *Character, slot InventorySlot) *core.Ref {
	var other InventorySlot
	switch slot {
	case SlotMainHand:
		other = SlotOffHand
	case SlotOffHand:
		other = SlotMainHand
	default:
		return nil
	}

	equipped := c.GetEquippedSlot(other)
	if equipped == nil {
		return nil
	}
	weapon := equipped.AsWeapon()
	if weapon == nil {
		return nil
	}
	return refs.Weapons.ByID(string(weapon.ID))
}

func equippedWeapon(c *Character, slot InventorySlot) (*weapons.Weapon, bool, error) {
	if slot == "" {
		return nil, false, rpgerr.New(rpgerr.CodeInvalidArgument, "no equipment slot named")
	}

	equipped := c.GetEquippedSlot(slot)
	if equipped == nil {
		itemID := c.equipmentSlots.Get(slot)
		if itemID == "" {
			switch slot {
			case SlotMainHand, SlotOffHand:
				weapon := weapons.SpecialWeapons[weapons.UnarmedStrike]
				return &weapon, true, nil
			default:
				return nil, false, rpgerr.Newf(rpgerr.CodeInvalidArgument, "%q holds no weapon", slot)
			}
		}
		return nil, false, rpgerr.Newf(
			rpgerr.CodeInvalidArgument, "%q names %q which is not in the inventory", slot, itemID)
	}

	weapon := equipped.AsWeapon()
	if weapon == nil {
		return nil, false, rpgerr.Newf(rpgerr.CodeInvalidArgument, "%q holds no weapon", slot)
	}
	return weapon, false, nil
}

func attackAbility(c *Character, weapon *weapons.Weapon) abilities.Ability {
	if weapon.HasProperty(weapons.PropertyFinesse) {
		if c.GetAbilityModifier(abilities.DEX) > c.GetAbilityModifier(abilities.STR) {
			return abilities.DEX
		}
		return abilities.STR
	}
	if weapon.IsRanged() {
		return abilities.DEX
	}
	return abilities.STR
}

func copyDamagePools(pools []damage.Damage) []damage.Damage {
	copy := make([]damage.Damage, len(pools))
	for index, pool := range pools {
		copy[index] = pool
		copy[index].Properties = append([]damage.Property(nil), pool.Properties...)
	}
	return copy
}

func copyRef(ref *core.Ref) *core.Ref {
	if ref == nil {
		return nil
	}
	copy := *ref
	return &copy
}
