package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

func canvasParams() encounter.DungeonParams {
	from := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	to := core.HexFromPosition(spatial.Position{X: 1, Y: 1})
	return encounter.DungeonParams{
		Key: "canvas-edge-defense", FloorSource: encounter.FloorSourceCanvas, Width: 4, Height: 3,
		PartyStart: encounter.PartyStartParams{SeatCount: 2},
		AuthoredEdges: []encounter.AuthoredEdge{{
			From: from, To: to, Kind: encounter.GeneratedEdgeKindDoor,
			DoorID: encounter.AuthoredDoorID("canvas-edge-defense", from, to),
		}},
	}
}

func TestCanvasAuthoredEdgeRuntimeAndPersistenceDefenses(t *testing.T) {
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	params := canvasParams()
	bad := params
	bad.AuthoredEdges = append([]encounter.AuthoredEdge(nil), params.AuthoredEdges...)
	bad.AuthoredEdges[0].From = core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	bad.AuthoredEdges[0].To = core.HexFromPosition(spatial.Position{X: 0, Y: -1})
	enc := encounter.New(context.Background(), "canvas-edge-defense", broker)
	require.ErrorContains(t, enc.InitDungeon(bad), "not a semantic floor cell")
	require.NoError(t, enc.InitDungeon(params))
	data := enc.ToData()
	require.Equal(t, encounter.FloorSourceCanvas, data.Space.FloorSource)
	require.Empty(t, data.Space.Regions)
	payload, err := json.Marshal(data)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"floor_source":"canvas"`)
	require.NotContains(t, string(payload), `"canvas":`)
	require.NotContains(t, string(payload), `"cells"`)
	var persisted encounter.Data
	require.NoError(t, json.Unmarshal(payload, &persisted))
	persisted.Space.AuthoredEdges[0].From = core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	persisted.Space.AuthoredEdges[0].To = core.HexFromPosition(spatial.Position{X: 0, Y: -1})
	_, err = encounter.LoadFromData(context.Background(), &persisted, broker)
	require.ErrorContains(t, err, "persisted to endpoint is not a semantic floor cell")
	persisted.Space.FloorSource = "unknown"
	_, err = encounter.LoadFromData(context.Background(), &persisted, broker)
	require.ErrorContains(t, err, "unknown floor source")
}

func TestCanvasDimensionsBoundedWithoutOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	for _, dimensions := range [][2]int{{maxInt, 2}, {maxInt, 1}, {encounter.CanvasMaxStructuralCells + 1, 1}} {
		_, err := encounter.ValidateCanvasDimensions(dimensions[0], dimensions[1])
		require.ErrorContains(t, err, "maximum of")
	}
	cellCount, err := encounter.ValidateCanvasDimensions(1024, 1024)
	require.NoError(t, err)
	require.Equal(t, encounter.CanvasMaxStructuralCells, cellCount)
}

func TestCanvasPersistedDataRejectsOversizeDimensions(t *testing.T) {
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	data := &encounter.Data{Space: &encounter.SpaceData{
		FloorSource: encounter.FloorSourceCanvas, Width: encounter.CanvasMaxStructuralCells + 1, Height: 1,
	}}
	_, err := encounter.LoadFromData(context.Background(), data, broker)
	require.ErrorContains(t, err, "maximum of")
}

func TestCanvasPartyStartEnvelopeAvoidsNamedContentAcrossSeeds(t *testing.T) {
	anchor := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	params := encounter.DungeonParams{
		FloorSource: encounter.FloorSourceCanvas, Width: 4, Height: 3,
		PartyStart: encounter.PartyStartParams{Anchor: &anchor, SeatCount: 4},
		AbsolutePlacedObstacles: []encounter.AbsolutePlacedObstacleSpec{{
			ID: "prop", Ref: "dnd5e:props:altar", At: core.HexFromPosition(spatial.Position{X: 1, Y: 0}),
			BlocksMovement: true, BlocksLoS: true,
		}},
		AbsoluteReservedCells: []encounter.AbsoluteReservedCell{{
			At: core.HexFromPosition(spatial.Position{X: 0, Y: 1}), Name: "placed monster \"skeleton\"",
		}},
	}
	var want []core.Hex
	for seed := int64(1); seed <= 8; seed++ {
		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		params.RandomSeed = seed
		enc := encounter.New(context.Background(), "canvas-party", broker)
		require.NoError(t, enc.InitDungeon(params))
		resolved, err := enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 4})
		require.NoError(t, err)
		if want == nil {
			want = resolved.Positions
		} else {
			require.Equal(t, want, resolved.Positions)
		}
		for _, position := range resolved.Positions {
			require.NotEqual(t, params.AbsolutePlacedObstacles[0].At, position)
			require.NotEqual(t, params.AbsoluteReservedCells[0].At, position)
		}
		_ = broker.Close()
		_ = transport.Close()
	}
}

func TestFloorSourceJSONRoundTripAndLegacyRoomChainReload(t *testing.T) {
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	enc := encounter.New(context.Background(), "canvas-round-trip", broker)
	require.NoError(t, enc.InitDungeon(canvasParams()))
	data := enc.ToData()
	payload, err := json.Marshal(data)
	require.NoError(t, err)
	var restored encounter.Data
	require.NoError(t, json.Unmarshal(payload, &restored))
	reloaded, err := encounter.LoadFromData(context.Background(), &restored, broker)
	require.NoError(t, err)
	require.Equal(t, encounter.FloorSourceCanvas, reloaded.ToData().Space.FloorSource)
	legacy := reloaded.ToData()
	legacy.Space.FloorSource = ""
	legacy.Space.AuthoredEdges = nil
	legacy.Doors = map[core.EntityID]*encounter.DoorData{}
	legacy.Space.Regions = []encounter.RegionData{{
		ID: "legacy", Hexes: core.NewHexSet(core.HexFromPosition(spatial.Position{})),
	}}
	legacy.Space.Width, legacy.Space.Height = 1, 1
	_, err = encounter.LoadFromData(context.Background(), legacy, broker)
	require.NoError(t, err, "omitted marker remains room-chain, never canvas")
}
