// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_keep_test.go is THE KEEP, THREE WAYS (rpg-project#605, use
// case §3): turn the garrison and let it storm the keep, turn it and have it
// hold the castle while the king still breathes, and put the king down. The
// two endings are the file's own; nothing here asks which route was taken to
// reach them.

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// throneCell is the throne-room floor the garrison storms to, as the file's
// `storm-the-keep` table writes it.
var throneCell = castleCell(0, -19)

// turnTheGarrison is the turn: the cook's gossip and Ashford, out of the
// kitchen through the great hall, the gate opened, and the captain
// told what his king did. It returns where every member stood just before
// he heard it.
func turnTheGarrison(t *testing.T) (*kingScene, map[encounter.MemberID]spatial.Position) {
	t.Helper()
	// THE THRONE DOOR IS SHUT, as the file authors it: the march opens it
	// on its way (rpg-project#715).
	s := openTheCastle(t, besideTheCook)
	s.said(kingCook, "gossip", "persuasion", true)
	s.said(kingCook, "ashford", "persuasion", true)
	s.outOfTheKitchen()
	s.throughTheGate(kingCaptain)
	posts := map[encounter.MemberID]spatial.Position{}
	members, err := s.enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		posts[m.ID] = m.Position
	}
	s.said(kingCaptain, "turn", "persuasion", true)

	return s, posts
}

// TestMadKingTurnTheGarrison is turning the garrison: the captain learns the
// truth about Ashford from the hero, the garrison and the loyalists are
// enemies from that moment, the captain storms the keep until he sees a
// loyalist, and the fight that forms has three sides in which the party and
// the garrison both fight the loyalists and never each other.
func TestMadKingTurnTheGarrison(t *testing.T) {
	t.Parallel()
	s, posts := turnTheGarrison(t)

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
		require.Less(t, s.enc.Distance(now, throneCell), s.enc.Distance(posts[kingCaptain], throneCell),
			"he walked from his post toward the throne room")
		require.NotEqual(t, throneCell, now, "and stopped short of it, where a loyalist came into his sight")
	})

	t.Run("the garrison opened the shut throne door on its way", func(t *testing.T) {
		var state encounter.DoorStateKind
		for _, d := range s.enc.Doors() {
			if d.ID == kingThroneDoor {
				state = d.State.Kind()
			}
		}
		require.Equal(t, encounter.DoorOpen, state)
		opener := s.throneDoorOpener()
		require.NotEmpty(t, opener, "a door beat names who opened it")
		t.Logf("the throne door was opened by %s", opener)
		require.Equal(t, garrison, s.factionOf(opener), "a turned guard, not the party")
	})

	t.Run("a gate guard who witnessed the turn marches with him", func(t *testing.T) {
		learned := s.learners("guards-turned")
		var marched bool
		for _, id := range []encounter.MemberID{kingGate1, kingGate2} {
			if !learned[id] {
				continue
			}
			// The march row is the gate patrol's only row that names a
			// cell, so a pick of `toward` at a cell is that row firing.
			if s.pickedTowardACell(id) &&
				s.enc.Distance(s.at(id), throneCell) < s.enc.Distance(posts[id], throneCell) {
				marched = true
			}
		}
		require.True(t, marched, "a guard who heard the captain took the march row toward the throne room")
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
			if s.factionOf(st.attacker) != garrison {
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
// the captain to hold the gate, and walking out of the front door after his
// answer ends the run deposed — with the king alive and in the run. Method-indifference at castle scale: the
// ending waits on a fact, and nothing about the king.
func TestMadKingDeposed(t *testing.T) {
	t.Parallel()
	s, _ := turnTheGarrison(t)
	require.Empty(t, s.ending(), "precondition: turning the garrison ends nothing")

	s.said(kingCaptain, "hold-the-castle", "persuasion", true)
	require.Empty(t, s.ending(), "the castle held ends nothing: the party has not left")
	require.Contains(t, memberIDsOf(t, s.enc), theKing, "the king is still in the run")
	require.False(t, s.standing.down[theKing], "and standing")

	out := s.leaveByTheFrontDoor()
	require.Equal(t, "king-deposed", out.Ending, "walked out with the castle held")
	require.Equal(t, encounter.ResultSuccess, out.Result)
}

// TestMadKingSlainAndDeposedEndsSlain pins the order of the castle's endings
// at the door, not only its bytes: the castle held (`throne-taken`) and the
// king down, both, and the party walks out. `king-slain` is listed first, so
// the run ends slain. Reorder the endings and this route fails.
func TestMadKingSlainAndDeposedEndsSlain(t *testing.T) {
	t.Parallel()
	s, _ := turnTheGarrison(t)
	s.said(kingCaptain, "hold-the-castle", "persuasion", true)
	s.struck(theKing, true)
	require.Empty(t, s.ending(), "neither the castle held nor the king's fall ends the run")
	require.True(t, s.learners("throne-taken")[kingCaptain], "precondition: the castle is held")

	out := s.leaveByTheFrontDoor()
	require.Equal(t, "king-slain", out.Ending, "slain outranks deposed, as the list orders them")
	require.Equal(t, encounter.ResultSuccess, out.Result)
}

// kingPorter is a second player who waits on the front door while the hero
// is in the throne room, so the last one out can leave by it.
const kingPorter = encounter.MemberID("porter")

// TestMadKingSlain is the king put down, and the run won at the door
// (rpg-project#716, ruled 2026-10-29: "walk out the front door to end. king
// taken care of? ... yes? success."). The hero is in the throne room at the
// king's side, the king's court sees him, and the hero's spear drops the king.
// The fall ends nothing. The hero leaves by the lobby, which ends nothing
// either: the porter is still inside. The porter, the last one out, leaves by
// the front door, and the run ends `king-slain`, a success.
func TestMadKingSlain(t *testing.T) {
	t.Parallel()
	s := openTheCastleAs(t, castleHost{
		at:      besideTheKing,
		players: []encounter.MemberInput{{ID: kingPorter, Kind: encounter.KindPlayer, Position: partyStart(t)}},
	})
	require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, theKing), "precondition: the court saw the hero")
	require.Empty(t, s.ending())

	s.struck(theKing, true)
	require.Empty(t, s.ending(), "the king's fall ends nothing: the party has not left")

	out, err := s.enc.Exit(&encounter.ExitInput{Member: hero})
	require.NoError(t, err)
	require.Nil(t, out.Closed, "the hero left by the lobby, and the porter is still inside")

	out, err = s.enc.Exit(&encounter.ExitInput{Member: kingPorter})
	require.NoError(t, err)
	require.NotNil(t, out.Closed, "the last one out, through the front door, ends it")
	require.Equal(t, "king-slain", out.Closed.Ending)
	require.Equal(t, encounter.ResultSuccess, out.Closed.Result)
}

// leaveByTheFrontDoor walks the hero back to the party start and leaves by
// the front door (rpg-project#716), returning what closed. The hero is the
// party's only player, so the hero is the last one out.
func (s *kingScene) leaveByTheFrontDoor() *encounter.Outcome {
	s.t.Helper()
	start := partyStart(s.t)
	door := castleCell(int(start.X), int(start.Y))
	if s.at(hero) != door {
		s.walkTo(door)
		require.Equal(s.t, encounter.ClockWorld, clockKind(s.t, s.enc, hero), "the walk to the door met no fight")
		if s.at(hero) != door {
			_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: door, EndWalk: true})
			require.NoError(s.t, err, "the hero steps onto the front door")
		}
	}
	out, err := s.enc.Exit(&encounter.ExitInput{Member: hero})
	require.NoError(s.t, err)
	require.NotNil(s.t, out.Closed, "the last one out, through the front door, ends it")

	return out.Closed
}

// TestMadKingLeftAlive is the run walked away from: the hero comes in and
// goes straight back out of the front door with the king still breathing,
// and the run ends `king-lives`, a failure.
func TestMadKingLeftAlive(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, partyStart(t))

	out, err := s.enc.Exit(&encounter.ExitInput{Member: hero})
	require.NoError(t, err)
	require.NotNil(t, out.Closed, "the last one out ends it")
	require.Equal(t, "king-lives", out.Closed.Ending)
	require.Equal(t, encounter.ResultFailure, out.Closed.Result)
}

// pickedTowardACell is whether a creature's own story shows it picking a
// `toward` row that names a cell.
func (s *kingScene) pickedTowardACell(id encounter.MemberID) bool {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: id})
	require.NoError(s.t, err)
	for _, beat := range story {
		var body struct{ Beat, Creature, Word, Selector string }
		require.NoError(s.t, json.Unmarshal(beat.Payload, &body))
		if body.Beat == encounter.BeatAnswered && body.Creature == string(id) &&
			body.Word == "toward" && body.Selector == "at" {
			return true
		}
	}

	return false
}

// throneDoorOpener is the actor of the door beat that opened the throne door,
// as the opener's own story holds it, or empty when no member's story does.
func (s *kingScene) throneDoorOpener() encounter.MemberID {
	s.t.Helper()
	members, err := s.enc.Members()
	require.NoError(s.t, err)
	for _, m := range members {
		story, err := s.enc.Story(&encounter.StoryInput{Audience: m.ID})
		require.NoError(s.t, err)
		for _, beat := range story {
			var body struct{ Beat, Door, State, Actor string }
			require.NoError(s.t, json.Unmarshal(beat.Payload, &body))
			if body.Beat == encounter.BeatDoor && body.Door == string(kingThroneDoor) &&
				body.State == string(encounter.DoorOpen) && body.Actor == string(m.ID) {
				return m.ID
			}
		}
	}

	return ""
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

// TestMadKingKeepFacesAlongItsPatrol is the castle's facing line
// (rpg-project#582, design R13). The castle authors no facings, so keep-1
// starts at the unstated 180; one world round later it faces along its patrol.
//
// A round at thirty feet is six cells, and keep-1's loop on the throne room's
// west side spends them across legs: one cell to (7,-19), two north to
// (7,-21), two west to (5,-21), and the first of the leg back to (4,-19),
// which is axial +r — a bearing of 150 on pointy-top hexes. The facing is that
// last step's, not any authored word and not the 180 it started with.
func TestMadKingKeepFacesAlongItsPatrol(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, partyStart(t))
	require.Equal(t, encounter.UnstatedFacing, facingOf(t, s.enc, kingKeep1), "the castle authors no facing")

	start := s.at(hero)
	beside := spatial.Position{X: start.X + 1, Y: start.Y}
	for i := range 6 {
		to := beside
		if i%2 == 1 {
			to = start
		}
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to})
		require.NoError(t, err)
	}

	require.Equal(t, spatial.Position{X: 5, Y: -20}, s.at(kingKeep1), "one round of patrol")
	require.InDelta(t, 150.0, facingOf(t, s.enc, kingKeep1), 1e-9, "keep-1 faces along the leg it is walking")
}
