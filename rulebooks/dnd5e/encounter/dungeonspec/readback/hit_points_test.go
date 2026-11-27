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
	rb := hitPointsReadBack(t, "{ when: { hit_points: { at_most: 30% } }, hold: {} }")
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

// hitPointsSaid finds the row whose long sentence has the words in it, and
// the short sentence in the folded form that has the short words in it.
func hitPointsSaid(t *testing.T, row, long, short string) (string, string) {
	t.Helper()
	rb := hitPointsReadBack(t, row)
	var sentence string
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineRow && strings.Contains(st.Sentence, long) {
			sentence = st.Sentence
		}
	}
	require.NotEmpty(t, sentence, "the row is said with %q", long)

	var found string
	var walk func(items []readback.Item)
	walk = func(items []readback.Item) {
		for _, it := range items {
			if strings.Contains(it.Text, short) {
				found = it.Text
			}
			walk(it.Items)
		}
	}
	for _, sec := range rb.Folded() {
		walk(sec.Items)
	}
	require.NotEmpty(t, found, "the folded form says %q", short)

	return sentence, found
}

// Test 4b-2.9: the site check says a number as a number and a share as a share.
func TestTheSiteCheckSaysAHitPointsNumberAndAShareEachAsItself(t *testing.T) {
	t.Run("a number", func(t *testing.T) {
		sentence, short := hitPointsSaid(t, "{ when: { hit_points: { at_most: 8 } }, hold: {} }",
			"hit points are at most 8", "hit points at most 8")
		t.Log(sentence, "|", short)
		require.Contains(t, sentence, "if Goblin (gate-patrol)'s hit points are at most 8, ")
		require.NotContains(t, sentence, "of its maximum")
		require.NotContains(t, sentence, "%")
		require.NotContains(t, short, "%")
	})
	t.Run("a share", func(t *testing.T) {
		sentence, short := hitPointsSaid(t, "{ when: { hit_points: { at_most: 30% } }, hold: {} }",
			"hit points are at most 30% of its maximum", "hit points at most 30%")
		t.Log(sentence, "|", short)
		require.Contains(t, sentence, "if Goblin (gate-patrol)'s hit points are at most 30% of its maximum, ")
	})
}
