// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ghost_relocate_test.go is THEY WATCHED THEM GO (rpg-project#961) on the
// relocation's shut hall: a witness of the leaving holds the mover as
// unknown at once — never a ghost on the cell they were seen to leave, never
// where they went; a witness of both ends holds the live sighting; one who
// saw neither keeps the ghost they held.

// ghostWarden is a castle creature in room a, neutral to the party.
const ghostWarden encounter.MemberID = "warden"

// sightingOf is observer's held sight testimony of subject, and whether it is
// current; ok false when the observer holds none.
func (s *RelocateSuite) sightingOf(enc *encounter.Encounter, observer, subject encounter.MemberID) (encounter.SightTestimony, bool, bool) {
	held, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range held {
		if h.Channel == perception.Sight && h.Subject == subject {
			t, ok := encounter.DecodeSightTestimony(h.Payload)
			s.Require().True(ok)
			return t, h.CurrentOn(perception.Sight), true
		}
	}
	return encounter.SightTestimony{}, false, false
}

// TestALeavingWitnessHoldsTheMoverUnknown: Alice is set down in room c.
// Bob, who saw her leave, holds her unknown — no ghost on her old cell;
// Carol, who saw her arrive, holds her live. Dan, who saw Alice in room a
// before he was himself set down in room b, saw neither end of her move and
// keeps his ghost of her where he last saw her.
func (s *RelocateSuite) TestALeavingWitnessHoldsTheMoverUnknown() {
	enc := s.hall(
		at(relAlice, encounter.KindPlayer, 2, 5), at(relBob, encounter.KindPlayer, 4, 5),
		at(relDan, encounter.KindPlayer, 6, 5), at(relCarol, encounter.KindPlayer, 25, 5),
		at(ghostWarden, encounter.KindMonster, 5, 7),
	)
	_, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relDan, To: cellAt(15, 5)}}, Cause: relCause})
	s.Require().NoError(err)
	dan, current, held := s.sightingOf(enc, relDan, relAlice)
	s.Require().True(held && !current, "precondition: Dan holds a ghost of Alice")
	s.Require().Equal(cellAt(2, 5), dan.Position)

	_, err = enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: cellAt(26, 5)}}, Cause: relCause})
	s.Require().NoError(err)

	bob, current, held := s.sightingOf(enc, relBob, relAlice)
	s.Require().True(held, "Bob still knows of Alice")
	s.False(current)
	s.Equal(encounter.LocationUnknown, bob.State, "he watched her go: no ghost on her old cell")
	s.Equal(spatial.Position{}, bob.Position, "and never where she went")

	// A CREATURE WATCHED HER GO TOO: the warden holds her unknown, with no
	// cell to walk to — rule 2 is everyone's, where the player-only look
	// rule would never have touched a creature's memory.
	warden, current, held := s.sightingOf(enc, ghostWarden, relAlice)
	s.Require().True(held)
	s.False(current)
	s.Equal(encounter.LocationUnknown, warden.State, "the warden watched her go")

	carol, current, _ := s.sightingOf(enc, relCarol, relAlice)
	s.True(current, "Carol sees her arrive")
	s.Equal(cellAt(26, 5), carol.Position)

	dan, current, _ = s.sightingOf(enc, relDan, relAlice)
	s.False(current)
	s.Equal(encounter.LocationKnown, dan.State, "Dan saw neither end and keeps his ghost")
	s.Equal(cellAt(2, 5), dan.Position)
}

// TestAWitnessOfBothEndsHoldsTheLiveSighting: set down across room a, Alice
// is seen leaving and arriving by Bob, who holds her live where she is now.
func (s *RelocateSuite) TestAWitnessOfBothEndsHoldsTheLiveSighting() {
	enc := s.party()
	_, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: cellAt(7, 8)}}, Cause: relCause})
	s.Require().NoError(err)
	bob, current, held := s.sightingOf(enc, relBob, relAlice)
	s.Require().True(held)
	s.True(current)
	s.Equal(cellAt(7, 8), bob.Position)
}

// TestACreaturesGhostOnASeenCellStays (rpg-project#961): the look rule is a
// player's. Behind the wall test's geometry, Alice steps out of sight of a
// warden, a creature, who still sees the cell she left: the warden keeps
// its ghost of her there — the memory its search reads — where a player
// holder would hold her unknown.
func (s *RelocateSuite) TestACreaturesGhostOnASeenCellStays() {
	watch := encounter.FactionID("watch")
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 20, 20)}, Props: wallRow(10, 7, 13),
			Factions: []encounter.FactionInput{{ID: watch}},
			Dispositions: []encounter.DispositionInput{
				{Between: [2]encounter.FactionID{watch, encounter.FactionParty}, Stance: encounter.StanceNeutral},
			},
		},
		Members: []encounter.MemberInput{
			{ID: relAlice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: ghostWarden, Kind: encounter.KindMonster, Faction: watch, Position: spatial.Position{X: 10, Y: 18}},
		},
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: (&OptionTestSuite{}).capabilities(),
	})
	s.Require().NoError(err)
	_, current, _ := s.sightingOf(enc, ghostWarden, relAlice)
	s.Require().True(current, "precondition: the warden sees Alice")

	_, err = enc.Step(&encounter.StepInput{Member: relAlice, To: cellAt(10, 2)})
	s.Require().NoError(err)
	ghost, current, held := s.sightingOf(enc, ghostWarden, relAlice)
	s.Require().True(held)
	s.False(current, "she is behind the wall from the warden")
	s.Equal(encounter.LocationKnown, ghost.State, "a creature keeps its ghost on a cell it sees")
	s.Equal(cellAt(2, 2), ghost.Position)
}

// TestTheUnknownSurvivesAReload: saved after Bob watched Alice go, the run
// loads with Bob still holding her unknown, and his story exactly as long.
func (s *RelocateSuite) TestTheUnknownSurvivesAReload() {
	enc := s.party()
	_, err := enc.Relocate(&encounter.RelocateInput{Moves: []encounter.Relocation{{Member: relAlice, To: cellAt(26, 5)}}, Cause: relCause})
	s.Require().NoError(err)
	before := len(storyBeats(s.T(), enc, relBob))

	blob, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(blob, &data))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: noiseHallSetup(
		encounter.DoorIsClosed(), encounter.DoorIsClosed(), sheetFacts{
			relAlice: {SpeedFeet: 30}, relBob: {SpeedFeet: 30}, relCarol: {SpeedFeet: 30}, relDan: {SpeedFeet: 30},
		}, passDriver{}).Capabilities})
	s.Require().NoError(err)
	bob, current, held := s.sightingOf(loaded, relBob, relAlice)
	s.Require().True(held)
	s.False(current)
	s.Equal(encounter.LocationUnknown, bob.State)
	s.Len(storyBeats(s.T(), loaded, relBob), before, "nothing fires again")
}
