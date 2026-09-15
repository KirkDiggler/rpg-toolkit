// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
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

	return assembleWeaponAttack(c, weapon, unarmed, in)
}

// assembleWeaponAttack hands the character's own numbers to the shared
// assembly. The character is the [weaponattack.Wielder] — it already answers
// all three of that interface's questions — and the one thing only this
// package can work out, what the other hand holds, is resolved here from the
// equipment slot and passed down.
func assembleWeaponAttack(
	c *Character,
	weapon *weapons.Weapon,
	unarmed bool,
	in *AssembleAttackInput,
) (combatActions.Definition, error) {
	return weaponattack.Assemble(&weaponattack.Input{
		Wielder:          c,
		Weapon:           weapon,
		TwoHanded:        in.TwoHanded,
		Cost:             in.Cost,
		OffHandWeaponRef: otherHandWeaponRef(c, in.Slot),
		AlwaysProficient: unarmed,
	})
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
