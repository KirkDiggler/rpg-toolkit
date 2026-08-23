// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// seen_test.go is #1157's end-to-end case: after a walker crosses a doorway,
// View reports the monster on the far side with Seen.Position equal to where
// that monster actually stands — the whole projection, exercised through the
// real SDK verbs rather than a synthetic intel.Holding.
//
// authoredTomb() (example_session_test.go) is this package's usual reference
// world, but it is one bare room: no doorway, no monster. Reusing it here
// would mean adding wall/doorway geometry to a fixture every other test in
// this file depends on staying simple, or duplicating the reference tomb's
// own wall math (canvas_test.go's tombField, in the encounter package) at
// this seam — a second hand-written copy of exactly the coordinate class of
// bug ADR-0040/#1140 was about. So this builds its own small two-room world,
// the same way onemap_test.go's offsetWorld and read_test.go's hexWorld do:
// a seam wall with a doorway-row gap (squareSeamWalls, mirroring
// hexSeamWalls in testprops_test.go and squareSeamWall in the encounter
// package's own tests), so sight is genuinely gated by the doorway rather
// than open across the whole shared edge.

type SeenTestSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestSeenTestSuite(t *testing.T) {
	suite.Run(t, new(SeenTestSuite))
}

func (s *SeenTestSuite) SetupTest() {
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// squareSeamWalls is entrance's east wall, open only at doorRow — the square
// analogue of hexSeamWalls (testprops_test.go), and the same construction as
// the encounter package's own squareSeamWall/tombSeamWall test helpers.
func squareSeamWalls(atX, rows, doorRow int) []spatial.Boundary {
	out := make([]spatial.Boundary, 0, rows*3)
	for y := 0; y < rows; y++ {
		for _, dy := range []int{-1, 0, 1} {
			to := y + dy
			if to < 0 || to >= rows {
				continue
			}
			if dy == 0 && y == doorRow {
				continue // the doorway itself
			}
			out = append(out, spatial.Boundary{
				From:              spatial.Position{X: float64(atX), Y: float64(y)},
				To:                spatial.Position{X: float64(atX + 1), Y: float64(to)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			})
		}
	}
	return out
}

// skeletonBehindADoor is a two-room world: entrance (0,0) and hall (6,0),
// each 6x6, joined by one doorway on row 2 with a solid wall everywhere else
// along the shared edge. skeleton-1 stands well inside hall at local (3,3) —
// absolute (9,3) — where nothing but the doorway can put it in sight.
func skeletonBehindADoor(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{},
		Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: "entrance", Width: 6, Height: 6, Boundaries: squareSeamWalls(5, 6, 2)},
				{ID: "hall", Width: 6, Height: 6, Origin: spatial.Position{X: 6, Y: 0}},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "door1", From: "entrance", To: "hall",
				FromPosition: spatial.Position{X: 5, Y: 2},
				ToPosition:   spatial.Position{X: 0, Y: 2},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "fighter", Kind: encounter.KindPlayer, Room: "entrance", Position: spatial.Position{X: 5, Y: 0}},
			{ID: "skeleton-1", Kind: encounter.KindMonster, Room: "hall", Position: spatial.Position{X: 3, Y: 3}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	if err != nil {
		t.Fatalf("building skeletonBehindADoor: %v", err)
	}
	data := enc.ToData()
	return &data
}

// TestSeenIsPopulatedAfterCrossingTheDoorway is #1157's headline case: the
// fighter cannot see skeleton-1 from behind the wall, walks through the one
// doorway, and View then reports skeleton-1 with a Seen.Position equal to
// where the skeleton actually stands — read independently via Where, not
// the local literal used to place it, so the assertion cannot pass by both
// sides sharing the same typo.
func (s *SeenTestSuite) TestSeenIsPopulatedAfterCrossingTheDoorway() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "skeleton-behind-a-door", World: skeletonBehindADoor(s.T()),
	})
	s.Require().NoError(err)

	// Before: the wall genuinely blocks it. Asserted first so a fixture that
	// accidentally puts the skeleton in the open cannot make the "after"
	// assertion trivially true.
	before, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	for _, sight := range before {
		s.NotEqual("skeleton-1", sight.Subject, "the wall must actually block sight before the walk")
	}

	// The fighter walks down to the doorway row and through it. The walk may
	// stop short of the full requested path: reaching the doorway's own gap
	// cell (5,2) already opens sight to skeleton-1 across it, and a sighting
	// between a player and a monster starts a fight, which is news the walk
	// reports rather than something it walks through (session/doc.go — the
	// composition detects contact wherever sight changes and stops the walker
	// there). That is itself part of what this test proves: sight opened
	// exactly because the walk reached the doorway, not before.
	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter",
		Path: []spatial.Position{{X: 5, Y: 1}, {X: 5, Y: 2}, {X: 6, Y: 2}},
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(out.Steps, "the fighter must have moved at all")
	s.Require().NotNil(out.Formed, "seeing the skeleton must have started the fight")

	where, err := s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "skeleton-1"})
	s.Require().NoError(err)

	after, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)

	var skeleton *session.Sighting
	for i := range after {
		if after[i].Subject == "skeleton-1" {
			skeleton = &after[i]
		}
	}
	s.Require().NotNil(skeleton, "the fighter must see the skeleton once through the doorway")
	s.Require().NotNil(skeleton.Seen, "a sight-channel sighting must carry Seen")
	s.Equal(where.Position, skeleton.Seen.Position,
		"Seen.Position must equal the skeleton's own reported placement, read independently via Where")
}

// TestDiscoveredAlsoCarriesSeen pins the other producer of Report: MoveOutput
// .Discovered's FirstContact list, which the walk above already populates the
// moment the skeleton first comes into view. Discovery is proportional (S6):
// this asserts against out.Discovered directly rather than repeating the walk.
func (s *SeenTestSuite) TestDiscoveredAlsoCarriesSeen() {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "skeleton-behind-a-door", World: skeletonBehindADoor(s.T()),
	})
	s.Require().NoError(err)

	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "fighter",
		Path: []spatial.Position{{X: 5, Y: 1}, {X: 5, Y: 2}, {X: 6, Y: 2}},
	})
	s.Require().NoError(err)

	where, err := s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "skeleton-1"})
	s.Require().NoError(err)

	discovery, ok := out.Discovered["fighter"]
	s.Require().True(ok, "the fighter's own perception must have changed on this walk")

	var report *session.Report
	for i := range discovery.FirstContact {
		if discovery.FirstContact[i].Subject == "skeleton-1" {
			report = &discovery.FirstContact[i]
		}
	}
	s.Require().NotNil(report, "skeleton-1 must be first contact — the fighter never held it before")
	s.Require().NotNil(report.Seen)
	s.Equal(where.Position, report.Seen.Position)
}
