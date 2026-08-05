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
	canvas := encounter.NewCanvasFloorSource(4, 3)
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

	params := canvasParams()
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
	require.Len(t, data.Space.Canvas.Cells, 12)

	payload, err := json.Marshal(data)
	require.NoError(t, err)
	var persisted encounter.Data
	require.NoError(t, json.Unmarshal(payload, &persisted))
	persisted.Space.AuthoredEdges[0].From = core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	persisted.Space.AuthoredEdges[0].To = core.HexFromPosition(spatial.Position{X: 0, Y: -1})
	_, err = encounter.LoadFromData(context.Background(), &persisted, broker)
	require.ErrorContains(t, err, "persisted to endpoint is not a semantic floor cell")

	var malformed encounter.Data
	require.NoError(t, json.Unmarshal(payload, &malformed))
	malformed.Space.Canvas.Cells = nil
	_, err = encounter.LoadFromData(context.Background(), &malformed, broker)
	require.ErrorContains(t, err, "persisted canvas floor")
}

func TestCanvasPartyStartEnvelopeAvoidsNamedContentAcrossSeeds(t *testing.T) {
	anchor := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	params := encounter.DungeonParams{
		Height: 3, Canvas: encounter.NewCanvasFloorSource(4, 3),
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
