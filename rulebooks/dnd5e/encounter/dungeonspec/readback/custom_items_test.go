// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// Test 8 (rpg-project#1166): the read-back of the custom-items site says the
// three words — the declared item, who has it equipped and which prop is one
// — by name. The item and the prop say nothing of looting or taking; the
// creature's statement says looting it gives the item (TestEquipmentSaysLootingTheBodyGivesIt).
func TestCustomItemsAreSaid(t *testing.T) {
	raw, err := os.ReadFile("../testdata/custom-items/site.yaml") //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	rb, err := readback.Of(raw)
	require.NoError(t, err)

	byKind := map[readback.Kind]readback.Statement{}
	for _, st := range rb.Statements() {
		switch st.Kind {
		case readback.KindItem, readback.KindEquipment, readback.KindPropItem:
			byKind[st.Kind] = st
		}
	}

	item := byKind[readback.KindItem]
	require.Equal(t, "customItems.lieutenants-cloak", item.Path)
	require.Equal(t, readback.Subject{Kind: readback.ConnItem, ID: "lieutenants-cloak"}, item.About)
	require.Contains(t, item.Sentence, "named “Lieutenant's cloak”")
	require.Contains(t, item.Sentence, "the rules judge them when a run starts")

	eq := byKind[readback.KindEquipment]
	require.Equal(t, "room.room.monsterBindings.lieutenant.equipment", eq.Path)
	require.Contains(t, eq.Sentence, "has “Lieutenant's cloak” equipped")
	require.Contains(t, eq.Connects, readback.Connection{Kind: readback.ConnItem, ID: "lieutenants-cloak", Role: readback.RoleEquips})

	prop := byKind[readback.KindPropItem]
	require.Equal(t, "room.room.propBindings.cloak-on-the-peg.item", prop.Path)
	require.Contains(t, prop.Sentence, "is the item “Lieutenant's cloak”")
	require.Contains(t, prop.Connects, readback.Connection{Kind: readback.ConnItem, ID: "lieutenants-cloak", Role: readback.RoleIsItem})

	// The creature's own statement now says looting its body gives the items
	// (rpg-project#1167), so only the other two keep the no-loot rule.
	for _, st := range []readback.Statement{item, prop} {
		for _, word := range []string{"loot", "take", "taken", "gives"} {
			require.False(t, strings.Contains(strings.ToLower(st.Sentence), word),
				"nothing in a run uses items yet, so %q does not say %q", st.Sentence, word)
		}
	}
	for _, un := range rb.NotYetSaid {
		require.NotContains(t, un.Path, "equipment")
		require.NotContains(t, un.Path, "customItems")
	}
}
