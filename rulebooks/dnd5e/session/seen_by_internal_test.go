// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestASeenRefusalNamesNobodyThePlayerCannotSee (the-world-runs R11,
// testimony per observer): the composition's SeenError lists every watcher
// that sees the mover, and the session names only the ones the mover's own
// sightings hold live. A watcher the mover cannot see is not named, and when
// none can be, the refusal says only that the mover is seen.
//
// WHY THIS IS AN INTERNAL TEST. Today every sightline is mutual and a
// character sees as far as any watcher (combat.DefaultSightFeet against a
// goblin's 60-foot darkvision), so no scene can yet make a watcher see a
// player the player cannot see back. The filter is driven here with a
// watcher list naming somebody behind a wall, which is exactly what the
// composition would hand over the day sight stops being mutual.
func TestASeenRefusalNamesNobodyThePlayerCannotSee(t *testing.T) {
	ctx := context.Background()
	mgr, err := NewManager(&Config{Record: DiscardRecord{}, Acts: DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: &scriptedDice{}, TurnDriver: Pass{},
		Sessions:   &strikeSessions{byID: map[string]*SessionData{}},
		Encounters: &strikeEncounters{byID: map[string]*encounter.EncounterData{}},
		Characters: &strikeCharacters{byID: map[string]*character.Data{
			"alice": strikeFixtureFighter("alice"), "bob": strikeFixtureFighter("bob"),
		}},
	})
	require.NoError(t, err)
	// Two rooms the void walls apart: alice cannot see bob.
	launchScene(t, mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 3, 3), rectRegion("vault", 5, 0, 3, 3)}},
		Party: []sceneSeat{seatAt("alice", 1, 1), seatAt("bob", 6, 1)},
	})
	data, err := mgr.loadSessionData(ctx, "sess")
	require.NoError(t, err)
	enc, err := mgr.loadWorld(ctx, data)
	require.NoError(t, err)

	got := seenBy(enc, "alice", []encounter.MemberID{"bob"})
	require.NotNil(t, got.Watchers, "empty, never nil")
	require.Empty(t, got.Watchers, "a watcher alice cannot see is not named")
	require.Equal(t, "member is seen", got.Error(), "no name and no count")
	require.ErrorIs(t, got, ErrSeen)
}
