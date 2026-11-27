// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// castleContentHash is the castle's content hash. A change here is a change
// to the castle or to what the hash covers: make it deliberately.
// The combined compiled shape includes StructuralWallInput.Sections, even nil.
// This pin must be recomputed after a base merge, not taken from either side.
// ContentHash is engine-version-local; legacy persisted SiteHash is separate.
const castleContentHash = "e337704d5b0373a5bf586a93004b3f9b51f1379f0982e66dc707321a9be32ae6"

func contentHashOf(t *testing.T, src string) encounter.ContentHash {
	t.Helper()
	c, err := dungeonspec.Load([]byte(src))
	require.NoError(t, err)
	h, err := c.ContentHash()
	require.NoError(t, err)
	return h
}

// fileOf is a content file's source.
func fileOf(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}

// hashEdited is src with the first place it writes old changed.
func hashEdited(t *testing.T, src, old, replacement string) string {
	t.Helper()
	require.Contains(t, src, old, "precondition: the file writes %q", old)
	return strings.Replace(src, old, replacement, 1)
}

// TestTheCastlesContentHashIsPinned: a change to the castle, or to what the
// hash covers, is made deliberately.
func TestTheCastlesContentHashIsPinned(t *testing.T) {
	require.Equal(t, encounter.ContentHash(castleContentHash), contentHashOf(t, fileOf(t, "testdata/world-builder-v4-castle.yaml")))
}

// TestTheContentHashNamesAStatus (rpg-project#1024): a status an author wrote
// is content — its name, its rules and its when_given, written or not, change
// the hash; how its YAML is laid out does not. Cut from the small vault so it
// stays quick.
func TestTheContentHashNamesAStatus(t *testing.T) {
	t.Parallel()
	const fed = "    again: ignored\n    when_given:\n      - heal: 2\n"
	vault := vaultWithAStatus(t, "    again: ignored\n", fed)
	v := contentHashOf(t, vault)
	require.Equal(t, v, contentHashOf(t, vault), "two compiles of one file")
	require.NotEqual(t, v, contentHashOf(t, hashEdited(t, vault, "name: Rested", "name: Rested Well")), "a status's name")
	require.NotEqual(t, v, contentHashOf(t, hashEdited(t, vault, "- ends_on: rest", "- ends_on: run")), "a status's maintenance")
	require.NotEqual(t, v, contentHashOf(t, hashEdited(t, vault, "- heal: 2", "- heal: 3")), "a status's when_given")
	require.NotEqual(t, v, contentHashOf(t, hashEdited(t, vault, fed, "    again: ignored\n")), "whether a status has a when_given")
	require.Equal(t, v, contentHashOf(t, hashEdited(t, vault, "    when_given:\n      - heal: 2\n", "    when_given: [ { heal: 2 } ]\n")),
		"a status's layout is not content")
}

// TestTheContentHashNamesWhatTheAuthorWrote (rpg-project#1001): two compiles
// of one file hash alike; a comment does not change it; a line's words, a
// DC, a machine's step and a post each do.
func TestTheContentHashNamesWhatTheAuthorWrote(t *testing.T) {
	machine := fileOf(t, "readback/testdata/machine-site.yaml")
	site := fileOf(t, "testdata/world-builder-v4-site.yaml")
	m, s := contentHashOf(t, machine), contentHashOf(t, site)
	require.Equal(t, m, contentHashOf(t, machine), "two compiles of one file")
	require.Equal(t, m, contentHashOf(t, hashEdited(t, machine, "# THE MACHINE SITE", "# The machine site")), "a comment is not content")
	require.NotEqual(t, m, contentHashOf(t, hashEdited(t, machine, "say: Halt.", "say: Stop.")), "a line's words")
	require.NotEqual(t, m, contentHashOf(t, hashEdited(t, machine, "reason: stood aside", "reason: stood back")), "a machine's step")
	require.NotEqual(t, m, contentHashOf(t, hashEdited(t, machine, "against: entering", "against: leaving")), "a post")
	require.NotEqual(t, s, contentHashOf(t, hashEdited(t, site, "dc: ", "dc: 2")), "a check's DC")

	// WHAT THE TYPES' JSON DROPS (rpg-project#1001): a void and an
	// orientation encode as {}, and still change the hash.
	tomb := fileOf(t, "testdata/reference-tomb.yaml")
	require.NotEqual(t, contentHashOf(t, tomb), contentHashOf(t, hashEdited(t, tomb, "void: opaque", "void: transparent")),
		"the void changes what every creature can see")
	pointy := dungeonspec.Compiled{Key: "k", Field: encounter.FieldInput{Canvas: encounter.CanvasInput{
		Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}}}
	flat := pointy
	flat.Field.Canvas.Orientation = encounter.HexesAreFlatTop()
	hp, err := pointy.ContentHash()
	require.NoError(t, err)
	hf, err := flat.ContentHash()
	require.NoError(t, err)
	require.NotEqual(t, hp, hf, "the hex orientation")
}
