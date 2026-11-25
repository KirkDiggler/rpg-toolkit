// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Test 2. An item's id on the looter's sheet.
func TestLootedItemID(t *testing.T) {
	id := lootedItemID("sess", "guard", "lieutenants-cloak")
	require.Equal(t, id, lootedItemID("sess", "guard", "lieutenants-cloak"), "the same when asked twice")
	require.NotEqual(t, id, lootedItemID("sess", "captain", "lieutenants-cloak"), "another body, another item")
	require.NotEqual(t, id, lootedItemID("other", "guard", "lieutenants-cloak"), "another run, another item")
	require.Regexp(t, regexp.MustCompile(`^lieutenants-cloak-[0-9a-f]{8}$`), id)
	require.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]+$`), lootedItemID("run:1/a b", "g.u:ard", "x_y"),
		"only letters, digits, '-' and '_' whatever the session and body are called")
}

// Test 6, one level down: the delivering function refuses a grant the rules
// refuse, names the item, and leaves the sheet as it was.
func TestDeliverTakenRefusesWhatTheRulesRefuse(t *testing.T) {
	sheet := &character.Data{ID: "alice"}
	bad := encounter.ItemGrant{ID: "lieutenants-cloak", Name: "Lieutenant's cloak",
		Item: json.RawMessage(`{"name":"Lieutenant's cloak","slot":"cape","lets":[{"use":"second wind","with":{"level":1}}]}`)}
	err := deliverTaken("sess", "guard", sheet, []encounter.TakenItem{{Member: "alice", Grant: bad}})
	require.ErrorIs(t, err, ErrUnknownContent)
	require.Contains(t, err.Error(), "lieutenants-cloak")
	require.Empty(t, sheet.Items)
}
