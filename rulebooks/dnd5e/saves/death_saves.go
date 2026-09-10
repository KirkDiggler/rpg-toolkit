// Package saves implements D&D 5e saving throw mechanics including death saves
package saves

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/rolls"
)

// DeathSaveState tracks the current state of death saving throws for a character at 0 HP.
// In D&D 5e, characters make death saves when starting their turn at 0 HP.
// Accumulating 3 successes stabilizes the character; 3 failures means death.
type DeathSaveState struct {
	// Successes is the number of successful death saves (0-3)
	Successes int

	// Failures is the number of failed death saves (0-3)
	Failures int

	// Stabilized is true when the character has accumulated 3 successes
	// A stabilized character is unconscious but no longer makes death saves
	Stabilized bool

	// Dead is true when the character has accumulated 3 failures
	Dead bool
}

// DeathSaveInput contains parameters for making a death saving throw
type DeathSaveInput struct {
	// Roller is the dice roller to use. If nil, defaults to dice.NewRoller().
	Roller dice.Roller

	// State is the current death save state to update
	State *DeathSaveState

	// D20Source is the canonical death-save action source.
	D20Source dnd5eEvents.RollSource

	// Contributions are already selected by the dying creature's condition owner.
	Contributions []dnd5eEvents.DiceContribution
}

// DeathSaveResult contains the outcome of a death saving throw
type DeathSaveResult struct {
	// Roll is the d20 roll result
	Roll int

	// SuccessesAdded and FailuresAdded report the exact progress authored by
	// this roll, so a provider does not have to infer a rule result by diffing
	// mutable counters.
	SuccessesAdded int
	FailuresAdded  int

	// State is the updated death save state after this roll
	State *DeathSaveState

	// IsCriticalFail is true if the roll was a 1 (adds 2 failures)
	IsCriticalFail bool

	// IsCriticalSuccess is true if the roll was a 20 (regain consciousness)
	IsCriticalSuccess bool

	// RegainedConsciousness is true if the character regained consciousness (rolled 20)
	RegainedConsciousness bool

	// HPRestored is the HP restored (1 on a natural 20, 0 otherwise)
	HPRestored int

	// Calculation is the complete sourced d20 and contribution arithmetic.
	Calculation *dnd5eEvents.RollCalculation
}

// DamageWhileUnconsciousInput contains parameters for taking damage while unconscious
type DamageWhileUnconsciousInput struct {
	// State is the current death save state to update
	State *DeathSaveState

	// IsCritical is true if the damage was from a critical hit (adds 2 failures instead of 1)
	IsCritical bool
}

// DamageWhileUnconsciousResult contains the outcome of taking damage while unconscious
type DamageWhileUnconsciousResult struct {
	// State is the updated death save state after taking damage
	State *DeathSaveState

	// FailuresAdded is the number of failures added (1 for normal, 2 for critical)
	FailuresAdded int
}

// MakeDeathSave executes a death saving throw and updates the state accordingly.
//
// D&D 5e death save rules:
//   - Roll 1: Add 2 failures (critical fail)
//   - Roll 2-19: checked total below 10 adds 1 failure; 10 or more adds 1 success
//   - Roll 20: Regain consciousness at 1 HP (critical success)
//   - 3 failures: Character dies
//   - 3 successes: Character stabilizes (unconscious, no more saves needed)
//
// Selected dice contributions affect only the checked total for faces 2..19;
// natural 1 and natural 20 retain the policy above.
func MakeDeathSave(ctx context.Context, input *DeathSaveInput) (*DeathSaveResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	if input.State == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "state cannot be nil")
	}

	if err := validateCalculationSource("d20", input.D20Source); err != nil {
		return nil, err
	}
	if err := rolls.ValidateContributions(input.Contributions); err != nil {
		return nil, rpgerr.Wrap(err, "death save contributions are invalid")
	}

	roller := input.Roller
	if roller == nil {
		roller = dice.NewRoller()
	}

	roll, err := roller.Roll(ctx, 20)
	if err != nil {
		return nil, err
	}
	resolved, err := rolls.ResolveContributions(ctx, &rolls.ResolveContributionsInput{
		Roller: roller, Contributions: input.Contributions,
	})
	if err != nil {
		return nil, rpgerr.Wrap(err, "failed to resolve death save contributions")
	}
	components := make([]dnd5eEvents.RollComponent, 0, 1+len(resolved.Components))
	components = append(components, dnd5eEvents.RollComponent{
		Source: cloneRollSource(input.D20Source),
		Dice: &dnd5eEvents.DiceTrace{
			Notation: "1d20", DieSize: 20, OriginalRolls: []int{roll},
			FinalRolls: []int{roll}, Subtotal: roll,
		},
	})
	components = append(components, resolved.Components...)
	calculation := calculationFor(components)
	if err := dnd5eEvents.ValidateRollCalculation(calculation); err != nil {
		return nil, rpgerr.Wrap(err, "death save calculation is invalid")
	}

	// Copy state to avoid modifying the original directly
	newState := &DeathSaveState{
		Successes:  input.State.Successes,
		Failures:   input.State.Failures,
		Stabilized: input.State.Stabilized,
		Dead:       input.State.Dead,
	}

	result := &DeathSaveResult{
		Roll:        roll,
		State:       newState,
		Calculation: calculation,
	}

	// Apply roll results
	switch {
	case roll == 1:
		// Critical fail: 2 failures
		result.IsCriticalFail = true
		result.FailuresAdded = 2
		newState.Failures += result.FailuresAdded
	case roll >= 2 && roll <= 19 && calculation.Total < 10:
		// Faces 2..19 use the checked total against DC 10.
		result.FailuresAdded = 1
		newState.Failures += result.FailuresAdded
	case roll >= 2 && roll <= 19:
		result.SuccessesAdded = 1
		newState.Successes += result.SuccessesAdded
	case roll == 20:
		// Critical success: regain consciousness at 1 HP
		result.IsCriticalSuccess = true
		result.RegainedConsciousness = true
		result.HPRestored = 1
		// Reset death save state on nat 20
		newState.Successes = 0
		newState.Failures = 0
	}

	// Check for death (3+ failures)
	if newState.Failures >= 3 {
		newState.Dead = true
	}

	// Check for stabilization (3+ successes)
	if newState.Successes >= 3 {
		newState.Stabilized = true
	}

	return result, nil
}

// TakeDamageWhileUnconscious handles taking damage while at 0 HP.
//
// D&D 5e rules:
//   - Any damage at 0 HP causes 1 automatic death save failure
//   - Critical hit damage causes 2 automatic death save failures
func TakeDamageWhileUnconscious(
	_ context.Context,
	input *DamageWhileUnconsciousInput,
) (*DamageWhileUnconsciousResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")
	}
	if input.State == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "state cannot be nil")
	}

	// Copy state to avoid modifying the original directly
	newState := &DeathSaveState{
		Successes:  input.State.Successes,
		Failures:   input.State.Failures,
		Stabilized: input.State.Stabilized,
		Dead:       input.State.Dead,
	}

	failuresToAdd := 1
	if input.IsCritical {
		failuresToAdd = 2
	}

	newState.Failures += failuresToAdd

	// Check for death (3+ failures)
	if newState.Failures >= 3 {
		newState.Dead = true
	}

	return &DamageWhileUnconsciousResult{
		State:         newState,
		FailuresAdded: failuresToAdd,
	}, nil
}
