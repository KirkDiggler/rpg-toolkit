// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"context"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const wallFromPath = "walls[0].from"

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

// TestLoad_WallsCompilesNormalizedAbsoluteEdgesAndStableDoorID proves authored
// endpoint cells are converted through the one pointy-top coordinate path and
// that the compiler owns deterministic door identity.
func TestLoad_WallsCompilesNormalizedAbsoluteEdgesAndStableDoorID(t *testing.T) {
	const walls = `
walls:
  - {from: [2, 1], to: [1, 1], kind: solid}
  - {from: [4, 2], to: [3, 2], kind: door}
`
	compiled, err := dungeonspec.Load([]byte(placedTombYAML + walls))
	require.NoError(t, err)
	require.Len(t, compiled.Params.AuthoredEdges, 2)
	assert.Equal(t, "reference-tomb", compiled.Params.Key)

	solid := compiled.Params.AuthoredEdges[0]
	assert.Equal(t, encounter.GeneratedEdgeKindSolid, solid.Kind)
	assert.Equal(t, core.HexFromPosition(spatial.Position{X: 1, Y: 1}), solid.From)
	assert.Equal(t, core.HexFromPosition(spatial.Position{X: 2, Y: 1}), solid.To)
	assert.Empty(t, solid.DoorID)

	door := compiled.Params.AuthoredEdges[1]
	assert.Equal(t, encounter.GeneratedEdgeKindDoor, door.Kind)
	assert.Equal(t, core.HexFromPosition(spatial.Position{X: 3, Y: 2}), door.From)
	assert.Equal(t, core.HexFromPosition(spatial.Position{X: 4, Y: 2}), door.To)
	assert.Equal(t, encounter.AuthoredDoorID(compiled.Params.Key, door.From, door.To), door.DoorID)
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
