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

// withoutFreeReactions is a member's stored conditions less the reactions it
// carries by existing ([conditions.FreeReactions]). A character's attach
// records those on its sheet (rpg-toolkit#1958 item 9), so every sheet that
// comes back holds them; a test about what an interaction put on or took off
// reads what is left.
func withoutFreeReactions(t testing.TB, member string, stored []json.RawMessage) []json.RawMessage {
	t.Helper()
	free := make(map[string]bool)
	for _, reaction := range conditions.FreeReactions(member) {
		free[reaction.Ref().String()] = true
	}
	kept := make([]json.RawMessage, 0, len(stored))
	for _, raw := range stored {
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		require.NoError(t, json.Unmarshal(raw, &peek))
		if !free[peek.Ref.String()] {
			kept = append(kept, raw)
		}
	}

	return kept
}
