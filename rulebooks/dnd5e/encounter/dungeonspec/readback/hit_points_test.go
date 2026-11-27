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

// hit_points_test.go: the site check says a row whose `when` reads the
// creature's own hit points (rpg-project#1131, slice 4b).

func hitPointsReadBack(t *testing.T, row string) *readback.ReadBack {
	t.Helper()
	raw, err := os.ReadFile("../testdata/world-builder-v4-patrols.yaml") //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	source := string(raw)
	require.Equal(t, 1, strings.Count(source, patrolRow), "the fixture still carries gate-patrol's one row")
	source = strings.Replace(source, patrolRow, "        - "+row+"\n"+patrolRow, 1)
	rb, err := readback.Of([]byte(source))
	require.NoError(t, err)

	return rb
}

// Test 12: the long sentence, the short one, no new connection, nothing "not
// yet said" for the row.
func TestAHitPointsConditionIsSaid(t *testing.T) {
	rb := hitPointsReadBack(t, "{ when: { hit_points: { at_most: 30 } }, hold: {} }")
	var row readback.Statement
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineRow && strings.Contains(st.Sentence, "hit points are at most 30% of its maximum") {
			row = st
		}
	}
	require.NotEmpty(t, row.Sentence, "the row is said")
	t.Log(row.Sentence)
	require.Contains(t, row.Sentence, "if Goblin (gate-patrol)'s hit points are at most 30% of its maximum, ")
	// The same row with no condition: the connections are the ones it had.
	var bare readback.Statement
	for _, st := range hitPointsReadBack(t, "{ hold: {} }").Statements() {
		if st.Kind == readback.KindRoutineRow && strings.Contains(st.Sentence, "holds where it stands") &&
			strings.Contains(st.Sentence, "row 1 of") {
			bare = st
		}
	}
	require.NotEmpty(t, bare.Sentence)
	require.Equal(t, bare.Connects, row.Connects, "it reads the creature's own state: nothing to connect to")

	var short string
	var walk func(items []readback.Item)
	walk = func(items []readback.Item) {
		for _, it := range items {
			if strings.Contains(it.Text, "hit points at most 30%") {
				short = it.Text
			}
			walk(it.Items)
		}
	}
	for _, sec := range rb.Folded() {
		walk(sec.Items)
	}
	require.NotEmpty(t, short, "the folded form says it in the short sentence")
	t.Log(short)

	for _, un := range rb.NotYetSaid {
		require.NotContains(t, un.Path, "hit_points", "%+v", un)
	}
}
