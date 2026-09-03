// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

func TestMakeDeathSaveReportsAuthoritativeOutcomes(t *testing.T) {
	tests := []struct {
		name            string
		roll            int
		outcome         DeathSaveOutcome
		successesAdded  int
		failuresAdded   int
		successes       int
		failures        int
		successesNeeded int
		failuresLeft    int
		hp              int
		continuation    DeathSaveContinuation
	}{
		{
			name: "natural 1", roll: 1, outcome: DeathSaveOutcomeCriticalFail,
			failuresAdded: 2, failures: 2, successesNeeded: 3, failuresLeft: 1,
			continuation: DeathSaveContinuationEndTurn,
		},
		{
			name: "lower failure boundary", roll: 2, outcome: DeathSaveOutcomeFailure,
			failuresAdded: 1, failures: 1, successesNeeded: 3, failuresLeft: 2,
			continuation: DeathSaveContinuationEndTurn,
		},
		{
			name: "upper failure boundary", roll: 9, outcome: DeathSaveOutcomeFailure,
			failuresAdded: 1, failures: 1, successesNeeded: 3, failuresLeft: 2,
			continuation: DeathSaveContinuationEndTurn,
		},
		{
			name: "lower success boundary", roll: 10, outcome: DeathSaveOutcomeSuccess,
			successesAdded: 1, successes: 1, successesNeeded: 2, failuresLeft: 3,
			continuation: DeathSaveContinuationEndTurn,
		},
		{
			name: "upper success boundary", roll: 19, outcome: DeathSaveOutcomeSuccess,
			successesAdded: 1, successes: 1, successesNeeded: 2, failuresLeft: 3,
			continuation: DeathSaveContinuationEndTurn,
		},
		{
			name: "natural 20", roll: 20, outcome: DeathSaveOutcomeRecovered,
			successesNeeded: 3, failuresLeft: 3, hp: 1,
			continuation: DeathSaveContinuationKeepTurn,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			char := dyingCharacterForTurn(t, &saves.DeathSaveState{})
			roller := &countingDeathSaveRoller{value: tc.roll}
			markSaved(char)

			out, err := char.MakeDeathSave(context.Background(), &MakeDeathSaveInput{Roller: roller})
			require.NoError(t, err)
			require.Equal(t, 1, roller.calls)
			require.Equal(t, []int{20}, roller.sides)
			require.Equal(t, tc.roll, out.Roll)
			require.Equal(t, tc.outcome, out.Outcome)
			require.Equal(t, tc.successesAdded, out.SuccessesAdded)
			require.Equal(t, tc.failuresAdded, out.FailuresAdded)
			require.Equal(t, tc.successes, out.Progress.Successes)
			require.Equal(t, tc.failures, out.Progress.Failures)
			require.Equal(t, tc.successesNeeded, out.Progress.SuccessesNeeded)
			require.Equal(t, tc.failuresLeft, out.Progress.FailuresRemaining)
			require.Equal(t, tc.hp, char.GetHitPoints())
			require.Equal(t, tc.hp, out.HPRestored)
			require.Equal(t, tc.roll == 20, out.RegainedConscious)
			require.Equal(t, tc.continuation, out.Continuation)
			require.True(t, char.IsDirty(), "an accepted roll must be persisted")
			require.Equal(t, 0, char.CapacityLeft(combat.CapacityDeathSave))
		})
	}
}

func TestMakeDeathSaveTerminalOutcomes(t *testing.T) {
	tests := []struct {
		name         string
		state        *saves.DeathSaveState
		roll         int
		outcome      DeathSaveOutcome
		continuation DeathSaveContinuation
		progress     DeathSaveProgress
	}{
		{
			name: "third success stabilizes", state: &saves.DeathSaveState{Successes: 2}, roll: 10,
			outcome: DeathSaveOutcomeStabilized, continuation: DeathSaveContinuationEndTurn,
			progress: DeathSaveProgress{
				Successes: 3, SuccessesNeeded: 0, FailuresRemaining: 3, Stabilized: true,
			},
		},
		{
			name: "third failure dies", state: &saves.DeathSaveState{Failures: 2}, roll: 9,
			outcome: DeathSaveOutcomeDead, continuation: DeathSaveContinuationAlreadyAdvanced,
			progress: DeathSaveProgress{
				Failures: 3, SuccessesNeeded: 3, FailuresRemaining: 0, Dead: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			char := dyingCharacterForTurn(t, tc.state)
			out, err := char.MakeDeathSave(context.Background(), &MakeDeathSaveInput{
				Roller: &countingDeathSaveRoller{value: tc.roll},
			})
			require.NoError(t, err)
			require.Equal(t, tc.outcome, out.Outcome)
			require.Equal(t, tc.continuation, out.Continuation)
			require.Equal(t, tc.progress, out.Progress)
		})
	}
}

func TestDyingTurnBanksExactlyOneDeathSave(t *testing.T) {
	char := bareCharacterAtZero(&saves.DeathSaveState{})
	char.actionEconomy = &ActionEconomyData{
		TurnNumber: 7, ActionsRemaining: 0, BonusActionsRemaining: 0,
		MovementRemaining: 5,
		Granted:           map[GrantedActionKey]int{GrantedDeathSaves: 9},
	}

	refreshed, err := char.RefreshForTurn(context.Background(), &RefreshForTurnInput{
		TurnNumber: 8,
		Speed:      30,
	})
	require.NoError(t, err)
	require.True(t, refreshed.Reseeded)
	require.Equal(t, 1, char.GetActionEconomy().ActionsRemaining)
	require.Equal(t, 1, char.GetActionEconomy().BonusActionsRemaining)
	require.Equal(t, 30, char.GetActionEconomy().MovementRemaining)
	require.Equal(t, 1, char.GetActionEconomy().Granted[GrantedDeathSaves])

	first := &countingDeathSaveRoller{value: 10}
	_, err = char.MakeDeathSave(context.Background(), &MakeDeathSaveInput{Roller: first})
	require.NoError(t, err)
	require.Equal(t, 1, first.calls)
	require.Zero(t, char.CapacityLeft(combat.CapacityDeathSave))

	second := &countingDeathSaveRoller{value: 10}
	out, err := char.MakeDeathSave(context.Background(), &MakeDeathSaveInput{Roller: second})
	require.Error(t, err)
	require.Nil(t, out)
	require.Equal(t, rpgerr.CodeResourceExhausted, rpgerr.GetCode(err))
	require.Zero(t, second.calls)
}

func TestMakeDeathSaveNatural20KeepsTheActiveTurn(t *testing.T) {
	char := dyingCharacterForTurn(t, &saves.DeathSaveState{Successes: 1, Failures: 2})
	before := *char.GetActionEconomy()

	out, err := char.MakeDeathSave(context.Background(), &MakeDeathSaveInput{
		Roller: &countingDeathSaveRoller{value: 20},
	})
	require.NoError(t, err)
	require.Equal(t, DeathSaveOutcomeRecovered, out.Outcome)
	require.Equal(t, 1, char.GetHitPoints())
	require.Equal(t, 0, char.GetDeathSaveState().Successes)
	require.Equal(t, 0, char.GetDeathSaveState().Failures)
	require.False(t, char.GetDeathSaveState().Stabilized)
	require.False(t, char.GetDeathSaveState().Dead)
	require.Equal(t, before.ActionsRemaining, char.GetActionEconomy().ActionsRemaining)
	require.Equal(t, before.BonusActionsRemaining, char.GetActionEconomy().BonusActionsRemaining)
	require.Equal(t, before.MovementRemaining, char.GetActionEconomy().MovementRemaining)
}

func TestDeathSaveProgressRoundTripsExactly(t *testing.T) {
	char := dyingCharacterForTurn(t, &saves.DeathSaveState{Successes: 1, Failures: 1})
	_, err := char.MakeDeathSave(context.Background(), &MakeDeathSaveInput{
		Roller: &countingDeathSaveRoller{value: 10},
	})
	require.NoError(t, err)
	require.True(t, char.IsDirty())

	data := char.ToData()
	require.Equal(t, &saves.DeathSaveState{Successes: 2, Failures: 1}, data.DeathSaveState)

	loaded, err := Load(context.Background(), data)
	require.NoError(t, err)
	require.Equal(t, &saves.DeathSaveState{Successes: 2, Failures: 1}, loaded.GetDeathSaveState())
}

func TestMakeDeathSaveRefusalsRollNoDice(t *testing.T) {
	tests := []struct {
		name    string
		char    func(*testing.T) *Character
		input   func(dice.Roller) *MakeDeathSaveInput
		code    rpgerr.Code
		message string
	}{
		{
			name: "nil input", char: func(t *testing.T) *Character { return dyingCharacterForTurn(t, nil) },
			input: func(dice.Roller) *MakeDeathSaveInput { return nil },
			code:  rpgerr.CodeInvalidArgument, message: "death save input is required",
		},
		{
			name: "nil roller", char: func(t *testing.T) *Character { return dyingCharacterForTurn(t, nil) },
			input: func(dice.Roller) *MakeDeathSaveInput { return &MakeDeathSaveInput{} },
			code:  rpgerr.CodeInvalidArgument, message: "death save roller is required",
		},
		{
			name: "conscious", char: func(t *testing.T) *Character {
				char := dyingCharacterForTurn(t, nil)
				char.hitPoints = 1
				return char
			},
			input: func(r dice.Roller) *MakeDeathSaveInput { return &MakeDeathSaveInput{Roller: r} },
			code:  rpgerr.CodeInvalidState, message: "character is conscious and cannot make a death save",
		},
		{
			name: "stabilized", char: func(t *testing.T) *Character {
				return dyingCharacterForTurn(t, &saves.DeathSaveState{Successes: 3, Stabilized: true})
			},
			input: func(r dice.Roller) *MakeDeathSaveInput { return &MakeDeathSaveInput{Roller: r} },
			code:  rpgerr.CodeInvalidState, message: "character is stabilized and cannot make a death save",
		},
		{
			name: "dead", char: func(t *testing.T) *Character {
				return dyingCharacterForTurn(t, &saves.DeathSaveState{Failures: 3, Dead: true})
			},
			input: func(r dice.Roller) *MakeDeathSaveInput { return &MakeDeathSaveInput{Roller: r} },
			code:  rpgerr.CodeInvalidState, message: "character is dead and cannot make a death save",
		},
		{
			name: "capacity spent", char: func(t *testing.T) *Character {
				char := dyingCharacterForTurn(t, nil)
				char.SpendCapacity(combat.CapacityDeathSave, 1)
				markSaved(char)
				return char
			},
			input: func(r dice.Roller) *MakeDeathSaveInput { return &MakeDeathSaveInput{Roller: r} },
			code:  rpgerr.CodeResourceExhausted, message: "death save capacity is spent",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			roller := &countingDeathSaveRoller{value: 10}
			char := tc.char(t)
			markSaved(char)

			out, err := char.MakeDeathSave(context.Background(), tc.input(roller))
			require.Error(t, err)
			require.Nil(t, out)
			require.Equal(t, tc.code, rpgerr.GetCode(err))
			require.Equal(t, tc.message, err.Error())
			require.Equal(t, 0, roller.calls)
			require.False(t, char.IsDirty())
		})
	}
}

func TestCostAndEligibilityOfDeathSave(t *testing.T) {
	char := dyingCharacterForTurn(t, nil)
	cost, err := CostOfDeathSave(char)
	require.NoError(t, err)
	require.Equal(t, &combat.SpendProfile{
		Capacity: map[combat.CapacityType]int{combat.CapacityDeathSave: 1},
	}, cost)
	require.True(t, CanMakeDeathSave(char))

	char.SpendCapacity(combat.CapacityDeathSave, 1)
	require.False(t, CanMakeDeathSave(char))

	cost, err = CostOfDeathSave(nil)
	require.Error(t, err)
	require.Nil(t, cost)
	require.Equal(t, rpgerr.CodeNil, rpgerr.GetCode(err))
}

func TestApplyDamageAtZeroOwnsDeathSaveFailures(t *testing.T) {
	tests := []struct {
		name       string
		state      *saves.DeathSaveState
		critical   bool
		want       *saves.DeathSaveState
		wantDamage int
	}{
		{
			name: "normal damage adds one failure", state: &saves.DeathSaveState{Successes: 1},
			want: &saves.DeathSaveState{Successes: 1, Failures: 1}, wantDamage: 4,
		},
		{
			name: "critical damage adds two failures", state: &saves.DeathSaveState{}, critical: true,
			want: &saves.DeathSaveState{Failures: 2}, wantDamage: 4,
		},
		{
			name:  "stabilized character loses stabilization before failure",
			state: &saves.DeathSaveState{Successes: 3, Stabilized: true},
			want:  &saves.DeathSaveState{Successes: 3, Failures: 1}, wantDamage: 4,
		},
		{
			name: "third failure is death", state: &saves.DeathSaveState{Failures: 2},
			want: &saves.DeathSaveState{Failures: 3, Dead: true}, wantDamage: 4,
		},
		{
			name: "dead ignores further failures", state: &saves.DeathSaveState{Failures: 3, Dead: true},
			want: &saves.DeathSaveState{Failures: 3, Dead: true}, wantDamage: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			char := bareCharacterAtZero(tc.state)
			markSaved(char)
			result := char.ApplyDamage(context.Background(), &combat.ApplyDamageInput{
				Instances:  []combat.DamageInstance{{Amount: 4, Type: string(damage.Slashing)}},
				IsCritical: tc.critical,
			})

			require.Equal(t, tc.wantDamage, result.TotalDamage)
			require.Equal(t, tc.want, char.GetDeathSaveState())
			require.Equal(t, tc.want, char.ToData().DeathSaveState)
			if tc.state.Dead {
				require.False(t, char.IsDirty(), "dead damage causes no authoritative transition")
			} else {
				require.True(t, char.IsDirty())
			}
		})
	}
}

func TestZeroAppliedDamageDoesNotChangeDeathSaveProgress(t *testing.T) {
	immunity := 0.0
	instances, total := combat.FinalDamage([]dnd5eEvents.DamageComponent{
		{DamageType: damage.Fire, FlatBonus: 12},
		{DamageType: damage.Fire, Multiplier: &immunity},
	})
	require.Zero(t, total)
	require.Empty(t, instances)

	immuneInput := &combat.ApplyDamageInput{}
	for _, instance := range instances {
		immuneInput.Instances = append(immuneInput.Instances, combat.DamageInstance{
			Amount: instance.Amount,
			Type:   string(instance.Type),
		})
	}
	inputs := []*combat.ApplyDamageInput{
		{},
		immuneInput,
		{Instances: []combat.DamageInstance{{Amount: 0, Type: string(damage.Fire)}}},
	}
	for _, input := range inputs {
		char := bareCharacterAtZero(&saves.DeathSaveState{Successes: 3, Stabilized: true})
		markSaved(char)
		result := char.ApplyDamage(context.Background(), input)
		require.Zero(t, result.TotalDamage)
		require.Equal(t, &saves.DeathSaveState{Successes: 3, Stabilized: true}, char.GetDeathSaveState())
		require.False(t, char.IsDirty())
	}
}

func TestHealingOwnsLifeStateTransitions(t *testing.T) {
	t.Run("dead refuses ordinary healing", func(t *testing.T) {
		char, bus := attachedCharacter(t, &saves.DeathSaveState{Failures: 3, Dead: true})
		markSaved(char)
		err := dnd5eEvents.HealingReceivedTopic.On(bus).Publish(context.Background(), dnd5eEvents.HealingReceivedEvent{
			TargetID: char.GetID(), Amount: 5, Source: "cure_wounds",
		})
		require.Error(t, err)
		require.Equal(t, rpgerr.CodeInvalidState, rpgerr.GetCode(err))
		require.Equal(t, 0, char.GetHitPoints())
		require.Equal(t, &saves.DeathSaveState{Failures: 3, Dead: true}, char.GetDeathSaveState())
		require.False(t, char.IsDirty())
	})

	for _, state := range []*saves.DeathSaveState{
		{Successes: 1, Failures: 1},
		{Successes: 3, Failures: 1, Stabilized: true},
	} {
		char, bus := attachedCharacter(t, state)
		err := dnd5eEvents.HealingReceivedTopic.On(bus).Publish(context.Background(), dnd5eEvents.HealingReceivedEvent{
			TargetID: char.GetID(), Amount: 5, Source: "cure_wounds",
		})
		require.NoError(t, err)
		require.Equal(t, 5, char.GetHitPoints())
		require.Equal(t, &saves.DeathSaveState{}, char.GetDeathSaveState())
		view, viewErr := char.StatusView(&StatusViewInput{})
		require.NoError(t, viewErr)
		require.Equal(t, combat.LifeStateConscious, view.View.LifeState)
		require.Nil(t, view.View.DeathSaves)
	}
}

func TestLegacyUnconsciousBlobCannotRunASecondDeathSaveLedger(t *testing.T) {
	legacy := json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"unconscious"},"member_id":"legacy-char","successes":2,"failures":0,"stabilized":false,"dead":false}`)
	data := &Data{
		ID: "legacy-char", HitPoints: 0, MaxHitPoints: 10,
		DeathSaveState: &saves.DeathSaveState{Successes: 1, Failures: 1},
		Conditions:     []json.RawMessage{legacy},
		ActionEconomy: &ActionEconomyData{
			TurnNumber: 0, Granted: map[GrantedActionKey]int{},
		},
	}
	char, err := Load(context.Background(), data)
	require.NoError(t, err)
	require.Len(t, char.conditions, 1)
	legacyCondition, ok := char.conditions[0].(*conditions.UnconsciousCondition)
	require.True(t, ok)
	roller := &countingDeathSaveRoller{value: 1}
	legacyCondition.Roller = roller

	bus := events.NewEventBus()
	require.NoError(t, Attach(context.Background(), char, bus))
	markSaved(char)

	require.NoError(t, dnd5eEvents.TurnStartTopic.On(bus).Publish(context.Background(), dnd5eEvents.TurnStartEvent{
		SubjectID: char.GetID(), Round: 1,
	}))
	require.Zero(t, roller.calls, "legacy condition must never auto-roll")
	require.Equal(t, &saves.DeathSaveState{Successes: 1, Failures: 1}, char.GetDeathSaveState())

	require.NoError(t, dnd5eEvents.DamageReceivedTopic.On(bus).Publish(context.Background(), dnd5eEvents.DamageReceivedEvent{
		TargetID: char.GetID(), Amount: 5, IsCritical: true,
	}))
	require.Equal(t, &saves.DeathSaveState{Successes: 1, Failures: 1}, char.GetDeathSaveState(),
		"only Character.ApplyDamage may author damage-at-zero failures")

	persisted := char.ToData()
	require.Len(t, persisted.Conditions, 1)
	var gotLegacy conditions.UnconsciousData
	require.NoError(t, json.Unmarshal(persisted.Conditions[0], &gotLegacy))
	require.Equal(t, 2, gotLegacy.Successes)
	require.Zero(t, gotLegacy.Failures)
	require.False(t, gotLegacy.Stabilized)
	require.False(t, gotLegacy.Dead)
	require.False(t, char.IsDirty())
}

func dyingCharacterForTurn(t *testing.T, state *saves.DeathSaveState) *Character {
	t.Helper()
	char := bareCharacterAtZero(state)
	char.actionEconomy = &ActionEconomyData{
		TurnNumber: 0, ActionsRemaining: 0, BonusActionsRemaining: 0,
		ReactionsRemaining: 0, MovementRemaining: 0,
		Granted: map[GrantedActionKey]int{GrantedAttacks: 4},
	}
	out, err := char.RefreshForTurn(context.Background(), &RefreshForTurnInput{TurnNumber: 1, Speed: 30})
	require.NoError(t, err)
	require.True(t, out.Reseeded)
	require.Equal(t, 1, char.actionEconomy.ActionsRemaining)
	require.Equal(t, 1, char.actionEconomy.BonusActionsRemaining)
	require.Equal(t, 1, char.actionEconomy.ReactionsRemaining)
	require.Equal(t, 30, char.actionEconomy.MovementRemaining)
	wantDeathSaves := 0
	if combat.ParticipationFor(char.lifeState()).NeedsDeathSave {
		wantDeathSaves = 1
	}
	require.Equal(t, wantDeathSaves, char.actionEconomy.Granted[GrantedDeathSaves])
	return char
}

func bareCharacterAtZero(state *saves.DeathSaveState) *Character {
	if state == nil {
		state = &saves.DeathSaveState{}
	}
	copyState := *state
	return &Character{
		id: "death-save-char", hitPoints: 0, maxHitPoints: 10,
		deathSaveState: &copyState,
	}
}

func attachedCharacter(t *testing.T, state *saves.DeathSaveState) (*Character, events.EventBus) {
	t.Helper()
	char := bareCharacterAtZero(state)
	bus := events.NewEventBus()
	require.NoError(t, char.SheetKeeper().Apply(context.Background(), bus))
	return char, bus
}

type countingDeathSaveRoller struct {
	value int
	calls int
	sides []int
}

func (r *countingDeathSaveRoller) Roll(_ context.Context, sides int) (int, error) {
	r.calls++
	r.sides = append(r.sides, sides)
	return r.value, nil
}

func (r *countingDeathSaveRoller) RollN(_ context.Context, count, _ int) ([]int, error) {
	return make([]int, count), nil
}
