// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// pursuit_memory_test.go is the encounter's half of pursuit (rpg-project#668,
// memory.go), through the public turn seam: a pass that loses a memory, the
// view's lost trail, and a search that gives up where it ends. Every route
// loads the goblin's own testimony of alice and runs blind, so no sight
// refresh rewrites what the route is about.

// blindSight sees nothing past each member's own cell.
type blindSight struct{}

func (blindSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 0
	}

	return out, nil
}

// searchCause is the cause a test's own routed search walk carries.
var searchCause = core.Ref{Module: "test", Type: "table", ID: "search"}

// blindGoblin is the adjacent skeleton fight, persisted, with the goblin's
// sighting of alice replaced by this testimony and reloaded blind.
func (s *MonsterTurnTestSuite) blindGoblin(testimony encounter.SightTestimony, driver *scriptedDriver) *encounter.Encounter {
	base := s.adjacentSkeletonEncounter(passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed})
	data := base.ToData()
	payload, err := encounter.EncodeSightTestimony(testimony)
	s.Require().NoError(err)
	editGoblinHolding(s.T(), data, alice, payload)
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data,
		Capabilities: encounter.Capabilities{
			Sight:         blindSight{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        adjacentSkeletonSheets,
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        driver,
			Actors: encounter.Actors{
				Striker:   &scriptedStriker{kind: encounter.OutcomeMissed},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)

	return loaded
}

// cellOf is where a member stands now.
func (s *MonsterTurnTestSuite) cellOf(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.Require().Failf("not a member", "%q", id)

	return spatial.Position{}
}

// goblinsAlice is the goblin's sight testimony of alice now.
func (s *MonsterTurnTestSuite) goblinsAlice(enc *encounter.Encounter) encounter.SightTestimony {
	holding := requireHolding(s.T(), enc, goblin, alice)
	testimony, ok := encounter.DecodeSightTestimony(holding.Payload)
	s.Require().True(ok)

	return testimony
}

// TestAPassThatLosesAMemoryKeepsWhereAndWhichWay is R3: the goblin's own
// remembered testimony of alice becomes lost — the cell and the facing it
// saw kept — and its next view carries alice as lost, not remembered.
func (s *MonsterTurnTestSuite) TestAPassThatLosesAMemoryKeepsWhereAndWhichWay() {
	east := 90.0
	far := spatial.Position{X: 6, Y: 5}
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Pass{Loses: alice}}}
	enc := s.blindGoblin(encounter.SightTestimony{State: encounter.LocationKnown, Position: far, Facing: &east}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(driver.calls, 1)
	s.Require().Len(driver.calls[0].Remembered, 1, "precondition: alice is remembered")

	lost := s.goblinsAlice(enc)
	s.Equal(encounter.LocationLost, lost.State)
	s.Equal(far, lost.Position, "the cell stays")
	s.Require().NotNil(lost.Facing)
	s.Equal(east, *lost.Facing, "and the way she was facing")

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(driver.calls, 2)
	s.Empty(driver.calls[1].Remembered, "lost is not remembered")
	s.Require().Len(driver.calls[1].Lost, 1)
	s.Equal(alice, driver.calls[1].Lost[0].ID)
	s.Equal(far, driver.calls[1].Lost[0].Position)
}

// TestAPassLosesOnlyAPlacedMemory: an unknown testimony has nothing to lose,
// and stays unknown.
func (s *MonsterTurnTestSuite) TestAPassLosesOnlyAPlacedMemory() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Pass{Loses: alice}}}
	enc := s.blindGoblin(encounter.SightTestimony{State: encounter.LocationUnknown}, driver)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(encounter.LocationUnknown, s.goblinsAlice(enc).State)
}

// TestALostTrailRunsStraightToTheWall is R4's trail: from the lost cell, one
// step direction along the held facing, repeated until the next cell is not
// floor. A testimony with no facing walks the goblin's own.
func (s *MonsterTurnTestSuite) TestALostTrailRunsStraightToTheWall() {
	east := 90.0
	from := spatial.Position{X: 2, Y: 5}
	driver := &scriptedDriver{}
	enc := s.blindGoblin(encounter.SightTestimony{State: encounter.LocationLost, Position: from, Facing: &east}, driver)
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(driver.calls, 1)
	s.Require().Len(driver.calls[0].Lost, 1)
	trail := driver.calls[0].Lost[0].Trail
	s.Require().GreaterOrEqual(len(trail), 2, "the room is wider than two cells east of the lost cell")

	step := spatial.Position{X: trail[0].X - from.X, Y: trail[0].Y - from.Y}
	prev := from
	for _, cell := range trail {
		s.Equal(spatial.Position{X: prev.X + step.X, Y: prev.Y + step.Y}, cell, "one step direction, repeated")
		_, floor := enc.RegionAt(cell)
		s.True(floor, "%v is floor", cell)
		prev = cell
	}
	_, floor := enc.RegionAt(spatial.Position{X: prev.X + step.X, Y: prev.Y + step.Y})
	s.False(floor, "the trail ends at the room's edge")
	s.Greater(trail[0].X, from.X, "east is the step")

	// No facing held: the goblin's own facing is the heading, so the trail
	// is the one the same testimony would give with that facing stated.
	own := enc.ToData()
	var goblinFacing float64
	for _, m := range own.Members {
		if m.ID == goblin && m.Facing != nil {
			goblinFacing = *m.Facing
		}
	}
	unstated := &scriptedDriver{}
	enc = s.blindGoblin(encounter.SightTestimony{State: encounter.LocationLost, Position: from}, unstated)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	stated := &scriptedDriver{}
	enc = s.blindGoblin(encounter.SightTestimony{State: encounter.LocationLost, Position: from, Facing: &goblinFacing}, stated)
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(stated.calls[0].Lost[0].Trail, unstated.calls[0].Lost[0].Trail, "the goblin's own facing is the heading")
	s.NotEqual(trail, unstated.calls[0].Lost[0].Trail, "precondition: the goblin does not face east")
}

// TestASearchGivesUpWhereItEnds is R5: a search walk that ends on its cell,
// or has nowhere to go, turns the memory unknown; one cut short by the turn's
// movement leaves it lost for the next turn; and a routed walk that names no
// search leaves it alone.
func (s *MonsterTurnTestSuite) TestASearchGivesUpWhereItEnds() {
	lost := encounter.SightTestimony{State: encounter.LocationLost, Position: spatial.Position{X: 6, Y: 5}}

	for _, tc := range []struct {
		name   string
		offset spatial.Position
		gives  encounter.MemberID
		want   encounter.LocationState
	}{
		{name: "arrived", offset: spatial.Position{X: 2}, gives: alice, want: encounter.LocationUnknown},
		{name: "already there", offset: spatial.Position{}, gives: alice, want: encounter.LocationUnknown},
		{name: "cut short by the turn", offset: spatial.Position{Y: 7}, gives: alice, want: encounter.LocationLost},
		{name: "not a search", offset: spatial.Position{X: 2}, want: encounter.LocationLost},
	} {
		s.Run(tc.name, func() {
			driver := &scriptedDriver{}
			enc := s.blindGoblin(lost, driver)
			here := s.cellOf(enc, goblin)
			cell := spatial.Position{X: here.X + tc.offset.X, Y: here.Y + tc.offset.Y}
			_, floor := enc.RegionAt(cell)
			s.Require().True(floor, "precondition: %v is floor", cell)
			driver.intents = []encounter.TurnIntent{encounter.Routed{
				Policy: encounter.MoveToward, AnchorAt: &cell, Cause: searchCause, GivesUp: tc.gives,
			}}

			_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
			s.Require().NoError(err)
			if tc.want == encounter.LocationUnknown {
				s.Equal(cell, s.cellOf(enc, goblin), "the search stands on its cell")
			}
			s.Equal(tc.want, s.goblinsAlice(enc).State)
		})
	}
}

// TestASearchMustNameACell: GivesUp beside a member anchor is a malformed
// search, refused as a bad intent — the turn ends and nothing is given up.
func (s *MonsterTurnTestSuite) TestASearchMustNameACell() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Routed{
		Policy: encounter.MoveToward, Anchor: alice, Cause: searchCause, GivesUp: alice,
	}}}
	enc := s.blindGoblin(encounter.SightTestimony{State: encounter.LocationLost, Position: spatial.Position{X: 6, Y: 5}}, driver)
	here := s.cellOf(enc, goblin)

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Equal(here, s.cellOf(enc, goblin), "nobody walked")
	s.Equal(encounter.LocationLost, s.goblinsAlice(enc).State, "and nothing was given up")
}

// TestALostMemoryIsNeverCurrent: a persisted lost sighting that is still
// sustained is a blob nobody wrote, refused at load.
func (s *MonsterTurnTestSuite) TestALostMemoryIsNeverCurrent() {
	base := s.adjacentSkeletonEncounter(passDriver{}, &scriptedStriker{kind: encounter.OutcomeMissed})
	data := base.ToData()
	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{State: encounter.LocationLost, Position: spatial.Position{X: 1, Y: 1}})
	s.Require().NoError(err)
	holdings := data.Perception.Intel.Holdings[perception.Qualify("member", goblin)]
	edited := false
	for key, holding := range holdings {
		if string(key) == string(perception.Qualify("member", alice)) {
			s.Require().NotEmpty(holding.CurrentVia, "precondition: the goblin sees alice now")
			holding.Payload = payload
			holdings[key] = holding
			edited = true
		}
	}
	s.Require().True(edited)

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: encounter.Capabilities{
		Sight: blindSight{}, Equipment: encounter.UnobservedEquipment{}, Conditions: encounter.UnobservedEquipment{},
		Sheets: adjacentSkeletonSheets, Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{},
		Actors: encounter.Actors{Striker: &scriptedStriker{kind: encounter.OutcomeMissed}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}})
	s.Require().ErrorIs(err, encounter.ErrInvalidData)
	s.Contains(err.Error(), "lost but current")
}

// TestALostTrailStopsAtAWallItCouldWalkRound: the trail is a straight line, so
// the cell past a wall ends it even when a doorway makes that cell reachable
// the long way round — a search does not detour to keep its line.
func (s *MonsterTurnTestSuite) TestALostTrailStopsAtAWallItCouldWalkRound() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{
				rectRegion(westID, int(westOrigin.X), int(westOrigin.Y), 6, 6),
				rectRegion(eastID, int(eastOrigin.X), int(eastOrigin.Y), 6, 6),
			},
			Walls: pursuitSeamWall(),
			Doors: []encounter.DoorInput{openDoorway(gateway, 25, 13, 26, 13)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 21, Y: 11}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 22, Y: 11}},
		},
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Sight: everyoneSeesTheWholeMap{}, Equipment: encounter.UnobservedEquipment{}, Conditions: encounter.UnobservedEquipment{},
			Sheets: adjacentSkeletonSheets, Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{},
			Actors: encounter.Actors{Striker: &scriptedStriker{kind: encounter.OutcomeMissed}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	s.Require().NoError(err)
	data := enc.ToData()
	east := 90.0
	payload, err := encounter.EncodeSightTestimony(encounter.SightTestimony{
		State: encounter.LocationLost, Position: cellAt(24, 11), Facing: &east,
	})
	s.Require().NoError(err)
	editGoblinHolding(s.T(), data, alice, payload)
	driver := &scriptedDriver{}
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: encounter.Capabilities{
		Sight: blindSight{}, Equipment: encounter.UnobservedEquipment{}, Conditions: encounter.UnobservedEquipment{},
		Sheets: adjacentSkeletonSheets, Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: driver,
		Actors: encounter.Actors{Striker: &scriptedStriker{kind: encounter.OutcomeMissed}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}})
	s.Require().NoError(err)

	_, err = loaded.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Len(driver.calls, 1)
	s.Require().Len(driver.calls[0].Lost, 1)
	s.Equal([]spatial.Position{cellAt(25, 11)}, driver.calls[0].Lost[0].Trail,
		"one cell east, then the seam wall: the east room past it is the doorway's long way round")
}

// TestAPassNeverLosesASightingBeingMade: the goblin sees alice now, so a pass
// naming her loses nothing — a current sighting is not a memory to revise.
func (s *MonsterTurnTestSuite) TestAPassNeverLosesASightingBeingMade() {
	driver := &scriptedDriver{intents: []encounter.TurnIntent{encounter.Pass{Loses: alice}}}
	enc := s.adjacentSkeletonEncounter(driver, &scriptedStriker{kind: encounter.OutcomeMissed})
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().NotEmpty(driver.calls, "precondition: the goblin had its turn")
	holding := requireHolding(s.T(), enc, goblin, alice)
	s.Require().NotEmpty(holding.CurrentVia, "precondition: the goblin sees alice")
	s.Equal(encounter.LocationKnown, s.goblinsAlice(enc).State)
}
