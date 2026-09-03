// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// DeathSaveOutcome identifies the rule outcome of one accepted death save.
type DeathSaveOutcome string

const (
	DeathSaveOutcomeSuccess      DeathSaveOutcome = "success"
	DeathSaveOutcomeFailure      DeathSaveOutcome = "failure"
	DeathSaveOutcomeCriticalFail DeathSaveOutcome = "critical_failure"
	DeathSaveOutcomeStabilized   DeathSaveOutcome = "stabilized"
	DeathSaveOutcomeDead         DeathSaveOutcome = "dead"
	DeathSaveOutcomeRecovered    DeathSaveOutcome = "recovered"
)

// DeathSaveProgress is the provider-owned projection of current death-save
// progress and the remaining distance to either terminal result.
type DeathSaveProgress struct {
	Successes         int
	Failures          int
	SuccessesNeeded   int
	FailuresRemaining int
	Stabilized        bool
	Dead              bool
}

// DeathSaveContinuation tells a turn runner what the ruled outcome means for
// the currently active turn.
type DeathSaveContinuation string

const (
	DeathSaveContinuationEndTurn         DeathSaveContinuation = "end_turn"
	DeathSaveContinuationKeepTurn        DeathSaveContinuation = "keep_turn"
	DeathSaveContinuationAlreadyAdvanced DeathSaveContinuation = "already_advanced"
)

// MakeDeathSaveInput contains the explicit roller for a death saving throw.
// Unlike low-level dice rules, the authoritative operation never substitutes a
// random roller: a selector execution without its dependency is malformed.
type MakeDeathSaveInput struct {
	Roller dice.Roller
}

// MakeDeathSaveOutput is the complete provider-owned result projected to a
// session. Callers do not have to reclassify a raw roll or inspect persistence.
type MakeDeathSaveOutput struct {
	Roll              int
	Outcome           DeathSaveOutcome
	SuccessesAdded    int
	FailuresAdded     int
	Progress          DeathSaveProgress
	HPRestored        int
	RegainedConscious bool
	Continuation      DeathSaveContinuation
}

// CanMakeDeathSave reports whether c is Dying and still holds this turn's one
// death-save capacity. It is a pure eligibility read.
func CanMakeDeathSave(c *Character) bool {
	if c == nil || c.lifeState() != combat.LifeStateDying {
		return false
	}

	cost, err := CostOfDeathSave(c)
	return err == nil && combat.CanPay(c, cost)
}

// CostOfDeathSave compiles the capacity-only price of a death save.
func CostOfDeathSave(c *Character) (*combat.SpendProfile, error) {
	if c == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "no character to price a death save for")
	}

	return &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{combat.CapacityDeathSave: 1},
	}, nil
}

// MakeDeathSave rolls and persists the character's authoritative death-save
// transition. Eligibility and capacity are checked before any die is rolled;
// an accepted transition consumes exactly one turn grant and marks the sheet.
func (c *Character) MakeDeathSave(
	ctx context.Context, input *MakeDeathSaveInput,
) (*MakeDeathSaveOutput, error) {
	if c == nil {
		return nil, rpgerr.New(rpgerr.CodeNil, "character is required to make a death save")
	}
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "death save input is required")
	}
	if input.Roller == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "death save roller is required")
	}

	switch c.lifeState() {
	case combat.LifeStateDying:
		// Eligible; capacity is checked below.
	case combat.LifeStateConscious:
		return nil, rpgerr.New(rpgerr.CodeInvalidState,
			"character is conscious and cannot make a death save")
	case combat.LifeStateStabilized:
		return nil, rpgerr.New(rpgerr.CodeInvalidState,
			"character is stabilized and cannot make a death save")
	case combat.LifeStateDead:
		return nil, rpgerr.New(rpgerr.CodeInvalidState,
			"character is dead and cannot make a death save")
	default:
		return nil, rpgerr.New(rpgerr.CodeInvalidState,
			"character has no known life state for a death save")
	}

	cost, err := CostOfDeathSave(c)
	if err != nil {
		return nil, err
	}
	if !combat.CanPay(c, cost) {
		return nil, rpgerr.New(rpgerr.CodeResourceExhausted,
			"death save capacity is spent")
	}

	state := c.deathSaveState
	if state == nil {
		state = &saves.DeathSaveState{}
	}
	result, err := saves.MakeDeathSave(ctx, &saves.DeathSaveInput{
		Roller: input.Roller,
		State:  state,
	})
	if err != nil {
		return nil, err
	}

	outcome := deathSaveOutcome(result)
	continuation := DeathSaveContinuationEndTurn
	switch outcome {
	case DeathSaveOutcomeRecovered:
		c.hitPoints = result.HPRestored
		continuation = DeathSaveContinuationKeepTurn
	case DeathSaveOutcomeDead:
		continuation = DeathSaveContinuationAlreadyAdvanced
	default:
	}
	c.deathSaveState = result.State

	// Capacity was checked before rolling, so this debit cannot refuse. It
	// marks the persisted economy; the explicit dirty set below also covers the
	// provider state and natural-20 hit-point transition.
	if err := combat.Pay(c, cost); err != nil {
		return nil, err
	}
	c.dirty = true

	return &MakeDeathSaveOutput{
		Roll:              result.Roll,
		Outcome:           outcome,
		SuccessesAdded:    result.SuccessesAdded,
		FailuresAdded:     result.FailuresAdded,
		Progress:          deathSaveProgress(result.State),
		HPRestored:        result.HPRestored,
		RegainedConscious: result.RegainedConsciousness,
		Continuation:      continuation,
	}, nil
}

func deathSaveOutcome(result *saves.DeathSaveResult) DeathSaveOutcome {
	switch {
	case result.RegainedConsciousness:
		return DeathSaveOutcomeRecovered
	case result.State.Dead:
		return DeathSaveOutcomeDead
	case result.State.Stabilized:
		return DeathSaveOutcomeStabilized
	case result.IsCriticalFail:
		return DeathSaveOutcomeCriticalFail
	case result.FailuresAdded > 0:
		return DeathSaveOutcomeFailure
	default:
		return DeathSaveOutcomeSuccess
	}
}

func deathSaveProgress(state *saves.DeathSaveState) DeathSaveProgress {
	if state == nil {
		state = &saves.DeathSaveState{}
	}

	return DeathSaveProgress{
		Successes:         state.Successes,
		Failures:          state.Failures,
		SuccessesNeeded:   remainingDeathSaves(state.Successes),
		FailuresRemaining: remainingDeathSaves(state.Failures),
		Stabilized:        state.Stabilized,
		Dead:              state.Dead,
	}
}

func remainingDeathSaves(progress int) int {
	remaining := 3 - progress
	if remaining < 0 {
		return 0
	}
	return remaining
}

// LifeState reports the character's authoritative derived life state without
// exposing mutable death-save progress.
func (c *Character) LifeState() combat.LifeState {
	return c.lifeState()
}

// lifeState derives current state exclusively from the character-owned hit
// points and death-save progress.
func (c *Character) lifeState() combat.LifeState {
	if c == nil {
		return combat.LifeStateUnknown
	}

	state := c.deathSaveState
	return combat.ClassifyLifeState(combat.LifeStateInput{
		Kind:       combat.CombatantKindCharacter,
		Down:       combat.IsDown(c),
		Stabilized: state != nil && state.Stabilized,
		Dead:       state != nil && state.Dead,
	})
}

// TakeDamageWhileUnconsciousInput is retained for source compatibility.
type TakeDamageWhileUnconsciousInput struct {
	IsCritical bool
}

// TakeDamageWhileUnconscious is an inert compatibility shim. Damage must enter
// through [Character.ApplyDamage], where positive applied damage and HP-at-zero
// eligibility are established before death-save progress can change.
//
// Deprecated: use [Character.ApplyDamage].
func (c *Character) TakeDamageWhileUnconscious(
	_ context.Context, input *TakeDamageWhileUnconsciousInput,
) (*saves.DamageWhileUnconsciousResult, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"damage while unconscious input is required")
	}
	return nil, rpgerr.New(rpgerr.CodeInvalidState,
		"direct unconscious damage is disabled; use Character.ApplyDamage")
}

// applyDeathSaveFailure is the one character-owned damage transition. It
// clears stabilization before applying the ruled failure and ignores Dead.
func (c *Character) applyDeathSaveFailure(
	ctx context.Context, critical bool,
) (*saves.DamageWhileUnconsciousResult, bool, error) {
	state := c.deathSaveState
	if state == nil {
		state = &saves.DeathSaveState{}
	}
	if state.Dead {
		copyState := *state
		return &saves.DamageWhileUnconsciousResult{State: &copyState}, false, nil
	}

	inputState := *state
	inputState.Stabilized = false
	result, err := saves.TakeDamageWhileUnconscious(ctx, &saves.DamageWhileUnconsciousInput{
		State:      &inputState,
		IsCritical: critical,
	})
	if err != nil {
		return nil, false, err
	}
	c.deathSaveState = result.State
	return result, true, nil
}

// GetDeathSaveState returns a defensive copy of the character's current
// authoritative state. The empty zero state is returned when no progress has
// been recorded.
func (c *Character) GetDeathSaveState() *saves.DeathSaveState {
	if c.deathSaveState == nil {
		return &saves.DeathSaveState{}
	}
	state := *c.deathSaveState
	return &state
}

// ResetDeathSaveState is an inert compatibility shim. Progress resets only as
// part of an authoritative recovery transition such as accepted healing,
// natural-20 recovery, or a long rest.
//
// Deprecated: use an authoritative recovery operation.
func (c *Character) ResetDeathSaveState() error {
	return rpgerr.New(rpgerr.CodeInvalidState,
		"direct death-save reset is disabled; use an authoritative recovery operation")
}
