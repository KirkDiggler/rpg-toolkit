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

// activate_test.go: the site check says a table row that activates an
// ability (rpg-project#1131, slice 3b-2).

const patrolRow = "        - {when: {enemy: none}, toward: {path: [{q: -2, r: 0}, {q: 0, r: -2}, {q: 2, r: 0}, {q: 0, r: 2}]}}\n"

// activateReadBack is the two-patrol room whose gate patrol's `time` table
// says activate, written on the placement's table, so the row travels the
// dialect's own compile (answerOf).
func activateReadBack(t *testing.T) *readback.ReadBack {
	t.Helper()
	raw, err := os.ReadFile("../testdata/world-builder-v4-patrols.yaml") //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	source := string(raw)
	require.Equal(t, 1, strings.Count(source, patrolRow), "the fixture still carries gate-patrol's one row")
	source = strings.Replace(source, patrolRow, "        - { activate: second wind }\n"+patrolRow, 1)
	rb, err := readback.Of([]byte(source))
	require.NoError(t, err)

	return rb
}

// Test 11: the long sentence, the short one, the one connection, and nothing
// "not yet said" for the row.
func TestAnActivateRowIsSaid(t *testing.T) {
	rb := activateReadBack(t)
	var row readback.Statement
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineRow && strings.Contains(st.Sentence, "activates") {
			row = st
		}
	}
	require.NotEmpty(t, row.Sentence, "the row is said")
	require.Contains(t, row.Sentence, "activates “second wind”, when it has that ability and can pay for it")
	require.Contains(t, row.Connects, readback.Connection{Kind: readback.ConnAbility, ID: "second wind", Role: readback.RoleActivates})
	var abilities int
	for _, c := range row.Connects {
		if c.Kind == readback.ConnAbility {
			abilities++
		}
	}
	require.Equal(t, 1, abilities)

	var short bool
	var walk func(items []readback.Item)
	walk = func(items []readback.Item) {
		for _, it := range items {
			short = short || strings.Contains(it.Text, "Activates “second wind”")
			walk(it.Items)
		}
	}
	for _, sec := range rb.Folded() {
		walk(sec.Items)
	}
	require.True(t, short, "the folded form says it in the short sentence")

	for _, un := range rb.NotYetSaid {
		require.NotContains(t, un.Path, "activate", "%+v", un)
	}
}
