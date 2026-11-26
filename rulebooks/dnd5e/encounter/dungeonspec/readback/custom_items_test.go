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

// Test 4 (rpg-project#1181): the statement for an item says whether a character
// owns at most one of it or any number.
func TestAnItemStatementSaysUniqueOrAnyNumber(t *testing.T) {
	const (
		one = "A character owns at most one of it: a looter who already has it leaves it on the body."
		any = "A character can own any number of it: each one looted is its own item."
	)
	for name, tc := range map[string]struct{ file, old, replacement, want string }{
		"unique true":  {"../testdata/unique-item/site.yaml", "", "", one},
		"unique false": {"../testdata/unique-item/site.yaml", "unique: true", "unique: false", any},
		"word absent":  {"../testdata/custom-items/site.yaml", "", "", any},
		"false is any": {"../testdata/custom-items/site.yaml", "    slot: cloak\n", "    slot: cloak\n    unique: false\n", any},
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(tc.file) //nolint:gosec // a test reading its own fixture
			require.NoError(t, err)
			source := string(raw)
			if tc.old != "" {
				require.Equal(t, 1, strings.Count(source, tc.old))
				source = strings.Replace(source, tc.old, tc.replacement, 1)
			}
			rb, err := readback.Of([]byte(source))
			require.NoError(t, err)
			for _, st := range rb.Statements() {
				if st.Kind == readback.KindItem {
					require.True(t, strings.HasSuffix(st.Sentence, tc.want), st.Sentence)
					other := one
					if tc.want == one {
						other = any
					}
					require.NotContains(t, st.Sentence, other)
					return
				}
			}
			require.Fail(t, "no item statement")
		})
	}
}
