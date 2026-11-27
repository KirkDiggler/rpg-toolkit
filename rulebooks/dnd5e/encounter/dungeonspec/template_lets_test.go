// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// template_lets_test.go is the authored half of A STAT BLOCK SAYS LETS
// (rpg-project#1131, slice 2c): a `templates:` entry's `lets`, judged for its
// SHAPE and carried compiled in the author's words. Nothing in a run reads it
// yet, and the rules judge the words inside it.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

const templateLetsSite = "testdata/template-lets/site.yaml"

// templateLetsWith is the site with the first place it writes old changed.
func templateLetsWith(t *testing.T, old, replacement string) string {
	t.Helper()
	raw, err := os.ReadFile(templateLetsSite) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	source := string(raw)
	require.Equal(t, 1, strings.Count(source, old), "the site says %q once", old)
	return strings.Replace(source, old, replacement, 1)
}

const templateLetsOld = "    lets:\n      - use: second wind\n        with: { level: 1 }\n        uses: { max: 1, refills: { on: [short rest, long rest] } }\n"

// Test 1: a stat block that says nothing leaves no trace in the picture. The
// kitchen site has templates and no lets.
func TestDungeonspec_AStatBlockWithNoLetsLeavesNoKeyInItsPicture(t *testing.T) {
	picture := pictureOf(t, "testdata/world-builder-v4-castle-kitchen.yaml")
	require.Contains(t, picture, `"hitDice"`, "the site has stat blocks, or this proves nothing")
	require.Zero(t, strings.Count(picture, `"lets"`), "a stat block that lets nothing leaves no key")
}

// Test 3: the example compiles and carries its entry in the author's words,
// and the compile is not the spec's own.
func TestDungeonspec_AStatBlockCarriesItsLetsInTheAuthorsWords(t *testing.T) {
	c := loadContent(t, templateLetsSite)
	block := c.Templates["gate-lieutenant"]
	require.Len(t, block.Lets, 1)
	entry := block.Lets[0]
	require.Equal(t, "second wind", entry["use"])
	require.Equal(t, map[string]any{"level": 1}, entry["with"])
	require.Equal(t, map[string]any{"max": 1, "refills": map[string]any{"on": []any{"short rest", "long rest"}}}, entry["uses"])

	raw, err := json.Marshal(block)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"lets":[{"use":"second wind","uses":{"max":1,"refills":{"on":["short rest","long rest"]}},"with":{"level":1}}]`)
}

// Changing the compiled value does not reach the spec it was made from.
func TestDungeonspec_ACompiledStatBlocksLetsAreNotTheSpecs(t *testing.T) {
	raw, err := os.ReadFile(templateLetsSite) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err)
	spec := decoded.Spec
	c, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: spec})
	require.NoError(t, err)

	got := c.Templates["gate-lieutenant"]
	got.Lets[0]["use"] = "changed"
	got.Lets[0]["with"].(map[string]any)["level"] = 99
	got.Lets[0] = nil

	kept := spec.Templates["gate-lieutenant"].Lets[0]
	require.Equal(t, "second wind", kept["use"])
	require.Equal(t, map[string]any{"level": 1}, kept["with"])
}

// Test 4: the shape is judged, at its place; the words are not.
func TestDungeonspec_AStatBlocksLetsAreJudgedForShapeNotForWords(t *testing.T) {
	const (
		at   = "templates.gate-lieutenant.lets"
		list = "lets is a list of entries: what the creature can use, each written with use"
	)
	for name, tc := range map[string]struct {
		replacement, path, message string
	}{
		"a scalar":                  {"    lets: second wind\n", at, "a list"},
		"an empty list":             {"    lets: []\n", at, "leave `lets` out for a creature that has none"},
		"a mapping":                 {"    lets: { use: second wind }\n", at, "a list"},
		"an entry that is text":     {"    lets:\n      - second wind\n", at + "[0]", "a mapping"},
		"an entry with no use":      {"    lets:\n      - with: { level: 1 }\n", at + "[0].use", "use"},
		"an empty use":              {"    lets:\n      - use: \"\"\n", at + "[0].use", "use"},
		"a use that is a number":    {"    lets:\n      - use: 7\n", at + "[0].use", "use"},
		"a value JSON cannot carry": {"    lets:\n      - use: second wind\n        with: { level: .nan }\n", at + "[0]", "cannot be written"},
	} {
		t.Run(name, func(t *testing.T) {
			errs := refusals(t, templateLetsWith(t, templateLetsOld, tc.replacement))
			require.NotEmpty(t, errs)
			requireRefusedAt(t, errs, tc.path, tc.message)
		})
	}

	// The words are the rules' to judge.
	for name, replacement := range map[string]string{
		"an ability nobody has heard of": "    lets:\n      - use: fireball\n",
		"a level of nought":              "    lets:\n      - use: second wind\n        with: { level: 0 }\n",
		"both cost and uses":             "    lets:\n      - use: second wind\n        cost: { charges: 1 }\n        uses: { max: 1 }\n",
	} {
		t.Run(name, func(t *testing.T) {
			require.Empty(t, refusals(t, templateLetsWith(t, templateLetsOld, replacement)))
		})
	}
}
