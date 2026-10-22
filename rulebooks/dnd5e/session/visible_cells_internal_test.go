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

// TestViewVisibleCellsIsTheEncounterAnswerExactly: "the session projects,
// never computes" (rpg-project#586). For each seat, View carries
// the composition's own VisibleCellsFor answer for that member, unchanged —
// same cells, same order — on the very world the read loaded.
func TestViewVisibleCellsIsTheEncounterAnswerExactly(t *testing.T) {
	ctx := context.Background()
	mgr, err := NewManager(&Config{Record: DiscardRecord{}, Acts: DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{},
		Dice: &scriptedDice{}, TurnDriver: Pass{},
		Sessions:   &strikeSessions{byID: map[string]*SessionData{}},
		Encounters: &strikeEncounters{byID: map[string]*encounter.EncounterData{}},
		Characters: &strikeCharacters{byID: map[string]*character.Data{
			"alice": strikeFixtureFighter("alice"), "bob": strikeFixtureFighter("bob"),
		}},
	})
	require.NoError(t, err)
	// Two rooms the void walls apart, one seat in each.
	launchScene(t, mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 3, 3), rectRegion("vault", 5, 0, 3, 3)}},
		Party: []sceneSeat{seatAt("alice", 1, 1), seatAt("bob", 6, 1)},
	})

	data, err := mgr.loadSessionData(ctx, "sess")
	require.NoError(t, err)
	enc, err := mgr.loadWorld(ctx, data)
	require.NoError(t, err)
	answers := map[string][]any{}
	for _, member := range []string{"alice", "bob"} {
		want, err := enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: encounter.MemberID(member)})
		require.NoError(t, err)
		require.NotEmpty(t, want.Cells)

		view, err := mgr.View(ctx, &ViewInput{Session: "sess", Member: member})
		require.NoError(t, err)
		require.Equal(t, want.Cells, view.VisibleCells, "%s's view is the encounter's answer", member)
		answers[member] = append(answers[member], view.VisibleCells)
	}
	require.NotEqual(t, answers["alice"], answers["bob"], "each seat gets its own answer, not a union")

	_, err = enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: "stranger"})
	require.ErrorIs(t, err, encounter.ErrNotMember)
	_, err = mgr.View(ctx, &ViewInput{Session: "sess", Member: "stranger"})
	require.ErrorIs(t, err, ErrNoMember, "a stranger is refused, never handed someone else's cells")
}
