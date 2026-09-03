// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func intPtr(v int) *int { return &v }

// validRollCalculation builds the representative sourced calculation with
// canonical string refs: a 2d6 greatsword trace whose die 0 was rerolled 1→4
// by Great Weapon Fighting, a +3 Strength modifier, and a participating zero
// Dexterity modifier, totaling 12. Every mutation test below starts from this
// and breaks exactly one fact, so a case that fails for the wrong reason
// cannot hide behind the others.
func validRollCalculation() *encounter.RollCalculation {
	zero := 0
	return &encounter.RollCalculation{
		Components: []encounter.RollComponent{
			{
				Source: encounter.RollSource{Ref: "dnd5e:weapons:greatsword", Name: "Greatsword"},
				Dice: &encounter.DiceTrace{
					Notation:      "2d6",
					DieSize:       6,
					OriginalRolls: []int{1, 5},
					Rerolls: []encounter.DiceReroll{{
						DieIndex: 0,
						Before:   1,
						After:    4,
						Source: encounter.RollSource{
							Ref:  "dnd5e:conditions:fighting_style_great_weapon_fighting",
							Name: "Great Weapon Fighting",
						},
					}},
					FinalRolls: []int{4, 5},
					Subtotal:   9,
				},
			},
			{
				Source:   encounter.RollSource{Ref: "dnd5e:abilities:strength", Name: "Strength"},
				Modifier: intPtr(3),
			},
			{
				Source:   encounter.RollSource{Ref: "dnd5e:abilities:dexterity", Name: "Dexterity"},
				Modifier: &zero,
			},
		},
		Total: 12,
	}
}

func TestRollCalculationValid(t *testing.T) {
	t.Run("representative trace with participating zero modifier", func(t *testing.T) {
		require.NoError(t, encounter.ValidateRollCalculation(validRollCalculation()))
	})

	t.Run("explicit single-die spelling", func(t *testing.T) {
		calc := validRollCalculation()
		calc.Components[0].Dice.Notation = "1d6"
		calc.Components[0].Dice.OriginalRolls = []int{5}
		calc.Components[0].Dice.Rerolls = nil
		calc.Components[0].Dice.FinalRolls = []int{5}
		calc.Components[0].Dice.Subtotal = 5
		calc.Total = 8

		require.NoError(t, encounter.ValidateRollCalculation(calc))
	})

	t.Run("ordered rerolls and kept dice", func(t *testing.T) {
		calc := validRollCalculation()
		calc.Components[0].Dice.Rerolls = append(calc.Components[0].Dice.Rerolls, encounter.DiceReroll{
			DieIndex: 0,
			Before:   4,
			After:    6,
			Source: encounter.RollSource{
				Ref:  "dnd5e:conditions:fighting_style_great_weapon_fighting",
				Name: "Great Weapon Fighting",
			},
		})
		calc.Components[0].Dice.FinalRolls = []int{6, 5}
		calc.Components[0].Dice.KeptIndices = []int{0}
		calc.Components[0].Dice.Subtotal = 6
		calc.Total = 9

		require.NoError(t, encounter.ValidateRollCalculation(calc))
	})

	t.Run("dice and modifier on one component", func(t *testing.T) {
		calc := validRollCalculation()
		calc.Components[0].Modifier = intPtr(2)
		calc.Total = 14

		require.NoError(t, encounter.ValidateRollCalculation(calc))
	})

	t.Run("negative modifier", func(t *testing.T) {
		calc := validRollCalculation()
		calc.Components[1].Modifier = intPtr(-3)
		calc.Total = 6

		require.NoError(t, encounter.ValidateRollCalculation(calc))
	})
}

func TestRollCalculationValidation(t *testing.T) {
	tests := []struct {
		name   string
		change func(*encounter.RollCalculation)
	}{
		{
			name: "calculation has no components",
			change: func(calc *encounter.RollCalculation) {
				*calc = encounter.RollCalculation{}
			},
		},
		{
			name: "invalid notation",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Notation = "not dice"
			},
		},
		{
			name: "die size does not match notation",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.DieSize = 8
			},
		},
		{
			name: "die size is not positive",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.DieSize = 0
			},
		},
		{
			name: "face is outside die range",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.OriginalRolls[0] = 0
			},
		},
		{
			name: "final face is outside die range",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.FinalRolls[0] = 9
			},
		},
		{
			name: "original face is outside die range with consistent reroll",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.OriginalRolls = []int{7, 5}
				calc.Components[0].Dice.Rerolls[0].Before = 7
				calc.Components[0].Dice.Rerolls[0].After = 4
				calc.Components[0].Dice.FinalRolls = []int{4, 5}
				calc.Components[0].Dice.Subtotal = 9
			},
		},
		{
			name: "original rolls are empty",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.OriginalRolls = nil
				calc.Components[0].Dice.Rerolls = nil
				calc.Components[0].Dice.FinalRolls = nil
				calc.Components[0].Dice.Subtotal = 0
				calc.Total = 3
			},
		},
		{
			name: "notation cardinality does not match rolls",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Notation = "1d6"
			},
		},
		{
			name: "signed negative notation",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Notation = "-d6"
				calc.Components[0].Dice.OriginalRolls = []int{5}
				calc.Components[0].Dice.Rerolls = nil
				calc.Components[0].Dice.FinalRolls = []int{5}
				calc.Components[0].Dice.Subtotal = 5
				calc.Total = 8
			},
		},
		{
			name: "signed composite notation",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Notation = "-d6+2d6"
			},
		},
		{
			name: "original and final cardinality differ",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.FinalRolls = []int{4}
			},
		},
		{
			name: "reroll index is outside rolls",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].DieIndex = 2
			},
		},
		{
			name: "reroll index is negative",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].DieIndex = -1
			},
		},
		{
			name: "reroll before does not match current face",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].Before = 2
			},
		},
		{
			name: "reroll after is outside die range",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].After = 7
			},
		},
		{
			name: "reroll after is not propagated to final rolls",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.FinalRolls[0] = 3
			},
		},
		{
			name: "kept index is duplicated",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.KeptIndices = []int{0, 0}
			},
		},
		{
			name: "kept index is outside final rolls",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.KeptIndices = []int{2}
			},
		},
		{
			name: "kept index is negative",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.KeptIndices = []int{-1}
			},
		},
		{
			name: "subtotal does not equal kept faces",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Subtotal = 8
			},
		},
		{
			name: "component source ref is missing",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Source.Ref = ""
			},
		},
		{
			name: "component source ref has too few segments",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Source.Ref = "dnd5e:weapons"
			},
		},
		{
			name: "component source ref has too many segments",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Source.Ref = "dnd5e:weapons:greatsword:v2"
			},
		},
		{
			name: "component source ref has invalid characters",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Source.Ref = "dnd5e:weapons:great sword"
			},
		},
		{
			name: "component source name is missing",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Source.Name = ""
			},
		},
		{
			name: "component source name is whitespace",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Source.Name = "   "
			},
		},
		{
			name: "reroll source ref is missing",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].Source.Ref = ""
			},
		},
		{
			name: "reroll source name is missing",
			change: func(calc *encounter.RollCalculation) {
				calc.Components[0].Dice.Rerolls[0].Source.Name = ""
			},
		},
		{
			name: "component has neither dice nor modifier",
			change: func(calc *encounter.RollCalculation) {
				calc.Components = append(calc.Components, encounter.RollComponent{
					Source: encounter.RollSource{Ref: "dnd5e:abilities:constitution", Name: "Constitution"},
				})
			},
		},
		{
			name: "total does not equal component results",
			change: func(calc *encounter.RollCalculation) {
				calc.Total = 11
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := validRollCalculation()
			tt.change(calc)

			require.Error(t, encounter.ValidateRollCalculation(calc))
		})
	}

	t.Run("nil pointer", func(t *testing.T) {
		require.Error(t, encounter.ValidateRollCalculation(nil))
	})
}

// TestRollCalculationPersistenceShape pins the JSON the encounter writes for
// a calculation: canonical ref strings, snake_case keys, and pointer presence
// semantics — a participating zero modifier survives as `"modifier":0` while
// an absent one writes no key at all.
func TestRollCalculationPersistenceShape(t *testing.T) {
	t.Run("dice component omits the absent modifier", func(t *testing.T) {
		component := encounter.RollComponent{
			Source: encounter.RollSource{Ref: "dnd5e:weapons:greatsword", Name: "Greatsword"},
			Dice: &encounter.DiceTrace{
				Notation:      "1d8",
				DieSize:       8,
				OriginalRolls: []int{4},
				FinalRolls:    []int{4},
				Subtotal:      4,
			},
		}

		b, err := json.Marshal(component)
		require.NoError(t, err)
		require.JSONEq(t, `{
			"source": {"ref": "dnd5e:weapons:greatsword", "name": "Greatsword"},
			"dice": {
				"notation": "1d8", "die_size": 8,
				"original_rolls": [4], "final_rolls": [4], "subtotal": 4
			}
		}`, string(b))
	})

	t.Run("modifier component keeps a participating zero", func(t *testing.T) {
		zero := 0
		component := encounter.RollComponent{
			Source:   encounter.RollSource{Ref: "dnd5e:abilities:dexterity", Name: "Dexterity"},
			Modifier: &zero,
		}

		b, err := json.Marshal(component)
		require.NoError(t, err)
		require.JSONEq(t, `{
			"source": {"ref": "dnd5e:abilities:dexterity", "name": "Dexterity"},
			"modifier": 0
		}`, string(b))
	})

	t.Run("rerolls and kept indices persist when present", func(t *testing.T) {
		calc := validRollCalculation()
		calc.Components[0].Dice.KeptIndices = []int{0}

		b, err := json.Marshal(calc)
		require.NoError(t, err)
		require.Contains(t, string(b), `"kept_indices":[0]`)
		require.Contains(t, string(b),
			`"rerolls":[{"die_index":0,"before":1,"after":4,"source":`+
				`{"ref":"dnd5e:conditions:fighting_style_great_weapon_fighting","name":"Great Weapon Fighting"}}]`)
	})
}

// TestRollTraceCarrierShapes pins the field sets of the neutral carriers so a
// new field needs an argument here, where its persistence meaning is decided.
func TestRollTraceCarrierShapes(t *testing.T) {
	require.Equal(t, []string{"Ref", "Name", "Label"}, structFieldNames(encounter.RollSource{}))
	require.Equal(t, []string{"DieIndex", "Before", "After", "Source"}, structFieldNames(encounter.DiceReroll{}))
	require.Equal(t, []string{
		"Notation", "DieSize", "OriginalRolls", "Rerolls", "FinalRolls", "KeptIndices", "Subtotal",
	}, structFieldNames(encounter.DiceTrace{}))
	require.Equal(t, []string{"Source", "Dice", "Modifier"}, structFieldNames(encounter.RollComponent{}))
	require.Equal(t, []string{"Components", "Total"}, structFieldNames(encounter.RollCalculation{}))
	require.Equal(t, []string{"Source", "Roll", "DamageType", "Multiplier"},
		structFieldNames(encounter.DamageComponent{}),
		"a damage component carries its category, its roll facts, its damage type, and its multiplier — nothing else")
}
