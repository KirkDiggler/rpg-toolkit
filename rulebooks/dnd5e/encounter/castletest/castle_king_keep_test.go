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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// throneCell is the throne-room floor the turned gate guards march to, as
// the file's gate patrols write it.
var throneCell = castleCell(0, -19)

// turnTheGarrison is the turn: the cook's gossip and Ashford, out of the
// kitchen through the great hall, the gate opened, and the captain
// told what his king did. It returns where every member stood just before
// he heard it.
func turnTheGarrison(t *testing.T) (*kingScene, map[encounter.MemberID]spatial.Position) {
	t.Helper()
	// THE THRONE DOOR STARTS OPEN HERE. The file shuts it, and a driven walk
	// routes only through open doors and no creature opens one
	// (rpg-toolkit#2069). With it shut, the turned gate guards stop on the
	// door's hall side, and the captain stalls in the courtyard against the
	// hall wall at offset (-6,-6), 11 cells short, with the hall door open.
	// Until that lands, this route stands the door open, as the party would
	// before the march it means to see.
	s := openTheCastleWithDoors(t, besideTheCook, map[encounter.DoorID]encounter.DoorState{kingThroneDoor: encounter.DoorIsOpen()})
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
// truth about Ashford from the hero, the garrison is the party's side and the
// loyalists' enemy from that moment (the first step of the garrison's
// `until`, rpg-project#717), the captain comes with the hero on his follow
// row while the gate guards march on the keep, and the fight the hero leads
// into the keep has three sides in which the party and the garrison both
// fight the loyalists and never each other.
func TestMadKingTurnTheGarrison(t *testing.T) {
	t.Parallel()
	s, posts := turnTheGarrison(t)

	t.Run("the captain, the garrison's mind, learned the guards have turned", func(t *testing.T) {
		learned := s.learners("guards-turned")
		require.True(t, learned[kingCaptain])
		require.True(t, learned[hero], "and the hero who told him heard his answer")
	})

	t.Run("the garrison is the party's side and the loyalists' enemy", func(t *testing.T) {
		require.Equal(t, encounter.StanceHostile, s.stance(garrison, loyalists))
		require.Equal(t, encounter.StanceAllied, s.stance(garrison, encounter.FactionParty))
	})

	s.walkTo(castleCell(0, -2))
	s.markTime(2, kingCaptain)

	t.Run("the hero walks into the courtyard and the captain comes with him", func(t *testing.T) {
		require.Positive(t, s.pickedTowardAlly(kingCaptain), "the record shows `toward ally` firing")
		require.False(t, s.pickedTowardACell(kingCaptain), "he no longer marches on the throne room")
		require.Less(t, s.enc.Distance(s.at(kingCaptain), s.at(hero)), s.enc.Distance(posts[kingCaptain], s.at(hero)),
			"he walked from the gate after the hero")
		require.LessOrEqual(t, s.enc.Distance(s.at(kingCaptain), s.at(hero)), 2.0, "and stands beside him")
	})

	t.Run("a gate guard who witnessed the turn marches on the keep", func(t *testing.T) {
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

	s.walkTo(throneCell)
	s.pressIntoTheKeep(8)

	t.Run("one fight, three sides: the party and the garrison against the loyalists", func(t *testing.T) {
		on, err := s.enc.ClockOf(&encounter.ClockOfInput{Member: hero})
		require.NoError(t, err)
		require.Equal(t, encounter.ClockTurn, on.Kind)
		require.Contains(t, on.Order, kingCaptain, "the captain followed the hero into the party's fight")
		require.True(t, s.hostile(hero, theKing))
		require.False(t, s.hostile(hero, kingCaptain), "the party and the garrison are not enemies")

		var garrisonSwung, captainSwung bool
		for _, st := range *s.strikes {
			if s.factionOf(st.attacker) != garrison {
				continue
			}
			require.NotEqual(t, hero, st.target, "the garrison never swings at the party")
			require.Equal(t, loyalists, s.factionOf(st.target), "%s swung at a loyalist", st.attacker)
			garrisonSwung = true
			captainSwung = captainSwung || st.attacker == kingCaptain
		}
		require.True(t, garrisonSwung, "the garrison fought the loyalists")
		require.True(t, captainSwung, "and the captain fought beside the hero")
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

// pressIntoTheKeep plays up to n turns of the fight the hero walked into:
// on each of the hero's turns he walks two cells on toward the throne room,
// and every turn, his or a creature's, is then ended. A follower outside the
// fight keeps following on the world's rounds between, so it comes through
// the door behind him and joins when it sees a loyalist itself.
func (s *kingScene) pressIntoTheKeep(n int) {
	s.t.Helper()
	for range n {
		on, err := s.enc.ClockOf(&encounter.ClockOfInput{Member: hero})
		require.NoError(s.t, err)
		if on.Kind != encounter.ClockTurn {
			return
		}
		if on.Active == hero {
			route, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveToward, Anchor: throneCell, Budget: 2})
			require.NoError(s.t, err)
			for j, cell := range route.Path {
				_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: j == len(route.Path)-1})
				require.NoError(s.t, err)
			}
		}
		s.reload()
		_, err = s.enc.EndTurn(&encounter.EndTurnInput{Member: on.Active})
		require.NoError(s.t, err, "%s ends its turn", on.Active)
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
