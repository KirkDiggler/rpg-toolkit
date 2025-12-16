// Package saves implements D&D 5e saving throw mechanics
package saves

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
)

// SavingThrowInput contains all parameters needed to make a saving throw
type SavingThrowInput struct {
	// Ability is the ability score being tested (STR, DEX, CON, INT, WIS, CHA)
	Ability abilities.Ability

	// DC is the Difficulty Class that must be met or exceeded
	DC int

	// Modifier is the total bonus/penalty to add to the roll
	// (typically ability modifier + proficiency bonus if proficient)
	Modifier int

	// HasAdvantage indicates rolling two d20s and taking the higher result
	HasAdvantage bool

	// HasDisadvantage indicates rolling two d20s and taking the lower result
	// Note: If both HasAdvantage and HasDisadvantage are true, they cancel out
	// and a single d20 is rolled (D&D 5e rule)
	HasDisadvantage bool
}

// SavingThrowResult contains the outcome of a saving throw
type SavingThrowResult struct {
	// Roll is the actual d20 roll result used (highest/lowest if advantage/disadvantage)
	Roll int

	// Total is the final value (Roll + Modifier)
	Total int

	// DC is the Difficulty Class that was tested against
	DC int

	// Success indicates whether the save succeeded (Total >= DC)
	Success bool

	// IsNat1 indicates if the d20 roll was a natural 1
	// Note: Unlike attack rolls, natural 1s don't automatically fail saving throws in D&D 5e
	IsNat1 bool

	// IsNat20 indicates if the d20 roll was a natural 20
	// Note: Unlike attack rolls, natural 20s don't automatically succeed saving throws in D&D 5e
	IsNat20 bool
}

// MakeSavingThrow executes a saving throw using the provided roller and input parameters
//
// The function handles:
//   - Normal rolls (single d20)
//   - Advantage (roll 2d20, take higher)
//   - Disadvantage (roll 2d20, take lower)
//   - Advantage + Disadvantage cancellation (single d20)
//   - Natural 1 and natural 20 detection
//
// Returns an error if the dice roller fails.
func MakeSavingThrow(ctx context.Context, roller dice.Roller, input *SavingThrowInput) (*SavingThrowResult, error) {
	var roll int
	var err error

	// D&D 5e Rule: Advantage and Disadvantage cancel each other out
	hasAdvantage := input.HasAdvantage && !input.HasDisadvantage
	hasDisadvantage := input.HasDisadvantage && !input.HasAdvantage

	switch {
	case hasAdvantage:
		// Roll with advantage: 2d20, take higher
		rolls, rollErr := roller.RollN(ctx, 2, 20)
		if rollErr != nil {
			return nil, rollErr
		}
		roll = maxInt(rolls[0], rolls[1])
	case hasDisadvantage:
		// Roll with disadvantage: 2d20, take lower
		rolls, rollErr := roller.RollN(ctx, 2, 20)
		if rollErr != nil {
			return nil, rollErr
		}
		roll = minInt(rolls[0], rolls[1])
	default:
		// Normal roll: 1d20
		roll, err = roller.Roll(ctx, 20)
		if err != nil {
			return nil, err
		}
	}

	// Calculate total
	total := roll + input.Modifier

	// Determine success
	success := total >= input.DC

	// Detect natural 1 and natural 20
	isNat1 := roll == 1
	isNat20 := roll == 20

	return &SavingThrowResult{
		Roll:    roll,
		Total:   total,
		DC:      input.DC,
		Success: success,
		IsNat1:  isNat1,
		IsNat20: isNat20,
	}, nil
}

// maxInt returns the larger of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minInt returns the smaller of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
