// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// AssembleOffHandAttackInput supplies the compiled price for a two-weapon
// bonus attack. Equipment and attack evidence come from the character.
type AssembleOffHandAttackInput struct {
	Cost *combat.SpendProfile
}

// CanMakeOffHandAttack reports whether the character currently wields a light
// melee weapon in each hand.
func CanMakeOffHandAttack(c *Character) bool {
	if c == nil {
		return false
	}

	return hasLightMeleeWeapon(c, SlotMainHand) && hasLightMeleeWeapon(c, SlotOffHand)
}

// AssembleOffHandAttack compiles the off-hand weapon as the bonus attack
// granted by two-weapon fighting.
func AssembleOffHandAttack(
	c *Character, input *AssembleOffHandAttackInput,
) (combatActions.Definition, error) {
	if c == nil {
		return combatActions.Definition{}, rpgerr.New(rpgerr.CodeNil, "no character to assemble a two-weapon attack for")
	}
	if input == nil {
		return combatActions.Definition{}, rpgerr.New(rpgerr.CodeNil, "no two-weapon attack input")
	}
	if !CanMakeOffHandAttack(c) {
		return combatActions.Definition{}, rpgerr.New(
			rpgerr.CodeInvalidArgument, "two-weapon attack requires two light melee weapons",
		)
	}

	definition, err := AssembleAttack(c, &AssembleAttackInput{Slot: SlotOffHand, Cost: input.Cost})
	if err != nil {
		return combatActions.Definition{}, err
	}
	definition.Attack.IsOffHandAttack = true
	if err := definition.Validate(); err != nil {
		return combatActions.Definition{}, rpgerr.Wrap(err, "assembled two-weapon attack is invalid")
	}

	return definition, nil
}

func hasLightMeleeWeapon(c *Character, slot InventorySlot) bool {
	equipped := c.GetEquippedSlot(slot)
	if equipped == nil {
		return false
	}
	weapon := equipped.AsWeapon()
	return weapon != nil && weapon.IsMelee() && weapon.HasProperty(weapons.PropertyLight)
}
