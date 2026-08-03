// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"context"
	"errors"
	"testing"

	toolkitcore "github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// partyStartProbe lets the seed sweep assert a resolved seat is truly
// placeable in the reconstructed spatial room, rather than merely absent from
// one serialized obstacle slice.
type partyStartProbe struct{}

const startDecodeError = "decode dungeon spec"

func (partyStartProbe) GetID() string                   { return "party-start-probe" }
func (partyStartProbe) GetType() toolkitcore.EntityType { return "party-start-probe" }

func TestDecode_StartOptionalNullAndStrictShape(t *testing.T) {
	cases := []struct {
		name      string
		yaml      string
		wantStart *[2]int
		wantErr   string
	}{
		{"omitted is nil", placedTombYAML, nil, ""},
		{"explicit null is nil", placedTombYAML + "\nstart: null\n", nil, ""},
		{"absolute pair decodes", placedTombYAML + "\nstart: [7, 4]\n", &[2]int{7, 4}, ""},
		{"one coordinate rejected", placedTombYAML + "\nstart: [7]\n", nil, startDecodeError},
		{"three coordinates rejected", placedTombYAML + "\nstart: [7, 4, 2]\n", nil, startDecodeError},
		{"scalar rejected", placedTombYAML + "\nstart: 7\n", nil, startDecodeError},
		{"noninteger coordinate rejected", placedTombYAML + "\nstart: [7, north]\n", nil, startDecodeError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := dungeonspec.Decode([]byte(tc.yaml))
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantStart, spec.Start)
		})
	}
}

func TestValidate_StartAbsoluteRoomDoorRowAndStaticConflicts(t *testing.T) {
	decodeValid := func(t *testing.T) *dungeonspec.DungeonSpec {
		t.Helper()
		spec, err := dungeonspec.Decode([]byte(validM1YAML))
		require.NoError(t, err)
		return spec
	}

	// validM1YAML's start columns are 0 (entrance), 11 (gallery), 20
	// (corridor), and 27 (tomb). Every semantic archetype accepts its own
	// absolute door-row start; this is deliberately unlike room-local place.
	for _, tc := range []struct {
		name string
		at   [2]int
	}{
		{"entrance", [2]int{0, 4}},
		{"chamber", [2]int{11, 4}},
		{"corridor", [2]int{20, 4}},
		{"boss", [2]int{27, 4}},
	} {
		t.Run("door-row accepted in "+tc.name, func(t *testing.T) {
			spec := decodeValid(t)
			spec.Start = &tc.at
			require.NoError(t, dungeonspec.Validate(spec))
		})
	}

	cases := []struct {
		name    string
		mutate  func(*dungeonspec.DungeonSpec)
		wantErr string
	}{
		{
			name: "connector door cell rejected",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				at := [2]int{10, 4} // entrance's right-hand connector/door column
				spec.Start = &at
			},
			wantErr: "connector gap/door cell",
		},
		{
			name: "out of footprint rejected",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				at := [2]int{39, 4} // total width is 39, so this is just outside
				spec.Start = &at
			},
			wantErr: "out of bounds",
		},
		{
			name: "outside floor row rejected",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				at := [2]int{0, 8}
				spec.Start = &at
			},
			wantErr: "out of bounds",
		},
		{
			name: "movement-blocking prop rejected",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				at := [2]int{33, 3} // tomb starts at 27; coffin is local [6,3]
				spec.Start = &at
			},
			wantErr: "movement-blocking prop",
		},
		{
			name: "placed monster rejected",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				at := [2]int{31, 2} // tomb-local placed skeleton [4,2]
				spec.Start = &at
			},
			wantErr: "placed monster",
		},
		{
			name: "pinned boss rejected",
			mutate: func(spec *dungeonspec.DungeonSpec) {
				at := [2]int{34, 5} // tomb-local boss.at [7,5]
				spec.Start = &at
			},
			wantErr: "pinned boss",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := decodeValid(t)
			tc.mutate(spec)
			err := dungeonspec.Validate(spec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}

	t.Run("nonblocking prop does not conflict", func(t *testing.T) {
		spec := decodeValid(t)
		falseValue := false
		// The coffin is tomb-local [6,3], absolute [33,3]. A start only
		// conflicts with a prop if it blocks movement.
		tomb(spec).Place[0].BlocksMovement = &falseValue
		at := [2]int{33, 3}
		spec.Start = &at
		require.NoError(t, dungeonspec.Validate(spec))
	})
}

func TestLoad_StartCompilesAbsoluteAnchorAndNormalFourSeatReservation(t *testing.T) {
	// The tomb begins at absolute column 7 in this two-room fixture. If start
	// were accidentally treated like a tomb-local coordinate, this would land
	// at column 14 instead of the asserted absolute column 7.
	compiled, err := dungeonspec.Load([]byte(placedTombYAML + "\nstart: [7, 4]\n"))
	require.NoError(t, err)
	require.NotNil(t, compiled.Params.PartyStart.Anchor)
	assert.Equal(t, core.HexFromPosition(spatial.Position{X: 7, Y: 4}), *compiled.Params.PartyStart.Anchor)
	assert.Equal(t, 4, compiled.Params.PartyStart.SeatCount)

	compiled.Params.RandomSeed = 44
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	enc := encounter.New(context.Background(), "absolute-start-compile", broker)
	require.NoError(t, enc.InitDungeon(compiled.Params))
	require.Equal(t, *compiled.Params.PartyStart.Anchor, enc.ToData().Space.Entrance)

	positions, err := enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 4})
	require.NoError(t, err)
	require.Len(t, positions.Positions, 4)
	require.Equal(t, enc.ToData().Space.Entrance, positions.Positions[0])
}

func TestLoadWithConfig_UsesHostPartyCapacity(t *testing.T) {
	compiled, err := dungeonspec.LoadWithConfig([]byte(placedTombYAML), dungeonspec.LoadConfig{
		PartyStartSeatCount: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, compiled.Params.PartyStart.SeatCount)

	_, err = dungeonspec.LoadWithConfig([]byte(placedTombYAML), dungeonspec.LoadConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "party start seat count must be at least 1")
}

func TestLoad_OmittedAndNullStartPreserveGeneratedBehavior(t *testing.T) {
	omitted, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	nullStart, err := dungeonspec.Load([]byte(placedTombYAML + "\nstart: null\n"))
	require.NoError(t, err)
	require.Equal(t, omitted, nullStart, "omitted and explicit null compile identically")

	build := func(compiled dungeonspec.CompiledDungeon) *encounter.Data {
		compiled.Params.RandomSeed = 73
		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
		enc := encounter.New(context.Background(), "omitted-null-start", broker)
		require.NoError(t, enc.InitDungeon(compiled.Params))
		return enc.ToData()
	}
	omittedData := build(omitted)
	nullData := build(nullStart)
	require.Equal(t, omittedData.Space.Entrance, nullData.Space.Entrance)
	require.Equal(t, omittedData.Space.PartyStartPositions, nullData.Space.PartyStartPositions)
	require.Equal(t, omittedData.Space.Walls, nullData.Space.Walls)
	require.Equal(t, omittedData.Space.Obstacles, nullData.Space.Obstacles)
	require.Equal(t, core.HexFromPosition(spatial.Position{X: 0, Y: 4}), omittedData.Space.Entrance)
	require.Equal(t, []core.Hex{
		core.HexFromPosition(spatial.Position{X: 0, Y: 4}),
		core.HexFromPosition(spatial.Position{X: 1, Y: 4}),
		core.HexFromPosition(spatial.Position{X: 2, Y: 4}),
		core.HexFromPosition(spatial.Position{X: 3, Y: 4}),
	}, omittedData.Space.PartyStartPositions, "generated start uses its existing clear entrance-to-door route")
}

// TestLoad_AuthoredStartReservationSurvivesScatteredPreferBorderSeeds runs the
// dungeonspec compiler's actual scattered -> random mapping plus both rolled
// obstacle paths over a named sweep. No seed may block, move, or reorder any
// normal four-seat reservation.
func TestLoad_AuthoredStartReservationSurvivesScatteredPreferBorderSeeds(t *testing.T) {
	const authoredStartSweepYAML = `
version: 1
key: authored-start-sweep
name: Authored Start Sweep
height: 8
start: [14, 4]
rooms:
  - id: entry
    archetype: entrance
    width: 8
    pattern: scattered
    obstacles:
      - { ref: "dnd5e:props:pillar", count: 12 }
  - id: gallery
    archetype: chamber
    width: 10
    pattern: scattered
    obstacles:
      - { ref: "dnd5e:props:bone-pile", count: 28, prefer_border: true }
      - { ref: "dnd5e:props:obelisk", count: 25 }
  - id: tomb
    archetype: boss
    width: 8
    boss: { ref: "dnd5e:monsters:skeleton-captain", at: [4, 5] }
connectors:
  - { from: entry, to: gallery }
  - { from: gallery, to: tomb }
`
	compiled, err := dungeonspec.Load([]byte(authoredStartSweepYAML))
	require.NoError(t, err)

	var want []core.Hex
	for seed := int64(1); seed <= 50; seed++ {
		params := compiled.Params
		params.RandomSeed = seed
		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		enc := encounter.New(context.Background(), "authored-start-seed-sweep", broker)
		require.NoError(t, enc.InitDungeon(params), "seed %d", seed)

		positions, resolveErr := enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 4})
		require.NoError(t, resolveErr, "seed %d", seed)
		if want == nil {
			want = append([]core.Hex(nil), positions.Positions...)
		} else {
			require.Equal(t, want, positions.Positions, "seed %d", seed)
		}
		require.Equal(t, core.HexFromPosition(spatial.Position{X: 14, Y: 4}), positions.Positions[0], "seed %d", seed)

		seen := make(map[core.Hex]struct{}, 4)
		for slot, position := range positions.Positions {
			_, duplicate := seen[position]
			require.False(t, duplicate, "seed %d seat %d", seed, slot)
			seen[position] = struct{}{}
			require.True(t, enc.Room().CanPlaceEntity(partyStartProbe{}, position.ToPosition()),
				"seed %d seat %d must remain usable", seed, slot)
		}
		_ = broker.Close()
		_ = transport.Close()
	}
}

func TestWorkbenchReport_AuthoredStartUsesPartyMarker(t *testing.T) {
	report, err := dungeonspec.WorkbenchReport([]byte(placedTombYAML+"\nstart: [7, 4]\n"), 42)
	require.NoError(t, err)
	assert.Contains(t, report, "@ party start")
	assert.Contains(t, report, "......D@...........")
}

func TestResolvePartySpawnPositions_TooLargeRequestCarriesCounts(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(placedTombYAML))
	require.NoError(t, err)
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	enc := encounter.New(context.Background(), "party-start-capacity", broker)
	require.NoError(t, enc.InitDungeon(compiled.Params))

	_, err = enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 5})
	require.Error(t, err)
	var capacityErr *encounter.PartySpawnCapacityError
	require.True(t, errors.As(err, &capacityErr))
	assert.Equal(t, 5, capacityErr.Requested)
	assert.Equal(t, 4, capacityErr.Available)
}
