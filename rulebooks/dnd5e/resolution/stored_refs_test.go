// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
)

// storedRefs lists the refs of a sheet's stored conditions, in stored order,
// nothing filtered out.
func storedRefs(t testing.TB, stored []json.RawMessage) []string {
	t.Helper()
	found := make([]string, 0, len(stored))
	for _, raw := range stored {
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		require.NoError(t, json.Unmarshal(raw, &peek))
		found = append(found, peek.Ref.String())
	}

	return found
}

// freeReactions is the free reactions a sheet carries as the rulebook writes
// them when it makes the sheet: stored with the member, because attach grants
// none (rpg-toolkit#1965). A fixture that wants a reactor stores them; a
// fixture that does not is a member nobody can provoke.
func freeReactions(t testing.TB, member string) []json.RawMessage {
	t.Helper()
	stored := make([]json.RawMessage, 0, 1)
	for _, reaction := range conditions.FreeReactions(member) {
		raw, err := reaction.ToJSON()
		require.NoError(t, err)
		stored = append(stored, raw)
	}

	return stored
}
