package encounter_test

// reload_door_blocking_test.go closes the first owed sweep named in
// rpg-toolkit#804 (riding PR #791's gate note, KirkDiggler/rpg-toolkit#791):
// PR #791's adversarial gate review proved InitRoom -> AddDoor -> ToData ->
// LoadFromData -> still-blocks empirically with a throwaway test. This
// commits that proof permanently.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// TestClosedDoor_StillBlocksAfterReload rebuilds an Encounter from its own
// ToData() snapshot (the LoadFromData path a host's Redis round-trip
// exercises on every call) and asserts a closed door still blocks
// movement and LoS on the reloaded instance — not just the original one
// that built it fresh via rebuildRoomFromData at AddDoor time.
func TestClosedDoor_StillBlocksAfterReload(t *testing.T) {
	transport := encounter.NewInMemoryTransport()
	defer func() { _ = transport.Close() }()
	broker := encounter.NewBroker(transport)
	defer func() { _ = broker.Close() }()

	enc := encounter.New(context.Background(), "enc-reload-door", broker)
	require.NoError(t, enc.InitRoom(20, 20, environments.PatternEmpty))
	require.NoError(t, enc.AddDoor("door-reload", lineHex(3), false))

	reloaded, err := encounter.LoadFromData(context.Background(), enc.ToData(), broker)
	require.NoError(t, err)

	require.True(t,
		reloaded.Room().IsLineOfSightBlocked(lineHex(0).ToPosition(), lineHex(6).ToPosition()),
		"a closed door persisted via ToData must still block LoS after LoadFromData",
	)
	require.False(t,
		reloaded.Room().CanPlaceEntity(probeEntity{}, lineHex(3).ToPosition()),
		"a closed door persisted via ToData must still block movement onto its own cell after LoadFromData",
	)
}

// TestReload_CoLocatedDoorAndWall_DedupsWithoutFailing closes the second
// owed sweep named in rpg-toolkit#804: rebuildRoomFromData's co-located
// door+wall dedup compares two SEPARATELY-DERIVED cube coordinates (one
// from Space.Walls, one from Doors[id].Position) — this pins that a door
// generation places directly on top of an existing wall cell doesn't fail
// the room rebuild, and the cell still blocks (from the pre-existing
// wall, since the door's own duplicate wall placement is skipped
// defensively — see rebuildRoomFromData's doc).
func TestReload_CoLocatedDoorAndWall_DedupsWithoutFailing(t *testing.T) {
	origin := core.Hex{Q: 0, R: 0, S: 0}

	wallCube := origin.ToCube()
	data := encounter.NewData("enc-colocated")
	data.Space = &encounter.SpaceData{
		Walls:  []environments.WallSegmentData{{Start: wallCube, End: wallCube, BlocksMovement: true, BlocksLoS: true}},
		Width:  10,
		Height: 10,
	}
	data.Doors["door-colocated"] = &encounter.DoorData{ID: "door-colocated", Position: origin, Open: false}

	transport := encounter.NewInMemoryTransport()
	defer func() { _ = transport.Close() }()
	broker := encounter.NewBroker(transport)
	defer func() { _ = broker.Close() }()

	enc, err := encounter.LoadFromData(context.Background(), data, broker)
	require.NoError(t, err, "a door co-located with an existing wall cell must not fail the room rebuild")

	require.False(t,
		enc.Room().CanPlaceEntity(probeEntity{}, origin.ToPosition()),
		"the cell must still block, from the pre-existing wall, even though the "+
			"door's own wall placement was skipped as a duplicate",
	)
}
