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

func newCanvas(t *testing.T, width, height int) *encounter.CanvasParams {
	t.Helper()
	canvas, err := encounter.NewCanvasParams(width, height)
	require.NoError(t, err)
	return canvas
}

func canvasParams(t *testing.T) encounter.DungeonParams {
	canvas := newCanvas(t, 4, 3)
	from := core.HexFromPosition(spatial.Position{X: 1, Y: 0})
	to := core.HexFromPosition(spatial.Position{X: 1, Y: 1})
	return encounter.DungeonParams{
		Key: "canvas-edge-defense", Height: 3, Canvas: canvas,
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

	params := canvasParams(t)
	bad := params
	bad.AuthoredEdges = append([]encounter.AuthoredEdge(nil), params.AuthoredEdges...)
	bad.AuthoredEdges[0].From = core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	bad.AuthoredEdges[0].To = core.HexFromPosition(spatial.Position{X: 0, Y: -1})
	enc := encounter.New(context.Background(), "canvas-edge-defense", broker)
	err := enc.InitDungeon(bad)
	require.ErrorContains(t, err, "not a semantic floor cell")

	require.NoError(t, enc.InitDungeon(params))
	data := enc.ToData()
	require.NotNil(t, data.Space.Canvas)
	require.Empty(t, data.Space.Regions)
	require.Equal(t, 4, data.Space.Canvas.Width)
	require.Equal(t, 3, data.Space.Canvas.Height)

	payload, err := json.Marshal(data)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "\"cells\"")
	var persisted encounter.Data
	require.NoError(t, json.Unmarshal(payload, &persisted))
	persisted.Space.AuthoredEdges[0].From = core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	persisted.Space.AuthoredEdges[0].To = core.HexFromPosition(spatial.Position{X: 0, Y: -1})
	_, err = encounter.LoadFromData(context.Background(), &persisted, broker)
	require.ErrorContains(t, err, "persisted to endpoint is not a semantic floor cell")

	var malformed encounter.Data
	require.NoError(t, json.Unmarshal(payload, &malformed))
	malformed.Space.Canvas.Width = 0
	_, err = encounter.LoadFromData(context.Background(), &malformed, broker)
	require.ErrorContains(t, err, "persisted canvas data")
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

func TestCanvasMalformedParamsAndPersistedDataRejectOversizeDimensions(t *testing.T) {
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })

	oversize, err := encounter.NewCanvasParams(encounter.CanvasMaxStructuralCells+1, 1)
	require.Nil(t, oversize)
	require.ErrorContains(t, err, "maximum of")

	_, err = encounter.LoadFromData(context.Background(), &encounter.Data{
		Space: &encounter.SpaceData{
			Width:  encounter.CanvasMaxStructuralCells + 1,
			Height: 1,
			Canvas: &encounter.CanvasData{Width: encounter.CanvasMaxStructuralCells + 1, Height: 1},
		},
	}, broker)
	require.ErrorContains(t, err, "maximum of")
}

func TestCanvasPartyStartEnvelopeAvoidsNamedContentAcrossSeeds(t *testing.T) {
	anchor := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	params := encounter.DungeonParams{
		Height: 3, Canvas: newCanvas(t, 4, 3),
		PartyStart: encounter.PartyStartParams{Anchor: &anchor, SeatCount: 4},
		CanvasPlacedObstacles: []encounter.CanvasPlacedObstacleSpec{{
			ID:             "prop",
			Ref:            "dnd5e:props:altar",
			At:             core.HexFromPosition(spatial.Position{X: 1, Y: 0}),
			BlocksMovement: true,
			BlocksLoS:      true,
		}},
		CanvasReservedCells: []encounter.CanvasReservedCell{{
			At: core.HexFromPosition(spatial.Position{X: 0, Y: 1}), Name: "placed monster \"skeleton\"",
		}},
	}
	var want []core.Hex
	for seed := int64(1); seed <= 8; seed++ {
		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		params.RandomSeed = seed
		enc := encounter.New(context.Background(), "canvas-party", broker)
		require.NoError(t, enc.InitDungeon(params), "seed %d", seed)
		resolved, err := enc.ResolvePartySpawnPositions(encounter.ResolvePartySpawnPositionsInput{Count: 4})
		require.NoError(t, err)
		if want == nil {
			want = resolved.Positions
		} else {
			require.Equal(t, want, resolved.Positions)
		}
		for _, position := range resolved.Positions {
			require.NotEqual(t, params.CanvasPlacedObstacles[0].At, position)
			require.NotEqual(t, params.CanvasReservedCells[0].At, position)
		}
		_ = broker.Close()
		_ = transport.Close()
	}
}

func TestCanvasParamsDataRoundTripIsolatedAndDerivesFloor(t *testing.T) {
	params := canvasParams(t)
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	enc := encounter.New(context.Background(), "canvas-round-trip", broker)
	require.NoError(t, enc.InitDungeon(params))

	data := enc.ToData()
	require.NotNil(t, data.Space.Canvas)
	require.NotSame(t, params.Canvas, data.Space.Canvas)
	data.Space.Canvas.Width = 2
	require.Equal(t, 4, params.Canvas.Width, "Params must not alias persisted data")
	data.Space.Canvas.Width = 4

	payload, err := json.Marshal(data)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "\"cells\"")
	var restored encounter.Data
	require.NoError(t, json.Unmarshal(payload, &restored))
	reloaded, err := encounter.LoadFromData(context.Background(), &restored, broker)
	require.NoError(t, err)
	require.Equal(t, data.Space.Canvas, reloaded.ToData().Space.Canvas)
}
