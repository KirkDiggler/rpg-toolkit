// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package definitions

import (
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

// RagingDamageBonus selects exactly one positive fixed or live Barbarian-level
// strategy. Live scaling never stores a copied actor level in the definition.
type RagingDamageBonus struct {
	Fixed          *int `json:"fixed,omitempty"`
	BarbarianLevel bool `json:"barbarian_level,omitempty"`
}

// RagingInput supplies configuration, not activation eligibility or participants.
type RagingInput struct{ DamageBonus RagingDamageBonus }

// RagingDefinition exposes both reusable groups of the complete named content.
// Each constructor call owns its nested values; compilation takes its own copy.
type RagingDefinition struct {
	Benefits    RagingBenefits
	Maintenance RagingMaintenance
}

// RagingBenefits exposes individually reusable configured effects. Whole-group
// projection reads these exact fields in stable declaration order.
type RagingBenefits struct {
	DamageBonus          effects.DamageBonus
	PhysicalResistance   effects.Resistance
	StrengthSavingThrows effects.Advantage
	StrengthChecks       effects.Advantage
}

// RagingMaintenance exposes native upkeep without imposing it on borrowed benefits.
// Its order deliberately gives inactivity priority over clock endings.
type RagingMaintenance struct {
	CombatActivity effects.CombatActivity
	Duration       effects.RoundDuration
	Endings        RagingEndings
}

// RagingEndings names ordinary holder-event policies, not a private state runner.
type RagingEndings struct {
	ZeroHP    effects.EndOn
	Rest      effects.EndOn
	CombatEnd effects.EndOn
}

// Raging creates the complete unbound definition. Missing/competing strategies or
// nonpositive fixed bonuses refuse; no actor, catalogue or runtime is fabricated.
func Raging(in *RagingInput) (*RagingDefinition, error) {
	if in == nil {
		return nil, fmt.Errorf("raging definition input is required")
	}
	choice := in.DamageBonus
	if (choice.Fixed != nil) == choice.BarbarianLevel {
		return nil, fmt.Errorf("raging damage_bonus requires exactly one fixed or barbarian_level strategy")
	}
	var amount generic.Amount
	if choice.Fixed != nil {
		if *choice.Fixed <= 0 {
			return nil, fmt.Errorf("raging fixed damage bonus must be positive")
		}
		fixed := *choice.Fixed
		amount.Fixed = &fixed
	} else {
		amount.ByClassLevel = &generic.ClassSteps{Class: string(classes.Barbarian), Steps: map[int]int{1: 2, 9: 3, 16: 4}}
	}
	fact := func(name machine.FactName, value any) machine.Guard {
		return machine.Guard{Fact: &machine.FactLeaf{Name: name, Op: machine.OpEq, Value: value}}
	}
	saveStrength, checkStrength := fact(effects.FactAbility, string(abilities.STR)), fact(effects.FactAbility, string(abilities.STR))
	return &RagingDefinition{
		Benefits: RagingBenefits{
			DamageBonus:          effects.DamageBonus{Amount: amount, When: &machine.Guard{All: []machine.Guard{fact(effects.FactMelee, true), fact(effects.FactWeaponPool, true), fact(effects.FactAbility, string(abilities.STR))}}},
			PhysicalResistance:   effects.Resistance{Types: []damage.Type{damage.Bludgeoning, damage.Piercing, damage.Slashing}},
			StrengthSavingThrows: effects.Advantage{On: effects.AdvantageOnSavingThrows, When: &saveStrength},
			StrengthChecks:       effects.Advantage{On: effects.AdvantageOnAbilityChecks, When: &checkStrength},
		},
		Maintenance: RagingMaintenance{
			CombatActivity: effects.CombatActivity{SkipFirst: true},
			Duration:       effects.RoundDuration{Rounds: 10, ExpiredReason: "duration_expired", ResetReason: "clock_reset"},
			Endings:        RagingEndings{ZeroHP: effects.EndOn{Beat: effects.BeatDroppedToZero, Reason: "unconscious"}, Rest: effects.EndOn{Beat: effects.BeatRest, Reason: "rest"}, CombatEnd: effects.EndOn{Beat: effects.BeatCombatEnd, Reason: "combat_ended"}},
		},
	}, nil
}

// DefinitionData projects the complete definition from its actual group fields.
func (d RagingDefinition) DefinitionData() (effects.DefinitionData, error) {
	return (effects.Composition{Members: []effects.Member{{Key: "benefits", Name: "Rage benefits", Definition: d.Benefits}, {Key: "maintenance", Name: "Rage upkeep", Definition: d.Maintenance}}}).DefinitionData()
}

// DefinitionData projects the four benefit fields without adding upkeep/identity.
func (b RagingBenefits) DefinitionData() (effects.DefinitionData, error) {
	return (effects.Composition{Members: []effects.Member{
		{Key: "damage-bonus", Name: "Rage damage", Description: "Strength-based melee weapon damage bonus", Definition: b.DamageBonus},
		{Key: "physical-resistance", Name: "Physical resistance", Description: "Bludgeoning, piercing and slashing resistance", Definition: b.PhysicalResistance},
		{Key: "strength-saving-throws", Name: "Strength saving throws", Description: "Advantage on Strength saves", Definition: b.StrengthSavingThrows},
		{Key: "strength-checks", Name: "Strength checks", Description: "Advantage on Strength ability checks", Definition: b.StrengthChecks},
	}}).DefinitionData()
}

// DefinitionData projects independent upkeep in its deliberate terminal order.
func (m RagingMaintenance) DefinitionData() (effects.DefinitionData, error) {
	return (effects.Composition{Members: []effects.Member{{Key: "combat-activity", Name: "Combat activity", Definition: m.CombatActivity}, {Key: "duration", Name: "Anchored round duration", Definition: m.Duration}, {Key: "endings", Name: "Early endings", Definition: m.Endings}}}).DefinitionData()
}

// DefinitionData projects the explicit ending policies from their actual fields.
func (e RagingEndings) DefinitionData() (effects.DefinitionData, error) {
	return (effects.Composition{Members: []effects.Member{{Key: "zero-hp", Name: "Dropped to zero", Definition: e.ZeroHP}, {Key: "rest", Name: "Rest", Definition: e.Rest}, {Key: "combat-end", Name: "Combat ended", Definition: e.CombatEnd}}}).DefinitionData()
}
