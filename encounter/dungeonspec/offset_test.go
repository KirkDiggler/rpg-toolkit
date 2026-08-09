// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const offsetCanvasYAML = `version: 1
key: offset-canvas
name: Offset Canvas
height: 1
canvas: { width: 4, height: 2 }
rooms: []
place:
  - { ref: "dnd5e:props:altar", at: [1, 0], facing: E, offset: [-0.25, 0, 1.5] }
  - { ref: "dnd5e:monsters:skeleton", at: [2, 0], offset: [0, 0, 0] }
`

func TestDecode_PlacementOffsetPreservesOptionalPresenceAndRoundTrip(t *testing.T) {
	withOffsets := strings.Replace(placedTombYAML,
		`boss: { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5] }`,
		`boss: { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5], offset: [-1.25, 0, 2.5] }`, 1)
	withOffsets = strings.Replace(withOffsets,
		`at: [6, 3], blocks_los: false }`,
		`at: [6, 3], blocks_los: false, offset: [0, 0, 0] }`, 1)

	decoded, err := dungeonspec.Decode([]byte(withOffsets))
	require.NoError(t, err)
	require.NotNil(t, decoded.Rooms[1].Boss.Offset)
	assert.Equal(t, dungeonspec.PlacementOffset{-1.25, 0, 2.5}, *decoded.Rooms[1].Boss.Offset)
	require.NotNil(t, decoded.Rooms[1].Place[0].Offset)
	assert.Equal(t, dungeonspec.PlacementOffset{0, 0, 0}, *decoded.Rooms[1].Place[0].Offset)
	assert.Nil(t, decoded.Rooms[1].Place[1].Offset, "omission must remain distinct from explicit zero")

	encoded, err := yaml.Marshal(decoded)
	require.NoError(t, err)
	roundTripped, err := dungeonspec.Decode(encoded)
	require.NoError(t, err)
	require.NotNil(t, roundTripped.Rooms[1].Boss.Offset)
	assert.Equal(t, *decoded.Rooms[1].Boss.Offset, *roundTripped.Rooms[1].Boss.Offset)
	require.NotNil(t, roundTripped.Rooms[1].Place[0].Offset)
	assert.Equal(t, *decoded.Rooms[1].Place[0].Offset, *roundTripped.Rooms[1].Place[0].Offset)
	assert.Nil(t, roundTripped.Rooms[1].Place[1].Offset)
}

//nolint:lll,goconst // malformed YAML specimens stay inline with their exact expected paths.
func TestDecode_PlacementOffsetRejectsMalformedShapeAtExactSourcePath(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		wantField string
	}{
		{name: "room place zero components", raw: strings.Replace(placedTombYAML, `at: [6, 3], blocks_los: false }`, `at: [6, 3], blocks_los: false, offset: [] }`, 1), wantField: "rooms[1].place[0].offset"},
		{name: "room place one component", raw: strings.Replace(placedTombYAML, `at: [6, 3], blocks_los: false }`, `at: [6, 3], blocks_los: false, offset: [1] }`, 1), wantField: "rooms[1].place[0].offset"},
		{name: "room place two components", raw: strings.Replace(placedTombYAML, `at: [6, 3], blocks_los: false }`, `at: [6, 3], blocks_los: false, offset: [1, 2] }`, 1), wantField: "rooms[1].place[0].offset"},
		{name: "room place four components", raw: strings.Replace(placedTombYAML, `at: [6, 3], blocks_los: false }`, `at: [6, 3], blocks_los: false, offset: [1, 2, 3, 4] }`, 1), wantField: "rooms[1].place[0].offset"},
		{name: "room place string component", raw: strings.Replace(placedTombYAML, `at: [6, 3], blocks_los: false }`, `at: [6, 3], blocks_los: false, offset: [1, nope, 3] }`, 1), wantField: "rooms[1].place[0].offset[1]"},
		{name: "boss null component", raw: strings.Replace(placedTombYAML, `at: [7, 5] }`, `at: [7, 5], offset: [1, null, 3] }`, 1), wantField: "rooms[1].boss.offset[1]"},
		{name: "canvas place null offset", raw: strings.Replace(offsetCanvasYAML, `offset: [-0.25, 0, 1.5]`, `offset: null`, 1), wantField: "place[0].offset"},
		{name: "canvas place NaN", raw: strings.Replace(offsetCanvasYAML, `offset: [-0.25, 0, 1.5]`, `offset: [.nan, 0, 1.5]`, 1), wantField: "place[0].offset[0]"},
		{name: "canvas place positive infinity", raw: strings.Replace(offsetCanvasYAML, `offset: [-0.25, 0, 1.5]`, `offset: [0, .inf, 1.5]`, 1), wantField: "place[0].offset[1]"},
		{name: "canvas place negative infinity", raw: strings.Replace(offsetCanvasYAML, `offset: [-0.25, 0, 1.5]`, `offset: [0, 1, -.inf]`, 1), wantField: "place[0].offset[2]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dungeonspec.Decode([]byte(tc.raw))
			require.Error(t, err)
			var validationErr *dungeonspec.ValidationError
			require.True(t, errors.As(err, &validationErr), "error must expose a structured field: %v", err)
			assert.Equal(t, tc.wantField, validationErr.Field)
		})
	}
}

func TestValidate_PlacementOffsetRejectsProgrammaticNonFiniteValueAtExactPath(t *testing.T) {
	spec, err := dungeonspec.Decode([]byte(placedTombYAML))
	require.NoError(t, err)
	spec.Rooms[1].Boss.Offset = &dungeonspec.PlacementOffset{0, math.Inf(1), 0}

	err = dungeonspec.Validate(spec)
	require.Error(t, err)
	var validationErr *dungeonspec.ValidationError
	require.True(t, errors.As(err, &validationErr))
	assert.Equal(t, "rooms[1].boss.offset[1]", validationErr.Field)
}

//nolint:lll // complete contract assertions are clearest beside their placement paths.
func TestLoad_PlacementOffsetCompilesEveryPlacementKindWithoutInterpretation(t *testing.T) {
	roomSource := strings.Replace(placedTombYAML,
		`boss: { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5] }`,
		`boss: { ref: "dnd5e:monsters:skeleton-captain", at: [7, 5], offset: [-1.25, 0, 2.5] }`, 1)
	roomSource = strings.Replace(roomSource,
		`at: [6, 3], blocks_los: false }`,
		`at: [6, 3], blocks_los: false, facing: E, offset: [0, 0, 0] }`, 1)
	roomSource = strings.Replace(roomSource,
		`at: [4, 2] }`,
		`at: [4, 2], offset: [0.75, -0.5, 0.25] }`, 1)

	compiled, err := dungeonspec.Load([]byte(roomSource))
	require.NoError(t, err)
	placements := placementsByPath(compiled.Placements)
	assertCompiledOffset(t, placements["rooms[1].place[0]"], dungeonspec.FloorPlanCell{Column: 13, Row: 3}, &dungeonspec.PlacementOffset{0, 0, 0})
	assert.True(t, placements["rooms[1].place[0]"].BlocksMovement)
	assert.False(t, placements["rooms[1].place[0]"].BlocksLoS, "explicit false prop blocker survives")
	assertCompiledOffset(t, placements["rooms[1].place[5]"], dungeonspec.FloorPlanCell{Column: 11, Row: 2}, &dungeonspec.PlacementOffset{0.75, -0.5, 0.25})
	assert.False(t, placements["rooms[1].place[5]"].BlocksMovement, "monster blockers are not applicable")
	assert.False(t, placements["rooms[1].place[5]"].BlocksLoS, "monster blockers are not applicable")
	assertCompiledOffset(t, placements["rooms[1].boss"], dungeonspec.FloorPlanCell{Column: 14, Row: 5}, &dungeonspec.PlacementOffset{-1.25, 0, 2.5})
	assert.False(t, placements["rooms[1].boss"].BlocksMovement, "boss blockers are not applicable")
	assert.False(t, placements["rooms[1].boss"].BlocksLoS, "boss blockers are not applicable")
	assert.Nil(t, placements["rooms[1].place[1]"].Offset)
	require.Equal(t, &dungeonspec.PlacementOffset{0, 0, 0}, compiled.Params.Regions[1].PlacedObstacles[0].Offset)
	require.Equal(t, &dungeonspec.PlacementOffset{-1.25, 0, 2.5}, compiled.Spawns[0].Offset, "boss runtime carrier")
	require.Equal(t, &dungeonspec.PlacementOffset{0.75, -0.5, 0.25}, compiled.Spawns[1].Offset, "room monster runtime carrier")
	assert.True(t, placements["rooms[1].place[1]"].BlocksMovement, "omitted prop blocker defaults true")
	assert.True(t, placements["rooms[1].place[1]"].BlocksLoS, "omitted prop blocker defaults true")

	plan, err := dungeonspec.BuildFloorPlan(context.Background(), dungeonspec.BuildFloorPlanInput{Compiled: compiled, Seed: 42})
	require.NoError(t, err)
	planPlacements := placementsByPath(plan.Placements)
	assert.Equal(t, placements, planPlacements)

	canvasCompiled, err := dungeonspec.Load([]byte(offsetCanvasYAML))
	require.NoError(t, err)
	canvasPlacements := placementsByPath(canvasCompiled.Placements)
	assertCompiledOffset(t, canvasPlacements["place[0]"], dungeonspec.FloorPlanCell{Column: 1, Row: 0}, &dungeonspec.PlacementOffset{-0.25, 0, 1.5})
	assert.True(t, canvasPlacements["place[0]"].BlocksMovement)
	assert.True(t, canvasPlacements["place[0]"].BlocksLoS)
	assertCompiledOffset(t, canvasPlacements["place[1]"], dungeonspec.FloorPlanCell{Column: 2, Row: 0}, &dungeonspec.PlacementOffset{0, 0, 0})
	assert.False(t, canvasPlacements["place[1]"].BlocksMovement)
	assert.False(t, canvasPlacements["place[1]"].BlocksLoS)
	require.Equal(t, &dungeonspec.PlacementOffset{-0.25, 0, 1.5}, canvasCompiled.Params.AbsolutePlacedObstacles[0].Offset)
	require.Equal(t, &dungeonspec.PlacementOffset{0, 0, 0}, canvasCompiled.Spawns[0].Offset)
}

const offsetCaseOmitted = "omitted"

//nolint:lll // complete table rows keep the optional triple next to its expected pointer.
func TestLoad_CanvasMonsterOffsetPresenceMatrix(t *testing.T) {
	const zeroField = ", offset: [0, 0, 0]"
	cases := []struct {
		name        string
		replacement string
		want        *dungeonspec.PlacementOffset
	}{
		{name: offsetCaseOmitted},
		{name: "explicit zero", replacement: zeroField, want: &dungeonspec.PlacementOffset{0, 0, 0}},
		{name: "signed", replacement: ", offset: [1.5, -0.25, -3]", want: &dungeonspec.PlacementOffset{1.5, -0.25, -3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := strings.Replace(offsetCanvasYAML, zeroField, tc.replacement, 1)
			compiled, err := dungeonspec.Load([]byte(raw))
			require.NoError(t, err)
			require.Len(t, compiled.Spawns, 1)
			assert.Equal(t, tc.want, compiled.Spawns[0].Offset, "AbsoluteAt runtime carrier")
			placements := placementsByPath(compiled.Placements)
			assert.Equal(t, tc.want, placements["place[1]"].Offset, "authoring projection")
		})
	}
}

//nolint:lll // messages name the full mechanics boundary under comparison.
func TestLoad_PlacementOffsetDoesNotChangeMechanicsInputs(t *testing.T) {
	withOffset := strings.Replace(placedTombYAML,
		`at: [6, 3], blocks_los: false }`,
		`at: [6, 3], blocks_los: false, offset: [-99.5, 42.25, 0] }`, 1)
	withOffset = strings.Replace(withOffset,
		`at: [4, 2] }`,
		`at: [4, 2], offset: [9, -8, 7] }`, 1)

	baseline, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	candidate, err := dungeonspec.Load([]byte(withOffset))
	require.NoError(t, err)
	clearRuntimeOffsets(&baseline)
	clearRuntimeOffsets(&candidate)
	assert.Equal(t, baseline.Params, candidate.Params, "after removing the inert carrier, floor/blocker/collision/pathing/LoS inputs are identical")
	assert.Equal(t, baseline.Spawns, candidate.Spawns, "after removing the inert carrier, monster/boss spawn and targeting inputs are identical")
}

func clearRuntimeOffsets(compiled *dungeonspec.CompiledDungeon) {
	for regionIndex := range compiled.Params.Regions {
		for placementIndex := range compiled.Params.Regions[regionIndex].PlacedObstacles {
			compiled.Params.Regions[regionIndex].PlacedObstacles[placementIndex].Offset = nil
		}
	}
	for placementIndex := range compiled.Params.AbsolutePlacedObstacles {
		compiled.Params.AbsolutePlacedObstacles[placementIndex].Offset = nil
	}
	for spawnIndex := range compiled.Spawns {
		compiled.Spawns[spawnIndex].Offset = nil
	}
	for placementIndex := range compiled.Placements {
		compiled.Placements[placementIndex].Offset = nil
	}
}

func placementsByPath(placements []dungeonspec.CompiledPlacement) map[string]dungeonspec.CompiledPlacement {
	out := make(map[string]dungeonspec.CompiledPlacement, len(placements))
	for _, placement := range placements {
		out[placement.SourcePath] = placement
	}
	return out
}

//nolint:lll // one-line helper signature keeps every contract component visible.
func assertCompiledOffset(t *testing.T, placement dungeonspec.CompiledPlacement, at dungeonspec.FloorPlanCell, offset *dungeonspec.PlacementOffset) {
	t.Helper()
	assert.Equal(t, at, placement.At)
	assert.Equal(t, offset, placement.Offset)
}
