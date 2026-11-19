// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// watch_beats_internal_test.go holds the beats the studio's watch asked of
// the encounter (rpg-project#1001) to a player's stream: a sighting's new
// `observer` changes nothing a player is shown, and a role's step is the
// record's alone.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestASightingIsProjectedTheSameWithItsObserver: the projection takes the
// observer from the recipient, so a sighting reads identically with and
// without the payload's `observer`.
func TestASightingIsProjectedTheSameWithItsObserver(t *testing.T) {
	before := piecesOf("alice", "sight", []byte(`{"beat":"sighted","gained":["g1"],"knowledge_changed":true}`))
	after := piecesOf("alice", "sight", []byte(`{"beat":"sighted","gained":["g1"],"knowledge_changed":true,"observer":"alice"}`))
	require.Len(t, after, 1)
	require.Empty(t, after[0].undecoded)
	require.Equal(t, before, after)
	require.Equal(t, member("alice"), after[0].body.(Perception).Observer, "the observer is the recipient")
}

// TestARoleStepIsRecordOnly: a role's step folds into no act.
func TestARoleStepIsRecordOnly(t *testing.T) {
	require.True(t, recordOnly[encounter.BeatRoleState])
	require.Empty(t, piecesOf("court", encounter.BeatRoleState, []byte(
		`{"beat":"role_state","member":"court","role":"investigate","recipe":"example:behaviors:investigate","from":"idle","to":"active","reason":""}`)))
}
