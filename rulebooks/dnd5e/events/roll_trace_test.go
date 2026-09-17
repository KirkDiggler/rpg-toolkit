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
			Source: RollSource{
				Ref: testRef(refs.Weapons.Greatsword()), Name: "Greatsword", SourceID: "hero-1",
			},
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

func TestRollCalculationValidDiceAndModifierOnOneComponent(t *testing.T) {
	calc := validRollCalculation()
	calc.Components[0].Modifier = intPtr(2)
	calc.Total = 14

	require.NoError(t, ValidateRollCalculation(calc))
}

func TestRollCalculationValidNegativeModifier(t *testing.T) {
	calc := validRollCalculation()
	calc.Components[1].Modifier = intPtr(-3)
	calc.Total = 6

	require.NoError(t, ValidateRollCalculation(calc))
}

func TestRollCalculationValidSubtractiveDiceAndSignedModifier(t *testing.T) {
	calc := &RollCalculation{Components: []RollComponent{
		{
			Source: RollSource{Ref: refs.Actions.Strike(), Name: "Strike", SourceID: "hero-1"},
			Dice: &DiceTrace{Notation: "1d20", DieSize: 20,
				OriginalRolls: []int{14}, FinalRolls: []int{14}, Subtotal: 14},
		},
		{
			Source:   RollSource{Ref: refs.Abilities.Charisma(), Name: "Charisma"},
			Modifier: intPtr(4),
		},
		{
			Source: RollSource{Ref: refs.Spells.Bane(), Name: "Bane", SourceID: "bard-a"},
			Dice: &DiceTrace{Notation: "1d4", DieSize: 4,
				OriginalRolls: []int{3}, FinalRolls: []int{3}, Subtotal: 3},
			Modifier: intPtr(-2), SubtractDice: true,
		},
	}, Total: 13}

	require.NoError(t, ValidateRollCalculation(calc))
}

func TestRollCalculationRejectsInvalidSubtractiveDice(t *testing.T) {
	tests := []struct {
		name   string
		change func(*RollCalculation)
	}{
		{
			name: "subtract without dice",
			change: func(calc *RollCalculation) {
				calc.Components[1].SubtractDice = true
			},
		},
		{
			name: "subtractive modification without responsible entity",
			change: func(calc *RollCalculation) {
				calc.Components[0] = RollComponent{
					Source: RollSource{Ref: refs.Spells.Bless(), Name: "Bless"},
					Dice: &DiceTrace{Notation: "1d4", DieSize: 4,
						OriginalRolls: []int{2}, FinalRolls: []int{2}, Subtotal: 2},
					SubtractDice: true,
				}
				calc.Total = 1
			},
		},
		{
			name: "total ignores subtraction",
			change: func(calc *RollCalculation) {
				calc.Components[0] = RollComponent{
					Source: RollSource{Ref: refs.Spells.Bane(), Name: "Bane", SourceID: "bard-a"},
					Dice: &DiceTrace{Notation: "1d4", DieSize: 4,
						OriginalRolls: []int{2}, FinalRolls: []int{2}, Subtotal: 2},
					SubtractDice: true,
				}
				calc.Total = 5
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calc := validRollCalculation()
			test.change(calc)
			require.Error(t, ValidateRollCalculation(calc))
		})
	}
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
			name: "zero face is outside die range",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.OriginalRolls[0] = 0
			},
		},
		{
			name: "negative face is outside die range",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.OriginalRolls[0] = -1
			},
		},
		{
			name: "final face is outside die range",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.FinalRolls[0] = 9
			},
		},
		{
			name: "original face is outside die range with consistent reroll",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.OriginalRolls = []int{7, 5}
				calc.Components[0].Dice.Rerolls[0].Before = 7
				calc.Components[0].Dice.Rerolls[0].After = 4
				calc.Components[0].Dice.FinalRolls = []int{4, 5}
				calc.Components[0].Dice.Subtotal = 9
			},
		},
		{
			name: "original rolls are empty",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.OriginalRolls = nil
				calc.Components[0].Dice.Rerolls = nil
				calc.Components[0].Dice.FinalRolls = nil
				calc.Components[0].Dice.Subtotal = 0
				calc.Total = 3
			},
		},
		{
			name: "notation cardinality does not match rolls",
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Notation = "1d6"
			},
		},
		{
			name: "signed negative notation",
			change: func(calc *RollCalculation) {
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
			change: func(calc *RollCalculation) {
				calc.Components[0].Dice.Notation = "-d6+2d6"
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

// keepSource is a well-formed keep source: a rule, and the entity that brought
// it. Every field is required — a record that cannot name who brought the rule
// cannot be rendered.
func keepSource(name string, ref *core.Ref, entity string) RollSource {
	return RollSource{Ref: testRef(ref), Name: name, SourceID: entity}
}

// advantageCalculation is a valid 2d20 pool kept under advantage: the higher
// face counts, one kept index, one granting source, nothing imposed.
func advantageCalculation() *RollCalculation {
	return &RollCalculation{Components: []RollComponent{{
		Source: RollSource{Ref: testRef(refs.Actions.Strike()), Name: "Strike", SourceID: "hero"},
		Dice: &DiceTrace{
			Notation: "2d20", DieSize: 20,
			OriginalRolls: []int{7, 18}, FinalRolls: []int{7, 18},
			KeptIndices: []int{1}, Subtotal: 18,
			Keep: &DiceKeep{
				Rule: KeepAdvantage,
				Granted: []RollSource{
					keepSource("Reckless Attack", refs.Conditions.RecklessAttack(), "hero"),
				},
			},
		},
	}}, Total: 18}
}

func TestRollCalculationValidAdvantageKeep(t *testing.T) {
	require.NoError(t, ValidateRollCalculation(advantageCalculation()))
}

func TestRollCalculationValidDisadvantageKeep(t *testing.T) {
	calc := advantageCalculation()
	calc.Components[0].Dice.KeptIndices = []int{0}
	calc.Components[0].Dice.Subtotal = 7
	calc.Components[0].Dice.Keep = &DiceKeep{
		Rule:    KeepDisadvantage,
		Imposed: []RollSource{keepSource("Untrained", refs.Rules.Untrained(), "hero")},
	}
	calc.Total = 7

	require.NoError(t, ValidateRollCalculation(calc))
}

// TestRollCalculationValidCancelledKeep is the case the log could never show:
// one die, no kept indices, and a record naming the two rules that met.
func TestRollCalculationValidCancelledKeep(t *testing.T) {
	calc := advantageCalculation()
	calc.Components[0].Dice.Notation = "1d20"
	calc.Components[0].Dice.OriginalRolls = []int{11}
	calc.Components[0].Dice.FinalRolls = []int{11}
	calc.Components[0].Dice.KeptIndices = nil
	calc.Components[0].Dice.Subtotal = 11
	calc.Components[0].Dice.Keep = &DiceKeep{
		Rule:    KeepCancelled,
		Granted: []RollSource{keepSource("Help", refs.Conditions.Helped(), "alice")},
		Imposed: []RollSource{keepSource("Untrained", refs.Rules.Untrained(), "hero")},
	}
	calc.Total = 11

	require.NoError(t, ValidateRollCalculation(calc))
}

// TestRollCalculationRefusesAWrongKeepRecord is the fail-closed half of R1: a
// builder that fills the record by hand and gets it wrong is refused at the
// seam rather than rendered wrong.
func TestRollCalculationRefusesAWrongKeepRecord(t *testing.T) {
	tests := []struct {
		name   string
		change func(*DiceTrace)
	}{
		{
			name: "advantage on a one-die pool",
			change: func(trace *DiceTrace) {
				trace.Notation = "1d20"
				trace.OriginalRolls = []int{18}
				trace.FinalRolls = []int{18}
				trace.KeptIndices = []int{0}
			},
		},
		{
			name: "advantage kept the lower face",
			change: func(trace *DiceTrace) {
				trace.KeptIndices = []int{0}
				trace.Subtotal = 7
			},
		},
		{
			name: "advantage kept two faces",
			change: func(trace *DiceTrace) {
				trace.KeptIndices = []int{0, 1}
				trace.Subtotal = 25
			},
		},
		{
			name: "advantage nobody granted",
			change: func(trace *DiceTrace) {
				trace.Keep.Granted = nil
			},
		},
		{
			name: "advantage with something imposed is a cancellation",
			change: func(trace *DiceTrace) {
				trace.Keep.Imposed = []RollSource{
					keepSource("Untrained", refs.Rules.Untrained(), "hero"),
				}
			},
		},
		{
			name: "disadvantage kept the higher face",
			change: func(trace *DiceTrace) {
				trace.Keep = &DiceKeep{
					Rule:    KeepDisadvantage,
					Imposed: []RollSource{keepSource("Untrained", refs.Rules.Untrained(), "hero")},
				}
			},
		},
		{
			// A cancellation recorded over a PAIR is the dangerous shape: no
			// kept indices means every face counts, so the subtotal is both
			// dice added together and the "cancelled" label hides an
			// advantage or disadvantage whose keep decision went unrecorded.
			name: "cancelled over a pair of dice",
			change: func(trace *DiceTrace) {
				trace.KeptIndices = nil
				trace.Subtotal = 25
				trace.Keep = &DiceKeep{
					Rule:    KeepCancelled,
					Granted: []RollSource{keepSource("Help", refs.Conditions.Helped(), "alice")},
					Imposed: []RollSource{keepSource("Untrained", refs.Rules.Untrained(), "hero")},
				}
			},
		},
		{
			name: "cancelled with nothing imposed",
			change: func(trace *DiceTrace) {
				trace.KeptIndices = nil
				trace.Subtotal = 25
				trace.Keep = &DiceKeep{
					Rule:    KeepCancelled,
					Granted: []RollSource{keepSource("Help", refs.Conditions.Helped(), "alice")},
				}
			},
		},
		{
			name: "cancelled that still kept a face",
			change: func(trace *DiceTrace) {
				trace.Keep = &DiceKeep{
					Rule:    KeepCancelled,
					Granted: []RollSource{keepSource("Help", refs.Conditions.Helped(), "alice")},
					Imposed: []RollSource{keepSource("Untrained", refs.Rules.Untrained(), "hero")},
				}
			},
		},
		{
			name: "a rule nobody has heard of",
			change: func(trace *DiceTrace) {
				trace.Keep.Rule = "lucky"
			},
		},
		{
			name: "an empty rule",
			change: func(trace *DiceTrace) {
				trace.Keep.Rule = ""
			},
		},
		{
			name: "a rule brought by nobody",
			change: func(trace *DiceTrace) {
				trace.Keep.Granted[0].SourceID = ""
			},
		},
		{
			name: "a rule with no ref to name it",
			change: func(trace *DiceTrace) {
				trace.Keep.Granted[0].Ref = nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calc := advantageCalculation()
			test.change(calc.Components[0].Dice)
			require.Error(t, ValidateRollCalculation(calc))
		})
	}
}

// TestRollCalculationRefusesAnAnonymousDicePool is R7 stated as a refusal:
// every dice pool names the entity whose rule threw it, and a roll with no
// entity behind it does not exist in this game.
func TestRollCalculationRefusesAnAnonymousDicePool(t *testing.T) {
	calc := advantageCalculation()
	calc.Components[0].Source.SourceID = ""

	require.ErrorContains(t, ValidateRollCalculation(calc), "dice source id is required")
}

// TestCloneRollCalculationCopiesTheKeepRecord pins that a clone is a clone:
// scribbling on the copy's keep record cannot reach the original.
func TestCloneRollCalculationCopiesTheKeepRecord(t *testing.T) {
	calc := advantageCalculation()

	clone := CloneRollCalculation(calc)
	require.NotNil(t, clone.Components[0].Dice.Keep)
	require.Equal(t, KeepAdvantage, clone.Components[0].Dice.Keep.Rule)
	require.Len(t, clone.Components[0].Dice.Keep.Granted, 1)

	clone.Components[0].Dice.Keep.Rule = KeepDisadvantage
	clone.Components[0].Dice.Keep.Granted[0].Name = "scribbled"
	clone.Components[0].Dice.Keep.Granted[0].Ref.ID = "scribbled"

	require.Equal(t, KeepAdvantage, calc.Components[0].Dice.Keep.Rule)
	require.Equal(t, "Reckless Attack", calc.Components[0].Dice.Keep.Granted[0].Name)
	require.Equal(t, refs.Conditions.RecklessAttack().ID, calc.Components[0].Dice.Keep.Granted[0].Ref.ID)
}
