// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// monster_fixture_test.go provides one shared helper for hand-built
// MonsterInput/AddMonster fixtures across this package's test files.
//
// rpg-toolkit#895's no-fallback rider makes AddMonster reject empty
// DataJSON (NPCAct's old empty-DataJSON fallback, npcActScripted, is
// deleted). Before this rider, most fixtures in this package left
// DataJSON unset — fine for tests that never rehydrate the monster onto
// the rules engine, but no longer accepted at all. testGoblinDataJSON
// gives every such fixture a real, rehydratable registry monster instead
// of a bespoke stat blob.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// testGoblinDataJSON returns valid, rehydratable monster.Data JSON for a
// real registry goblin with the given entity id. Takes core.EntityID (not
// string) because most fixture IDs across this package are already typed
// core.EntityID constants (MonsterInput.ID's own type) — untyped string
// constants/literals convert to it implicitly, so this accepts both without
// per-call-site conversions.
func testGoblinDataJSON(t *testing.T, id core.EntityID) []byte {
	t.Helper()
	b, err := json.Marshal(monster.NewGoblin(string(id)).ToData())
	require.NoError(t, err)
	return b
}
