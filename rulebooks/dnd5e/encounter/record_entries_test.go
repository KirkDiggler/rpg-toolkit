// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// recordEntries is the encounter's record from seq from on, as loaded: each
// entry's audience materialised, never its reference into the persisted
// audience table (play/record v0.3.0).
func recordEntries(t testing.TB, enc *encounter.Encounter, from uint64) []record.Entry {
	t.Helper()
	loaded, err := record.LoadLog(enc.ToData().Log)
	require.NoError(t, err, "the encounter's record loads")
	entries, err := loaded.All(&record.AllInput{FromSeq: from})
	require.NoError(t, err)
	return entries
}
