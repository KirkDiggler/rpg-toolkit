package encounter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
)

// TestDescribeGeneratedEdges_ThreeRoomGeometry verifies the authoring seam
// against deterministic multi-room geometry rather than reconstructing it
// from DungeonParams. The initialized encounter is the canonical runtime
// truth: it contains exterior boundary edges, connector-facing solid edges,
// and the connector doors the authoring API needs to project.
func TestDescribeGeneratedEdges_ThreeRoomGeometry(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(dungeonSeed)))

	output, err := enc.DescribeGeneratedEdges(encounter.DescribeGeneratedEdgesInput{})
	require.NoError(t, err)
	require.NotEmpty(t, output.Edges)

	var exterior, interiorSolid, door *encounter.GeneratedEdge
	for i := range output.Edges {
		edge := &output.Edges[i]
		switch {
		case edge.Kind == encounter.GeneratedEdgeKindDoor && edge.DoorID == dungeonDoor0ID:
			door = edge
		case edge.Kind == encounter.GeneratedEdgeKindSolid && edge.From != edge.To && !inBounds(edge.To, enc.ToData().Space):
			exterior = edge
		case edge.Kind == encounter.GeneratedEdgeKindSolid && edge.From != edge.To && inBounds(edge.To, enc.ToData().Space):
			interiorSolid = edge
		}
	}

	require.NotNil(t, exterior, "generated geometry must include an exterior solid edge")
	require.NotNil(t, interiorSolid, "generated geometry must include an interior-facing connector solid edge")
	require.NotNil(t, door, "generated geometry must include the connector door")
	require.Equal(t, dungeonDoor0ID, door.DoorID)
	require.Empty(t, exterior.DoorID)
	require.Empty(t, interiorSolid.DoorID)
}

// TestDescribeGeneratedEdges_DeduplicatesReverseSolidsAndRejectsConflicts
// pins the seam's one-record-per-physical-edge rule. Legacy duplicated wall
// records may be reversed, so equivalent solid entries collapse; a door on
// that same physical edge is a conflict and is rejected rather than leaking
// competing authoring geometry to a caller.
func TestDescribeGeneratedEdges_DeduplicatesReverseSolidsAndRejectsConflicts(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(threeRegionDungeonParams(dungeonSeed)))

	data := enc.ToData()
	var original environments.WallSegmentData
	for _, wall := range data.Space.Walls {
		if wall.Start != wall.End {
			original = wall
			break
		}
	}
	require.NotEqual(t, original.Start, original.End, "fixture requires a physical boundary edge")

	data.Space.Walls = append(data.Space.Walls, environments.WallSegmentData{
		Start:          original.End,
		End:            original.Start,
		BlocksMovement: original.BlocksMovement,
		BlocksLoS:      original.BlocksLoS,
	})
	output, err := enc.DescribeGeneratedEdges(encounter.DescribeGeneratedEdgesInput{})
	require.NoError(t, err)

	matches := 0
	for _, edge := range output.Edges {
		if edge.Kind == encounter.GeneratedEdgeKindSolid && sameUndirectedEdge(edge.From, edge.To,
			core.HexFromCube(original.Start), core.HexFromCube(original.End)) {
			matches++
		}
	}
	require.Equal(t, 1, matches, "reversed equivalent solid walls must collapse to one physical edge")

	var door encounter.GeneratedEdge
	for _, edge := range output.Edges {
		if edge.Kind == encounter.GeneratedEdgeKindDoor {
			door = edge
			break
		}
	}
	require.NotEmpty(t, door.DoorID, "fixture requires a connector door")
	data.Space.Walls = append(data.Space.Walls, environments.WallSegmentData{
		Start:          door.From.ToCube(),
		End:            door.To.ToCube(),
		BlocksMovement: true,
		BlocksLoS:      true,
	})
	_, err = enc.DescribeGeneratedEdges(encounter.DescribeGeneratedEdgesInput{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "conflicting generated edges")
}

// inBounds asks the generated SpaceData's dimensions whether a hex lies in
// the combined dungeon grid. It deliberately does not reproduce generator
// layout arithmetic.
func inBounds(hex core.Hex, space *encounter.SpaceData) bool {
	position := hex.ToPosition()
	return int(position.X) >= 0 && int(position.X) < space.Width &&
		int(position.Y) >= 0 && int(position.Y) < space.Height
}

// sameUndirectedEdge compares two physical edges without imposing an endpoint
// ordering on the exported record itself.
func sameUndirectedEdge(aFrom, aTo, bFrom, bTo core.Hex) bool {
	return (aFrom == bFrom && aTo == bTo) || (aFrom == bTo && aTo == bFrom)
}
