// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func intPtr(v int) *int { return &v }

func testRef(ref *core.Ref) *core.Ref {
	clone := *ref
	return &clone
}

func validRollCalculation() *RollCalculation {
	zero := 0
	return &RollCalculation{Components: []RollComponent{
		{
			Source: RollSource{Ref: testRef(refs.Weapons.Greatsword()), Name: "Greatsword"},
			Dice: &DiceTrace{
				Notation:      "2d6",
				DieSize:       6,
				OriginalRolls: []int{1, 5},
				Rerolls: []DiceReroll{{
					DieIndex: 0,
					Before:   1,
					After:    4,
					Source: RollSource{
						Ref:  testRef(refs.Conditions.FightingStyleGreatWeaponFighting()),
						Name: "Great Weapon Fighting",
					},
				}},
				FinalRolls: []int{4, 5},
				Subtotal:   9,
			},
		},
		{
			Source:   RollSource{Ref: testRef(refs.Abilities.Strength()), Name: "Strength"},
			Modifier: intPtr(3),
		},
		{
			Source:   RollSource{Ref: testRef(refs.Abilities.Dexterity()), Name: "Dexterity"},
			Modifier: &zero,
		},
	}, Total: 12}
}

func TestRollCalculationValid(t *testing.T) {
	calc := validRollCalculation()

	require.NoError(t, ValidateRollCalculation(calc))
}

func TestRollCalculationValidExplicitSingleDieNotation(t *testing.T) {
	calc := validRollCalculation()
	calc.Components[0].Dice.Notation = "1d6"
	calc.Components[0].Dice.OriginalRolls = []int{5}
	calc.Components[0].Dice.Rerolls = nil
	calc.Components[0].Dice.FinalRolls = []int{5}
	calc.Components[0].Dice.Subtotal = 5
	calc.Total = 8

	require.NoError(t, ValidateRollCalculation(calc))
}

func TestRollCalculationValidOrderedRerollsAndKeptDice(t *testing.T) {
	calc := validRollCalculation()
	calc.Components[0].Dice.Rerolls = append(calc.Components[0].Dice.Rerolls, DiceReroll{
		DieIndex: 0,
		Before:   4,
		After:    6,
		Source: RollSource{
			Ref:  testRef(refs.Conditions.FightingStyleGreatWeaponFighting()),
			Name: "Great Weapon Fighting",
		},
	})
	calc.Components[0].Dice.FinalRolls = []int{6, 5}
	calc.Components[0].Dice.KeptIndices = []int{0}
	calc.Components[0].Dice.Subtotal = 6
	calc.Total = 9

	require.NoError(t, ValidateRollCalculation(calc))
}

func TestRollCalculationValidation(t *testing.T) {
	tests := []struct {
		name   string
		change func(*RollCalculation)
	}{
		{
			name: "calculation has no components",
			change: func(calc *RollCalculation) {
				*calc = RollCalculation{}
			},
		},
		{
			name: "invalid notation",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Notation = "not dice"
			},
		},
		{
			name: "die size does not match notation",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.DieSize = 8
			},
		},
		{
			name: "die size is not positive",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.DieSize = 0
			},
		},
		{
			name: "face is outside die range",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.OriginalRolls[0] = 0
			},
		},
		{
			name: "final face is outside die range",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.FinalRolls[0] = 9
			},
		},
		{
			name: "notation cardinality does not match rolls",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Notation = "1d6"
			},
		},
		{
			name: "original and final cardinality differ",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.FinalRolls = []int{4}
			},
		},
		{
			name: "reroll index is outside rolls",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].DieIndex = 2
			},
		},
		{
			name: "reroll index is negative",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].DieIndex = -1
			},
		},
		{
			name: "reroll before does not match current face",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].Before = 2
			},
		},
		{
			name: "reroll after is outside die range",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].After = 7
			},
		},
		{
			name: "reroll after is not propagated to final rolls",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.FinalRolls[0] = 3
			},
		},
		{
			name: "kept index is duplicated",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.KeptIndices = []int{0, 0}
			},
		},
		{
			name: "kept index is outside final rolls",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.KeptIndices = []int{2}
			},
		},
		{
			name: "kept index is negative",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.KeptIndices = []int{-1}
			},
		},
		{
			name: "subtotal does not equal kept faces",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Subtotal = 8
			},
		},
		{
			name: "component source ref is missing",
			change: func(calc *RollCalculation) {
				calc.Components[0].Source.Ref = nil
			},
		},
		{
			name: "component source ref is invalid",
			change: func(calc *RollCalculation) {
				calc.Components[0].Source.Ref = &core.Ref{Module: "dnd5e", Type: "weapons"}
			},
		},
		{
			name: "component source name is missing",
			change: func(calc *RollCalculation) {
				calc.Components[0].Source.Name = ""
			},
		},
		{
			name: "reroll source ref is missing",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].Source.Ref = nil
			},
		},
		{
			name: "reroll source name is missing",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].Source.Name = ""
			},
		},
		{
			name: "component has neither dice nor modifier",
			change: func(calc *RollCalculation) {
				calc.Components = append(calc.Components, RollComponent{
					Source: RollSource{Ref: refs.Abilities.Constitution(), Name: "Constitution"},
				})
			},
		},
		{
			name: "total does not equal component results",
			change: func(calc *RollCalculation) {
				calc.Total = 11
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := validRollCalculation()
			tt.change(calc)

			require.Error(t, ValidateRollCalculation(calc))
		})
	}

	t.Run("nil pointer", func(t *testing.T) {
		require.Error(t, ValidateRollCalculation(nil))
	})
}

func TestCloneRollCalculation(t *testing.T) {
	original := validRollCalculation()
	original.Components[0].Dice.KeptIndices = []int{0, 1}
	original.Components[1].Source.Label = "Ability modifier"

	clone := CloneRollCalculation(original)

	require.Equal(t, original, clone)
	require.NotSame(t, original, clone)
	require.NotSame(t, original.Components[0].Source.Ref, clone.Components[0].Source.Ref)
	require.NotSame(t, original.Components[0].Dice, clone.Components[0].Dice)
	require.NotSame(t, original.Components[0].Dice.Rerolls[0].Source.Ref,
		clone.Components[0].Dice.Rerolls[0].Source.Ref)
	require.NotSame(t, original.Components[1].Modifier, clone.Components[1].Modifier)
	require.NotSame(t, original.Components[2].Modifier, clone.Components[2].Modifier)

	original.Components[0].Source.Ref.Module = "changed"
	original.Components[0].Source.Name = "Changed"
	original.Components[0].Dice.OriginalRolls[0] = 6
	original.Components[0].Dice.Rerolls[0].Before = 6
	original.Components[0].Dice.Rerolls[0].Source.Ref.ID = "changed"
	original.Components[0].Dice.FinalRolls[0] = 6
	original.Components[0].Dice.KeptIndices[0] = 1
	*original.Components[1].Modifier = 10
	original.Components[1].Source.Label = "Changed"
	original.Components[1] = RollComponent{}
	*original.Components[2].Modifier = 5
	original.Components[2].Source.Name = "Changed"

	require.Equal(t, "dnd5e", clone.Components[0].Source.Ref.Module)
	require.Equal(t, "Greatsword", clone.Components[0].Source.Name)
	require.Equal(t, []int{1, 5}, clone.Components[0].Dice.OriginalRolls)
	require.Equal(t, 1, clone.Components[0].Dice.Rerolls[0].Before)
	require.Equal(t, "fighting_style_great_weapon_fighting",
		clone.Components[0].Dice.Rerolls[0].Source.Ref.ID)
	require.Equal(t, []int{4, 5}, clone.Components[0].Dice.FinalRolls)
	require.Equal(t, []int{0, 1}, clone.Components[0].Dice.KeptIndices)
	require.Equal(t, 3, *clone.Components[1].Modifier)
	require.Equal(t, "Strength", clone.Components[1].Source.Name)
	require.Equal(t, "Ability modifier", clone.Components[1].Source.Label)
	require.Equal(t, 0, *clone.Components[2].Modifier)
	require.Equal(t, "Dexterity", clone.Components[2].Source.Name)
}

func TestCloneRollCalculationNil(t *testing.T) {
	require.Nil(t, CloneRollCalculation(nil))
}
