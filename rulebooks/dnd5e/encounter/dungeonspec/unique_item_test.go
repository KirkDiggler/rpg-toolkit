// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// unique_item_test.go is A SITE FILE SAYS A CUSTOM ITEM IS UNIQUE
// (rpg-project#850, rpg-project#1181): `unique: true` on an item under
// `customItems:`. The format judges its shape and carries it as written; the
// rules and the session read it elsewhere.
//
// The site is testdata/unique-item/site.yaml, a copy of
// testdata/custom-items/site.yaml with `unique: true` on the cloak.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const uniqueItemSite = "testdata/unique-item/site.yaml"

const uniqueWord = `"unique" is true or false: true means a character owns at most one of this item`

// uniqueWith is the unique site with its `unique: true` line replaced.
func uniqueWith(t *testing.T, replacement string) string {
	t.Helper()
	raw, err := os.ReadFile(uniqueItemSite) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(raw), "    unique: true\n"))
	return strings.Replace(string(raw), "    unique: true\n", replacement, 1)
}

// Test 1: the word loads and is carried as written.
func TestDungeonspec_UniqueLoadsAndIsCarriedAsWritten(t *testing.T) {
	raw, err := os.ReadFile(uniqueItemSite) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	require.Empty(t, refusals(t, string(raw)))

	c := loadContent(t, uniqueItemSite)
	block := string(monsterNamed(t, c, "lieutenant").Equipment[0].Item)
	require.Contains(t, block, `"unique":true`)
	require.Equal(t, string(placedNamed(t, c, "cloak-on-the-peg").Item.Item), block)
}

// `false` is a boolean too.
func TestDungeonspec_UniqueFalseIsAccepted(t *testing.T) {
	require.Empty(t, refusals(t, uniqueWith(t, "    unique: false\n")))
}

// Test 2: anything but a YAML boolean is refused at the word's own path. This
// module's YAML reader (yaml.v3, YAML 1.2 core) reads `yes` as the text "yes",
// so it is refused like `sometimes`.
func TestDungeonspec_UniqueThatIsNotABooleanIsRefused(t *testing.T) {
	for name, line := range map[string]string{
		"a word":        "    unique: sometimes\n",
		"yes":           "    unique: yes\n",
		"a number":      "    unique: 2\n",
		"a list":        "    unique: [true]\n",
		"nothing":       "    unique:\n",
		"a quoted true": "    unique: \"true\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			requireExactDefect(t, refusals(t, uniqueWith(t, line)), "customItems.lieutenants-cloak.unique", uniqueWord)
		})
	}
}

// Test 5: the site that says nothing compiles to the block it always did.
func TestDungeonspec_ASiteWithoutUniqueCompilesAsBefore(t *testing.T) {
	c := loadContent(t, customItemsSite)
	require.Equal(t, cloakBlock, string(monsterNamed(t, c, "lieutenant").Equipment[0].Item))
	require.NotContains(t, cloakBlock, "unique")
}
