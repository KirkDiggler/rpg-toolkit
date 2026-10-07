// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// opportunityAttack is the ref of the free reaction every attached character
// sheet stores exactly once (rpg-toolkit#1959): a test that lists a sheet's
// conditions names it, so a write path that drops or doubles it fails.
var opportunityAttack = refs.Conditions.OpportunityAttack().String()

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
