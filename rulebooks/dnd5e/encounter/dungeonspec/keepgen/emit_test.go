// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEmit_Deterministic: equal inputs, equal bytes; a different seed moves
// the props and nothing about the geometry.
func TestEmit_Deterministic(t *testing.T) {
	g := keepAt(t, 10000)
	o := g.opts
	a, ra := g.doc, g.report
	b, rb, err := Generate(DefaultPlan(), o)
	require.NoError(t, err)
	require.Equal(t, a, b)
	require.Equal(t, ra, rb)

	o.Seed = 2
	c, rc, err := Generate(DefaultPlan(), o)
	require.NoError(t, err)
	require.NotEqual(t, a, c, "the seed moves the placements")
	require.Equal(t, ra.GeometryBytes, rc.GeometryBytes, "and not the walls or the regions")
	require.Equal(t, ra.Walls, rc.Walls)
}

// TestEmit_HeaderCarriesCommand: the opening comment records the command
// line and every option.
func TestEmit_HeaderCarriesCommand(t *testing.T) {
	g := keepAt(t, 30000)
	head := string(g.doc[:strings.Index(string(g.doc), "version: 5")])
	require.Contains(t, head, "# keepgen-command: keepgen\n")
	require.Contains(t, head, "# keepgen-options: hexes=30000 seed=1 props=12.5 grounds-props=2.5 monsters=1 key=bench-keep\n")
	require.True(t, strings.HasPrefix(string(g.doc), "# "), "the header opens the file")
}

// TestOptionsFromHeader_RoundTrip: the header gives back the Options that
// made the file, so regenerating from it gives the same bytes.
func TestOptionsFromHeader_RoundTrip(t *testing.T) {
	o := Options{
		Hexes: 12345, Seed: -7, PropDensity: 3.3333, GroundsDensity: 0, MonsterDensity: 0.25,
		Key: "bench-keep-odd", CommandLine: "keepgen -hexes 12345 -seed -7 -props 3.3333 -grounds-props 0 -monsters 0.25 -key bench-keep-odd -out k.yaml",
	}
	doc, _, err := Generate(DefaultPlan(), o)
	require.NoError(t, err)
	got, err := OptionsFromHeader(doc)
	require.NoError(t, err)
	require.Equal(t, o, got)
	again, _, err := Generate(DefaultPlan(), got)
	require.NoError(t, err)
	require.Equal(t, doc, again)

	g := keepAt(t, 10000)
	got, err = OptionsFromHeader(g.doc)
	require.NoError(t, err)
	require.Equal(t, g.opts, got)
}

// TestOptionsFromHeader_Refuses: no header, a missing option, a bad value.
func TestOptionsFromHeader_Refuses(t *testing.T) {
	_, err := OptionsFromHeader([]byte("version: 5\n"))
	require.ErrorContains(t, err, "no \"# keepgen-options:\" line")
	_, err = OptionsFromHeader([]byte("# keepgen-command: x\n# keepgen-options: hexes=1 seed=1\n"))
	require.ErrorContains(t, err, `option "props" is missing`)
	_, err = OptionsFromHeader([]byte("# keepgen-command: x\n# keepgen-options: hexes=ten\n"))
	require.ErrorContains(t, err, "hexes")
}

// goldenKeep is the SHA-256 of the 10,000-hex keep at seed 1 and the default
// densities, key bench-keep, command line "keepgen". A change here is a
// change in what the generator writes: regenerate the api's shelf copies
// (rpg-api content/bench-keep*.yaml) when it moves.
const goldenKeep = "62e1aa12b46e03c773445cb5d850fb9a330773697ba9efe0a416872a082c3104"

// TestGenerate_Golden pins the emitter's output, so any shift in it is
// visible in the toolkit PR that causes it.
func TestGenerate_Golden(t *testing.T) {
	g := keepAt(t, 10000)
	require.Equal(t, "keepgen", g.opts.CommandLine)
	require.Equal(t, DefaultOptions().Seed, g.opts.Seed)
	sum := sha256.Sum256(g.doc)
	require.Equal(t, goldenKeep, hex.EncodeToString(sum[:]))
}
