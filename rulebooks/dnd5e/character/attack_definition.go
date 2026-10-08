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
	override, err := weaponAttackOverride(c, in.Slot)
	if err != nil {
		return combatActions.Definition{}, err
	}
	return weaponattack.Assemble(&weaponattack.Input{
		Override:         override,
		Wielder:          c,
		Weapon:           weapon,
		TwoHanded:        in.TwoHanded,
		Cost:             in.Cost,
		OffHandWeaponRef: otherHandWeaponRef(c, in.Slot),
		Slot:             string(in.Slot),
		AlwaysProficient: unarmed,
	})
}

// weaponAttackOverride returns the one ability-or-die offer a loaded condition
// makes for the swing from slot, or nil when none does.
//
// Two offers for the same swing — a monk's Martial Arts and Shillelagh on the
// club in that hand — fail closed. Design R22 makes that pick the player's
// (rpg-project#535), and until the pick exists the assembly refuses rather
// than letting the order conditions sit on the sheet choose one, because an
// order-chosen offer is a die the player never picked.
//
// Each provider is handed this sheet as its level record, so a die that
// scales with a class (Martial Arts) reads the level at this swing. A provider
// that cannot answer fails the assembly.
func weaponAttackOverride(c *Character, slot InventorySlot) (*weaponattack.Override, error) {
	var override *weaponattack.Override
	var offeredBy []string
	in := &weaponattack.OverrideInput{Slot: string(slot), ItemID: c.equipmentSlots.Get(slot), Levels: c}
	for _, condition := range c.conditions {
		provider, ok := condition.(weaponattack.OverrideProvider)
		if !ok {
			continue
		}
		offer, err := provider.WeaponAttackOverride(in)
		if err != nil {
			return nil, rpgerr.Wrapf(err, "%s cannot offer for %q", condition.Ref(), slot)
		}
		if offer == nil || offer.Override == nil {
			continue
		}
		override = offer.Override
		offeredBy = append(offeredBy, condition.Ref().String())
	}
	if len(offeredBy) > 1 {
		return nil, rpgerr.Newf(rpgerr.CodeConflictingState,
			"%q has competing attack offers %v; the pick is the player's and none was taken (R22)",
			slot, offeredBy)
	}
	return override, nil
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
