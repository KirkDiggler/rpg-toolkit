// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func atlasOfManager(t *testing.T) *session.Manager {
	t.Helper()
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")), Events: session.DiscardEvents{},
	})
	require.NoError(t, err)
	return mgr
}

// TestAtlasOfMatchesTheLaunchedAtlas pins "one world builder, two readers":
// for the same compiled dungeon, the preview is the author's whole truth and
// the map of the launched run is that same atlas restricted to the cells the
// member has learned (rpg-project#586 R1). The dungeon conceals nothing, so
// the only thing between the two is discovery by cell — and the vault behind
// the gate is known only as far as alice's sight reached through it.
func TestAtlasOfMatchesTheLaunchedAtlas(t *testing.T) {
	ctx := context.Background()
	mgr := atlasOfManager(t)
	in := sceneInput(hexWorld())
	in.DungeonKey = "hex-world"

	preview, err := mgr.AtlasOf(ctx, &session.AtlasOfInput{Dungeon: in.Dungeon, DungeonKey: "hex-world"})
	require.NoError(t, err)

	_, err = mgr.Launch(ctx, in)
	require.NoError(t, err)
	live, err := mgr.Atlas(ctx, &session.AtlasInput{Session: in.Session, Member: "alice"})
	require.NoError(t, err)

	requireKnownSliceOf(t, preview, live)
	require.Less(t, len(live.Cells), len(preview.Cells), "the live map withholds the cells alice has not seen")
	require.Equal(t, "hex-world", preview.DungeonKey)
}

// requireKnownSliceOf asserts live is whole restricted to live's own known
// cells: the frame and key are the same, every known cell, region, prop and
// boundary is the author's own record, and every author boundary touching a
// known cell survives the projection.
func requireKnownSliceOf(t *testing.T, whole, live *session.Atlas) {
	t.Helper()
	require.Equal(t, whole.Grid, live.Grid)
	require.Equal(t, whole.Layout, live.Layout)
	require.Equal(t, whole.DungeonKey, live.DungeonKey)
	require.NotEmpty(t, live.Cells)
	require.Subset(t, whole.Cells, live.Cells, "a known cell is an authored cell")
	known := make(map[spatial.Position]bool, len(live.Cells))
	for _, cell := range live.Cells {
		known[cell] = true
	}
	authored := make(map[string]session.AtlasRegion, len(whole.Regions))
	for _, region := range whole.Regions {
		authored[region.ID] = region
	}
	for _, region := range live.Regions {
		full, ok := authored[region.ID]
		require.True(t, ok, "region %q is authored", region.ID)
		require.Subset(t, full.Cells, region.Cells, "region %q carries only its own cells", region.ID)
		full.Cells = region.Cells
		require.Equal(t, full, region, "region %q is the author's record but for its cells", region.ID)
	}
	var props []session.AtlasProp
	for _, prop := range whole.Props {
		if known[prop.At] {
			props = append(props, prop)
		}
	}
	require.ElementsMatch(t, props, live.Props, "exactly the props on known cells")
	var boundaries []session.AtlasBoundary
	for _, boundary := range whole.Boundaries {
		if known[boundary.From] || known[boundary.To] {
			boundaries = append(boundaries, boundary)
		}
	}
	require.NotEmpty(t, boundaries)
	require.ElementsMatch(t, boundaries, live.Boundaries, "exactly the walls touching a known cell")
}

func TestAtlasOfRefusesWithoutADungeon(t *testing.T) {
	mgr := atlasOfManager(t)

	_, err := mgr.AtlasOf(context.Background(), nil)
	require.ErrorIs(t, err, session.ErrNilInput)
	_, err = mgr.AtlasOf(context.Background(), &session.AtlasOfInput{})
	require.ErrorIs(t, err, session.ErrInvalidWorld)
}

func TestAtlasOfRefusesADungeonNamingTwoBosses(t *testing.T) {
	mgr := atlasOfManager(t)
	compiled := compileCamp(t, campSource(t))
	compiled.Monsters = append([]dungeonspec.MonsterPlacement(nil), compiled.Monsters...)
	for i := range compiled.Monsters[:2] {
		compiled.Monsters[i].Boss = true
	}

	_, err := mgr.AtlasOf(context.Background(), &session.AtlasOfInput{Dungeon: &compiled})
	require.ErrorIs(t, err, session.ErrInvalidWorld)
}

// TestAtlasOfDeclaresTheEndingsALaunchDoes pins that the preview is built by
// the launch's own builder: a dungeon whose own ending key collides with one a
// launch declares is refused by both, not only by the run.
func TestAtlasOfDeclaresTheEndingsALaunchDoes(t *testing.T) {
	mgr := atlasOfManager(t)
	compiled := compileCamp(t, campSource(t))
	compiled.Endings = append(append([]encounter.EndingInput(nil), compiled.Endings...),
		encounter.EndingInput{Key: session.EndingWithdrawn, Trigger: encounter.TriggerExternal{}})

	_, err := mgr.AtlasOf(context.Background(), &session.AtlasOfInput{Dungeon: &compiled})
	require.ErrorIs(t, err, session.ErrInvalidWorld)
}
