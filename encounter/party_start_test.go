package encounter_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

// partyStartHex converts an absolute authoring column/row into the encounter
// coordinate type accepted by DungeonParams.PartyStart.
func partyStartHex(column, row int) core.Hex {
	return core.HexFromPosition(spatial.Position{X: float64(column), Y: float64(row)})
}

// partyStartParams places an authored anchor in the second semantic room's
// door row. That proves an authored start is not constrained to the entrance
// archetype or ordinary ReservedCells' door-row rule.
func partyStartParams(seed int64) encounter.DungeonParams {
	anchor := partyStartHex(13, 4) // second room begins at absolute column 9
	return encounter.DungeonParams{
		Height:     8,
		RandomSeed: seed,
		PartyStart: encounter.PartyStartParams{Anchor: &anchor, SeatCount: 4},
		Regions: []encounter.DungeonRegionParams{
			{ID: "entry", Archetype: encounter.ArchetypeEntrance, Width: 8, Pattern: environments.PatternEmpty},
			{
				ID: "gallery", Archetype: encounter.ArchetypeChamber, Width: 10, Pattern: environments.PatternRandom,
				Obstacles: []encounter.ObstacleSpec{
					// Exercise the preferential border candidate path and the
					// normal/scattered candidate path in every seed.
					{Ref: "test:props:border", Count: 28, BlocksMovement: true, PreferBorder: true},
					{Ref: "test:props:scattered", Count: 25, BlocksMovement: true},
				},
			},
		},
		Connectors: []encounter.DungeonConnectorParams{{DoorID: "entry-gallery"}},
	}
}

// TestInitDungeon_PartyStartReservationSurvivesScatteredAndPreferBorderSeeds
// proves the anchor and all normal seats are selected before both random wall
// generation and both rolled-obstacle draw paths. The deliberately named seed
// sweep covers random/scattered walls plus PreferBorder and normal obstacles.
func TestInitDungeon_PartyStartReservationSurvivesScatteredAndPreferBorderSeeds(t *testing.T) {
	var want []core.Hex
	for seed := int64(1); seed <= 50; seed++ {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(partyStartParams(seed)), "seed %d", seed)

		positions, err := enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 4})
		require.NoError(t, err, "seed %d", seed)
		require.Len(t, positions.Positions, 4, "seed %d", seed)
		require.Equal(t, enc.ToData().Space.Entrance, positions.Positions[0], "seed %d", seed)
		require.Equal(t, partyStartHex(13, 4), positions.Positions[0], "seed %d", seed)

		if want == nil {
			want = append([]core.Hex(nil), positions.Positions...)
		} else {
			require.Equal(t, want, positions.Positions, "seed %d must not relocate or reorder party seats", seed)
		}

		seen := make(map[core.Hex]struct{}, len(positions.Positions))
		for slot, position := range positions.Positions {
			_, duplicate := seen[position]
			require.False(t, duplicate, "seed %d: duplicate party-start seat %d at %v", seed, slot, position)
			seen[position] = struct{}{}

			roomID, inRoom := enc.ToData().Space.RegionAt(position)
			require.True(t, inRoom, "seed %d seat %d must remain in a semantic room", seed, slot)
			require.Equal(t, "gallery", roomID, "seed %d seat %d must remain in the authored anchor room", seed, slot)
			require.True(t, enc.Room().CanPlaceEntity(probeEntity{}, position.ToPosition()),
				"seed %d seat %d at %v must remain usable, not a wall or rolled obstacle", seed, slot, position)
		}
	}
}

// TestResolvePartySpawnPositions_OrderedAndCapacityBounded owns the API seam:
// callers receive an immutable ordered prefix, and an oversized request returns
// a typed requested-versus-available error without inventing a fifth position.
func TestResolvePartySpawnPositions_OrderedAndCapacityBounded(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(partyStartParams(42)))

	all, err := enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 4})
	require.NoError(t, err)
	seats := append([]core.Hex(nil), all.Positions...)
	one, err := enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 1})
	require.NoError(t, err)
	require.Equal(t, all.Positions[:1], one.Positions)
	require.Equal(t, enc.ToData().Space.Entrance, all.Positions[0])

	// Output is a copy rather than an alias to persisted SpaceData.
	all.Positions[0] = core.Hex{}
	again, err := enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 1})
	require.NoError(t, err)
	require.Equal(t, enc.ToData().Space.Entrance, again.Positions[0])

	// The stored reservation survives the ordinary persisted-data reload path;
	// startup never needs to derive positions from spatial coordinates again.
	reloadTransport := encounter.NewInMemoryTransport()
	reloadBroker := encounter.NewBroker(reloadTransport)
	t.Cleanup(func() { _ = reloadBroker.Close(); _ = reloadTransport.Close() })
	reloaded, err := encounter.LoadFromData(context.Background(), enc.ToData(), reloadBroker)
	require.NoError(t, err)
	reloadedSeats, err := reloaded.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 4})
	require.NoError(t, err)
	require.Equal(t, seats, reloadedSeats.Positions)
	require.Equal(t, enc.ToData().Space.Entrance, reloaded.ToData().Space.Entrance)

	// A four-member caller maps ordered members straight onto the toolkit
	// seats; no position arithmetic or secondary placement policy is needed.
	for i, position := range seats {
		playerID := core.PlayerID(fmt.Sprintf("party-player-%d", i))
		require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
			PlayerID: playerID,
			EntityID: core.EntityID(fmt.Sprintf("party-character-%d", i)),
			Position: position, SightRange: 1,
		}))
		require.Equal(t, position, enc.ToData().Players[playerID].View.Position)
	}

	_, err = enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 5})
	require.Error(t, err)
	var capacityErr *encounter.PartySpawnCapacityError
	require.True(t, errors.As(err, &capacityErr))
	require.Equal(t, 5, capacityErr.Requested)
	require.Equal(t, 4, capacityErr.Available)
}

// TestInitDungeon_PartyStartReservationRejectsInsufficientCapacity proves a
// capacity failure stops generation rather than moving the authored anchor or
// silently reducing the envelope.
func TestInitDungeon_PartyStartReservationRejectsInsufficientCapacity(t *testing.T) {
	anchor := partyStartHex(0, 2)
	params := encounter.DungeonParams{
		Height: 4,
		PartyStart: encounter.PartyStartParams{
			Anchor:    &anchor,
			SeatCount: 17, // a 4x4 semantic room has exactly 16 cells
		},
		Regions: []encounter.DungeonRegionParams{
			{ID: "entry", Archetype: encounter.ArchetypeEntrance, Width: 4, Pattern: environments.PatternEmpty},
			{ID: "gallery", Archetype: encounter.ArchetypeChamber, Width: 4, Pattern: environments.PatternEmpty},
		},
		Connectors: []encounter.DungeonConnectorParams{{DoorID: "entry-gallery"}},
	}
	enc := newTestEncounter(t)
	err := enc.InitDungeon(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires 17 seats")
	require.Contains(t, err.Error(), "only 16")
	require.Nil(t, enc.ToData().Space, "capacity failure must not commit a relocated/fallback dungeon")
}

// TestInitDungeon_PartyStartReservationRejectsDirectAuthoredBlocker is the
// engine-side defense behind dungeonspec.Validate: direct toolkit callers get
// the same no-collision guarantee before a room is committed.
func TestInitDungeon_PartyStartReservationRejectsDirectAuthoredBlocker(t *testing.T) {
	anchor := partyStartHex(1, 1)
	params := partyStartParams(1)
	params.PartyStart = encounter.PartyStartParams{Anchor: &anchor, SeatCount: 4}
	params.Regions[0].PlacedObstacles = []encounter.PlacedObstacleSpec{{
		Ref: "test:props:blocking", At: encounter.LocalHex{Col: 1, Row: 1}, BlocksMovement: true,
	}}
	enc := newTestEncounter(t)
	err := enc.InitDungeon(params)
	require.Error(t, err)
	require.Contains(t, err.Error(), "collides with authored blocking")
	require.Nil(t, enc.ToData().Space)
}
