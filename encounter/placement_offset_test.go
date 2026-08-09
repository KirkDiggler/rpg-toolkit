// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func TestPlacementOffsetRuntimeCarrierPreservesPresenceMovementMemoryAndEvents(t *testing.T) {
	zero := core.PlacementOffset{0, 0, 0}
	nonzero := core.PlacementOffset{-1.25, 0.5, 2}
	origin := core.Hex{Q: 0, R: 0, S: 0}
	destination := core.Hex{Q: 1, R: -1, S: 0}

	e := &Encounter{data: NewData("placement-offset-runtime")}
	e.data.Space = &SpaceData{Obstacles: []ObstacleData{
		{ID: "prop-zero", Ref: "dnd5e:props:altar", Position: origin, Offset: &zero},
		{ID: "prop-omitted", Ref: "dnd5e:props:coffin", Position: origin},
	}}
	e.data.Monsters["monster-offset"] = &MonsterData{
		ID: "monster-offset", Position: origin, MonsterRef: "dnd5e:monsters:skeleton", Offset: &nonzero,
	}

	atOrigin := placementsByEntityID(e.placementsAt(origin))
	require.Equal(t, &zero, atOrigin["prop-zero"].Offset)
	require.Nil(t, atOrigin["prop-omitted"].Offset)
	require.Equal(t, &nonzero, atOrigin["monster-offset"].Offset)

	// A remembered observation freezes the authorized placement value. Moving
	// the runtime monster changes only its canonical origin; it neither rotates
	// nor removes the offset, and the vacated current record is total/empty for
	// that entity while memory remains the last observation.
	memory := perception.NewMemory()
	memory.Observe(perception.HexObservation{
		Position: origin, State: perception.KnowledgeStateVisible, Contents: e.placementsAt(origin),
	})
	e.data.Monsters["monster-offset"].Position = destination
	require.NotContains(t, placementsByEntityID(e.placementsAt(origin)), core.EntityID("monster-offset"))
	require.Equal(t, &nonzero, placementsByEntityID(e.placementsAt(destination))["monster-offset"].Offset)
	require.Equal(t, &nonzero, placementsByEntityID(memory[origin].Contents)["monster-offset"].Offset)

	eventHexes := knownHexesToEvents(map[core.Hex]perception.HexObservation{origin: memory[origin]})
	require.Len(t, eventHexes, 1)
	eventPlacements := make(map[core.EntityID]*core.PlacementOffset)
	for _, placement := range eventHexes[0].Contents {
		eventPlacements[placement.EntityID] = placement.Offset
	}
	require.Equal(t, &zero, eventPlacements["prop-zero"])
	require.Nil(t, eventPlacements["prop-omitted"])
	require.Equal(t, &nonzero, eventPlacements["monster-offset"])

	payload, err := json.Marshal(e.data)
	require.NoError(t, err)
	var persisted Data
	require.NoError(t, json.Unmarshal(payload, &persisted))
	require.Equal(t, &zero, persisted.Space.Obstacles[0].Offset)
	require.Nil(t, persisted.Space.Obstacles[1].Offset)
	require.Equal(t, &nonzero, persisted.Monsters["monster-offset"].Offset)
}

func TestPlacementOffsetRealInitSeedReloadKnowledgeAndMovementPath(t *testing.T) {
	zero := core.PlacementOffset{0, 0, 0}
	signed := core.PlacementOffset{-0.75, 1.25, -2.5}
	params := smDungeonParams()
	params.Regions[0].PlacedObstacles = []PlacedObstacleSpec{
		{Ref: "dnd5e:props:altar", At: LocalHex{Col: 1, Row: 1}, BlocksMovement: true, BlocksLoS: true, Offset: &zero},
		{Ref: "dnd5e:props:coffin", At: LocalHex{Col: 2, Row: 1}, BlocksMovement: false, BlocksLoS: false},
	}

	enc := smNewEncounter(t)
	require.NoError(t, enc.InitDungeon(params))
	bossAt := LocalHex{Col: 7, Row: 5}
	monsterAt := LocalHex{Col: 3, Row: 1}
	require.NoError(t, enc.SeedMonsters([]SpawnInstruction{
		{RoomID: smRoomIDBoss, MonsterRef: smRefSkeletonCaptain, Count: 1, At: &bossAt, Offset: &signed},
		{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &monsterAt, Offset: &zero},
	}))

	// Real immutable snapshot path: ToData -> JSON -> LoadFromData. Omitted and
	// explicit zero remain distinct on props; signed boss and zero monster
	// survive their toolkit-minted runtime identities.
	payload, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var persisted Data
	require.NoError(t, json.Unmarshal(payload, &persisted))
	reloaded, err := LoadFromData(context.Background(), &persisted, enc.broker)
	require.NoError(t, err)
	require.Equal(t, &zero, reloaded.data.Space.Obstacles[0].Offset)
	require.Nil(t, reloaded.data.Space.Obstacles[1].Offset)
	require.Equal(t, &signed, reloaded.data.Monsters["monster-boss-0"].Offset)
	require.Equal(t, &zero, reloaded.data.Monsters["monster-entrance-0"].Offset)

	viewerPosition := core.HexFromPosition(structuralPosition(0, 0))
	require.NoError(t, reloaded.AddPlayer(PlayerInput{
		PlayerID: "offset-viewer", EntityID: "offset-viewer-entity", Position: viewerPosition, SightRange: 10,
	}))
	known := reloaded.KnownHexes("offset-viewer")
	visiblePlacements := allKnownPlacements(known)
	require.Equal(t, &zero, visiblePlacements["obstacle-entrance-0"].Offset)
	require.Nil(t, visiblePlacements["obstacle-entrance-1"].Offset)
	require.Equal(t, &zero, visiblePlacements["monster-entrance-0"].Offset)
	require.NotContains(t, visiblePlacements, core.EntityID("monster-boss-0"),
		"closed connector keeps boss placement and offset fog-authorized")

	// KnownHexPlacement is the event mirror consumed by API. It must carry
	// exactly the authorized placement values and never surface the hidden boss.
	eventKnown := knownHexesToEvents(known)
	eventOffsets := make(map[core.EntityID]*core.PlacementOffset)
	for _, hex := range eventKnown {
		for _, placement := range hex.Contents {
			eventOffsets[placement.EntityID] = placement.Offset
		}
	}
	require.Equal(t, &zero, eventOffsets["obstacle-entrance-0"])
	require.Nil(t, eventOffsets["obstacle-entrance-1"])
	require.Equal(t, &zero, eventOffsets["monster-entrance-0"])
	require.NotContains(t, eventOffsets, core.EntityID("monster-boss-0"))

	monster := reloaded.data.Monsters["monster-entrance-0"]
	origin := monster.Position
	destination := core.HexFromPosition(structuralPosition(4, 1))
	view := reloaded.data.Players["offset-viewer"].View
	view.SightRange = 0
	require.NoError(t, reloaded.MoveNPCSteps("monster-entrance-0", []core.Hex{destination}))
	require.Equal(t, &zero, monster.Offset, "the public movement lifecycle mutates Position only")
	remembered := reloaded.KnownHexes("offset-viewer")[origin]
	require.Equal(t, perception.KnowledgeStateRemembered, remembered.State)
	require.Equal(t, &zero, placementsByEntityID(remembered.Contents)["monster-entrance-0"].Offset,
		"a hidden move cannot rewrite the viewer's frozen observation")

	// Resight refreshes both total records: the vacated origin no longer has
	// the monster, while the destination carries the same runtime-owned offset.
	view.SightRange = 10
	require.NoError(t, reloaded.refreshObservations(view, core.NewHexSet(origin, destination)))
	resighted := reloaded.KnownHexes("offset-viewer")
	require.NotContains(t, placementsByEntityID(resighted[origin].Contents), core.EntityID("monster-entrance-0"))
	require.Equal(t, &zero, placementsByEntityID(resighted[destination].Contents)["monster-entrance-0"].Offset)

	// The pass-through appearance overlay is the only helper that may create
	// a placement not already in a hex observation. It must consult the
	// runtime identity rather than appending a bare entity id.
	overlay := reloaded.eventObservationsForAppearance(
		origin, map[core.PlayerID]struct{}{core.PlayerID("offset-viewer"): {}}, "monster-entrance-0",
	)
	require.Equal(t, &zero, eventPlacementsByEntityID(overlay["offset-viewer"].Contents)["monster-entrance-0"].Offset)

	// Event JSON is the broker/replay boundary; lowercase optional offset
	// preserves explicit zero while an omitted offset remains nil.
	eventPayload, err := json.Marshal([]events.KnownHexPlacement{
		{EntityID: "zero", Offset: &zero}, {EntityID: "omitted"},
	})
	require.NoError(t, err)
	var eventRoundTrip []events.KnownHexPlacement
	require.NoError(t, json.Unmarshal(eventPayload, &eventRoundTrip))
	require.Equal(t, &zero, eventRoundTrip[0].Offset)
	require.Nil(t, eventRoundTrip[1].Offset)
}

func TestPlacementOffsetRealLifecycleMatrix(t *testing.T) {
	zero := core.PlacementOffset{0, 0, 0}
	signed := core.PlacementOffset{-1.75, 0.25, 3.5}
	cases := []struct {
		name   string
		offset *core.PlacementOffset
	}{
		{name: "omitted"},
		{name: "explicit zero", offset: &zero},
		{name: "signed", offset: &signed},
	}

	for _, tc := range cases {
		t.Run("room chain "+tc.name, func(t *testing.T) {
			params := smDungeonParams()
			params.Regions[0].PlacedObstacles = []PlacedObstacleSpec{{
				Ref: "dnd5e:props:altar", At: LocalHex{Col: 1, Row: 1}, Offset: tc.offset,
			}}
			enc := smNewEncounter(t)
			require.NoError(t, enc.InitDungeon(params))
			bossAt := LocalHex{Col: 7, Row: 5}
			monsterAt := LocalHex{Col: 3, Row: 1}
			require.NoError(t, enc.SeedMonsters([]SpawnInstruction{
				{RoomID: smRoomIDBoss, MonsterRef: smRefSkeletonCaptain, Count: 1, At: &bossAt, Offset: tc.offset},
				{RoomID: smRoomIDEntrance, MonsterRef: smRefSkeleton, Count: 1, At: &monsterAt, Offset: tc.offset},
			}))

			reloaded := reloadPlacementOffsetEncounter(t, enc)
			require.Equal(t, tc.offset, reloaded.data.Space.Obstacles[0].Offset, "room prop snapshot")
			require.Equal(t, tc.offset, reloaded.data.Monsters["monster-entrance-0"].Offset, "room monster snapshot")
			require.Equal(t, tc.offset, reloaded.data.Monsters["monster-boss-0"].Offset, "boss snapshot")

			require.NoError(t, reloaded.AddPlayer(PlayerInput{
				PlayerID: "entrance-viewer", EntityID: "entrance-viewer-entity",
				Position: core.HexFromPosition(structuralPosition(0, 1)), SightRange: 10,
			}))
			require.NoError(t, reloaded.AddPlayer(PlayerInput{
				PlayerID: "boss-viewer", EntityID: "boss-viewer-entity",
				Position: core.HexFromPosition(structuralPosition(smEntranceWidth+2, 1)), SightRange: 10,
			}))
			known := mergeKnownPlacements(
				reloaded.KnownHexes("entrance-viewer"), reloaded.KnownHexes("boss-viewer"),
			)
			require.Equal(t, tc.offset, known["obstacle-entrance-0"].Offset, "room prop perception")
			require.Equal(t, tc.offset, known["monster-entrance-0"].Offset, "room monster perception")
			require.Equal(t, tc.offset, known["monster-boss-0"].Offset, "boss perception")

			roomEvents := append(
				knownHexesToEvents(reloaded.KnownHexes("entrance-viewer")),
				knownHexesToEvents(reloaded.KnownHexes("boss-viewer"))...,
			)
			eventOffsets := mergeKnownEventOffsets(roundTripKnownHexEvents(t, roomEvents))
			require.Equal(t, tc.offset, eventOffsets["obstacle-entrance-0"], "room prop event")
			require.Equal(t, tc.offset, eventOffsets["monster-entrance-0"], "room monster event")
			require.Equal(t, tc.offset, eventOffsets["monster-boss-0"], "boss event")

			memoryData := roundTripPlacementOffsetData(t, reloaded.ToData())
			memoryPlacements := mergeKnownPlacements(
				memoryData.Players["entrance-viewer"].View.Memory,
				memoryData.Players["boss-viewer"].View.Memory,
			)
			require.Equal(t, tc.offset, memoryPlacements["obstacle-entrance-0"].Offset, "room prop memory JSON")
			require.Equal(t, tc.offset, memoryPlacements["monster-entrance-0"].Offset, "room monster memory JSON")
			require.Equal(t, tc.offset, memoryPlacements["monster-boss-0"].Offset, "boss memory JSON")
		})

		t.Run("canvas "+tc.name, func(t *testing.T) {
			propAt := core.HexFromPosition(structuralPosition(1, 1))
			monsterAt := core.HexFromPosition(structuralPosition(3, 1))
			params := DungeonParams{
				FloorSource: FloorSourceCanvas, Key: "canvas-offset-lifecycle", Width: 6, Height: 4,
				RandomSeed: 1, PartyStart: PartyStartParams{SeatCount: 1},
				AbsolutePlacedObstacles: []AbsolutePlacedObstacleSpec{{
					ID: "canvas-prop", Ref: "dnd5e:props:altar", At: propAt, Offset: tc.offset,
				}},
				AbsoluteReservedCells: []AbsoluteReservedCell{{At: monsterAt, Name: "canvas monster"}},
			}
			enc := smNewEncounter(t)
			require.NoError(t, enc.InitDungeon(params))
			require.NoError(t, enc.SeedMonsters([]SpawnInstruction{{
				MonsterRef: smRefSkeleton, Count: 1, AbsoluteAt: &monsterAt, Offset: tc.offset,
			}}))

			reloaded := reloadPlacementOffsetEncounter(t, enc)
			require.Equal(t, tc.offset, reloaded.data.Space.Obstacles[0].Offset, "canvas prop snapshot")
			require.Equal(t, tc.offset, reloaded.data.Monsters["monster-canvas-0"].Offset, "canvas monster snapshot")

			require.NoError(t, reloaded.AddPlayer(PlayerInput{
				PlayerID: "canvas-viewer", EntityID: "canvas-viewer-entity",
				Position: core.HexFromPosition(structuralPosition(0, 1)), SightRange: 10,
			}))
			known := allKnownPlacements(reloaded.KnownHexes("canvas-viewer"))
			require.Equal(t, tc.offset, known["canvas-prop"].Offset, "canvas prop perception")
			require.Equal(t, tc.offset, known["monster-canvas-0"].Offset, "canvas monster perception")

			canvasEvents := roundTripKnownHexEvents(t, knownHexesToEvents(reloaded.KnownHexes("canvas-viewer")))
			eventOffsets := mergeKnownEventOffsets(canvasEvents)
			require.Equal(t, tc.offset, eventOffsets["canvas-prop"], "canvas prop event")
			require.Equal(t, tc.offset, eventOffsets["monster-canvas-0"], "canvas monster event")

			memoryData := roundTripPlacementOffsetData(t, reloaded.ToData())
			memoryPlacements := allKnownPlacements(memoryData.Players["canvas-viewer"].View.Memory)
			require.Equal(t, tc.offset, memoryPlacements["canvas-prop"].Offset, "canvas prop memory JSON")
			require.Equal(t, tc.offset, memoryPlacements["monster-canvas-0"].Offset, "canvas monster memory JSON")
		})
	}
}

func roundTripPlacementOffsetData(t *testing.T, data *Data) *Data {
	t.Helper()
	payload, err := json.Marshal(data)
	require.NoError(t, err)
	var persisted Data
	require.NoError(t, json.Unmarshal(payload, &persisted))
	return &persisted
}

func reloadPlacementOffsetEncounter(t *testing.T, enc *Encounter) *Encounter {
	t.Helper()
	payload, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var persisted Data
	require.NoError(t, json.Unmarshal(payload, &persisted))
	reloaded, err := LoadFromData(context.Background(), &persisted, enc.broker)
	require.NoError(t, err)
	return reloaded
}

func mergeKnownPlacements(knownSets ...map[core.Hex]perception.HexObservation) map[core.EntityID]perception.Placement {
	out := make(map[core.EntityID]perception.Placement)
	for _, known := range knownSets {
		for entityID, placement := range allKnownPlacements(known) {
			out[entityID] = placement
		}
	}
	return out
}

func roundTripKnownHexEvents(t *testing.T, known []events.KnownHex) []events.KnownHex {
	t.Helper()
	payload, err := json.Marshal(known)
	require.NoError(t, err)
	var replay []events.KnownHex
	require.NoError(t, json.Unmarshal(payload, &replay))
	return replay
}

func mergeKnownEventOffsets(knownSets ...[]events.KnownHex) map[core.EntityID]*core.PlacementOffset {
	out := make(map[core.EntityID]*core.PlacementOffset)
	for _, known := range knownSets {
		for _, hex := range known {
			for _, placement := range hex.Contents {
				out[placement.EntityID] = placement.Offset
			}
		}
	}
	return out
}

func TestCanvasMonsterPlacementOffsetRealRuntimeMatrix(t *testing.T) {
	zero := core.PlacementOffset{0, 0, 0}
	signed := core.PlacementOffset{1.5, -0.25, -3}
	cases := []struct {
		name   string
		offset *core.PlacementOffset
	}{
		{name: "omitted"},
		{name: "explicit zero", offset: &zero},
		{name: "signed", offset: &signed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := smNewEncounter(t)
			params := DungeonParams{
				FloorSource: FloorSourceCanvas, Key: "canvas-offset-runtime", Width: 4, Height: 2,
				RandomSeed: 1, PartyStart: PartyStartParams{SeatCount: 1},
			}
			require.NoError(t, enc.InitDungeon(params))
			at := core.HexFromPosition(structuralPosition(2, 0))
			require.NoError(t, enc.SeedMonsters([]SpawnInstruction{{
				MonsterRef: smRefSkeleton, Count: 1, AbsoluteAt: &at, Offset: tc.offset,
			}}))

			payload, err := json.Marshal(enc.ToData())
			require.NoError(t, err)
			var persisted Data
			require.NoError(t, json.Unmarshal(payload, &persisted))
			reloaded, err := LoadFromData(context.Background(), &persisted, enc.broker)
			require.NoError(t, err)
			require.Equal(t, tc.offset, reloaded.data.Monsters["monster-canvas-0"].Offset)

			require.NoError(t, reloaded.AddPlayer(PlayerInput{
				PlayerID: "canvas-viewer", EntityID: "canvas-viewer-entity",
				Position: core.HexFromPosition(structuralPosition(0, 0)), SightRange: 10,
			}))
			known := reloaded.KnownHexes("canvas-viewer")
			require.Equal(t, tc.offset, allKnownPlacements(known)["monster-canvas-0"].Offset)
			eventOffsets := make(map[core.EntityID]*core.PlacementOffset)
			for _, hex := range knownHexesToEvents(known) {
				for _, placement := range hex.Contents {
					eventOffsets[placement.EntityID] = placement.Offset
				}
			}
			require.Equal(t, tc.offset, eventOffsets["monster-canvas-0"])

			origin := reloaded.data.Monsters["monster-canvas-0"].Position
			destination := core.HexFromPosition(structuralPosition(3, 0))
			view := reloaded.data.Players["canvas-viewer"].View
			view.SightRange = 0
			require.NoError(t, reloaded.MoveNPCSteps("monster-canvas-0", []core.Hex{destination}))
			require.Equal(t, tc.offset, reloaded.data.Monsters["monster-canvas-0"].Offset)
			remembered := reloaded.KnownHexes("canvas-viewer")[origin]
			require.Equal(t, perception.KnowledgeStateRemembered, remembered.State)
			require.Equal(t, tc.offset, placementsByEntityID(remembered.Contents)["monster-canvas-0"].Offset)

			view.SightRange = 10
			require.NoError(t, reloaded.refreshObservations(view, core.NewHexSet(origin, destination)))
			resighted := reloaded.KnownHexes("canvas-viewer")
			require.NotContains(t, placementsByEntityID(resighted[origin].Contents), core.EntityID("monster-canvas-0"))
			require.Equal(t, tc.offset, placementsByEntityID(resighted[destination].Contents)["monster-canvas-0"].Offset)

			overlay := reloaded.eventObservationsForAppearance(
				origin, map[core.PlayerID]struct{}{core.PlayerID("canvas-viewer"): {}}, "monster-canvas-0",
			)
			require.Equal(t, tc.offset, eventPlacementsByEntityID(overlay["canvas-viewer"].Contents)["monster-canvas-0"].Offset)
		})
	}
}

func eventPlacementsByEntityID(placements []events.KnownHexPlacement) map[core.EntityID]events.KnownHexPlacement {
	out := make(map[core.EntityID]events.KnownHexPlacement, len(placements))
	for _, placement := range placements {
		out[placement.EntityID] = placement
	}
	return out
}

// structuralPosition names the existing [column,row] -> core.Hex conversion
// used throughout the runtime tests without importing dungeonspec.
func structuralPosition(column, row int) spatial.Position {
	return spatial.Position{X: float64(column), Y: float64(row)}
}

func allKnownPlacements(known map[core.Hex]perception.HexObservation) map[core.EntityID]perception.Placement {
	out := make(map[core.EntityID]perception.Placement)
	for _, observation := range known {
		for _, placement := range observation.Contents {
			out[placement.EntityID] = placement
		}
	}
	return out
}

func placementsByEntityID(placements []perception.Placement) map[core.EntityID]perception.Placement {
	out := make(map[core.EntityID]perception.Placement, len(placements))
	for _, placement := range placements {
		out[placement.EntityID] = placement
	}
	return out
}
