package dungeonspec_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const ringSource = `version: 1
key: ring-room
name: Ring Room
canvas: { width: 5, height: 5, floor_source: regions }
rooms: []
start: [1, 1]
regions:
  - id: ring
    cells: [[1,1], [1,2], [1,3], [2,1], [2,3], [3,1], [3,2], [3,3]]
`

func compileCandidate(
	t *testing.T,
	source string,
	mode dungeonspec.CompileMode,
	seats int,
) *dungeonspec.CompileDungeonOutput {
	t.Helper()
	out, err := dungeonspec.CompileDungeon(context.Background(), dungeonspec.CompileDungeonInput{
		Source: []byte(source), Mode: mode, PartyStartSeatCount: seats, PreviewSeed: 17,
	})
	require.NoError(t, err)
	return out
}

func TestWaveARegionRingProjectsCanonicalMaskAndCompleteEnvelope(t *testing.T) {
	out := compileCandidate(t, ringSource, dungeonspec.CompileModeDraft, 4)
	require.Empty(t, out.FieldErrors)
	require.NotNil(t, out.FloorPlan)
	require.Equal(t, dungeonspec.FloorSourceRegions, out.FloorPlan.FloorSource)
	require.Len(t, out.FloorPlan.FloorCells, 8)
	floor := make(map[dungeonspec.FloorPlanCell]struct{}, 8)
	for index, cell := range out.FloorPlan.FloorCells {
		floor[cell] = struct{}{}
		if index > 0 {
			require.True(t, cellLess(out.FloorPlan.FloorCells[index-1], cell))
		}
	}
	center := dungeonspec.FloorPlanCell{Column: 2, Row: 2}
	centerEdges := 0
	for index, edge := range out.FloorPlan.Edges {
		_, fromFloor := floor[edge.From]
		_, toFloor := floor[edge.To]
		require.NotEqual(t, fromFloor, toFloor, "edge %d must have exactly one floor owner: %+v", index, edge)
		require.Equal(t, dungeonspec.FloorPlanEdgeKindSolid, edge.Kind)
		if edge.From == center || edge.To == center {
			centerEdges++
		}
	}
	require.Equal(t, 6, centerEdges, "the one-cell interior void must have its complete six-side envelope")

	rim := `version: 1
key: rim
name: Rim
canvas: { width: 2, height: 2, floor_source: regions }
rooms: []
regions: [{ id: rim, cells: [[0,0]] }]
`
	rimPlan := compileCandidate(t, rim, dungeonspec.CompileModeDraft, 1).FloorPlan
	require.Len(t, rimPlan.Edges, 6)
	offCanvas := 0
	for _, edge := range rimPlan.Edges {
		for _, endpoint := range []dungeonspec.FloorPlanCell{edge.From, edge.To} {
			if endpoint.Column < 0 || endpoint.Row < 0 || endpoint.Column >= 2 || endpoint.Row >= 2 {
				offCanvas++
			}
		}
	}
	require.Positive(t, offCanvas, "rim envelope must preserve actual off-canvas neighbor coordinates")
}

func TestWaveATinyAndDisconnectedDraftLifecycle(t *testing.T) {
	tiny := `version: 1
key: tiny
name: Tiny
canvas: { width: 3, height: 2, floor_source: regions }
rooms: []
regions: [{ id: tiny, cells: [[0,0], [1,0]] }]
`
	draft := compileCandidate(t, tiny, dungeonspec.CompileModeDraft, 4)
	require.Empty(t, draft.FieldErrors)
	require.Nil(t, draft.FloorPlan.Entrance)
	strict := compileCandidate(t, tiny, dungeonspec.CompileModeStrict, 4)
	require.Equal(t, "canvas.floor_source", strict.FieldErrors[0].Field)
	require.Equal(t, "entrance_unavailable", strict.FieldErrors[0].Code)

	islands := `version: 1
key: islands
name: Islands
canvas: { width: 6, height: 3, floor_source: regions }
rooms: []
regions:
  - { id: large, cells: [[0,0], [0,1], [1,0], [1,1]] }
  - { id: island, cells: [[5,2]] }
`
	projected := compileCandidate(t, islands, dungeonspec.CompileModeDraft, 4)
	require.Empty(t, projected.FieldErrors)
	require.NotNil(t, projected.FloorPlan.Entrance)
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	preview := encounter.New(context.Background(), "draft-cannot-start", broker)
	require.Error(t, preview.InitDungeon(projected.Compiled.Params))
	rejected := compileCandidate(t, islands, dungeonspec.CompileModeStrict, 4)
	require.Equal(t, "canvas.floor_source", rejected.FieldErrors[0].Field)
	require.Equal(t, "floor_disconnected", rejected.FieldErrors[0].Code)
}

func TestWaveABoundsOmissionAndCompleteCandidateReplacement(t *testing.T) {
	omitted := `version: 1
key: bounds
name: Bounds
canvas: { width: 3, height: 2 }
rooms: []
`
	plan := compileCandidate(t, omitted, dungeonspec.CompileModeDraft, 1).FloorPlan
	require.Equal(t, dungeonspec.FloorSourceBounds, plan.FloorSource)
	require.Len(t, plan.FloorCells, 6)

	previous := `version: 1
key: replace
name: Replace
canvas: { width: 4, height: 2 }
rooms: []
place: [{ ref: "dnd5e:props:pillar", at: [3,0] }]
`
	require.Empty(t, compileCandidate(t, previous, dungeonspec.CompileModeDraft, 1).FieldErrors)
	shrunk := `version: 1
key: replace
name: Replace
canvas: { width: 2, height: 2 }
rooms: []
`
	replacement := compileCandidate(t, shrunk, dungeonspec.CompileModeDraft, 1)
	require.Empty(t, replacement.FieldErrors)
	require.Len(t, replacement.FloorPlan.FloorCells, 4)
}

func TestWaveARemovedFloorRejectsDependentContentAtExactPaths(t *testing.T) {
	base := `version: 1
key: removed-content
name: Removed Content
canvas: { width: 3, height: 3, floor_source: regions }
rooms: []
regions: [{ id: floor, cells: [[0,0], [0,1], [1,0], [1,1]] }]
%s
`
	cases := []struct{ name, content, field string }{
		{name: "start", content: "start: [2,2]", field: "start"},
		{name: "prop", content: `place: [{ ref: "dnd5e:props:pillar", at: [2,2] }]`, field: "place[0].at"},
		{name: "monster", content: `place: [{ ref: "dnd5e:monsters:skeleton", at: [2,2] }]`, field: "place[0].at"},
		{name: "wall", content: `walls: [{ from: [0,0], to: [2,2], kind: solid }]`, field: "walls[0].to"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := compileCandidate(t, fmt.Sprintf(base, tc.content), dungeonspec.CompileModeDraft, 1)
			require.Equal(t, tc.field, out.FieldErrors[0].Field)
			require.Equal(t, "outside_floor", out.FieldErrors[0].Code)
		})
	}
}

func TestWaveADuplicateEmptyRegionsHasExactPath(t *testing.T) {
	source := `version: 1
key: empty-duplicates
name: Empty Duplicates
canvas: { width: 2, height: 2, floor_source: regions }
rooms: []
regions:
  - { id: first, cells: [] }
  - { id: second, cells: [] }
`
	out := compileCandidate(t, source, dungeonspec.CompileModeDraft, 1)
	require.Equal(t, "regions[1].cells", out.FieldErrors[0].Field)
	require.Equal(t, "duplicate_region", out.FieldErrors[0].Code)
}

func TestWaveASnapshotReloadAndVoidMechanicsUsePersistedMask(t *testing.T) {
	compiled := compileCandidate(t, ringSource, dungeonspec.CompileModeStrict, 4)
	require.Empty(t, compiled.FieldErrors)
	ctx := context.Background()
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	enc := encounter.New(ctx, "wave-a-mask", broker)
	require.NoError(t, enc.InitDungeon(compiled.Compiled.Params))
	floorBefore := append([]core.Hex(nil), enc.ToData().Space.FloorCells...)
	edgesBefore := append([]encounter.GeneratedEdge(nil), enc.ToData().Space.EnvelopeEdges...)
	void := core.HexFromPosition(structuralPosition(2, 2))
	start := core.HexFromPosition(structuralPosition(1, 1))
	require.Error(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "void", EntityID: "void", Position: void, SightRange: 4,
	}))
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: "alice", Position: start, SightRange: 4,
	}))
	require.NotContains(t, enc.KnownHexes("alice"), void)
	acrossVoid := core.HexFromPosition(structuralPosition(3, 3))
	require.NoError(t, enc.AddMonster(encounter.MonsterInput{ID: "hidden", Position: acrossVoid, HP: 1, MaxHP: 1}))
	require.Equal(t, core.ModeFreeRoam, enc.Mode(), "target across void must not be reachable/visible")
	require.NoError(t, enc.Move("alice", []core.Hex{void}))
	require.Equal(t, start, enc.SnapshotFor("alice").Position)
	require.Error(t, enc.AddObstacle("void-prop", "dnd5e:props:pillar", void, true, true))
	require.NoError(t, enc.Move("alice", []core.Hex{acrossVoid}),
		"a sparse request must fail closed without surfacing an infrastructure error")
	require.Equal(t, start, enc.SnapshotFor("alice").Position)
	require.False(t, enc.Room().GetGrid().IsValidPosition(void.ToPosition()))
	require.True(t, enc.Room().IsLineOfSightBlocked(start.ToPosition(), acrossVoid.ToPosition()))

	snapshot := enc.ToData()
	edited := `version: 1
key: ring-room
name: Ring Room
canvas: { width: 2, height: 2, floor_source: regions }
rooms: []
regions: [{ id: new, cells: [[0,0], [0,1], [1,0], [1,1]] }]
`
	require.Empty(t, compileCandidate(t, edited, dungeonspec.CompileModeStrict, 4).FieldErrors)
	reloaded, err := encounter.LoadFromData(ctx, snapshot, broker)
	require.NoError(t, err)
	require.Equal(t, floorBefore, reloaded.ToData().Space.FloorCells)
	require.Equal(t, edgesBefore, reloaded.ToData().Space.EnvelopeEdges)
	require.False(t, reloaded.Room().GetGrid().IsValidPosition(void.ToPosition()))
}

func cellLess(a, b dungeonspec.FloorPlanCell) bool {
	return a.Column < b.Column || a.Column == b.Column && a.Row < b.Row
}
func structuralPosition(column, row int) spatial.Position {
	return spatial.Position{X: float64(column), Y: float64(row)}
}
