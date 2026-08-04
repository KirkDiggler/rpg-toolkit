// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	wallFromPath       = "walls[0].from"
	wallAdjacencyError = "walls[0]: endpoints must be adjacent pointy-top odd-q floor hexes"
)

// TestDecode_WallsOptionalNullAndStrictShape keeps the authored-wall grammar
// strict where an entry is present while preserving the ordinary omitted/null
// parity expected of an optional top-level collection.
func TestDecode_WallsOptionalNullAndStrictShape(t *testing.T) {
	validWall := `
walls:
  - {from: [1, 1], to: [2, 1], kind: solid}
`
	cases := []struct {
		name    string
		raw     string
		wantLen int
		wantErr string
	}{
		{name: "omitted", raw: placedTombYAML},
		{name: "null", raw: placedTombYAML + "\nwalls: null\n"},
		{name: "valid", raw: placedTombYAML + validWall, wantLen: 1},
		{
			name:    "from short sequence",
			raw:     placedTombYAML + "\nwalls: [{from: [1], to: [2, 1], kind: solid}]\n",
			wantErr: startDecodeError,
		},
		{
			name:    "to scalar",
			raw:     placedTombYAML + "\nwalls: [{from: [1, 1], to: 2, kind: solid}]\n",
			wantErr: startDecodeError,
		},
		{
			name:    "null endpoint",
			raw:     placedTombYAML + "\nwalls: [{from: null, to: [2, 1], kind: solid}]\n",
			wantErr: wallFromPath,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := dungeonspec.Decode([]byte(tc.raw))
			if tc.wantErr == startDecodeError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			if tc.wantErr != "" {
				err = dungeonspec.Validate(spec)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.Len(t, spec.Walls, tc.wantLen)
			if tc.wantLen == 1 {
				require.NotNil(t, spec.Walls[0].From)
				assert.Equal(t, [2]int{1, 1}, *spec.Walls[0].From)
			}
		})
	}
}

// TestValidate_WallsRejectsNonCanonicalOrNonFloorEdges proves the source
// grammar reports the authored field path for every prohibited topology.
func TestValidate_WallsRejectsNonCanonicalOrNonFloorEdges(t *testing.T) {
	cases := []struct {
		name    string
		walls   string
		wantErr string
	}{
		{
			name:    "unknown kind",
			walls:   "walls: [{from: [1, 1], to: [2, 1], kind: curtain}]",
			wantErr: "walls[0].kind",
		},
		{
			name:    "same endpoint",
			walls:   "walls: [{from: [1, 1], to: [1, 1], kind: solid}]",
			wantErr: "walls[0]",
		},
		{
			name:    "non adjacent",
			walls:   "walls: [{from: [1, 1], to: [4, 1], kind: solid}]",
			wantErr: "walls[0]",
		},
		{
			name:    "outside footprint",
			walls:   "walls: [{from: [-1, 1], to: [0, 1], kind: solid}]",
			wantErr: wallFromPath,
		},
		{
			name:    "connector gap",
			walls:   "walls: [{from: [10, 1], to: [9, 1], kind: solid}]",
			wantErr: wallFromPath,
		},
		{
			name: "duplicate reversed",
			walls: `walls:
  - {from: [1, 1], to: [2, 1], kind: solid}
  - {from: [2, 1], to: [1, 1], kind: solid}`,
			wantErr: "walls[1]",
		},
		{
			name: "conflicting reversed",
			walls: `walls:
  - {from: [1, 1], to: [2, 1], kind: solid}
  - {from: [2, 1], to: [1, 1], kind: door}`,
			wantErr: "walls[1]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := dungeonspec.Decode([]byte(validM1YAML + "\n" + tc.walls + "\n"))
			require.NoError(t, err)
			err = dungeonspec.Validate(spec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestLoad_WallsCompilesCanonicalPointyTopOddQSpecimenEdges proves corrected
// specimen pairs are converted through the one pointy-top odd-q [column,row]
// path and that the compiler owns deterministic door identity.
func TestLoad_WallsCompilesCanonicalPointyTopOddQSpecimenEdges(t *testing.T) {
	const walls = `
walls:
  - {from: [7, 1], to: [8, 1], kind: solid}
  - {from: [7, 3], to: [8, 3], kind: door}
  - {from: [10, 3], to: [11, 3], kind: door}
`
	compiled, err := dungeonspec.Load([]byte(placedTombYAML + walls))
	require.NoError(t, err)
	assert.Equal(t, "reference-tomb", compiled.Params.Key)

	hex := func(column, row int) core.Hex {
		return core.HexFromPosition(spatial.Position{X: float64(column), Y: float64(row)})
	}
	sevenThree, eightThree := hex(7, 3), hex(8, 3)
	sevenOne, eightOne := hex(7, 1), hex(8, 1)
	tenThree, elevenThree := hex(10, 3), hex(11, 3)
	assert.Equal(t, []encounter.AuthoredEdge{
		{
			From: sevenThree, To: eightThree, Kind: encounter.GeneratedEdgeKindDoor,
			DoorID: encounter.AuthoredDoorID(compiled.Params.Key, sevenThree, eightThree),
		},
		{From: sevenOne, To: eightOne, Kind: encounter.GeneratedEdgeKindSolid},
		{
			From: tenThree, To: elevenThree, Kind: encounter.GeneratedEdgeKindDoor,
			DoorID: encounter.AuthoredDoorID(compiled.Params.Key, tenThree, elevenThree),
		},
	}, compiled.Params.AuthoredEdges)
}

// TestValidate_WallsUsesPointyTopOddQAdjacency fixes the serializer's former
// even-q diagonal pairs in place. Both column parities must preserve odd-q's
// actual neighboring rows; axial and even-q interpretations are not accepted.
func TestValidate_WallsUsesPointyTopOddQAdjacency(t *testing.T) {
	cases := []struct {
		name    string
		from    [2]int
		to      [2]int
		wantErr string
	}{
		{name: "odd column horizontal adjacent", from: [2]int{7, 1}, to: [2]int{8, 1}},
		{name: "odd column diagonal adjacent", from: [2]int{7, 1}, to: [2]int{8, 2}},
		{name: "even column horizontal adjacent", from: [2]int{10, 3}, to: [2]int{11, 3}},
		{name: "even column diagonal adjacent", from: [2]int{10, 3}, to: [2]int{11, 2}},
		{
			name:    "original serializer pair [7,1]-[8,0] is not adjacent",
			from:    [2]int{7, 1},
			to:      [2]int{8, 0},
			wantErr: wallAdjacencyError,
		},
		{
			name:    "original serializer pair [7,3]-[8,2] is not adjacent",
			from:    [2]int{7, 3},
			to:      [2]int{8, 2},
			wantErr: wallAdjacencyError,
		},
		{
			name:    "even column even-q diagonal is not adjacent",
			from:    [2]int{10, 3},
			to:      [2]int{11, 4},
			wantErr: wallAdjacencyError,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := placedTombYAML + fmt.Sprintf(`
walls:
  - {from: [%d, %d], to: [%d, %d], kind: solid}
`, tc.from[0], tc.from[1], tc.to[0], tc.to[1])
			spec, err := dungeonspec.Decode([]byte(raw))
			require.NoError(t, err)

			err = dungeonspec.Validate(spec)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestLoad_WallsOmittedNullAndYAMLOrderAreInvariant keeps legacy specs byte- and
// behavior-compatible while making authored normalization independent of an
// editor's list ordering or drag direction.
func TestLoad_WallsOmittedNullAndYAMLOrderAreInvariant(t *testing.T) {
	omitted, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	nullWalls, err := dungeonspec.Load([]byte(placedTombYAML + "\nwalls: null\n"))
	require.NoError(t, err)
	assert.Equal(t, omitted, nullWalls)

	first, err := dungeonspec.Load([]byte(placedTombYAML + `
walls:
  - {from: [1, 1], to: [2, 1], kind: solid}
  - {from: [3, 2], to: [4, 2], kind: door}
`))
	require.NoError(t, err)
	second, err := dungeonspec.Load([]byte(placedTombYAML + `
walls:
  - {from: [4, 2], to: [3, 2], kind: door}
  - {from: [2, 1], to: [1, 1], kind: solid}
`))
	require.NoError(t, err)
	assert.Equal(t, first.Params.AuthoredEdges, second.Params.AuthoredEdges)

	build := func(compiled dungeonspec.CompiledDungeon) *encounter.Data {
		compiled.Params.RandomSeed = 51
		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
		enc := encounter.New(context.Background(), "omitted-null-walls", broker)
		require.NoError(t, enc.InitDungeon(compiled.Params))
		return enc.ToData()
	}
	omittedData := build(omitted)
	nullData := build(nullWalls)
	assert.Equal(t, omittedData.Space.Walls, nullData.Space.Walls)
	assert.Equal(t, omittedData.Space.AuthoredEdges, nullData.Space.AuthoredEdges)
	assert.Empty(t, omittedData.Space.AuthoredEdges)
	assert.Empty(t, omittedData.Space.DungeonKey, "old wall-free snapshots must not gain persistence fields")
	assert.Empty(t, nullData.Space.DungeonKey, "explicit walls: null must remain old-spec compatible")
}

// TestLoad_WallsErrorCarriesItsLaterEntryPath makes duplicate/conflict feedback
// actionable for the YAML editor instead of returning an unlocatable topology
// error.
func TestLoad_WallsErrorCarriesItsLaterEntryPath(t *testing.T) {
	_, err := dungeonspec.Load([]byte(placedTombYAML + strings.TrimSpace(`

walls:
  - {from: [1, 1], to: [2, 1], kind: solid}
  - {from: [2, 1], to: [1, 1], kind: door}
`) + "\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "walls[1]")
	assert.Contains(t, err.Error(), "conflicting")
}
