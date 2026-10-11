// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

func TestParticipationReturnsProviderPolicyAndProgressInInputOrder(t *testing.T) {
	input := &ParticipationInput{Participants: []Participant{
		{Monster: &monster.Data{ID: "standing-monster", HitPoints: 4, MaxHitPoints: 4}},
		{Character: deathSaveCharacter(0, &saves.DeathSaveState{Successes: 1, Failures: 1})},
		{Character: participationCharacter("conscious-character", 4, nil)},
		{Monster: &monster.Data{ID: "defeated-monster", HitPoints: 0, MaxHitPoints: 4}},
		{Character: participationCharacter("stabilized-character", 0,
			&saves.DeathSaveState{Successes: 3, Stabilized: true})},
		{Character: participationCharacter("dead-character", 0,
			&saves.DeathSaveState{Failures: 3, Dead: true})},
	}}

	out, err := Participation(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, []ParticipantParticipation{
		{
			Member:        "standing-monster",
			Participation: combat.ParticipationFor(combat.LifeStateConscious),
		},
		{
			Member:        deathSaveCharacterID,
			Participation: combat.ParticipationFor(combat.LifeStateDying),
			DeathSaves: &character.DeathSaveProgress{
				Successes: 1, Failures: 1, SuccessesNeeded: 2, FailuresRemaining: 2,
			},
		},
		{
			Member:        "conscious-character",
			Participation: combat.ParticipationFor(combat.LifeStateConscious),
		},
		{
			Member:        "defeated-monster",
			Participation: combat.ParticipationFor(combat.LifeStateDefeated),
		},
		{
			Member:        "stabilized-character",
			Participation: combat.ParticipationFor(combat.LifeStateStabilized),
			DeathSaves: &character.DeathSaveProgress{
				Successes: 3, SuccessesNeeded: 0, FailuresRemaining: 3, Stabilized: true,
			},
		},
		{
			Member:        "dead-character",
			Participation: combat.ParticipationFor(combat.LifeStateDead),
			DeathSaves: &character.DeathSaveProgress{
				Failures: 3, SuccessesNeeded: 3, FailuresRemaining: 0, Dead: true,
			},
		},
	}, out.Members, "the answer preserves caller order even though attachment is sorted")

	out.Members[1].DeathSaves.Failures = 99
	require.Equal(t, 1, input.Participants[1].Character.DeathSaveState.Failures,
		"the provider projection is detached from persisted progress")
}

func TestParticipationRefusesMalformedParticipantsBeforeReading(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		out, err := Participation(context.Background(), nil)
		require.ErrorIs(t, err, ErrNilInput)
		require.Nil(t, out)
	})

	t.Run("empty participant", func(t *testing.T) {
		out, err := Participation(context.Background(), &ParticipationInput{
			Participants: []Participant{{}},
		})
		require.ErrorIs(t, err, ErrBadParticipant)
		require.Nil(t, out)
	})

	t.Run("participant with both records", func(t *testing.T) {
		out, err := Participation(context.Background(), &ParticipationInput{
			Participants: []Participant{{
				Character: participationCharacter("both-character", 1, nil),
				Monster:   &monster.Data{ID: "both-monster", HitPoints: 1},
			}},
		})
		require.ErrorIs(t, err, ErrBadParticipant)
		require.Nil(t, out)
	})
}

func participationCharacter(id string, hp int, state *saves.DeathSaveState) *character.Data {
	data := deathSaveCharacter(hp, state)
	data.ID = id
	data.ActionEconomy = nil
	return data
}

// Participation's leniency is safe because its answer reads no condition: an
// unreadable blob beside a Dying sheet changes nothing about the answer. The
// day a condition-sensitive field joins ParticipantParticipation, this is the
// test that says the drop is no longer safe.
func TestParticipationIgnoresAnUnreadableCondition(t *testing.T) {
	ctx := context.Background()
	clean := deathSaveCharacter(0, &saves.DeathSaveState{Successes: 1, Failures: 1})
	blobbed := deathSaveCharacter(0, &saves.DeathSaveState{Successes: 1, Failures: 1})
	blobbed.Conditions = append(blobbed.Conditions, json.RawMessage(`{"ref":"nonsense","x":`))

	want, err := Participation(ctx, &ParticipationInput{Participants: []Participant{{Character: clean}}})
	require.NoError(t, err)
	got, err := Participation(ctx, &ParticipationInput{Participants: []Participant{{Character: blobbed}}})
	require.NoError(t, err, "an unreadable condition does not refuse participation")
	require.Equal(t, want.Members, got.Members, "and does not change its answer")
}
