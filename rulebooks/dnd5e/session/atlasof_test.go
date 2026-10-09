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
)

func atlasOfManager(t *testing.T) *session.Manager {
	t.Helper()
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(armedFighter("alice"), armedFighter("bob")), Events: session.DiscardEvents{},
	})
	require.NoError(t, err)
	return mgr
}

// TestAtlasOfMatchesTheLaunchedAtlas pins "one world builder, two readers":
// for the same compiled dungeon, the preview and the map of the launched run
// answer equal atlases, dungeon key included. The dungeon conceals nothing,
// because the live map withholds what a member has not had revealed and the
// preview is the author's whole truth.
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

	require.Equal(t, live, preview)
	require.Equal(t, "hex-world", preview.DungeonKey)
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
