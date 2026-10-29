// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_who_test.go is `who` through the save (rpg-toolkit#2149
// review, item a): the knights' `fell: { who: king }` row, compiled from the
// castle file, is written into the blob as `who` and comes back from it
// whole, so a reloaded knight still waits on the king and not on every fall.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// fellWhoOf is the `who` of a member's first `time` row as the blob holds it.
func fellWhoOf(t *testing.T, data encounter.EncounterData, id encounter.MemberID) string {
	t.Helper()
	for _, m := range data.Members {
		if m.ID != id {
			continue
		}
		rows := m.Table[string(encounter.AnswerTime)]
		require.NotEmpty(t, rows)
		require.NotEmpty(t, rows[0].When)
		require.Equal(t, "fell", rows[0].When[0].Deed)

		return rows[0].When[0].Who
	}
	require.FailNowf(t, "not in the blob", "%s", id)

	return ""
}

func TestMadKingWhoSurvivesTheBlob(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, partyStart(t))

	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(t, err)
	require.Contains(t, string(blob), `"who":"king"`, "the row's who is written by its own key")

	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(blob, &data))
	require.Equal(t, "king", fellWhoOf(t, data, kingKeep1))

	s.reload()
	again := s.enc.ToData()
	require.Equal(t, "king", fellWhoOf(t, again, kingKeep1), "and read back, it is still the king's fall")
	require.Equal(t, "king", fellWhoOf(t, again, kingKeep2))
}
