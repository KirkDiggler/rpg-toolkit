// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

const deathSaveCharacterID = "death-save-hero"

type deathSaveRoller struct {
	value int
	err   error
	calls int
	sides []int
}

func (r *deathSaveRoller) Roll(_ context.Context, sides int) (int, error) {
	r.calls++
	r.sides = append(r.sides, sides)
	return r.value, r.err
}

func (*deathSaveRoller) RollN(context.Context, int, int) ([]int, error) {
	return nil, errors.New("death save test: RollN was not expected")
}

func deathSaveCharacter(hp int, state *saves.DeathSaveState) *character.Data {
	return &character.Data{
		ID:             deathSaveCharacterID,
		PlayerID:       "player-1",
		Name:           "Mara",
		Level:          1,
		ClassID:        classes.Fighter,
		HitPoints:      hp,
		MaxHitPoints:   12,
		ArmorClass:     14,
		DeathSaveState: state,
		ActionEconomy: &character.ActionEconomyData{
			TurnNumber: 7,
			Granted: map[character.GrantedActionKey]int{
				character.GrantedDeathSaves: 1,
			},
		},
	}
}

func TestDeathSavePersistsTheProviderResultWithoutReclassification(t *testing.T) {
	input := deathSaveCharacter(0, &saves.DeathSaveState{Successes: 1, Failures: 1})
	before, err := json.Marshal(input)
	require.NoError(t, err)
	roller := &deathSaveRoller{value: 9}

	out, err := DeathSave(context.Background(), &DeathSaveInput{
		Character: input,
		Roller:    roller,
	})

	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, 1, roller.calls)
	require.Equal(t, []int{20}, roller.sides)
	require.Equal(t, character.MakeDeathSaveOutput{
		Roll:          9,
		Outcome:       character.DeathSaveOutcomeFailure,
		FailuresAdded: 1,
		Progress: character.DeathSaveProgress{
			Successes:         1,
			Failures:          2,
			SuccessesNeeded:   2,
			FailuresRemaining: 1,
		},
		Continuation: character.DeathSaveContinuationEndTurn,
	}, out.Result)
	require.Equal(t, &saves.DeathSaveState{Successes: 1, Failures: 2}, out.Character.DeathSaveState)
	require.Zero(t, out.Character.ActionEconomy.Granted[character.GrantedDeathSaves],
		"the provider's capacity debit is part of the persisted result")

	after, err := json.Marshal(input)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after), "the caller's record is never the live sheet")
	require.NotSame(t, input, out.Character)
	require.NotSame(t, input.DeathSaveState, out.Character.DeathSaveState)
	require.NotSame(t, input.ActionEconomy, out.Character.ActionEconomy)

	out.Character.DeathSaveState.Failures = 99
	out.Character.ActionEconomy.Granted[character.GrantedDeathSaves] = 99
	require.Equal(t, 1, input.DeathSaveState.Failures,
		"the returned snapshot owns its death-save state")
	require.Equal(t, 1, input.ActionEconomy.Granted[character.GrantedDeathSaves],
		"the returned snapshot owns its nested grant bank")
}

func TestDeathSaveThreadsTerminalOutcomeAndContinuationTypes(t *testing.T) {
	t.Run("third failure already advanced", func(t *testing.T) {
		out, err := DeathSave(context.Background(), &DeathSaveInput{
			Character: deathSaveCharacter(0, &saves.DeathSaveState{Failures: 2}),
			Roller:    &deathSaveRoller{value: 9},
		})

		require.NoError(t, err)
		require.Equal(t, character.DeathSaveOutcomeDead, out.Result.Outcome)
		require.Equal(t, character.DeathSaveContinuationAlreadyAdvanced, out.Result.Continuation)
		require.Equal(t, &saves.DeathSaveState{Failures: 3, Dead: true}, out.Character.DeathSaveState)
	})

	t.Run("natural twenty keeps the turn", func(t *testing.T) {
		out, err := DeathSave(context.Background(), &DeathSaveInput{
			Character: deathSaveCharacter(0, &saves.DeathSaveState{Successes: 1, Failures: 2}),
			Roller:    &deathSaveRoller{value: 20},
		})

		require.NoError(t, err)
		require.Equal(t, character.DeathSaveOutcomeRecovered, out.Result.Outcome)
		require.Equal(t, character.DeathSaveContinuationKeepTurn, out.Result.Continuation)
		require.True(t, out.Result.RegainedConscious)
		require.Equal(t, 1, out.Character.HitPoints)
		require.Equal(t, &saves.DeathSaveState{}, out.Character.DeathSaveState)
	})
}

func TestDeathSaveRefusalsHappenBeforeRolling(t *testing.T) {
	tests := []struct {
		name  string
		input *DeathSaveInput
		is    error
	}{
		{name: "nil input", is: ErrNilInput},
		{name: "nil character", input: &DeathSaveInput{Roller: &deathSaveRoller{value: 10}}, is: ErrBadParticipant},
		{name: "character without ID", input: &DeathSaveInput{Character: &character.Data{}, Roller: &deathSaveRoller{value: 10}}, is: ErrBadParticipant},
		{name: "nil roller", input: &DeathSaveInput{Character: deathSaveCharacter(0, nil)}, is: ErrNoRoller},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			roller := &deathSaveRoller{value: 10}
			in := tc.input
			if in != nil && in.Roller != nil {
				in.Roller = roller
			}

			out, err := DeathSave(context.Background(), in)

			require.ErrorIs(t, err, tc.is)
			require.Nil(t, out)
			require.Zero(t, roller.calls)
		})
	}
}

func TestDeathSaveReadsPersistedEffectsStrictly(t *testing.T) {
	input := deathSaveCharacter(0, nil)
	input.Conditions = []json.RawMessage{
		json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"prone"},"x":`),
	}
	roller := &deathSaveRoller{value: 10}

	out, err := DeathSave(context.Background(), &DeathSaveInput{Character: input, Roller: roller})

	require.Error(t, err)
	require.Contains(t, err.Error(), "condition")
	require.Contains(t, err.Error(), "prone")
	require.Nil(t, out)
	require.Zero(t, roller.calls)
}

func TestConsciousCharacterRefusesADeathSaveWithoutRolling(t *testing.T) {
	roller := &deathSaveRoller{value: 20}
	out, err := DeathSave(context.Background(), &DeathSaveInput{
		Character: deathSaveCharacter(1, nil),
		Roller:    roller,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "conscious")
	require.Nil(t, out)
	require.Zero(t, roller.calls)
}

func TestDeathSaveSuccessTearsDownEveryRegistration(t *testing.T) {
	bus := newLongRestFaultBus()
	surf := newSurface(bus)

	out, err := deathSaveOn(context.Background(), &DeathSaveInput{
		Character: deathSaveCharacter(0, nil),
		Roller:    &deathSaveRoller{value: 10},
	}, surf)

	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotEmpty(t, surf.registrations(), "a real character was attached")
	require.Empty(t, bus.active, "every granted registration was revoked")
}

func TestDeathSaveRollFailureTearsDownEveryRegistration(t *testing.T) {
	rollErr := errors.New("death save die failed")
	bus := newLongRestFaultBus()
	surf := newSurface(bus)

	out, err := deathSaveOn(context.Background(), &DeathSaveInput{
		Character: deathSaveCharacter(0, nil),
		Roller:    &deathSaveRoller{err: rollErr},
	}, surf)

	require.ErrorIs(t, err, rollErr)
	require.Nil(t, out)
	require.NotEmpty(t, surf.registrations(), "the roll failed only after attachment")
	require.Empty(t, bus.active)
}

func TestDeathSaveRuleAndTeardownFailuresRemainReachable(t *testing.T) {
	teardownErr := errors.New("death save teardown failed")
	bus := newLongRestFaultBus()
	bus.unsubscribeErr = teardownErr
	surf := newSurface(bus)

	out, err := deathSaveOn(context.Background(), &DeathSaveInput{
		Character: deathSaveCharacter(1, nil),
		Roller:    &deathSaveRoller{value: 10},
	}, surf)

	require.ErrorContains(t, err, "conscious")
	require.ErrorIs(t, err, teardownErr)
	require.Nil(t, out)
	require.NotEmpty(t, surf.registrations(), "the provider refused only after attachment")
	require.Empty(t, bus.active)
}

func TestDeathSaveTeardownFailureRefusesAnOtherwiseSuccessfulResult(t *testing.T) {
	teardownErr := errors.New("death save teardown failed")
	bus := newLongRestFaultBus()
	bus.unsubscribeErr = teardownErr
	surf := newSurface(bus)

	out, err := deathSaveOn(context.Background(), &DeathSaveInput{
		Character: deathSaveCharacter(0, nil),
		Roller:    &deathSaveRoller{value: 10},
	}, surf)

	require.ErrorIs(t, err, teardownErr)
	require.ErrorContains(t, err, "resolution: teardown")
	require.Nil(t, out)
	require.Empty(t, bus.active)
}

func TestDeathSaveAttachFailureRollsBackAndTearsDown(t *testing.T) {
	attachErr := errors.New("death save subscription refused")
	bus := newLongRestFaultBus()
	bus.failSubscribeAt = 3
	bus.subscribeErr = attachErr
	surf := newSurface(bus)
	roller := &deathSaveRoller{value: 10}

	out, err := deathSaveOn(context.Background(), &DeathSaveInput{
		Character: deathSaveCharacter(0, nil),
		Roller:    roller,
	}, surf)

	require.ErrorIs(t, err, attachErr)
	require.Nil(t, out)
	require.Zero(t, roller.calls)
	require.NotEmpty(t, surf.registrations(), "registrations landed before the refusal")
	require.Empty(t, bus.active)
}

var _ dice.Roller = (*deathSaveRoller)(nil)
