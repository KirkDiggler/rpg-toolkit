// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
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

func authoredEdgeOfKind(
	t *testing.T,
	edges []encounter.AuthoredEdge,
	kind encounter.GeneratedEdgeKind,
) encounter.AuthoredEdge {
	t.Helper()
	for _, edge := range edges {
		if edge.Kind == kind {
			return edge
		}
	}
	t.Fatalf("authored edge kind %q not found", kind)
	return encounter.AuthoredEdge{}
}

func observedDoorEdge(t *testing.T, edges []perception.Edge, id core.EntityID) perception.Edge {
	t.Helper()
	for _, edge := range edges {
		if edge.DoorID == string(id) {
			return edge
		}
	}
	t.Fatalf("observed door %q not found", id)
	return perception.Edge{}
}

func containsObservedSolid(edges []perception.Edge, want encounter.AuthoredEdge) bool {
	for _, edge := range edges {
		if edge.DoorID == "" && edge.BlocksMovement && edge.BlocksLoS &&
			edge.From == want.From && edge.To == want.To {
			return true
		}
	}
	return false
}

func nextDirectHexAcross(t *testing.T, enc *encounter.Encounter, from, through core.Hex) core.Hex {
	t.Helper()
	grid := enc.Room().GetGrid()
	for _, position := range grid.GetNeighbors(through.ToPosition()) {
		candidate := core.HexFromPosition(position)
		ray := grid.GetLineOfSight(from.ToPosition(), position)
		if len(ray) >= 3 && ray[1].Equals(through.ToPosition()) {
			return candidate
		}
	}
	t.Fatalf("no direct hex continues from %v through %v", from, through)
	return core.Hex{}
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

// TestInitDungeon_AuthoredEdgesPersistDescribeAndBlock proves Phase 2B's
// end-to-end authority: normalized edge data remains the one read projection,
// while solids and closed doors become spatial crossings without occupying
// either endpoint or changing either endpoint's semantic region.
func TestInitDungeon_AuthoredEdgesPersistDescribeAndBlock(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))

	data := enc.ToData()
	require.NotNil(t, data.Space)
	assert.Equal(t, authoredEdgeDungeonKey, data.Space.DungeonKey)
	require.Len(t, data.Space.AuthoredEdges, 2)
	for _, edge := range data.Space.AuthoredEdges {
		assertAuthoredEdgeNormalized(t, edge)
	}
	solid := authoredEdgeOfKind(t, data.Space.AuthoredEdges, encounter.GeneratedEdgeKindSolid)
	door := authoredEdgeOfKind(t, data.Space.AuthoredEdges, encounter.GeneratedEdgeKindDoor)
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
	assert.True(t, containsEdge(combined.Edges, solid))
	assert.True(t, containsEdge(combined.Edges, door))
	assert.True(t, containsDoorID(combined.Edges, "legacy-connector"))
	assertCombinedEdgesSorted(t, combined.Edges)
	// Authored records are normalized durable identities. Generated records
	// intentionally retain their source orientation so knowledge indexes a
	// perimeter edge under its interior floor endpoint rather than exterior.
	assertAuthoredEdgeNormalized(t, solid)
	assertAuthoredEdgeNormalized(t, door)

	for _, edge := range []encounter.AuthoredEdge{solid, door} {
		assert.True(t, enc.Room().CanPlaceEntity(probeEntity{}, edge.From.ToPosition()),
			"an authored boundary must not occupy its first endpoint")
		assert.True(t, enc.Room().CanPlaceEntity(probeEntity{}, edge.To.ToPosition()),
			"an authored boundary must not occupy its second endpoint")
		assert.True(t, enc.Room().IsLineOfSightBlocked(edge.From.ToPosition(), edge.To.ToPosition()))
		assert.True(t, enc.Room().IsLineOfSightBlocked(edge.To.ToPosition(), edge.From.ToPosition()))
		fromRegion, fromOK := data.Space.RegionAt(edge.From)
		toRegion, toOK := data.Space.RegionAt(edge.To)
		assert.True(t, fromOK)
		assert.True(t, toOK)
		assert.Equal(t, fromRegion, toRegion, "an inner authored edge must not split semantic regions")
	}

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
	assert.True(t, reloaded.Room().IsLineOfSightBlocked(door.From.ToPosition(), door.To.ToPosition()))
	reloadedCombined, err := reloaded.DescribeEdges(encounter.DescribeEdgesInput{})
	require.NoError(t, err)
	assert.Equal(t, combined.Edges, reloadedCombined.Edges)
}

// TestDescribeEdges_AuthoredEdgeRejectsCollidingLegacySolid keeps the strict
// combined read seam aligned with persisted validation: a legacy segment whose
// End would become an authored endpoint cell blocker is never an effective
// authored-edge overlay.
func TestDescribeEdges_AuthoredEdgeRejectsCollidingLegacySolid(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))

	data := enc.ToData()
	authoredDoor := data.Space.AuthoredEdges[1]
	data.Space.Walls = append(data.Space.Walls, environments.WallSegmentData{
		Start: authoredDoor.From.ToCube(), End: authoredDoor.To.ToCube(), BlocksMovement: true, BlocksLoS: true,
	})

	generated, err := enc.DescribeGeneratedEdges(encounter.DescribeGeneratedEdgesInput{})
	require.NoError(t, err)
	assert.True(t, containsSolidEdge(generated.Edges, authoredDoor.From, authoredDoor.To))

	_, err = enc.DescribeEdges(encounter.DescribeEdgesInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "legacy wall")
	assert.Contains(t, err.Error(), "authored edge endpoint")
}

// TestAuthoredDoorLifecycle_OpensFromEitherEndpoint keeps authored doors on
// the existing OpenDoor/reveal/memory path while preserving their edge-native
// geometry. #179 authored doors are always unlocked, so AttemptUnlock keeps
// returning ErrDoorNotLocked.
func TestAuthoredDoorLifecycle_OpensFromEitherEndpoint(t *testing.T) {
	for _, side := range []struct {
		name string
		pick func(encounter.AuthoredEdge) core.Hex
	}{
		{name: "from", pick: func(edge encounter.AuthoredEdge) core.Hex { return edge.From }},
		{name: "to", pick: func(edge encounter.AuthoredEdge) core.Hex { return edge.To }},
	} {
		t.Run(side.name, func(t *testing.T) {
			enc := newTestEncounter(t)
			require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
			door := authoredEdgeOfKind(t, enc.ToData().Space.AuthoredEdges, encounter.GeneratedEdgeKindDoor)
			position := side.pick(door)
			require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
				PlayerID: "author", EntityID: "author-character", Position: position, SightRange: 3,
			}))

			before := observedDoorEdge(t, enc.KnownHexes("author")[position].Edges, door.DoorID)
			assert.True(t, before.BlocksMovement)
			assert.True(t, before.BlocksLoS)
			assert.False(t, before.DoorOpen)
			other := door.From
			if position == door.From {
				other = door.To
			}
			require.NoError(t, enc.Move("author", []core.Hex{other}))
			assert.Equal(t, position, enc.ToData().Players["author"].View.Position,
				"a closed authored door blocks movement across its boundary")
			_, err := enc.AttemptUnlock("author", door.DoorID)
			require.ErrorIs(t, err, encounter.ErrDoorNotLocked)

			require.NoError(t, enc.OpenDoor("author", door.DoorID))
			assert.True(t, enc.ToData().Doors[door.DoorID].Open)
			assert.False(t, enc.Room().IsLineOfSightBlocked(door.From.ToPosition(), door.To.ToPosition()))
			after := observedDoorEdge(t, enc.KnownHexes("author")[position].Edges, door.DoorID)
			assert.False(t, after.BlocksMovement)
			assert.False(t, after.BlocksLoS)
			assert.True(t, after.DoorOpen)
			require.NoError(t, enc.Move("author", []core.Hex{other}))
			assert.Equal(t, other, enc.ToData().Players["author"].View.Position,
				"opening removes the authored boundary so movement can cross")
			require.Error(t, enc.OpenDoor("author", door.DoorID), "double-open remains refused")
		})
	}

	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	door := authoredEdgeOfKind(t, enc.ToData().Space.AuthoredEdges, encounter.GeneratedEdgeKindDoor)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "remote", EntityID: "remote-character", Position: authoredEdgeHex(0, 0), SightRange: 1,
	}))
	err := enc.OpenDoor("remote", door.DoorID)
	require.ErrorIs(t, err, encounter.ErrOutOfRange, "remote actors cannot open an authored boundary")
	enc.ToData().Doors[door.DoorID].Locked = true
	_, err = enc.AttemptUnlock("remote", door.DoorID)
	require.ErrorIs(t, err, encounter.ErrDoorNotLocked, "#179 authored doors never enter the lock prompt flow")
	enc.ToData().Doors[door.DoorID].Locked = false

	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "reloader", EntityID: "reload-character", Position: door.From, SightRange: 1,
	}))
	require.NoError(t, enc.OpenDoor("reloader", door.DoorID))
	encoded, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var restored encounter.Data
	require.NoError(t, json.Unmarshal(encoded, &restored))
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	reloaded, err := encounter.LoadFromData(context.Background(), &restored, broker)
	require.NoError(t, err)
	assert.True(t, reloaded.ToData().Doors[door.DoorID].Open)
	assert.False(t, reloaded.Room().IsLineOfSightBlocked(door.From.ToPosition(), door.To.ToPosition()))
}

// TestAddDoor_FailedOverwritePreservesAuthoredDoorData protects the stable
// authored DoorData record from AddDoor's rebuild rollback path.
func TestAddDoor_FailedOverwritePreservesAuthoredDoorData(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	door := authoredEdgeOfKind(t, enc.ToData().Space.AuthoredEdges, encounter.GeneratedEdgeKindDoor)
	original := *enc.ToData().Doors[door.DoorID]

	err := enc.AddDoor(door.DoorID, authoredEdgeHex(0, 0), false)
	require.Error(t, err)
	assert.Equal(t, original, *enc.ToData().Doors[door.DoorID])
	_, err = enc.DescribeEdges(encounter.DescribeEdgesInput{})
	require.NoError(t, err, "failed overwrite must leave authored persistence valid")
}

// TestAddDoor_AuthoredEndpointRejectedAtomically proves AddDoor cannot add
// legacy door-cell geometry at an authored edge endpoint, and returns before
// changing either the persisted door map or the registered room.
func TestAddDoor_AuthoredEndpointRejectedAtomically(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	edge := authoredEdgeOfKind(t, enc.ToData().Space.AuthoredEdges, encounter.GeneratedEdgeKindSolid)
	previousSpace := enc.ToData().Space
	previousRoom := enc.Room()

	err := enc.AddDoor("legacy-at-authored-endpoint", edge.From, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authored edge endpoint")
	assert.Same(t, previousSpace, enc.ToData().Space)
	assert.Same(t, previousRoom, enc.Room())
	_, exists := enc.ToData().Doors["legacy-at-authored-endpoint"]
	assert.False(t, exists)
	assert.True(t, enc.Room().CanPlaceEntity(probeEntity{}, edge.From.ToPosition()))
}

// TestInitDungeon_RejectsClosedLegacyDoorAtAuthoredEndpointAtomically proves
// a pre-existing legacy closed door cannot be carried into new authored edge
// geometry, while an open legacy door remains harmless cell-wise.
func TestInitDungeon_RejectsClosedLegacyDoorAtAuthoredEndpointAtomically(t *testing.T) {
	params := authoredEdgeParams()
	edge := authoredEdgeOfKind(t, params.AuthoredEdges, encounter.GeneratedEdgeKindSolid)

	enc := newTestEncounter(t)
	require.NoError(t, enc.InitRoom(20, 20, environments.PatternEmpty))
	require.NoError(t, enc.AddDoor("legacy-closed", edge.From, false))
	previousSpace := enc.ToData().Space
	previousRoom := enc.Room()
	previousDoor := *enc.ToData().Doors["legacy-closed"]

	err := enc.InitDungeon(params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed legacy door")
	assert.Same(t, previousSpace, enc.ToData().Space)
	assert.Same(t, previousRoom, enc.Room())
	assert.Equal(t, previousDoor, *enc.ToData().Doors["legacy-closed"])

	open := newTestEncounter(t)
	require.NoError(t, open.InitRoom(20, 20, environments.PatternEmpty))
	require.NoError(t, open.AddDoor("legacy-open", edge.From, true))
	require.NoError(t, open.InitDungeon(params), "an open legacy door has no cell blocker to conflict")
	assert.True(t, open.ToData().Doors["legacy-open"].Open)
}

// TestAuthoredEdges_BlockSparsePlayerAndNPCMovement proves encounter-owned
// movement expands a requested segment through its actual grid ray rather
// than letting either actor jump over an authored boundary by omitting
// intermediate hexes from a path.
func TestAuthoredEdges_BlockSparsePlayerAndNPCMovement(t *testing.T) {
	playerEncounter := newTestEncounter(t)
	require.NoError(t, playerEncounter.InitDungeon(authoredEdgeParams()))
	solid := authoredEdgeOfKind(t, playerEncounter.ToData().Space.AuthoredEdges, encounter.GeneratedEdgeKindSolid)
	far := nextDirectHexAcross(t, playerEncounter, solid.From, solid.To)
	require.NoError(t, playerEncounter.AddPlayer(encounter.PlayerInput{
		PlayerID: "player", EntityID: "player-character", Position: solid.From, SightRange: 1,
	}))
	require.NoError(t, playerEncounter.Move("player", []core.Hex{far}))
	assert.Equal(t, solid.From, playerEncounter.ToData().Players["player"].View.Position,
		"a sparse player request cannot jump over an authored boundary")

	npcEncounter := newTestEncounter(t)
	require.NoError(t, npcEncounter.InitDungeon(authoredEdgeParams()))
	solid = authoredEdgeOfKind(t, npcEncounter.ToData().Space.AuthoredEdges, encounter.GeneratedEdgeKindSolid)
	far = nextDirectHexAcross(t, npcEncounter, solid.From, solid.To)
	require.NoError(t, npcEncounter.AddMonster(encounter.MonsterInput{
		ID: "npc", Position: solid.From, HP: 7, MaxHP: 7, AC: 12, Speed: 6,
	}))
	require.NoError(t, npcEncounter.MoveNPCSteps("npc", []core.Hex{far}))
	assert.Equal(t, solid.From, npcEncounter.ToData().Monsters["npc"].Position,
		"a sparse NPC request cannot jump over an authored boundary")
}

// TestAuthoredEdges_KnownHexesUseEffectiveCanonicalEdges verifies that both
// incident sides receive the exact same normalized authored records ready for
// HexRecord projection, including live door state after OpenDoor refreshes
// visible memory.
func TestAuthoredEdges_KnownHexesUseEffectiveCanonicalEdges(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	data := enc.ToData()
	solid := authoredEdgeOfKind(t, data.Space.AuthoredEdges, encounter.GeneratedEdgeKindSolid)
	door := authoredEdgeOfKind(t, data.Space.AuthoredEdges, encounter.GeneratedEdgeKindDoor)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "solid-viewer", EntityID: "solid-character", Position: solid.To, SightRange: 1,
	}))
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "door-viewer", EntityID: "door-character", Position: door.To, SightRange: 3,
	}))

	solidKnown := enc.KnownHexes("solid-viewer")[solid.To]
	require.True(t, containsObservedSolid(solidKnown.Edges, solid))
	doorKnown := enc.KnownHexes("door-viewer")[door.To]
	closed := observedDoorEdge(t, doorKnown.Edges, door.DoorID)
	assert.Equal(t, door.From, closed.From)
	assert.Equal(t, door.To, closed.To)
	assert.True(t, closed.BlocksMovement)
	assert.True(t, closed.BlocksLoS)
	assert.False(t, closed.DoorOpen)

	require.NoError(t, enc.OpenDoor("door-viewer", door.DoorID))
	open := observedDoorEdge(t, enc.KnownHexes("door-viewer")[door.To].Edges, door.DoorID)
	assert.False(t, open.BlocksMovement)
	assert.False(t, open.BlocksLoS)
	assert.True(t, open.DoorOpen)
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

	err = load(t, func(data *encounter.Data) {
		doorID := data.Space.AuthoredEdges[1].DoorID
		data.Doors[doorID].Locked = true
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remain unlocked")
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

// TestAuthoredEdges_CanonicalSparseTraversalUsesTheSameCrossings proves that
// encounter's sparse player/NPC segment validation uses spatial's canonical
// unordered-pair ray. This pointy-top pair has deliberately different forward
// and reverse grid rays; the authored edge exists only on the canonical ray.
func TestAuthoredEdges_CanonicalSparseTraversalUsesTheSameCrossings(t *testing.T) {
	start := authoredEdgeHex(0, 0)
	end := authoredEdgeHex(7, 24)
	edgeFrom := authoredEdgeHex(4, 16)
	edgeTo := authoredEdgeHex(5, 16)
	params := encounter.DungeonParams{
		Key:    "canonical-sparse-traversal",
		Height: 25,
		Regions: []encounter.DungeonRegionParams{
			{ID: "entry", Archetype: encounter.ArchetypeEntrance, Width: 8, Pattern: environments.PatternEmpty},
			{ID: "boss", Archetype: encounter.ArchetypeBoss, Width: 8, Pattern: environments.PatternEmpty},
		},
		Connectors: []encounter.DungeonConnectorParams{{DoorID: "canonical-connector"}},
		AuthoredEdges: []encounter.AuthoredEdge{{
			From: edgeFrom, To: edgeTo, Kind: encounter.GeneratedEdgeKindSolid,
		}},
	}

	forward := newTestEncounter(t)
	require.NoError(t, forward.InitDungeon(params))
	require.NoError(t, forward.AddPlayer(encounter.PlayerInput{
		PlayerID: "forward", EntityID: "forward-character", Position: start, SightRange: 0,
	}))
	require.NoError(t, forward.Move("forward", []core.Hex{end}))
	assert.Equal(t, start, forward.ToData().Players["forward"].View.Position)

	reverse := newTestEncounter(t)
	require.NoError(t, reverse.InitDungeon(params))
	require.NoError(t, reverse.AddPlayer(encounter.PlayerInput{
		PlayerID: "reverse", EntityID: "reverse-character", Position: end, SightRange: 0,
	}))
	require.NoError(t, reverse.Move("reverse", []core.Hex{start}))
	assert.Equal(t, end, reverse.ToData().Players["reverse"].View.Position,
		"the reverse sparse request must cross the same canonical physical edge")

	// Canonicalization belongs only to crossings. This cell exists on the
	// caller-requested reverse tie ray but not the canonical boundary ray, so
	// static blockers must still stop the reverse movement.
	cellBlocked := newTestEncounter(t)
	require.NoError(t, cellBlocked.InitDungeon(params))
	require.NoError(t, cellBlocked.AddObstacle(
		"reverse-tie-cell", "test:props:wall", authoredEdgeHex(5, 15), true, true,
	))
	require.NoError(t, cellBlocked.AddPlayer(encounter.PlayerInput{
		PlayerID: "cell-blocked", EntityID: "cell-blocked-character", Position: end, SightRange: 0,
	}))
	require.NoError(t, cellBlocked.Move("cell-blocked", []core.Hex{start}))
	assert.Equal(t, end, cellBlocked.ToData().Players["cell-blocked"].View.Position,
		"canonical boundary checks must not bypass a blocker on the requested cell ray")
}

// TestInitDungeon_AuthoredEndpointsStripLegacyWalls keeps the authored edge
// crossing distinct from generated wall-cell geometry without reserving the
// endpoint from ordinary content placement.
func TestInitDungeon_AuthoredEndpointsStripLegacyWalls(t *testing.T) {
	for _, seed := range []int64{1, 2, 3, 4, 5, 17, 19, 42, 100, 909091} {
		params := authoredEdgeParams()
		params.RandomSeed = seed
		params.Regions[0].Pattern = environments.PatternRandom

		enc := newTestEncounter(t)
		require.NoError(t, enc.InitDungeon(params), "seed %d", seed)
		for _, edge := range enc.ToData().Space.AuthoredEdges {
			for _, wall := range enc.ToData().Space.Walls {
				assert.False(t, wall.Start == edge.From.ToCube() && wall.End == wall.Start, "seed %d from", seed)
				assert.False(t, wall.Start == edge.To.ToCube() && wall.End == wall.Start, "seed %d to", seed)
			}
		}
	}
}

// TestAuthoredEndpointsAllowPropsAndPartyStarts proves endpoint sharing is
// reserved only against legacy wall/door geometry. A normal prop and party
// start remain legal and the prop independently blocks its endpoint cell.
func TestAuthoredEndpointsAllowPropsAndPartyStarts(t *testing.T) {
	params := authoredEdgeParams()
	anchor := params.AuthoredEdges[0].From
	params.PartyStart = encounter.PartyStartParams{Anchor: &anchor, SeatCount: 1}
	propEndpoint := params.AuthoredEdges[0].To
	params.Regions[0].PlacedObstacles = []encounter.PlacedObstacleSpec{{
		Ref: "test:props:fixed", At: encounter.LocalHex{Col: 2, Row: 1}, BlocksMovement: true, BlocksLoS: true,
	}}

	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(params))
	data := enc.ToData()
	assert.Equal(t, anchor, data.Space.Entrance)
	assert.Equal(t, []core.Hex{anchor}, data.Space.PartyStartPositions)
	require.Len(t, data.Space.Obstacles, 1)
	assert.Equal(t, propEndpoint, data.Space.Obstacles[0].Position)
	assert.False(t, enc.Room().CanPlaceEntity(probeEntity{}, propEndpoint.ToPosition()),
		"the prop independently blocks the endpoint cell; the authored edge still owns only its crossing")

	door := authoredEdgeOfKind(t, data.Space.AuthoredEdges, encounter.GeneratedEdgeKindDoor)
	require.NoError(t, enc.AddObstacle("runtime-endpoint-prop", "test:props:runtime", door.From, true, true))
	assert.False(t, enc.Room().CanPlaceEntity(probeEntity{}, door.From.ToPosition()))
}

// TestAuthoredDoorOpen_ProjectsEitherIncidentPosition keeps the legacy
// single-position projector compatible while proving the additive two-endpoint
// path publishes/reveals/refreshes observers who see only the To endpoint.
func TestAuthoredDoorOpen_ProjectsEitherIncidentPosition(t *testing.T) {
	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() {
		_ = broker.Close()
		_ = transport.Close()
	})
	enc := encounter.New(context.Background(), "enc-authored-door-projection", broker)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	door := authoredEdgeOfKind(t, enc.ToData().Space.AuthoredEdges, encounter.GeneratedEdgeKindDoor)
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "opener", EntityID: "opener-character", Position: door.From, SightRange: 0,
	}))
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "to-zero", EntityID: "to-zero-character", Position: door.To, SightRange: 0,
	}))
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "to-reveal", EntityID: "to-reveal-character", Position: door.To, SightRange: 1,
	}))
	zeroSub, err := broker.Subscribe(enc.ID(), "to-zero")
	require.NoError(t, err)
	t.Cleanup(func() { _ = zeroSub.Close() })
	revealSub, err := broker.Subscribe(enc.ID(), "to-reveal")
	require.NoError(t, err)
	t.Cleanup(func() { _ = revealSub.Close() })

	require.NoError(t, enc.OpenDoor("opener", door.DoorID))
	zeroEvents := collectEventsTyped(zeroSub, 300*time.Millisecond)
	var zeroDoorEvent *events.DoorOpenedEvent
	for _, event := range zeroEvents {
		if opened, ok := event.(*events.DoorOpenedEvent); ok {
			zeroDoorEvent = opened
			break
		}
	}
	require.NotNil(t, zeroDoorEvent, "a zero-range viewer standing on To receives DoorOpened")
	assert.True(t, zeroDoorEvent.PerPlayer["to-zero"].Visible)
	zeroDoor := observedDoorEdge(t, enc.KnownHexes("to-zero")[door.To].Edges, door.DoorID)
	assert.True(t, zeroDoor.DoorOpen, "the To-side viewer memory is refreshed to the live open state")

	revealEvents := collectEventsTyped(revealSub, 300*time.Millisecond)
	var sawDoorOpen, sawReveal bool
	for _, event := range revealEvents {
		switch event.(type) {
		case *events.DoorOpenedEvent:
			sawDoorOpen = true
		case *events.HexRevealedEvent:
			sawReveal = true
		}
	}
	assert.True(t, sawDoorOpen, "a To-side viewer with range sees DoorOpened")
	assert.True(t, sawReveal, "opening through To refreshes the normal reveal path")
}

// TestLoadFromData_AuthoredEdgesRejectInvalidCubeBeforeTraversal confirms
// malformed persisted coordinates fail at the authored-edge validation gate,
// before normalization, distance, or offset conversion can reinterpret them.
func TestLoadFromData_AuthoredEdgesRejectInvalidCubeBeforeTraversal(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	payload, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data encounter.Data
	require.NoError(t, json.Unmarshal(payload, &data))
	// This tuple wrapped to zero under the old native-int sum, allowing
	// normalization/distance work to run on corrupted persistence.
	data.Space.AuthoredEdges[0].From = core.Hex{Q: math.MaxInt, R: math.MaxInt, S: 2}

	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() {
		_ = broker.Close()
		_ = transport.Close()
	})
	_, err = encounter.LoadFromData(context.Background(), &data, broker)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cube")
	assert.NotContains(t, err.Error(), "not normalized")
	assert.NotContains(t, err.Error(), "rebuild room")
}

// TestLoadFromData_AuthoredEndpointsRejectLegacyCellGeometry covers every
// persisted legacy cell blocker that would otherwise coexist with an authored
// edge endpoint. Authored door records and harmless open legacy doors stay
// valid because neither produces legacy door-cell geometry during rebuild.
func TestLoadFromData_AuthoredEndpointsRejectLegacyCellGeometry(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	payload, err := json.Marshal(enc.ToData())
	require.NoError(t, err)

	load := func(t *testing.T, mutate func(*encounter.Data)) (*encounter.Encounter, error) {
		t.Helper()
		var data encounter.Data
		require.NoError(t, json.Unmarshal(payload, &data))
		mutate(&data)
		transport := encounter.NewInMemoryTransport()
		broker := encounter.NewBroker(transport)
		t.Cleanup(func() {
			_ = broker.Close()
			_ = transport.Close()
		})
		return encounter.LoadFromData(context.Background(), &data, broker)
	}

	t.Run("degenerate wall at endpoint", func(t *testing.T) {
		_, err := load(t, func(data *encounter.Data) {
			endpoint := data.Space.AuthoredEdges[0].From.ToCube()
			data.Space.Walls = append(data.Space.Walls, environments.WallSegmentData{
				Start: endpoint, End: endpoint, BlocksMovement: true, BlocksLoS: true,
			})
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "legacy wall")
		assert.Contains(t, err.Error(), "authored edge endpoint")
	})

	t.Run("in-grid boundary wall ends at endpoint", func(t *testing.T) {
		_, err := load(t, func(data *encounter.Data) {
			edge := data.Space.AuthoredEdges[0]
			data.Space.Walls = append(data.Space.Walls, environments.WallSegmentData{
				Start: edge.To.ToCube(), End: edge.From.ToCube(), BlocksMovement: true, BlocksLoS: true,
			})
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "legacy wall")
		assert.Contains(t, err.Error(), "authored edge endpoint")
	})

	t.Run("closed legacy door at endpoint", func(t *testing.T) {
		_, err := load(t, func(data *encounter.Data) {
			endpoint := data.Space.AuthoredEdges[0].From
			data.Doors["legacy-closed"] = &encounter.DoorData{ID: "legacy-closed", Position: endpoint}
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "closed legacy door")
		assert.Contains(t, err.Error(), "authored edge endpoint")
	})

	t.Run("authored and open legacy doors remain valid", func(t *testing.T) {
		reloaded, err := load(t, func(data *encounter.Data) {
			endpoint := data.Space.AuthoredEdges[0].From
			data.Doors["legacy-open"] = &encounter.DoorData{ID: "legacy-open", Position: endpoint, Open: true}
		})
		require.NoError(t, err)
		require.NotNil(t, reloaded)
	})
}

// TestLoadFromData_RejectsNullDoorData protects room reconstruction from a
// JSON-null DoorData map entry instead of dereferencing it while rebuilding.
func TestLoadFromData_RejectsNullDoorData(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	payload, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data encounter.Data
	require.NoError(t, json.Unmarshal(payload, &data))
	data.Doors["null-door"] = nil

	transport := encounter.NewInMemoryTransport()
	broker := encounter.NewBroker(transport)
	t.Cleanup(func() {
		_ = broker.Close()
		_ = transport.Close()
	})
	var loadErr error
	require.NotPanics(t, func() {
		_, loadErr = encounter.LoadFromData(context.Background(), &data, broker)
	})
	require.Error(t, loadErr)
	assert.Contains(t, loadErr.Error(), "null door data")
}

// TestGeneratedPerimeterKnowledgeRetainsTheFloorFacingSource proves generated
// records keep their source orientation. Reversing a west perimeter edge
// would index it under the exterior hex and silently hide it from the player
// standing on the entrance floor cell.
func TestGeneratedPerimeterKnowledgeRetainsTheFloorFacingSource(t *testing.T) {
	enc := newTestEncounter(t)
	require.NoError(t, enc.InitDungeon(authoredEdgeParams()))
	data := enc.ToData()
	entrance := data.Space.Entrance
	var perimeter environments.WallSegmentData
	found := false
	for _, wall := range data.Space.Walls {
		if wall.Start != entrance.ToCube() || wall.Start == wall.End {
			continue
		}
		endPosition := wall.End.ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
		if !enc.Room().GetGrid().IsValidPosition(endPosition) {
			perimeter = wall
			found = true
			break
		}
	}
	require.True(t, found, "fixture entrance has a generated exterior edge")
	require.NoError(t, enc.AddPlayer(encounter.PlayerInput{
		PlayerID: "perimeter-viewer", EntityID: "perimeter-character", Position: entrance, SightRange: 0,
	}))

	known := enc.KnownHexes("perimeter-viewer")[entrance]
	assert.Contains(t, known.Edges, perception.Edge{
		From: entrance, To: core.HexFromCube(perimeter.End), BlocksMovement: true, BlocksLoS: true,
	})
	combined, err := enc.DescribeEdges(encounter.DescribeEdgesInput{})
	require.NoError(t, err)
	assert.True(t, containsSolidEdge(combined.Edges, entrance, core.HexFromCube(perimeter.End)))
	for _, edge := range combined.Edges {
		if edge.From == entrance && edge.To == core.HexFromCube(perimeter.End) {
			return
		}
	}
	t.Fatal("combined generated perimeter edge lost its floor-facing source orientation")
}

// TestInitDungeon_AuthoredDoorIDsAreAtomicAcrossCollisionsAndRebuildFailure
// verifies both failure classes leave pre-existing doors and room data intact.
func TestInitDungeon_AuthoredDoorIDsAreAtomicAcrossCollisionsAndRebuildFailure(t *testing.T) {
	t.Run("collision", func(t *testing.T) {
		enc := newTestEncounter(t)
		previous := &encounter.DoorData{ID: "legacy-connector", Position: authoredEdgeHex(0, 0), Open: true}
		enc.ToData().Doors[previous.ID] = previous

		err := enc.InitDungeon(authoredEdgeParams())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Nil(t, enc.ToData().Space)
		assert.Same(t, previous, enc.ToData().Doors[previous.ID])
		assert.True(t, enc.ToData().Doors[previous.ID].Open)
	})

	t.Run("later rebuild failure", func(t *testing.T) {
		enc := newTestEncounter(t)
		require.NoError(t, enc.InitRoom(20, 20, environments.PatternEmpty))
		previousPosition := authoredEdgeHex(0, 12)
		require.NoError(t, enc.AddDoor("preexisting", previousPosition, false))
		previousSpace := enc.ToData().Space
		previousRoom := enc.Room()
		previous := *enc.ToData().Doors["preexisting"]

		err := enc.InitDungeon(authoredEdgeParams())
		require.Error(t, err)
		assert.Same(t, previousSpace, enc.ToData().Space)
		assert.Same(t, previousRoom, enc.Room())
		assert.Equal(t, previous, *enc.ToData().Doors["preexisting"])
		_, stagedConnector := enc.ToData().Doors["legacy-connector"]
		assert.False(t, stagedConnector)
		for _, edge := range authoredEdgeParams().AuthoredEdges {
			if edge.Kind == encounter.GeneratedEdgeKindDoor {
				_, stagedDoor := enc.ToData().Doors[edge.DoorID]
				assert.False(t, stagedDoor)
			}
		}
	})
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
