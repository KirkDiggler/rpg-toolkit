// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// an_empty_item_test.go guards the picture of a compiled site
// (rpg-project#850, rpg-project#1173). A host compares today's compile of a
// site with an older picture of it, so a field the item work added to a type
// inside the compiled site must vanish when it is empty, or every site grows a
// line it never had: `"Item": null` on every placed prop. The fix is a tag
// (`json:"Item,omitempty"`), and this test is what fails HERE, not two
// repositories later, when the next such field forgets it.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// pictureOf is the compiled site as a host makes its picture of it.
func pictureOf(t *testing.T, path string) string {
	t.Helper()
	raw, err := json.MarshalIndent(loadContent(t, path), "", "  ")
	require.NoError(t, err)
	return string(raw)
}

// A site with no custom items compiles to a picture with no item words in it.
// The raider letter has placed props and creatures.
func TestDungeonspec_ASiteWithNoItemsHasNoItemKeysInItsPicture(t *testing.T) {
	picture := pictureOf(t, "testdata/world-builder-v4-raider-letter.yaml")
	require.Contains(t, picture, `"Placed"`, "the site has placed props, or this proves nothing")
	require.Zero(t, strings.Count(picture, `"Item": null`), "a prop that is no item leaves no null")
	require.Zero(t, strings.Count(picture, `"Item"`), "a prop that is no item leaves no key")
	require.Zero(t, strings.Count(picture, `"Equipment"`), "a creature with no gear leaves no key")
}

// The site made for the item work still carries both keys, with the cloak.
func TestDungeonspec_ASiteWithItemsKeepsItsItemKeysInItsPicture(t *testing.T) {
	picture := pictureOf(t, customItemsSite)
	require.Contains(t, picture, `"Item": {`)
	require.Contains(t, picture, `"Equipment": [`)
	require.Contains(t, picture, `"id": "lieutenants-cloak"`)
}
