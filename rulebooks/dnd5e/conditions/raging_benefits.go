// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

// RagingBenefitsInput selects the existing fixed or live-level Rage damage
// strategy. It does not select activation eligibility, maintenance or identity.
type RagingBenefitsInput struct{ DamageBonus RagingDamageBonus }

// RagingBenefitsOutput owns four typed benefit parts with no lifetime policies.
// They may be composed together or replaced by individually authored effects.
type RagingBenefitsOutput struct{ Parts []effects.PartSpec }

// RagingBenefits returns the shared native Rage benefit declarations, not an
// opaque Raging condition. Nil input, missing/competing strategies and a
// nonpositive fixed bonus are errors. Each call owns its amount/guard/slice data;
// it binds no actor, reads no level, rolls nothing and creates no runtime.
func RagingBenefits(in *RagingBenefitsInput) (*RagingBenefitsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("raging benefits input is required")
	}
	amount, err := ragingDamageAmount(in.DamageBonus)
	if err != nil {
		return nil, err
	}
	return &RagingBenefitsOutput{Parts: ragingBenefitParts(amount)}, nil
}

func ragingDamageAmount(choice RagingDamageBonus) (generic.Amount, error) {
	if (choice.Fixed != nil) == choice.BarbarianLevel {
		return generic.Amount{}, fmt.Errorf("raging damage_bonus requires exactly one fixed or barbarian_level strategy")
	}
	if choice.Fixed != nil {
		if *choice.Fixed <= 0 {
			return generic.Amount{}, fmt.Errorf("raging fixed damage bonus must be positive")
		}
		fixed := *choice.Fixed
		return generic.Amount{Fixed: &fixed}, nil
	}
	return generic.Amount{ByClassLevel: &generic.ClassSteps{Class: string(classes.Barbarian), Steps: map[int]int{1: 2, 9: 3, 16: 4}}}, nil
}

// The named bundle and native recipe both read this one benefit declaration.
// Each leaf owns its guard; editing the save choice cannot alter the check.
func ragingBenefitParts(amount generic.Amount) []effects.PartSpec {
	fact := func(name machine.FactName, value any) machine.Guard {
		return machine.Guard{Fact: &machine.FactLeaf{Name: name, Op: machine.OpEq, Value: value}}
	}
	saveStrength := fact(effects.FactAbility, string(abilities.STR))
	checkStrength := fact(effects.FactAbility, string(abilities.STR))
	ownedAmount := generic.Output{Amount: &amount}.Clone().Amount
	return []effects.PartSpec{
		{Key: "damage", Effect: effects.DamageBonus{Amount: *ownedAmount, When: &machine.Guard{All: []machine.Guard{fact(effects.FactMelee, true), fact(effects.FactWeaponPool, true), fact(effects.FactAbility, string(abilities.STR))}}}, OnTerminal: generic.EndPart},
		{Key: "physical-resistance", Effect: effects.Resistance{Types: []damage.Type{damage.Bludgeoning, damage.Piercing, damage.Slashing}}, OnTerminal: generic.EndPart},
		{Key: "strength-saves", Effect: effects.Advantage{On: effects.AdvantageOnSavingThrows, When: &saveStrength}, OnTerminal: generic.EndPart},
		{Key: "strength-checks", Effect: effects.Advantage{On: effects.AdvantageOnAbilityChecks, When: &checkStrength}, OnTerminal: generic.EndPart},
	}
}
