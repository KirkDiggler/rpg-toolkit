// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

func TestParticipationViewLoadsShieldWithoutConsultingDisplayCatalog(t *testing.T) {
	shield, err := conditions.NewShieldSpellCondition("shield-char").ToJSON()
	require.NoError(t, err)

	char, err := Load(context.Background(), &Data{
		ID:             "shield-char",
		HitPoints:      0,
		MaxHitPoints:   10,
		DeathSaveState: &saves.DeathSaveState{Successes: 1, Failures: 2},
		Conditions:     []json.RawMessage{shield},
	})
	require.NoError(t, err, "Shield is a real loadable condition")
	require.Len(t, char.conditions, 1)

	require.Equal(t, ParticipationView{
		LifeState: combat.LifeStateDying,
		DeathSaves: &DeathSaveProgress{
			Successes: 1, Failures: 2, SuccessesNeeded: 2, FailuresRemaining: 1,
		},
	}, char.ParticipationView())

	out, err := char.StatusView(&StatusViewInput{})
	require.ErrorContains(t, err, "is not in the status-view display catalog")
	require.Nil(t, out, "full status remains intentionally strict about undisplayable spell status")
}

func TestParticipationViewProjectsLiteralStateTable(t *testing.T) {
	tests := []struct {
		name  string
		hp    int
		state *saves.DeathSaveState
		want  ParticipationView
	}{
		{
			name:  "conscious",
			hp:    1,
			state: &saves.DeathSaveState{Successes: 2, Failures: 1},
			want:  ParticipationView{LifeState: combat.LifeStateConscious},
		},
		{
			name:  "dying",
			state: &saves.DeathSaveState{Successes: 1, Failures: 2},
			want: ParticipationView{
				LifeState: combat.LifeStateDying,
				DeathSaves: &DeathSaveProgress{
					Successes: 1, Failures: 2, SuccessesNeeded: 2, FailuresRemaining: 1,
				},
			},
		},
		{
			name:  "stabilized",
			state: &saves.DeathSaveState{Successes: 3, Failures: 1, Stabilized: true},
			want: ParticipationView{
				LifeState: combat.LifeStateStabilized,
				DeathSaves: &DeathSaveProgress{
					Successes: 3, Failures: 1, SuccessesNeeded: 0, FailuresRemaining: 2,
					Stabilized: true,
				},
			},
		},
		{
			name:  "dead",
			state: &saves.DeathSaveState{Successes: 1, Failures: 3, Dead: true},
			want: ParticipationView{
				LifeState: combat.LifeStateDead,
				DeathSaves: &DeathSaveProgress{
					Successes: 1, Failures: 3, SuccessesNeeded: 2, FailuresRemaining: 0,
					Dead: true,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			char := &Character{
				hitPoints: tc.hp, maxHitPoints: 10,
				deathSaveState: cloneDeathSaveState(tc.state),
			}

			require.Equal(t, tc.want, char.ParticipationView())
		})
	}
}

func TestParticipationViewReturnsDetachedProgressWithoutDirtyingSheet(t *testing.T) {
	char := &Character{
		hitPoints: 0, maxHitPoints: 10,
		deathSaveState: &saves.DeathSaveState{Successes: 1, Failures: 2},
	}
	wantProgress := DeathSaveProgress{
		Successes: 1, Failures: 2, SuccessesNeeded: 2, FailuresRemaining: 1,
	}
	wantState := &saves.DeathSaveState{Successes: 1, Failures: 2}

	view := char.ParticipationView()
	require.Equal(t, combat.LifeStateDying, view.LifeState)
	require.Equal(t, &wantProgress, view.DeathSaves)
	require.False(t, char.IsDirty())

	*view.DeathSaves = DeathSaveProgress{
		Successes: 99, Failures: 99, SuccessesNeeded: 99, FailuresRemaining: 99,
		Stabilized: true, Dead: true,
	}

	require.Equal(t, combat.LifeStateDying, view.LifeState)
	require.Equal(t, combat.LifeStateDying, char.LifeState())
	require.Equal(t, wantState, char.GetDeathSaveState())
	require.False(t, char.IsDirty(), "reading and mutating detached progress cannot dirty the sheet")
	require.Equal(t, ParticipationView{
		LifeState:  combat.LifeStateDying,
		DeathSaves: &wantProgress,
	}, char.ParticipationView(), "a later view must be projected from unchanged provider state")
}

func TestParticipationViewNilReceiverReturnsUnknown(t *testing.T) {
	var char *Character

	require.Equal(t, ParticipationView{LifeState: combat.LifeStateUnknown}, char.ParticipationView())
}
