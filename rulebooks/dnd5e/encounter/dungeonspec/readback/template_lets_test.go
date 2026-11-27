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

// template_lets_test.go: the site check says what a stat block's creature can
// use (rpg-project#1131, slice 2c).

const templateLetsSite = "../testdata/template-lets/site.yaml"

const templateLetsOld = "    lets:\n      - use: second wind\n        with: { level: 1 }\n        uses: { max: 1, refills: { on: [short rest, long rest] } }\n"

func templateLetsReadBack(t *testing.T, old, replacement string) *readback.ReadBack {
	t.Helper()
	raw, err := os.ReadFile(templateLetsSite) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	source := string(raw)
	if old != "" {
		require.Equal(t, 1, strings.Count(source, old))
		source = strings.Replace(source, old, replacement, 1)
	}
	rb, err := readback.Of([]byte(source))
	require.NoError(t, err)
	return rb
}

func letsStatements(rb *readback.ReadBack) []readback.Statement {
	var out []readback.Statement
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindTemplateLets {
			out = append(out, st)
		}
	}
	return out
}

// Test 5: exactly one statement, in the sentence, at its place, with its link.
func TestAStatBlockThatLetsIsSaid(t *testing.T) {
	got := letsStatements(templateLetsReadBack(t, "", ""))
	require.Len(t, got, 1)
	st := got[0]
	require.Equal(t, "templates.gate-lieutenant.lets", st.Path)
	require.Equal(t, readback.Subject{Kind: "template", ID: "gate-lieutenant"}, st.About)
	require.Equal(t, "The stat block `gate-lieutenant` (Lieutenant) can use “second wind”, 1 use. "+
		"What that is and how it is counted is written in its own words under `templates.gate-lieutenant.lets`, "+
		"and the rules judge them. Nothing makes a creature use what it has yet.", st.Sentence)
	require.Equal(t, []readback.Connection{{Kind: readback.ConnAbility, ID: "second wind", Role: readback.RoleLets}}, st.Connects)
}

func TestAStatBlockEntryWithNoUsesSaysNoCount(t *testing.T) {
	got := letsStatements(templateLetsReadBack(t, templateLetsOld, "    lets:\n      - use: second wind\n"))
	require.Len(t, got, 1)
	require.Contains(t, got[0].Sentence, "can use “second wind”, with no count of its own.")
}

func TestAStatBlockWithTwoEntriesSaysBoth(t *testing.T) {
	got := letsStatements(templateLetsReadBack(t, templateLetsOld,
		"    lets:\n      - use: second wind\n        uses: { max: 2 }\n      - use: action surge\n"))
	require.Len(t, got, 1)
	require.Contains(t, got[0].Sentence, "can use “second wind”, 2 uses and “action surge”, with no count of its own.")
	require.Equal(t, []readback.Connection{
		{Kind: readback.ConnAbility, ID: "second wind", Role: readback.RoleLets},
		{Kind: readback.ConnAbility, ID: "action surge", Role: readback.RoleLets},
	}, got[0].Connects)
}

func TestASiteWithNoLetsHasNoLetsStatement(t *testing.T) {
	rb := templateLetsReadBack(t, templateLetsOld, "")
	require.Empty(t, letsStatements(rb))
	var found bool
	for _, st := range rb.Statements() {
		found = found || st.Kind == readback.KindTemplate
	}
	require.True(t, found, "the stat block itself is still said")
}

// Test 6: nothing under a template's lets is "not yet said".
func TestAStatBlocksLetsAreNotUnsaid(t *testing.T) {
	rb := templateLetsReadBack(t, "", "")
	for _, un := range rb.NotYetSaid {
		require.NotContains(t, un.Path, "lets", "%+v", un)
	}
}
