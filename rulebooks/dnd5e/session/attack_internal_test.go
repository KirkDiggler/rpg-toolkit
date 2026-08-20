// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// TestRecordUsesAggregateFromTypedStrikeOutcome pins the session boundary:
// resolution may retain typed damage evidence, but session recording writes
// only the aggregate amount into encounter history.
func TestRecordUsesAggregateFromTypedStrikeOutcome(t *testing.T) {
	struck := resolution.StrikeOutcome{
		Hit:    true,
		Damage: 9,
		DamageInstances: []damage.Instance{
			{Amount: 5, Type: damage.Slashing},
			{Amount: 4, Type: damage.Fire},
		},
	}

	record := recordFor(&AttackInput{Attacker: "alice", Target: "bob"}, struck)
	require.Equal(t, 9, record.Values[encounter.ValueAmount])
}

// halfBrokenCharacters writes the first sheet and refuses the second.
type halfBrokenCharacters struct {
	written []string
	err     error
}

func (h *halfBrokenCharacters) GetCharacter(context.Context, string) (*character.Data, error) {
	return nil, ErrNotFound
}

func (h *halfBrokenCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	if len(h.written) > 0 {
		return h.err
	}
	h.written = append(h.written, data.ID)
	return nil
}

// TestAPartialCharacterSaveNamesWhatLanded pins S6 across MULTIPLE sheets.
//
// One swing can dirty both sides — the target takes damage, and an effect that
// writes the attacker's own sheet dirties them too. If the second write fails,
// the first is ALREADY DURABLE, and a caller told only about the failure would
// retry a write that succeeded. Naming both halves is the difference between
// repair and retry, which is the whole reason the report exists.
//
// Tested against saveDirty directly rather than through a swing, because no v1
// interaction dirties two character sheets: the strike damages its target and
// nothing yet writes the attacker back. A test that drove this through Attack
// would skip, and a skipping test proves nothing. When an effect that writes
// the swinger arrives, this pin is already here and already honest.
func TestAPartialCharacterSaveNamesWhatLanded(t *testing.T) {
	broken := errors.New("store is down")
	store := &halfBrokenCharacters{err: broken}
	m := &Manager{characters: store}

	err := m.saveDirty(context.Background(), &writeScope{data: &SessionData{}}, &resolution.Output{
		DirtyCharacters: []*character.Data{{ID: "alice"}, {ID: "bob"}},
	})

	require.Error(t, err)
	var saved *SaveError
	require.ErrorAs(t, err, &saved)
	require.ErrorIs(t, err, broken)

	require.Equal(t, []string{"character:alice"}, saved.Report.Written,
		"alice's sheet is durable and the caller must not be told to retry it")
	require.Equal(t, []string{"character:bob"}, saved.Report.Failed,
		"and bob's is the one that needs repair")
}
