// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// move_held_turnclock_test.go is a held opening on the turn clock
// (rpg-project#852, R5; S1's TestABumpOnTheTurnClockPaysNoMovement shape): a
// skeleton posted on the hall's open edge door holds its crossing while it
// stands on its post. Alice walks one cell, then through the door: the cell
// she walked is paid, the crossing the post refused is not a step and costs
// nothing, and the walk answers `stopped` with the holder's name.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func TestAHeldCrossingOnTheTurnClockPaysNoMovement(t *testing.T) {
	ctx := context.Background()
	characters := newFakeCharacters(armedFighter("alice"))
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: characters,
	})
	require.NoError(t, err)

	// first is the cell beside alice; through is past the door from it. The
	// skeleton stands two cells off, on its post, and holds the door.
	first, through, post := hexCell(2, 1), hexCell(2, 0), hexCell(3, 1)
	ref, err := core.ParseString("dnd5e:machines:posted")
	require.NoError(t, err)
	skeleton := monsterAt("skel-1", refs.Monsters.Skeleton().String(), 3, 1)
	skeleton.Machine = &machine.Recipe{Ref: *ref, Initial: "posted", States: map[string]machine.State{"posted": {}}}
	skeleton.Posts = map[string]encounter.Post{"posted": {Door: "gate", Cells: []spatial.Position{post}}}

	launched := launchScene(t, mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Doors: []encounter.DoorInput{{
				ID: "gate", Edges: []encounter.DoorEdge{{From: first, To: through}}, State: encounter.DoorIsOpen(),
			}},
		},
		Party:    []sceneSeat{seatAt("alice", 1, 1)},
		Monsters: []dungeonspec.MonsterPlacement{skeleton},
	})
	require.NotEmpty(t, launched.Formed, "precondition: arriving in plain sight starts a fight")
	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active, "precondition: alice's turn")

	enc, err := mgr.LoadWorldForTest(ctx, "sess")
	require.NoError(t, err)
	skel := memberOf(t, enc, "skel-1")
	require.Equal(t, post, skel.Position, "precondition: the skeleton stands on its post")

	remaining := func() int {
		afford, aerr := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
		require.NoError(t, aerr)
		for _, d := range afford.Declarations {
			if d.Verb == session.VerbMove {
				require.NotNil(t, d.Remaining)
				return *d.Remaining
			}
		}
		require.FailNow(t, "no VerbMove declaration")
		return 0
	}
	before := remaining()

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", DeclarationID: currentMoveID(t, mgr, "sess", "alice"),
		Path: []spatial.Position{first, through},
	})
	require.NoError(t, err, "a held crossing is a stop, not a refusal")
	require.Equal(t, session.MovementStopped, out.Status)
	require.Equal(t, skel.Name+" bars the way", out.StopReason, "the stop names the holder she sees")
	require.NotEmpty(t, out.Steps)
	require.Equal(t, first, out.Steps[len(out.Steps)-1].Position, "the step before the door stays")
	where, err := mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, first, where.Position)
	require.Equal(t, before-5, characters.byID["alice"].ActionEconomy.MovementRemaining,
		"the walked cell is paid, the held crossing is not")
}
