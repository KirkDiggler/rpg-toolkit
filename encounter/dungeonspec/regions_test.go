package dungeonspec

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

const runnableRegionsFixture = `version: 1
key: semantic-scopes
name: Semantic Scopes
canvas: { width: 5, height: 3 }
rooms: []
regions:
  - id: outer
    name: Outer
    archetype: chamber
    cells: [[0,0], [0,1], [1,0], [1,1], [1,1]]
  - id: inner
    cells: [[0,0]]
  - id: disconnected
    archetype: boss
    cells: [[3,0], [4,2]]
  - id: empty
    cells: []
`

func TestRunnableSemanticRegionsCompileProjectAndReload(t *testing.T) {
	compiled, err := LoadWithConfig([]byte(runnableRegionsFixture), LoadConfig{PartyStartSeatCount: 1})
	require.NoError(t, err)
	require.Len(t, compiled.Params.SemanticRegions, 4)
	require.Len(t, compiled.Params.SemanticRegions[0].Cells, 4, "same-scope duplicate cells canonicalize")

	plan, err := BuildFloorPlan(context.Background(), BuildFloorPlanInput{Compiled: compiled, Seed: 1})
	require.NoError(t, err)
	regionIDs := []string{plan.Regions[0].ID, plan.Regions[1].ID, plan.Regions[2].ID, plan.Regions[3].ID}
	require.Equal(t, []string{"outer", "inner", "disconnected", "empty"}, regionIDs)
	require.Equal(t, "outer", *plan.Regions[1].ParentID)
	require.Nil(t, plan.Regions[3].ParentID)
	require.Len(t, plan.Regions[0].Cells, 4)

	broker := encounter.NewBroker(encounter.NewInMemoryTransport())
	enc := encounter.New(context.Background(), "semantic", broker)
	require.NoError(t, enc.InitDungeon(compiled.Params))
	inner := core.HexFromPosition(spatial.Position{X: 0, Y: 0})
	root := core.HexFromPosition(spatial.Position{X: 2, Y: 2})
	require.Equal(t, "inner", enc.ToData().Space.ZoneAt(inner).ID)
	innerZone := enc.ToData().Space.ZoneAt(inner)
	require.Equal(t, encounter.ArchetypeChamber, *innerZone.Archetype, "archetype inherits from outer")
	require.Empty(t, enc.ToData().Space.ZoneAt(root).ID)

	payload, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	require.NotContains(t, string(payload), "parent_id", "derived parents are never durable facts")
	var restored encounter.Data
	require.NoError(t, json.Unmarshal(payload, &restored))
	reloaded, err := encounter.LoadFromData(context.Background(), &restored, broker)
	require.NoError(t, err)
	parent, ok := reloaded.ToData().Space.SemanticRegionParent("inner")
	require.True(t, ok)
	require.Equal(t, "outer", parent)
	require.Equal(t, "inner", reloaded.ToData().Space.ZoneAt(inner).ID)
}

func TestRunnableSemanticRegionRoleLightweightCases(t *testing.T) {
	const header = `version: 1
key: lightweight-regions
name: Lightweight Regions
canvas: { width: 4, height: 4 }
rooms: []
`
	cases := map[string]string{
		"zero regions": header,
		"no explicit role": header + `regions:
  - { id: unlabeled, cells: [[0,0]] }
`,
		"empty only": header + `regions:
  - { id: empty, cells: [] }
`,
		"one explicit role": header + `regions:
  - { id: entry, archetype: entrance, cells: [[0,0]] }
`,
		"many explicit roles": header + `regions:
  - { id: entry, archetype: entrance, cells: [[0,0]] }
  - { id: hall, archetype: chamber, cells: [[1,0]] }
  - { id: corridor, archetype: corridor, cells: [[2,0]] }
  - { id: boss-a, archetype: boss, cells: [[3,0]] }
  - { id: boss-b, archetype: boss, cells: [[3,1]] }
`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadWithConfig([]byte(source), LoadConfig{PartyStartSeatCount: 1})
			require.NoError(t, err)
		})
	}
}

func TestSemanticRegionStructuralRejectsOnlyScopeFailures(t *testing.T) {
	cases := map[string]string{
		"duplicate id":          strings.Replace(runnableRegionsFixture, "- id: inner", "- id: outer", 1),
		"out of floor":          strings.Replace(runnableRegionsFixture, "cells: [[0,0]]", "cells: [[9,9]]", 1),
		"unsupported archetype": strings.Replace(runnableRegionsFixture, "archetype: chamber", "archetype: library", 1),
		"partial overlap":       strings.Replace(runnableRegionsFixture, "cells: [[0,0]]", "cells: [[0,0], [2,2]]", 1),
		"equal overlap": strings.Replace(
			runnableRegionsFixture, "cells: [[0,0]]", "cells: [[0,0], [0,1], [1,0], [1,1]]", 1,
		),
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load([]byte(source))
			require.Error(t, err)
		})
	}
}
