// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func heldFaerieFire() contributions.Effect {
	return contributions.Effect{
		ID:            "target:" + refs.Conditions.FaerieFire().String() + "@cleric",
		Source:        contributions.Source{Ref: refs.Conditions.FaerieFire(), Name: "Faerie Fire", SourceID: "cleric"},
		Description:   "Attack rolls against you have advantage if the attacker can see you.",
		State:         contributions.StateApplies,
		Reason:        "You can see the outlined target",
		Participation: contributions.ContributesNow,
		Benefit:       "Advantage on the attack roll",
	}
}

// TestHeldRowsFailClosedOnABrokenAnswer: a target resolution did not answer
// for, or a held row whose ID collides with a declaration row, fails the read
// rather than dropping or joining rows.
func TestHeldRowsFailClosedOnABrokenAnswer(t *testing.T) {
	_, err := heldRowsOf(nil, map[string][]contributions.Effect{}, "gob")
	require.ErrorIs(t, err, ErrBadAttack, "a target with no answer at all")

	collides := []EffectRow{{ID: heldFaerieFire().ID}}
	_, err = heldRowsOf(collides, map[string][]contributions.Effect{"gob": {heldFaerieFire()}}, "gob")
	require.ErrorIs(t, err, ErrBadAttack, "a held ID equal to a declaration row's")
}

// TestHeldRowsAreNilWhenNothingBears: an answered target with no held rows —
// nothing that bears, or holdings unknown — carries none.
func TestHeldRowsAreNilWhenNothingBears(t *testing.T) {
	for name, answer := range map[string][]contributions.Effect{"nil": nil, "empty": {}} {
		rows, err := heldRowsOf(nil, map[string][]contributions.Effect{"gob": answer}, "gob")
		require.NoError(t, err, name)
		require.Nil(t, rows, name)
	}

	rows, err := heldRowsOf(nil, map[string][]contributions.Effect{"gob": {heldFaerieFire()}}, "gob")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, heldFaerieFire().ID, rows[0].ID)
	require.Equal(t, EffectApplies, rows[0].State)
}
