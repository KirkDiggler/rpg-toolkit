// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/stretchr/testify/require"
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

func placementsByEntityID(placements []perception.Placement) map[core.EntityID]perception.Placement {
	out := make(map[core.EntityID]perception.Placement, len(placements))
	for _, placement := range placements {
		out[placement.EntityID] = placement
	}
	return out
}
