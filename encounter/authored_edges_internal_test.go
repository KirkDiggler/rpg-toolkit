// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"fmt"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/environments"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

// TestValidateAuthoredEdgeOverlay_RejectsConnectorDoorCollision exercises the
// defense-in-depth branch for a future generator that exposes a connector
// physical edge whose endpoints both happen to be semantic floor cells. The
// current v1 connector column is rejected earlier as non-floor; this production
// helper is still the authoritative no-overlay rule if that geometry evolves.
// TestBuildPerception_ConstrainsMonsterAStar verifies that encounter turns
// its reconstructed cell/edge geometry into the optional rulebook traversal
// contract: a monster detours around an authored solid, and an ordinary cell
// blocker remains rejected through both BlockedHexes and the predicate.
func TestBuildPerception_ConstrainsMonsterAStar(t *testing.T) {
	transport := NewInMemoryTransport()
	broker := NewBroker(transport)
	t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
	from := core.HexFromPosition(spatial.Position{X: 1, Y: 1})
	to := core.HexFromPosition(spatial.Position{X: 2, Y: 1})
	enc := New(context.Background(), "enc-authored-perception", broker)
	require.NoError(t, enc.InitDungeon(DungeonParams{
		Key: "authored-perception", Height: 8,
		Regions: []DungeonRegionParams{
			{ID: "entry", Archetype: ArchetypeEntrance, Width: 8, Pattern: environments.PatternEmpty},
			{ID: "boss", Archetype: ArchetypeBoss, Width: 8, Pattern: environments.PatternEmpty},
		},
		Connectors:    []DungeonConnectorParams{{DoorID: "connector"}},
		AuthoredEdges: []AuthoredEdge{{From: from, To: to, Kind: GeneratedEdgeKindSolid}},
	}))
	require.NoError(t, enc.AddPlayer(PlayerInput{
		PlayerID: "target", EntityID: "target-character", Position: to, SightRange: 1, HP: 7, MaxHP: 7,
	}))

	perception := enc.buildPerception(&MonsterData{Position: from})
	require.Len(t, perception.Enemies, 1)
	require.False(t, perception.Enemies[0].Adjacent,
		"a closed authored boundary must prevent the monster from selecting melee through its crossing")
	require.NotNil(t, perception.TraversalPredicate)
	require.Positive(t, perception.TraversalLimit.MaxSteps)
	path := spatial.NewSimplePathFinder().FindPathWithTraversal(
		from.ToCube(), to.ToCube(), nil, perception.TraversalPredicate, perception.TraversalLimit,
	)
	require.NotEmpty(t, path, "the bounded in-grid search must find a detour")
	require.Greater(t, len(path), 1, "the direct authored crossing is blocked")
	previous := from.ToCube()
	for _, step := range path {
		require.True(t, perception.TraversalPredicate(previous, step))
		previous = step
	}

	blockerFrom := core.HexFromPosition(spatial.Position{X: 4, Y: 1})
	blocker := core.HexFromPosition(spatial.Position{X: 5, Y: 1})
	require.NoError(t, enc.AddObstacle("monster-cell-blocker", "test", blocker, true, true))
	perception = enc.buildPerception(&MonsterData{Position: from})
	require.False(t, perception.TraversalPredicate(blockerFrom.ToCube(), blocker.ToCube()))
	require.Contains(t, perception.BlockedHexes, blocker.ToCube())

	sealedGoal := core.HexFromPosition(spatial.Position{X: 5, Y: 3})
	for index, neighbor := range sealedGoal.ToCube().GetNeighbors() {
		sealedBlocker := core.HexFromCube(neighbor)
		require.NoError(t, enc.AddObstacle(
			core.EntityID(fmt.Sprintf("sealed-cell-%d", index)), "test", sealedBlocker, true, true,
		))
	}
	perception = enc.buildPerception(&MonsterData{Position: from})
	noRoute := spatial.NewSimplePathFinder().FindPathWithTraversal(
		from.ToCube(), sealedGoal.ToCube(), nil, perception.TraversalPredicate, perception.TraversalLimit,
	)
	require.Empty(t, noRoute, "the finite room predicate must not escape a sealed goal through the unbounded hex plane")
}

func TestValidateAuthoredEdgeOverlay_RejectsConnectorDoorCollision(t *testing.T) {
	from := core.Hex{Q: 0, R: 0, S: 0}
	to := core.Hex{Q: 1, R: -1, S: 0}
	err := validateAuthoredEdgeOverlay([]generatedEdgeRecord{{
		edge: GeneratedEdge{From: from, To: to, Kind: GeneratedEdgeKindDoor, DoorID: "connector-door"},
	}}, []AuthoredEdge{{From: from, To: to, Kind: GeneratedEdgeKindSolid}})
	require.Error(t, err)
	require.ErrorContains(t, err, "connector-derived")
}
