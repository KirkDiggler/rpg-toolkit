// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_king_keep_test.go is THE KEEP, THREE WAYS (rpg-project#605, use
// case §3): turn the garrison and let it storm the keep, turn it and have it
// hold the castle while the king still breathes, and put the king down. The
// two endings are the file's own; nothing here asks which route was taken to
// reach them.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// throneCell is the throne-room floor the garrison storms to, as the file's
// `storm-the-keep` table writes it.
var throneCell = castleCell(0, -19)

// turnTheGarrison is the turn: the cook's gossip and Ashford, the hall door
// opened on the way back across the yard, the gate opened, and the captain
// told what his king did. It returns where the captain stood when he heard it.
func turnTheGarrison(t *testing.T) (*kingScene, spatial.Position) {
	t.Helper()
	s := openTheCastle(t, besideTheCook)
	s.said(kingCook, "gossip", "persuasion", true)
	s.said(kingCook, "ashford", "persuasion", true)
	s.walkTo(castleCell(0, -6))
	s.open(kingHallDoor)
	s.throughTheGate(kingCaptain)
	post := s.at(kingCaptain)
	s.said(kingCaptain, "turn", "persuasion", true)

	return s, post
}

// TestMadKingTurnTheGarrison is turning the garrison: the captain learns the
// truth about Ashford from the hero, the garrison and the loyalists are
// enemies from that moment, the captain storms the keep until he sees a
// loyalist, and the fight that forms has three sides in which the party and
// the garrison both fight the loyalists and never each other.
func TestMadKingTurnTheGarrison(t *testing.T) {
	t.Parallel()
	s, post := turnTheGarrison(t)

	t.Run("the captain, the garrison's mind, learned the guards have turned", func(t *testing.T) {
		learned := s.learners("guards-turned")
		require.True(t, learned[kingCaptain])
		require.True(t, learned[hero], "and the hero who told him heard his answer")
	})

	t.Run("the garrison and the loyalists are enemies, and the party is neither the garrison's", func(t *testing.T) {
		require.Equal(t, encounter.StanceHostile, s.stance(garrison, loyalists))
		require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty))
	})

	t.Run("the captain storms the keep and stops where he sees a loyalist", func(t *testing.T) {
		s.followIntoTheKeep(kingCaptain)
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, kingCaptain), "the captain is fighting")
		now := s.at(kingCaptain)
		require.Less(t, s.enc.Distance(now, throneCell), s.enc.Distance(post, throneCell),
			"he walked from his post toward the throne room")
		require.NotEqual(t, throneCell, now, "and stopped short of it, where a loyalist came into his sight")
	})

	t.Run("one fight, three sides: the party and the garrison against the loyalists", func(t *testing.T) {
		on, err := s.enc.ClockOf(&encounter.ClockOfInput{Member: hero})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockTurn, on.Kind)
		require.Contains(t, on.Order, kingCaptain, "the captain is in the party's fight")
		require.True(t, s.hostile(hero, theKing))
		require.False(t, s.hostile(hero, kingCaptain), "the party and the garrison are not enemies")

		for i := 0; i < 3; i++ {
			s.endTurn()
		}
		var garrisonSwung bool
		for _, st := range *s.strikes {
			if st.attacker != kingCaptain && st.attacker != kingHall1 {
				continue
			}
			require.NotEqual(t, hero, st.target, "the garrison never swings at the party")
			require.True(t, s.hostile(st.attacker, st.target), "%s swung at an enemy", st.attacker)
			garrisonSwung = true
		}
		require.True(t, garrisonSwung, "the garrison fought the loyalists")
	})
}

// TestMadKingDeposed is the castle held: the garrison turned, the hero asks
// the captain to hold the gate, and his answer ends the run deposed — with
// the king alive and in the run. Method-indifference at castle scale: the
// ending waits on a fact, and nothing about the king.
func TestMadKingDeposed(t *testing.T) {
	t.Parallel()
	s, _ := turnTheGarrison(t)
	require.Empty(t, s.ending(), "precondition: turning the garrison ends nothing")

	s.said(kingCaptain, "hold-the-castle", "persuasion", true)

	require.Equal(t, "king-deposed", s.ending())
	require.Contains(t, memberIDsOf(t, s.enc), theKing, "the king is still in the run")
	require.False(t, s.standing.down[theKing], "and standing")
}

// TestMadKingSlain is the king put down: the hero is in the throne room at
// the king's side when the run opens, the king's court sees him, and the
// hero's spear drops the king. The ending is the file's `{ down: king }`.
func TestMadKingSlain(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, besideTheKing)
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, theKing), "precondition: the court saw the hero")
	require.Empty(t, s.ending())

	s.struck(theKing, true)

	require.Equal(t, "king-slain", s.ending())
}

// followIntoTheKeep has the hero follow a member at a walk, a few cells
// behind, until a fight takes the hero: the first cells of the pathfinder's
// way to them while they are farther than that, and a cell out and back
// while they are not. Every cell is world time, which is what lets the
// followed member walk on, and keeping back lets the leader, not the hero,
// be the first through a door.
func (s *kingScene) followIntoTheKeep(leader encounter.MemberID) {
	s.t.Helper()
	const behind = 3
	for i := 0; i < 120 && clockKind(s.t, s.enc, hero) == encounter.ClockWorld; i++ {
		var walk []spatial.Position
		if s.enc.Distance(s.at(hero), s.at(leader)) > behind {
			walk = s.routeFor(s.at(leader))
		} else {
			back, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveAway, Anchor: s.at(leader), Budget: 1})
			require.NoError(s.t, err)
			require.NotEmpty(s.t, back.Path, "the hero has room to mark time (%s)", back.StoppedBy)
			walk = []spatial.Position{back.Path[0], s.at(hero)}
		}
		require.NotEmpty(s.t, walk, "the hero can keep up")
		for j, cell := range walk {
			// A walk ends on the first cell past `behind` the hero may stop
			// on: a doorway is passed through, never stood in.
			end := j >= behind-1 || j == len(walk)-1
			_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: end})
			if end && j < len(walk)-1 && errors.Is(err, encounter.ErrBadPlacement) {
				_, err = s.enc.Step(&encounter.StepInput{Member: hero, To: cell})
				end = false
			}
			require.NoError(s.t, err)
			if clockKind(s.t, s.enc, hero) == encounter.ClockTurn || end {
				break
			}
		}
	}
}

// routeFor is the pathfinder's whole way for the hero toward a cell.
func (s *kingScene) routeFor(to spatial.Position) []spatial.Position {
	s.t.Helper()
	route, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveToward, Anchor: to, Budget: 200})
	require.NoError(s.t, err)

	return route.Path
}
