// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// drivendoor_test.go is A WALKING CREATURE OPENS THE DOOR IT WALKS INTO
// (rpg-project#715, Frank's list S1; the primitive question rpg-toolkit#2069).
// A driven walk routes through a closed, unlocked door at the price of the
// step onto it, and the encounter opens the door before the step is taken —
// the state, the door beat and the door's `opened` fact a player's
// [encounter.Encounter.OpenDoor] writes. A locked door still stops it (lane
// rpg-project#722, D5: never, for S1).
//
// THE NOISE HALL (noise_test.go): rooms a | b | c, door-ab on row 5 between
// [9,5] and [10,5], door-bc between [19,5] and [20,5].

type DrivenDoorSuite struct{ suite.Suite }

func TestDrivenDoorSuite(t *testing.T) { suite.Run(t, new(DrivenDoorSuite)) }

const drivenDoorFact encounter.FactID = "hall-door-opened"

// drivenDoorSheets is thirty feet for everybody: six cells a round.
var drivenDoorSheets = sheetFacts{
	noiseKeen:  {SpeedFeet: 30},
	noiseSharp: {SpeedFeet: 30},
	noiseAlice: {SpeedFeet: 30},
	noiseDull:  {SpeedFeet: 30},
}

// drivenHall is the hall with door-ab in the state given and teaching
// drivenDoorFact, door-bc shut, every creature on its own table.
func (s *DrivenDoorSuite) drivenHall(ab encounter.DoorState, members ...encounter.MemberInput) *encounter.Encounter {
	in := noiseHallSetup(ab, encounter.DoorIsClosed(), drivenDoorSheets, tableDriver(), members...)
	in.Field.Doors[0].Opened = drivenDoorFact
	enc, err := encounter.NewEncounter(in)
	s.Require().NoError(err)

	return enc
}

// marcher is keen, a castle guard at [5,5] in room a, sent to [11,5] in room
// b: six cells, one round of the world at thirty feet, and the fourth of them
// is through door-ab.
func marcher() encounter.MemberInput {
	m := at(noiseKeen, encounter.KindMonster, 5, 5)
	m.Table = walksTo(cellAt(11, 5))

	return m
}

// theMarchersHall is the marcher, Sharp watching from room a, and Alice — the
// one who makes time pass — in room c behind the shut door-bc, who cannot see
// door-ab at all.
func theMarchersHall() []encounter.MemberInput {
	return []encounter.MemberInput{
		marcher(),
		at(noiseSharp, encounter.KindPlayer, 4, 2),
		at(noiseAlice, encounter.KindPlayer, 25, 2),
	}
}

// doorState is one door's state right now.
func (s *DrivenDoorSuite) doorState(enc *encounter.Encounter, id encounter.DoorID) encounter.DoorStateKind {
	for _, d := range enc.Doors() {
		if d.ID == id {
			return d.State.Kind()
		}
	}
	s.FailNow("no such door", "%q", id)

	return ""
}

// beats is every beat in the guard's own story, decoded, with its seq: the
// actor holds the beat of its own deed.
func (s *DrivenDoorSuite) beats(enc *encounter.Encounter) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: noiseKeen})
	s.Require().NoError(err)
	out := make([]map[string]any, 0, len(story))
	for _, entry := range story {
		beat := map[string]any{}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		beat["seq"] = entry.Seq
		out = append(out, beat)
	}

	return out
}

// doorBeatsBy is every door beat naming this actor.
func (s *DrivenDoorSuite) doorBeatsBy(enc *encounter.Encounter, actor encounter.MemberID) []map[string]any {
	var out []map[string]any
	for _, b := range s.beats(enc) {
		if b["beat"] == encounter.BeatDoor && b["actor"] == string(actor) {
			out = append(out, b)
		}
	}

	return out
}

// movedOnto is the seq of the member's moved beat onto a cell, or zero.
func (s *DrivenDoorSuite) movedOnto(enc *encounter.Encounter, member encounter.MemberID, cell spatial.Position) uint64 {
	for _, b := range s.beats(enc) {
		if b["beat"] != encounter.BeatMoved || b["member"] != string(member) {
			continue
		}
		pos, _ := b["position"].(map[string]any)
		if pos["x"] == cell.X && pos["y"] == cell.Y {
			return b["seq"].(uint64)
		}
	}

	return 0
}

// TestAMarchOpensTheShutDoorAndArrives: one round of the world, six cells,
// and the fourth is through the shut door — the guard arrives on the cell,
// so opening it cost nothing the step did not.
func (s *DrivenDoorSuite) TestAMarchOpensTheShutDoorAndArrives() {
	enc := s.drivenHall(encounter.DoorIsClosed(), theMarchersHall()...)

	_, err := aRound(enc)
	s.Require().NoError(err)

	s.Equal(cellAt(11, 5), whereIs(s.T(), enc, noiseKeen), "six cells in one round: the open is part of the walk")
	s.Equal(encounter.DoorOpen, s.doorState(enc, noiseDoorAB), "and the door stands open behind it")
}

// TestTheOpenIsTheDoorBeatAPlayerWrites: one door beat, naming the guard and
// the table word that walked it, before the step through.
func (s *DrivenDoorSuite) TestTheOpenIsTheDoorBeatAPlayerWrites() {
	enc := s.drivenHall(encounter.DoorIsClosed(), theMarchersHall()...)

	_, err := aRound(enc)
	s.Require().NoError(err)

	opens := s.doorBeatsBy(enc, noiseKeen)
	s.Require().Len(opens, 1, "one open line")
	s.Equal(string(noiseDoorAB), opens[0]["door"])
	s.Equal(string(encounter.DoorOpen), opens[0]["state"])
	s.Equal("encounter:table:toward", opens[0]["cause"], "the word that walked it")

	through := s.movedOnto(enc, noiseKeen, cellAt(10, 5))
	s.Require().NotZero(through, "the guard stepped through")
	s.Less(opens[0]["seq"].(uint64), through, "the door opens before the step through it")
}

// TestTheOpenedFactLandsOnWhoSawTheDoor: the door's `opened` fact goes to
// the guard and to Sharp, who watched from room a — not to Alice, behind a
// wall and a shut door in room c.
func (s *DrivenDoorSuite) TestTheOpenedFactLandsOnWhoSawTheDoor() {
	enc := s.drivenHall(encounter.DoorIsClosed(), theMarchersHall()...)

	_, err := aRound(enc)
	s.Require().NoError(err)

	s.Equal([]string{string(noiseKeen), string(noiseSharp)}, learnersOf(enc, drivenDoorFact))
}

// TestALockedDoorStillStopsTheMarch is D5 (rpg-project#722): a creature never
// opens a locked door in this slice. The guard walks to the near side and
// stays, and the round after, its stayed beat says what stopped it: that
// door, locked, in the words a step refused by the lock is told. A march
// stopped by a lock must not read as a march with nowhere to go (the
// operator's walk of this slice).
func (s *DrivenDoorSuite) TestALockedDoorStillStopsTheMarch() {
	lock := encounter.DoorIsLocked(encounter.Lock{Approaches: []encounter.CheckApproach{{Ability: "str", DC: 15}}})
	enc := s.drivenHall(lock, theMarchersHall()...)

	for range 3 {
		_, err := aRound(enc)
		s.Require().NoError(err)
	}

	region, ok := enc.RegionAt(whereIs(s.T(), enc, noiseKeen))
	s.Require().True(ok)
	s.Equal(encounter.RegionID("room-a"), region, "the guard never set foot in room b")
	s.Equal(encounter.DoorLocked, s.doorState(enc, noiseDoorAB), "and the lock held")
	s.Empty(s.doorBeatsBy(enc, noiseKeen), "nobody opened it")

	var why []any
	for _, b := range s.beats(enc) {
		if b["beat"] == encounter.BeatStayed && b["member"] == string(noiseKeen) {
			why = append(why, b["why"])
			s.Equal("encounter:table:toward", b["cause"])
		}
	}
	s.Require().NotEmpty(why, "the guard's stop is on the record")
	for _, w := range why {
		s.Equal(`door "door-ab" is locked, DC 15 (str)`, w, "the stop names the door and its lock")
	}
}

// TestAMarchWithNoWayAtAllStillSaysNowhereNearer is the control: a cell that
// no route reaches, lock or no lock, keeps the route's own sentence. A shut
// wall is not a locked door, and the record does not invent one.
func (s *DrivenDoorSuite) TestAMarchWithNoWayAtAllStillSaysNowhereNearer() {
	in := noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(), drivenDoorSheets, tableDriver(),
		theMarchersHall()...)
	in.Field.Walls = append(squareSeamWall(9, 10), squareSeamWall(19, 10, 5)...)
	in.Field.Doors = in.Field.Doors[1:] // room a is sealed: no door into b at all
	enc, err := encounter.NewEncounter(in)
	s.Require().NoError(err)

	for range 3 {
		_, err := aRound(enc)
		s.Require().NoError(err)
	}

	var why []any
	for _, b := range s.beats(enc) {
		if b["beat"] == encounter.BeatStayed && b["member"] == string(noiseKeen) {
			why = append(why, b["why"])
		}
	}
	s.Contains(why, fmt.Sprintf("nowhere nearer to %v within 6 cells", cellAt(11, 5)))
}

// TestAPatrolLoopsThroughTheShutDoor: a `toward: { path }` loop whose two
// long legs cross a two-leaf door, one leaf each way. The guard walks the
// whole loop and the door stands open.
func (s *DrivenDoorSuite) TestAPatrolLoopsThroughTheShutDoor() {
	in, route := patrolHall()
	enc, err := encounter.NewEncounter(in)
	s.Require().NoError(err)

	// Twenty-six cells round; five rounds of six walk it with four to spare.
	for range 5 {
		_, err := aRound(enc)
		s.Require().NoError(err)
	}

	for _, waypoint := range route[1:] {
		s.NotZero(s.movedOnto(enc, noiseKeen, waypoint), "the loop reached %v", waypoint)
	}
	s.Equal(encounter.DoorOpen, s.doorState(enc, noiseDoorAB), "the door stands open behind the patrol")
	s.Len(s.doorBeatsBy(enc, noiseKeen), 1, "opened once; the way back finds it open")
}

// patrolHall is the hall with a gap two rows wide in the a|b seam and one
// shut door standing in both rows, and keen walking a loop whose two long
// legs cross it, one leaf each way. Alice, who makes time pass, is in room c.
func patrolHall() (*encounter.SetupInput, []spatial.Position) {
	route := []spatial.Position{cellAt(3, 4), cellAt(15, 4), cellAt(15, 5), cellAt(3, 5)}
	guard := at(noiseKeen, encounter.KindMonster, 3, 4)
	guard.Table = encounter.Table{encounter.AnswerTime: {{Weight: 1, Toward: &encounter.Selector{Path: route}}}}

	in := noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(), drivenDoorSheets, tableDriver(),
		guard, at(noiseAlice, encounter.KindPlayer, 25, 2))
	in.Field.Walls = append(squareSeamWall(9, 10, 4, 5), squareSeamWall(19, 10, 5)...)
	in.Field.Doors[0].Edges = []encounter.DoorEdge{
		{From: cellAt(9, 4), To: cellAt(10, 4)},
		{From: cellAt(9, 5), To: cellAt(10, 5)},
	}

	return in, route
}

// TestAHiddenDoorIsNeverOpened is the masquerade law on the open side: the
// same patrol, with the door a secret nobody has found. A patrol leg is an
// authored line, not a route the masquerade could steer, so the walk itself
// must refuse the door: the guard stops at the wall it believes is there, and
// the door stays shut with no beat of anybody opening it.
func (s *DrivenDoorSuite) TestAHiddenDoorIsNeverOpened() {
	in, _ := patrolHall()
	in.Field.Concealments = []encounter.ConcealmentInput{
		{ID: "secret", Checks: vaultFind(), Doors: []encounter.DoorID{noiseDoorAB}},
	}
	in.CheckResolver = findsNothing{}
	in.Witness = nobodyPerceives{}
	enc, err := encounter.NewEncounter(in)
	s.Require().NoError(err)

	for range 3 {
		_, err := aRound(enc)
		s.Require().NoError(err)
	}

	s.Equal(encounter.DoorClosed, s.doorState(enc, noiseDoorAB), "nobody opens a door they have not found")
	s.Empty(s.doorBeatsBy(enc, noiseKeen))
	region, ok := enc.RegionAt(whereIs(s.T(), enc, noiseKeen))
	s.Require().True(ok)
	s.Equal(encounter.RegionID("room-a"), region, "the guard never passed the wall it believes is there")
}

// TestAMarchWithAnAllyAtTheDoorOpensItAndPassesThrough: a castle guard
// stands on [9,5], the cell in front of door-ab. The marcher may cross an
// ally and may not stop on one, so the walk is judged whole: it crosses the
// ally's cell, opens the door, and arrives. (Review of rpg-toolkit#2150: a
// walk trimmed door to door ended a cell short of the ally, asked about the
// step onto it, opened nothing, and stood still every round.)
func (s *DrivenDoorSuite) TestAMarchWithAnAllyAtTheDoorOpensItAndPassesThrough() {
	enc := s.drivenHall(encounter.DoorIsClosed(), append(theMarchersHall(),
		at(noiseDull, encounter.KindMonster, 9, 5))...)

	for range 3 {
		_, err := aRound(enc)
		s.Require().NoError(err)
	}

	s.Equal(encounter.DoorOpen, s.doorState(enc, noiseDoorAB), "the march opens the door past its ally")
	s.Equal(cellAt(11, 5), whereIs(s.T(), enc, noiseKeen), "and arrives")
	s.Equal(cellAt(9, 5), whereIs(s.T(), enc, noiseDull), "the ally stays at its post")
}

// TestADirectedWalkThroughAShutDoorStopsAtIt: [encounter.Encounter.Direct]
// walks the route its caller hands it, and nothing upstream stops a caller
// from handing it one through a shut door. A push is nobody's walk, so it
// opens nothing: the walk ends in front of the door and the door stays shut.
func (s *DrivenDoorSuite) TestADirectedWalkThroughAShutDoorStopsAtIt() {
	enc := s.drivenHall(encounter.DoorIsClosed(), theMarchersHall()...)
	route := []spatial.Position{cellAt(6, 5), cellAt(7, 5), cellAt(8, 5), cellAt(9, 5), cellAt(10, 5), cellAt(11, 5)}

	out, err := enc.Direct(context.Background(), encounter.DirectInput{
		Mover: noiseKeen, Cause: core.Ref{Module: "test", Type: "spell", ID: "shove"}, Route: route,
	})
	s.Require().NoError(err)

	s.Equal(4, out.Moved, "it is walked to the door and no further")
	s.Equal(cellAt(9, 5), whereIs(s.T(), enc, noiseKeen))
	s.Equal(encounter.DoorClosed, s.doorState(enc, noiseDoorAB), "a shove does not open a door")
	s.Empty(s.doorBeatsBy(enc, noiseKeen))
}

// TestADoorOpenedBeforeAPausedStepIsSeen: on the turn clock a hostile guard
// in front of door-ab marches through it, and the very step through is held
// for a reaction. The door is open and no cell was walked; Sharp, beyond the
// door in room b, sees the guard now, because the walk's settle counts the
// door it opened as well as the steps it took.
func (s *DrivenDoorSuite) TestADoorOpenedBeforeAPausedStepIsSeen() {
	guard := at(noiseKeen, encounter.KindMonster, 9, 5)
	guard.Faction = "" // opposed to the party: the fight is on
	guard.Table = walksTo(cellAt(12, 5))
	in := noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(), drivenDoorSheets, tableDriver(),
		at(noiseAlice, encounter.KindPlayer, 4, 5), guard, at(noiseSharp, encounter.KindPlayer, 15, 5))
	in.Field.Doors[0].Opened = drivenDoorFact
	in.Mover = &pausingMover{pauseAt: map[int]bool{0: true}}
	enc, err := encounter.NewEncounter(in)
	s.Require().NoError(err)
	s.Require().False(s.seesNow(enc, noiseSharp, noiseKeen), "precondition: the shut door hides the guard from Sharp")

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: noiseAlice})
	s.Require().NoError(err)

	kind, paused := enc.PauseKind()
	s.Require().True(paused, "precondition: the step through is held")
	s.Require().Equal(encounter.PauseTurn, kind)
	s.Equal(cellAt(9, 5), whereIs(s.T(), enc, noiseKeen), "no cell was walked")
	s.Equal(encounter.DoorOpen, s.doorState(enc, noiseDoorAB))
	s.True(s.seesNow(enc, noiseSharp, noiseKeen), "the opened door let Sharp see the guard")
}

// seesNow is whether an observer's sight of a subject is current, false when
// it holds nothing on them at all.
func (s *DrivenDoorSuite) seesNow(enc *encounter.Encounter, observer, subject encounter.MemberID) bool {
	view, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject == subject {
			return h.CurrentOn(perception.Sight)
		}
	}

	return false
}

// TestAPursuerOpensTheDoorShutInItsFace: a hostile guard hunts Alice. She
// slips through door-ab, shuts it behind her and steps out of its line; on
// its turn the guard walks to where it last saw her, opens the door on the
// way, and the chase goes on.
func (s *DrivenDoorSuite) TestAPursuerOpensTheDoorShutInItsFace() {
	alice := at(noiseAlice, encounter.KindPlayer, 8, 5)
	// No faction: opposed to the party, so seeing her starts the fight.
	guard := at(noiseKeen, encounter.KindMonster, 6, 5)
	guard.Faction = ""
	guard.Table = hunts()
	enc := s.drivenHall(encounter.DoorIsOpen(), alice, guard)
	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: noiseAlice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, on.Kind, "precondition: seeing each other started the fight")
	s.Require().Equal(noiseAlice, on.Active, "precondition: Alice goes first")

	for _, cell := range []spatial.Position{cellAt(9, 5), cellAt(10, 5), cellAt(11, 5)} {
		_, err := enc.Step(&encounter.StepInput{Member: noiseAlice, To: cell})
		s.Require().NoError(err)
	}
	_, err = enc.CloseDoor(&encounter.CloseDoorInput{Door: noiseDoorAB, Actor: noiseAlice})
	s.Require().NoError(err)
	for _, cell := range []spatial.Position{cellAt(12, 4), cellAt(12, 3)} {
		_, err := enc.Step(&encounter.StepInput{Member: noiseAlice, To: cell})
		s.Require().NoError(err)
	}
	current, _ := seen(s.T(), enc, noiseKeen, noiseAlice)
	s.Require().False(current, "precondition: the shut door took her out of its sight")

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: noiseAlice})
	s.Require().NoError(err)

	s.Equal(encounter.DoorOpen, s.doorState(enc, noiseDoorAB), "the guard opened the door she shut")
	s.Len(s.doorBeatsBy(enc, noiseKeen), 1, "one open line")
	region, ok := enc.RegionAt(whereIs(s.T(), enc, noiseKeen))
	s.Require().True(ok)
	s.Equal(encounter.RegionID("room-b"), region, "and went through after her")
	current, _ = seen(s.T(), enc, noiseKeen, noiseAlice)
	s.True(current, "the chase goes on: it has her in sight again")
}

// TestADirectedMoveStillStopsAtTheShutDoor: the public route — a push, a
// rout, a hero's path — is nobody's walk, and a shut door still closes it.
// Only a creature's own driven walk opens one.
func (s *DrivenDoorSuite) TestADirectedMoveStillStopsAtTheShutDoor() {
	enc := s.drivenHall(encounter.DoorIsClosed(), theMarchersHall()...)

	route, err := enc.Route(encounter.RouteInput{
		Mover: noiseKeen, Policy: encounter.MoveToward, Anchor: cellAt(14, 5), Budget: 12,
	})
	s.Require().NoError(err)
	for _, cell := range route.Path {
		region, _ := enc.RegionAt(cell)
		s.NotEqual(encounter.RegionID("room-b"), region, "the route never passes the shut door")
	}
}
