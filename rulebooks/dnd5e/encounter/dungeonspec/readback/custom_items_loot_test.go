// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// Test 10 (rpg-project#1167): the statement for a creature with equipment says
// looting its body gives those items to the looter; a prop's item statement
// does not change, because nothing takes a prop yet.
func TestEquipmentSaysLootingTheBodyGivesIt(t *testing.T) {
	raw, err := os.ReadFile("../testdata/custom-items/site.yaml") //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	rb, err := readback.Of(raw)
	require.NoError(t, err)

	var eq, prop *readback.Statement
	for i, st := range rb.Statements() {
		switch st.Kind {
		case readback.KindEquipment:
			eq = &rb.Statements()[i]
		case readback.KindPropItem:
			prop = &rb.Statements()[i]
		}
	}
	require.NotNil(t, eq)
	require.Contains(t, eq.Sentence, "has “Lieutenant's cloak” equipped")
	require.Contains(t, eq.Sentence, "Looting its body gives “Lieutenant's cloak” to the looter.")
	require.NotNil(t, prop)
	require.Contains(t, prop.Sentence, "is the item “Lieutenant's cloak”")
	require.NotContains(t, prop.Sentence, "loot")
	require.NotContains(t, prop.Sentence, "gives")
}
