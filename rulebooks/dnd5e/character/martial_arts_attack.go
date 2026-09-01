// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// AssembleMartialArtsBonusAttackInput supplies the compiled price for a
// Martial Arts bonus unarmed strike.
type AssembleMartialArtsBonusAttackInput struct {
	Cost *combat.SpendProfile
}

func validateMartialArtsBonusAttack(c *Character) error {
	if !CanMakeMartialArtsBonusAttack(c) {
		return rpgerr.New(
			rpgerr.CodeInvalidArgument,
			"Martial Arts bonus attack requires an eligible unarmored Martial Arts attack",
		)
	}
	return nil
}

// CostOfMartialArtsBonusAttack compiles the price of a Martial Arts bonus
// unarmed strike.
func CostOfMartialArtsBonusAttack(c *Character) (*combat.SpendProfile, error) {
	if c == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no character to price a Martial Arts bonus attack for")
	}
	if err := validateMartialArtsBonusAttack(c); err != nil {
		return nil, err
	}
	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{
			coreCombat.ActionBonus: 1,
		},
		Capacity: map[combat.CapacityType]int{
			combat.CapacityMartialArtsBonusAttack: 1,
		},
	}, nil
}

// AssembleMartialArtsBonusAttack compiles the inert Unarmed Strike definition
// granted by Martial Arts.
func AssembleMartialArtsBonusAttack(
	c *Character, input *AssembleMartialArtsBonusAttackInput,
) (combatActions.Definition, error) {
	if c == nil {
		return combatActions.Definition{}, rpgerr.New(rpgerr.CodeNil, "no character to assemble a Martial Arts bonus attack for")
	}
	if input == nil {
		return combatActions.Definition{}, rpgerr.New(rpgerr.CodeNil, "no Martial Arts bonus attack input")
	}
	if err := validateMartialArtsBonusAttack(c); err != nil {
		return combatActions.Definition{}, err
	}
	unarmed := weapons.SpecialWeapons[weapons.UnarmedStrike]
	return assembleWeaponAttack(c, &unarmed, true, &AssembleAttackInput{Cost: input.Cost})
}
