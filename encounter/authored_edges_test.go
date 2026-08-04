// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	authoredEdgeDungeonKey = "authored-edge-regression"
	authoredEntryRegionID  = "entry"
)

// authoredEdgeHex uses the same absolute pointy-top conversion the compiler
// uses. Tests must not construct cube values by hand, which would let an
// orientation drift hide behind test data.
func authoredEdgeHex(column, row int) core.Hex {
	return core.HexFromPosition(spatial.Position{X: float64(column), Y: float64(row)})
}

func authoredEdgeParams() encounter.DungeonParams {
	solidFrom, solidTo := authoredEdgeHex(1, 1), authoredEdgeHex(2, 1)
	doorFrom, doorTo := authoredEdgeHex(3, 2), authoredEdgeHex(4, 2)
	return encounter.DungeonParams{
		Key:    authoredEdgeDungeonKey,
		Height: 8,
		Regions: []encounter.DungeonRegionParams{
			{ID: authoredEntryRegionID, Archetype: encounter.ArchetypeEntrance, Width: 8, Pattern: environments.PatternEmpty},
			{ID: "boss", Archetype: encounter.ArchetypeBoss, Width: 8, Pattern: environments.PatternEmpty},
		},
		Connectors: []encounter.DungeonConnectorParams{{DoorID: "legacy-connector"}},
		AuthoredEdges: []encounter.AuthoredEdge{
			{From: solidFrom, To: solidTo, Kind: encounter.GeneratedEdgeKindSolid},
			{From: doorFrom, To: doorTo, Kind: encounter.GeneratedEdgeKindDoor,
				DoorID: encounter.AuthoredDoorID(authoredEdgeDungeonKey, doorFrom, doorTo)},
		},
	}
}

// TestInitDungeon_AuthoredEdgesPersistAndDescribeCombined proves the Phase 2A
// state path end-to-end: authored edges become normalized dungeon-owned data,
// authored doors get ordinary closed/unlocked DoorData, JSON reload keeps the
// same state, and DescribeEdges is the one combined sorted read projection.
func TestInitDungeon_AuthoredEdgesPersistAndDescribeCombined(t *testing.T) {
	params := authoredEdgeParams()
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(params))

	data := enc.ToData()
	require.NotNil(t, data.Space)
	assert.Equal(t, authoredEdgeDungeonKey, data.Space.DungeonKey)
	require.Len(t, data.Space.AuthoredEdges, 2)
	for _, edge := range data.Space.AuthoredEdges {
		assertAuthoredEdgeNormalized(t, edge)
	}

	door := data.Space.AuthoredEdges[1]
	require.Equal(t, encounter.GeneratedEdgeKindDoor, door.Kind)
	persistedDoor, ok := data.Doors[door.DoorID]
	require.True(t, ok)
	assert.Equal(t, door.DoorID, persistedDoor.ID)
	assert.False(t, persistedDoor.Open)
	assert.False(t, persistedDoor.Locked)

	generated, err := enc.DescribeGeneratedEdges(encounter.DescribeGeneratedEdgesInput{})
	require.NoError(t, err)
	assert.False(t, containsEdge(generated.Edges, door),
		"DescribeGeneratedEdges must retain its generated-only source compatibility")
	assert.True(t, containsDoorID(generated.Edges, "legacy-connector"),
		"legacy connector doors remain on the generated seam")

	combined, err := enc.DescribeEdges(encounter.DescribeEdgesInput{})
	require.NoError(t, err)
	assert.True(t, containsEdge(combined.Edges, data.Space.AuthoredEdges[0]))
	assert.True(t, containsEdge(combined.Edges, door))
	assert.True(t, containsDoorID(combined.Edges, "legacy-connector"))
	assertCombinedEdgesSorted(t, combined.Edges)
	for _, edge := range combined.Edges {
		assert.False(t, hexLess(edge.To, edge.From), "combined seam must normalize every undirected endpoint pair")
	}

	// Phase 2A owns data/projection only. These records must not silently turn
	// into spatial blockers before Phase 2B registers boundary crossings.
	assert.True(t, enc.Room().CanPlaceEntity(probeEntity{}, data.Space.AuthoredEdges[0].From.ToPosition()))
	assert.True(t, enc.Room().CanPlaceEntity(probeEntity{}, door.From.ToPosition()))

	encoded, err := json.Marshal(data)
	require.NoError(t, err)
	var restoredData encounter.Data
	require.NoError(t, json.Unmarshal(encoded, &restoredData))

	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	reloaded, err := encounter.LoadFromData(context.Background(), &restoredData, broker)
	require.NoError(t, err)
	require.Equal(t, data.Space.AuthoredEdges, reloaded.ToData().Space.AuthoredEdges)
	reloadedDoor := reloaded.ToData().Doors[door.DoorID]
	require.NotNil(t, reloadedDoor)
	assert.False(t, reloadedDoor.Open)
	assert.False(t, reloadedDoor.Locked)
	reloadedCombined, err := reloaded.DescribeEdges(encounter.DescribeEdgesInput{})
	require.NoError(t, err)
	assert.Equal(t, combined.Edges, reloadedCombined.Edges)
}

// TestDescribeEdges_AuthoredEdgeReplacesCollidingGeneratedSolid pins the
// overlay rule without changing DescribeGeneratedEdges. Current patterned
// rooms serialize interior blockers as self-loops, so the physical generated
// solid below deliberately models the persisted future/non-connector shape
// this compatibility rule protects.
func TestDescribeEdges_AuthoredEdgeReplacesCollidingGeneratedSolid(t *testing.T) {
	params := authoredEdgeParams()
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(params))

	data := enc.ToData()
	authoredDoor := data.Space.AuthoredEdges[1]
	data.Space.Walls = append(data.Space.Walls, environments.WallSegmentData{
		Start: authoredDoor.From.ToCube(), End: authoredDoor.To.ToCube(), BlocksMovement: true, BlocksLoS: true,
	})

	generated, err := enc.DescribeGeneratedEdges(encounter.DescribeGeneratedEdgesInput{})
	require.NoError(t, err)
	assert.True(t, containsSolidEdge(generated.Edges, authoredDoor.From, authoredDoor.To))

	combined, err := enc.DescribeEdges(encounter.DescribeEdgesInput{})
	require.NoError(t, err)
	assert.True(t, containsEdge(combined.Edges, authoredDoor))
	assert.False(t, containsSolidEdge(combined.Edges, authoredDoor.From, authoredDoor.To),
		"an authored edge replaces its colliding generated non-connector edge")
}

// TestInitDungeon_AuthoredDoorRemainsNonInteractiveInPhase2A verifies the
// explicit scope boundary. DoorData is initialized now for durable identity,
// but movement/LoS registration and either-endpoint interaction land later.
func TestInitDungeon_AuthoredDoorRemainsNonInteractiveInPhase2A(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	door := enc.ToData().Space.AuthoredEdges[1]
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "author", EntityID: "author-character", Position: door.From, SightRange: 1,
	}))

	err := enc.OpenDoor("author", door.DoorID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authored door")
	assert.False(t, enc.ToData().Doors[door.DoorID].Open)

	// Even a malformed in-memory lock state cannot route this Phase 2A door
	// through the existing prompt lifecycle before authored interactions land.
	enc.ToData().Doors[door.DoorID].Locked = true
	_, err = enc.AttemptUnlock("author", door.DoorID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authored door")
	assert.True(t, enc.ToData().Doors[door.DoorID].Locked)
}

// TestLoadFromData_AuthoredEdgesRejectsNonCanonicalDoorRecords makes reload
// refuse hand-mutated state that could otherwise steal a legacy connector door
// or make persistence ordering dependent on map/YAML ordering.
func TestLoadFromData_AuthoredEdgesRejectsNonCanonicalDoorRecords(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	encoded, err := json.Marshal(enc.ToData())
	require.NoError(t, err)

	load := func(t *testing.T, mutate func(*encounter.Data)) error {
		t.Helper()
		var data encounter.Data
		require.NoError(t, json.Unmarshal(encoded, &data))
		mutate(&data)
		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
		_, err := encounter.LoadFromData(context.Background(), &data, broker)
		return err
	}

	err = load(t, func(data *encounter.Data) {
		data.Space.AuthoredEdges[0], data.Space.AuthoredEdges[1] = data.Space.AuthoredEdges[1], data.Space.AuthoredEdges[0]
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical-sorted")

	err = load(t, func(data *encounter.Data) {
		data.Space.AuthoredEdges[1].DoorID = "legacy-connector"
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match its normalized endpoint")

	err = load(t, func(data *encounter.Data) {
		oldID := data.Space.AuthoredEdges[1].DoorID
		newID := core.EntityID("renamed-authored-door")
		door := data.Doors[oldID]
		delete(data.Doors, oldID)
		door.ID = newID
		data.Doors[newID] = door
		data.Space.AuthoredEdges[1].DoorID = newID
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not stable")
}

// TestInitDungeon_AuthoredEdgesRejectsDirectNonFloorAndUnstableDoorInputs is
// the public-engine defense behind dungeonspec validation. A host bypassing
// YAML cannot create a gap/connector collision or choose a competing door id.
func TestInitDungeon_AuthoredEdgesRejectsDirectNonFloorAndUnstableDoorInputs(t *testing.T) {
	params := authoredEdgeParams()
	params.AuthoredEdges[0].From = authoredEdgeHex(8, 1) // connector column
	params.AuthoredEdges[0].To = authoredEdgeHex(7, 1)
	enc := newTestEncounter(t)
	err := enc.InitDungeon(params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "semantic floor")

	params = authoredEdgeParams()
	params.AuthoredEdges[1].DoorID = "caller-chosen-door-id"
	enc = newTestEncounter(t)
	err = enc.InitDungeon(params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stable authored door id")
}

func assertAuthoredEdgeNormalized(t *testing.T, edge encounter.AuthoredEdge) {
	t.Helper()
	assert.False(t, hexLess(edge.To, edge.From), "authored edge must persist normalized: %+v", edge)
}

func containsEdge(edges []encounter.GeneratedEdge, want encounter.AuthoredEdge) bool {
	for _, edge := range edges {
		if edge.From == want.From && edge.To == want.To && edge.Kind == want.Kind && edge.DoorID == want.DoorID {
			return true
		}
	}
	return false
}

func containsDoorID(edges []encounter.GeneratedEdge, id core.EntityID) bool {
	for _, edge := range edges {
		if edge.DoorID == id {
			return true
		}
	}
	return false
}

func containsSolidEdge(edges []encounter.GeneratedEdge, from, to core.Hex) bool {
	for _, edge := range edges {
		if edge.Kind != encounter.GeneratedEdgeKindSolid {
			continue
		}
		if (edge.From == from && edge.To == to) || (edge.From == to && edge.To == from) {
			return true
		}
	}
	return false
}

func assertCombinedEdgesSorted(t *testing.T, edges []encounter.GeneratedEdge) {
	t.Helper()
	assert.True(t, sort.SliceIsSorted(edges, func(i, j int) bool {
		return combinedEdgeLess(edges[i], edges[j])
	}))
}

func combinedEdgeLess(left, right encounter.GeneratedEdge) bool {
	if hexLess(left.From, right.From) {
		return true
	}
	if hexLess(right.From, left.From) {
		return false
	}
	if hexLess(left.To, right.To) {
		return true
	}
	if hexLess(right.To, left.To) {
		return false
	}
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	return left.DoorID < right.DoorID
}

func hexLess(left, right core.Hex) bool {
	if left.Q != right.Q {
		return left.Q < right.Q
	}
	if left.R != right.R {
		return left.R < right.R
	}
	return left.S < right.S
}
