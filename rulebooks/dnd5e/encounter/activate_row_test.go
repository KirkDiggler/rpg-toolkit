// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// activate_row_test.go: the saved form of the `activate` row (rpg-project#1131,
// slice 3b-2). Modelled on TestAReloadDescribesNoTableTheRecordHolds
// (tablebeat_test.go): save, marshal, load.

func TestAnActivateRowIsSavedAndLoadedAndOthersSayNothing(t *testing.T) {
	table := silenceTable()
	table[encounter.AnswerTime] = append([]encounter.Answer{{Weight: 1, Activate: "second wind"}}, table[encounter.AnswerTime]...)
	enc, members := symbolsSceneWith(t, table)
	goblinTurn(t, enc)

	data := enc.ToData()
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	// The member's policy is saved as mind/behavior's own rows, whose key is
	// `Activate`, left out when empty: one row says it and no other carries it.
	require.Equal(t, 1, strings.Count(string(raw), `"Activate"`), "one row says it and no other row carries the key")
	require.Contains(t, string(raw), `"Activate":"second wind"`)

	var saved encounter.EncounterData
	require.NoError(t, json.Unmarshal(raw, &saved))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: saved, Capabilities: symbolsCaps(members)})
	require.NoError(t, err)
	var rows []behavior.Answer
	for _, m := range loaded.ToData().Members {
		if m.ID == goblin {
			rows = m.Behaviors.Instances[0].Instance.Table[encounter.AnswerTime]
		}
	}
	require.NotEmpty(t, rows)
	require.Equal(t, "second wind", rows[0].Activate)
	for _, r := range rows[1:] {
		require.Empty(t, r.Activate)
	}
}
