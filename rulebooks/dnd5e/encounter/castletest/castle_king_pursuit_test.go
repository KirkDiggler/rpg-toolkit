// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_pursuit_test.go is PURSUIT AT THE GATE (rpg-project#668,
// ideas/smarter-monsters): a gate guard who loses sight of the hero walks to
// where he last saw him, searches six cells on the way the hero was heading,
// gives up, and walks back to his loop. A guard who never saw the hero keeps
// his loop.
//
// THE FIXTURE IS CHOSEN TO PROVE THE MECHANIC. gate-2 and the captain are
// down, so gate-1 is the only one hunting; the hero runs at sixty feet and the
// night is thirty feet deep, so one turn's run puts him out of every cell the
// guard stands on; and the hero is first seen stepping WEST on the open
// ground south of the gatehouse, so the heading the guard holds points down
// six cells of open floor.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The pursuit's cells, dungeon-absolute.
var (
	// pursuitStart is where the hero stands when gate-1 is turned: on the open
	// ground south of the gatehouse, in the guard's sight through the
	// doorway. An offset cell, for [encounter.MemberInput.Position].
	pursuitStart = spatial.Position{X: 2, Y: 14}

	// lastSeen is the one cell west of the start: the last the guard sees
	// the hero on, stepping west, before the gatehouse's south wall takes
	// him out of sight. The "corner" of the operator's checklist.
	lastSeen = castleCell(1, 14)

	// searchedTo is six cells west of lastSeen along the open ground: where
	// a six-cell search along the hero's westward heading ends.
	searchedTo = castleCell(-5, 14)

	// pursuitHide is the hero's hiding place up the west side of the
	// gatehouse, behind its west wall from every cell the guard stands on.
	pursuitHide = castleCell(-7, 7)
)

// openThePursuit opens the castle for the pursuit routes with the hero at an
// offset cell facing west, and only gate-1 on its feet among the gate.
func openThePursuit(t *testing.T, at spatial.Position) *kingScene {
	t.Helper()
	west := 270.0
	s := openTheCastleAs(t, castleHost{
		at: at, heroFacing: &west, sightFt: 30,
		speedFt: map[encounter.MemberID]int{hero: 60},
	})
	s.standing.down[kingGate2] = true
	s.standing.down[kingCaptain] = true

	return s
}

// sightState is the observer's sight testimony of the hero: its state and
// cell, or ok=false when it holds none.
func (s *kingScene) sightState(observer encounter.MemberID) (encounter.SightTestimony, bool) {
	s.t.Helper()
	h := s.holding(observer, hero)
	if h == nil {
		return encounter.SightTestimony{}, false
	}
	testimony, ok := encounter.DecodeSightTestimony(h.Payload)
	require.True(s.t, ok, "%s's sighting of the hero decodes", observer)

	return testimony, true
}

// runLeg steps the hero through cells on the turn clock, within one of its
// turns.
func (s *kingScene) runLeg(cells []spatial.Position) {
	s.t.Helper()
	require.LessOrEqual(s.t, len(cells), encounter.CellsFromFeet(60), "a leg is one turn's run")
	for _, cell := range cells {
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell})
		require.NoError(s.t, err)
		// PER VERB, as a host reloads: the pursuit's memory must survive
		// the blob between every step (rpg-toolkit#2114's AnswerData).
		s.reload()
	}
	require.Equal(s.t, encounter.ClockTurn, clockKind(s.t, s.enc, hero), "precondition: the fight is still on")
}

// TestMadKingGateGuardPursuesTheRememberedHero is #668's checklist on the
// castle: drawn round the corner, the guard comes to the corner — not to the
// hero's cell and not to his post — walks about six cells on with the hero
// still hidden, and returns to his loop; a guard that never saw the hero stays
// on his loop.
func TestMadKingGateGuardPursuesTheRememberedHero(t *testing.T) {
	t.Parallel()

	t.Run("to the corner, six on, and back to the loop", func(t *testing.T) {
		t.Parallel()
		s := openThePursuit(t, pursuitStart)
		require.True(t, s.seesLive(kingGate1, hero), "precondition: gate-1 sees the hero at the start")
		s.struck(kingGate1, false)
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, hero), "the hero's spear turns gate-1 and the fight forms")
		posts := []spatial.Position{westDoorPost, westBeatPost}

		// The hero's run: one step west in the guard's sight, then round the
		// gatehouse's south-west corner and up its west side — the rest of
		// one turn's run, and the last cell of it on the next.
		route, err := s.enc.Route(encounter.RouteInput{
			Mover: hero, Policy: encounter.MoveToward, Anchor: castleCell(-6, 6), Budget: 40,
		})
		require.NoError(t, err)
		require.Equal(t, lastSeen, route.Path[0], "precondition: the run starts with the step west")
		require.Equal(t, pursuitHide, route.Path[len(route.Path)-1], "precondition: and ends in hiding")
		run := encounter.CellsFromFeet(60)
		s.runLeg(route.Path[:1])
		require.True(t, s.seesLive(kingGate1, hero), "precondition: the guard sees the hero step west")
		s.runLeg(route.Path[1:run])
		require.False(t, s.seesLive(kingGate1, hero), "precondition: the hero is out of sight")

		// Round one: the guard comes to the corner and finds nobody.
		s.endTurn()
		require.Equal(t, lastSeen, s.at(kingGate1), "the guard comes to the corner")
		require.NotEqual(t, s.at(hero), s.at(kingGate1), "not to the hero's cell")
		require.NotContains(t, posts, s.at(kingGate1), "not to his post")
		seen, held := s.sightState(kingGate1)
		require.True(t, held)
		require.Equal(t, encounter.LocationLost, seen.State, "he remembers the hero gone from there")
		require.Equal(t, lastSeen, seen.Position, "and still where he last saw him")
		s.runLeg(route.Path[run:])
		require.False(t, s.seesLive(kingGate1, hero), "precondition: the hero is in hiding")

		// Round two: six cells on, the way the hero was heading, with the
		// hero still hidden — then he gives up.
		s.endTurn()
		require.False(t, s.seesLive(kingGate1, hero), "the hero is still hidden")
		require.Equal(t, searchedTo, s.at(kingGate1), "he searches on to the end of the hero's heading")
		require.Equal(t, 6.0, s.enc.Distance(lastSeen, s.at(kingGate1)), "six cells on from the corner")
		seen, held = s.sightState(kingGate1)
		require.True(t, held, "he still knows the hero exists")
		require.Equal(t, encounter.LocationUnknown, seen.State, "and no longer where")

		// Then his loop: he walks back and stands his posts.
		visited := map[spatial.Position]bool{}
		for round := 0; round < 8 && len(visited) < len(posts); round++ {
			s.endTurn()
			require.False(t, s.seesLive(kingGate1, hero), "the hero stays hidden")
			for _, post := range posts {
				if s.at(kingGate1) == post {
					visited[post] = true
				}
			}
		}
		require.Len(t, visited, len(posts), "back on his loop, he stands both his posts")
	})

	t.Run("a guard that never saw the hero stays on his loop", func(t *testing.T) {
		t.Parallel()
		s := openThePursuit(t, spatial.Position{X: -7, Y: 7})
		require.Equal(t, pursuitHide, s.at(hero), "precondition: the hero starts in hiding")
		_, held := s.sightState(kingGate1)
		require.False(t, held, "precondition: gate-1 has never seen the hero")
		s.struck(kingGate1, false)
		require.True(t, s.hostile(hero, kingGate1), "precondition: the spear turned gate-1")

		posts := []spatial.Position{westDoorPost, westBeatPost}
		visited := map[spatial.Position]bool{}
		for round := 0; round < 8 && len(visited) < len(posts); round++ {
			start := s.enc.ToData().Clock.HighWater
			if clockKind(t, s.enc, hero) == encounter.ClockTurn {
				s.endTurn()
			} else {
				require.True(t, s.pace(pursuitHide, castleCell(-7, 8), 12,
					func() bool { return s.enc.ToData().Clock.HighWater > start }), "a round passes")
			}
			_, held := s.sightState(kingGate1)
			require.False(t, held, "round %d: still never seen", round)
			require.Equal(t, labelOf(t, westBeatPost), labelOf(t, s.at(kingGate1)),
				"round %d: he stays in the gatehouse", round)
			for _, post := range posts {
				if s.at(kingGate1) == post {
					visited[post] = true
				}
			}
		}
		require.Len(t, visited, len(posts), "he walks his loop, both posts")
	})
}
