// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// footing_test.go is ACCEPTANCE A10 and A11 (rpg-project#360, wall-geometry
// design §7): what a member who has not found a secret is shown of the walls
// around it, now that a wall is a line rather than a bag of crossings.
//
// Two rules, and they are the same rule from two sides:
//
//   - C18, FOOTING. A presented wall stands on floor the recipient can see.
//     Every cell a presented segment passes through is in their atlas as floor
//     nobody owns, even when the region that owns it is hidden. Without it the
//     floor stops one cell short of the wall and there is a black sliver
//     exactly where the secret is — the same tell the masquerade exists to
//     remove, one layer down.
//
//   - C19, THE GAP. A concealed unfound door in a wall presents as the wall's
//     WHOLE segment with no gap in it. The mechanism is that a door's gap is
//     the reader's own arithmetic — the doorway projected onto the segment —
//     so withholding the doorway withholds the gap, and the line arrives
//     unbroken without anything having to redraw it.
//
// The yardstick is the same one rpg-project#351 set: a non-knower's atlas is
// the atlas of a dungeon HONESTLY AUTHORED as what they believe.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type FootingSuite struct {
	suite.Suite

	witness *scriptedWitness
}

func TestFootingSuite(t *testing.T) {
	suite.Run(t, new(FootingSuite))
}

func (s *FootingSuite) SetupTest() { s.witness = &scriptedWitness{} }

const (
	watcher = core.EntityID("watcher")
	knower  = core.EntityID("knower")
)

// hugSeam is the seam between column 3 and column 4 as ONE LINE — the quarter
// line the tomb's own seams are, standing a quarter of a width inside column
// 4, so it passes through the cells of BOTH rooms it separates.
//
// The footprint is what A11 is about. A wall between a visible room and a
// hidden one stands on cells of each, and the hidden ones have to reach the
// recipient's atlas or the floor ends in nothing.
func hugSeam(rows int) encounter.SegmentInput {
	seg := encounter.SegmentInput{
		Name:   "the seam",
		From:   encounter.AxialPointF{Q: 4, R: -0.5},
		To:     encounter.AxialPointF{Q: 4 - float64(rows), R: float64(rows) - 0.5},
		Height: 2,
	}
	for row := 0; row < rows; row++ {
		if row%2 == 0 {
			seg.Footprint = append(seg.Footprint, spatial.Position{X: 4, Y: float64(row)})
			continue
		}
		seg.Footprint = append(seg.Footprint, spatial.Position{X: 3, Y: float64(row)})
	}

	return seg
}

// hugField is a visible hall and a hidden vault either side of that one line,
// with a concealed door standing in it.
func (s *FootingSuite) hugField(rows int, withDoor bool) encounter.FieldInput {
	field := encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 4, rows),
			func() encounter.RegionInput {
				r := rectRegion("vault", 4, 0, 3, rows)
				r.Concealed = true
				return r
			}(),
		},
		// THE CROSSINGS AND THE SEGMENT CARRY THE SAME HEIGHT, because the
		// compiler derives both from one authored line and cannot make them
		// differ. A fixture that raised only the segment would be describing
		// a dungeon dungeonspec could not produce.
		Walls:    withHeight(seamWallExcept(3, rows), 2),
		Segments: []encounter.SegmentInput{hugSeam(rows)},
	}
	if !withDoor {
		return field
	}
	field.Walls = withHeight(seamWallExcept(3, rows, 1), 2)
	field.Doors = []encounter.DoorInput{{
		ID:        "panel",
		Edges:     doorEdgesAcross(3, 1),
		State:     encounter.DoorIsClosed(),
		Concealed: []encounter.CheckApproach{{Ability: "perception", DC: 15}},
	}}
	field.Segments[0].DoorIDs = []encounter.DoorID{"panel"}

	return field
}

// twinField is the dungeon a non-knower's atlas claims to be: the same hall,
// no vault at all, and SCENERY where the wall's footprint falls on the far
// side — floor that belongs to nobody, which is exactly what footing is.
func (s *FootingSuite) twinField(rows int) encounter.FieldInput {
	seam := hugSeam(rows)
	var scenery []spatial.Position
	for _, c := range seam.Footprint {
		if c.X >= 4 {
			scenery = append(scenery, c)
		}
	}

	return encounter.FieldInput{
		Canvas:   pointyCanvas(),
		Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 4, rows)},
		Scenery:  scenery,
		Walls:    nil,
		Segments: []encounter.SegmentInput{seam},
	}
}

func (s *FootingSuite) open(
	field encounter.FieldInput, resolver encounter.CheckResolver, members ...encounter.MemberInput,
) *encounter.Encounter {
	s.T().Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: resolver, Witness: s.witness,
		Field:   field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

// TestA11_APresentedWallStandsOnFloorTheWatcherCanSee is C18.
//
// The watcher cannot see the vault, so its cells are withheld — except the
// ones the seam stands on, which arrive as floor nobody owns. The whole atlas
// is then compared against the twin authored with scenery exactly there, which
// is the yardstick doing its job: not "the footing is present" but "the map is
// the map of a dungeon somebody could have drawn".
func (s *FootingSuite) TestA11_APresentedWallStandsOnFloorTheWatcherCanSee() {
	const rows = 6
	enc := s.open(s.hugField(rows, false), findsNothing{},
		encounter.MemberInput{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}})

	scoped, err := enc.AtlasFor(watcher)
	s.Require().NoError(err)

	twin, err := s.open(s.twinField(rows), findsNothing{},
		encounter.MemberInput{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}}).Atlas()
	s.Require().NoError(err)

	s.Equal(twin.Cells, scoped.Cells,
		"floor on both sides of the wall, and the far side belongs to nobody")
	s.Equal(twin.Regions, scoped.Regions, "the vault is still not there")
	s.Equal(twin.Sealed, scoped.Sealed, "and its footing is floor nobody stands on, like any scenery")
	s.Equal(twin.Segments, scoped.Segments, "the wall itself is the same line")

	// The half that can fail: the footing really is the vault's cells, and
	// they really were hidden. A projection that simply forgot to hide the
	// vault would pass the twin comparison only if the twin had the whole
	// vault in it, which it does not.
	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Len(full.Cells, 4*rows+3*rows, "the whole dungeon is hall plus vault")
	s.Len(scoped.Cells, 4*rows+rows/2, "the watcher gets the hall, plus the seam's own footing in the vault")
	for _, c := range scoped.Cells {
		s.Contains(full.Cells, c, "nothing is invented: every cell shown is a cell there is")
	}
}

// TestA11_AWithheldWallDoesNotFoot is the other half of C18, and the reason it
// says PRESENTED walls: a wall wholly inside hidden space is withheld with the
// room, footing and all. Footing a withheld wall would trace the secret's
// interior in floor.
func (s *FootingSuite) TestA11_AWithheldWallDoesNotFoot() {
	const rows = 6
	field := s.hugField(rows, false)
	// A second wall, deep inside the vault: every cell it stands on belongs
	// to the room the watcher cannot see.
	field.Segments = append(field.Segments, encounter.SegmentInput{
		Name:      "the inner buttress",
		From:      encounter.AxialPointF{Q: 6, R: 1.5},
		To:        encounter.AxialPointF{Q: 5, R: 3.5},
		Footprint: []spatial.Position{{X: 6, Y: 2}, {X: 5, Y: 3}},
	})

	enc := s.open(field, findsNothing{},
		encounter.MemberInput{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}})
	scoped, err := enc.AtlasFor(watcher)
	s.Require().NoError(err)

	s.Len(scoped.Segments, 1, "the buttress is withheld with the room it stands in")
	s.NotContains(scoped.Cells, cellAt(6, 2), "and so is the floor under it")
	s.NotContains(scoped.Cells, cellAt(5, 3))

	// And it is there for somebody who knows.
	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Len(full.Segments, 2)
}

// TestA10_AConcealedDoorPresentsTheWholeWall is C19.
//
// A door's gap is derived by whoever draws the map, from the doorway and the
// segment it stands in. So the non-knower's atlas carries the full segment and
// NO doorway — an unbroken wall — while the knower's carries the same segment
// and the doorway that cuts it. The wall is one line either way, which is the
// whole point: nothing about its shape says a door was subtracted.
func (s *FootingSuite) TestA10_AConcealedDoorPresentsTheWholeWall() {
	const rows = 6
	enc := s.open(s.hugField(rows, true), findsEverything{},
		encounter.MemberInput{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: knower, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 3}})

	blind, err := enc.AtlasFor(watcher)
	s.Require().NoError(err)
	s.Require().Len(blind.Segments, 1)
	s.Empty(blind.Doorways, "no doorway, so nothing cuts the line")
	s.Equal(hugSeam(rows).From, blind.Segments[0].From, "and the line runs its whole length")
	s.Equal(hugSeam(rows).To, blind.Segments[0].To)
	s.Equal(2.0, blind.Segments[0].Height, "at the height the wall was authored at")

	// The crossing the door stands in reads as wall, at the SAME height as
	// the wall it hides in — a standard-height patch inside a raised wall
	// would be a notch exactly where the secret is.
	mask, present := hasBoundary(blind, cellAt(3, 1), cellAt(4, 1))
	s.Require().True(present, "the concealed door's crossing reads as wall")
	s.Equal(2.0, mask.Height, "the height of the wall it stands in, read off the segment")

	// The knower gets the same wall and the doorway that cuts it.
	_, err = enc.Search(&encounter.SearchInput{Member: knower, Region: "hall"})
	s.Require().NoError(err)
	seen, err := enc.AtlasFor(knower)
	s.Require().NoError(err)
	s.Equal(blind.Segments, seen.Segments, "the wall does not change shape when the door is found")
	s.Require().Len(seen.Doorways, 1, "only the gap appears")
	s.Equal(encounter.DoorID("panel"), seen.Doorways[0].Door)

	// The yardstick: the non-knower's map is the map of a dungeon with a
	// solid wall there and no door at all.
	solid := s.open(s.hugField(rows, false), findsNothing{},
		encounter.MemberInput{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}})
	twin, err := solid.AtlasFor(watcher)
	s.Require().NoError(err)
	s.Equal(twin.Cells, blind.Cells)
	s.Equal(twin.Segments, blind.Segments)
	s.Equal(twin.Doorways, blind.Doorways)
	s.Equal(twin.Boundaries, blind.Boundaries,
		"every crossing reads the same, the door's included")
}

// TestA10_AMaskTakesTheHeightOfTheWallItHidesIn is the half of C19 the seam
// scene above cannot reach.
//
// In a long seam the masked crossing has authored walls on both its cells —
// the rest of the same wall — so a mask could read the height off a
// NEIGHBOURING crossing and look right. Here the wall's only crossing is the
// door's, and it was subtracted, so there is no neighbouring crossing to read:
// a secret panel in a short stretch of wall, which is the ordinary shape of
// the thing. The only place the height still exists is the segment.
//
// Get this wrong and the panel presents at standard height inside a raised
// wall — a notch exactly where the secret is, which is the tell the whole rule
// exists to remove.
func (s *FootingSuite) TestA10_AMaskTakesTheHeightOfTheWallItHidesIn() {
	const rows = 4
	field := encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 4, rows),
			func() encounter.RegionInput {
				r := rectRegion("vault", 4, 0, 2, rows)
				r.Concealed = true
				return r
			}(),
		},
		// No authored walls at all: the panel's crossing was the wall's only
		// one, and a door's crossing is subtracted from the wall list.
		Doors: []encounter.DoorInput{{
			ID:        "panel",
			Edges:     doorEdgesAcross(3, 1),
			State:     encounter.DoorIsClosed(),
			Concealed: []encounter.CheckApproach{{Ability: "perception", DC: 15}},
		}},
		Segments: []encounter.SegmentInput{{
			Name:      "the panel's wall",
			From:      encounter.AxialPointF{Q: 3.5, R: 1},
			To:        encounter.AxialPointF{Q: 2.5, R: 3},
			Height:    3,
			Footprint: []spatial.Position{{X: 3, Y: 1}, {X: 4, Y: 1}},
			DoorIDs:   []encounter.DoorID{"panel"},
		}},
	}

	enc := s.open(field, findsNothing{},
		encounter.MemberInput{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}})
	blind, err := enc.AtlasFor(watcher)
	s.Require().NoError(err)

	mask, present := hasBoundary(blind, cellAt(3, 1), cellAt(4, 1))
	s.Require().True(present, "the panel's crossing reads as wall")
	s.Equal(3.0, mask.Height,
		"the height of the wall it hides in, which only the segment still knows")
	s.Empty(blind.Doorways, "and the gap is withheld with the door")
}

// TestASealedCellKeepsItsRoomAndLosesItsFeet is the runtime half of design
// C10, and the whole of what slice 2 changed about standing.
//
// A cell a wall halves keeps its OWNER — its region, its lighting, its
// archetype, its place in the atlas — and loses only feet. That split is why
// membership in a region stopped implying standable, and why the atlas has to
// say which cells are sealed rather than leaving a client to work it out from
// the region lists.
//
// Every caller that asks "may feet be here" got the new answer without being
// touched: a seat at construction, a step in play, an ending's trigger cell.
// The refusal says WHICH of the three reasons it met, because "is not floor"
// would be a lie about a cell the atlas lists as floor.
func (s *FootingSuite) TestASealedCellKeepsItsRoomAndLosesItsFeet() {
	field := encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 4)},
		Sealed:  []spatial.Position{{X: 3, Y: 1}},
	}
	enc := s.open(field, findsNothing{},
		encounter.MemberInput{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}})

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Contains(atlas.Cells, cellAt(3, 1), "the cell is still floor")
	s.Equal([]spatial.Position{cellAt(3, 1)}, atlas.Sealed, "and the atlas says nobody stands on it")

	region, owned := enc.RegionAt(cellAt(3, 1))
	s.Require().True(owned, "it keeps its room, its lighting and its archetype")
	s.Equal(encounter.RegionID("hall"), region)
	for _, r := range atlas.Regions {
		if r.ID == "hall" {
			s.Contains(r.Cells, cellAt(3, 1), "and its place in the room's own cell list")
		}
	}

	// A step onto it is refused, saying which of the three reasons this is.
	_, err = enc.Step(&encounter.StepInput{Member: watcher, To: cellAt(3, 1)})
	s.Require().Error(err)
	s.Contains(err.Error(), "is sealed: a wall leaves no room to stand there")

	// And a seat on it is refused at construction, for the same reason.
	_, err = encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
		Field:   field,
		Members: []encounter.MemberInput{{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "is sealed")

	// A SEALED CELL MUST BE A ROOM'S. Sealing something nobody owns is either
	// void (a defect) or scenery (already unstandable, and saying so twice
	// would give the runtime two lists to disagree about).
	orphan := field
	orphan.Sealed = []spatial.Position{{X: 40, Y: 40}}
	_, err = encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
		Field:   orphan,
		Members: []encounter.MemberInput{{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "is not a region's cell")
}

// TestSegmentsAndSealedRideThePersistence — a saved encounter comes back with
// the same answers, and Load never recomputes them.
//
// THE AREA RULE LIVES IN THE AUTHORING COMPILER AND NOWHERE ELSE (design C9).
// A Load that re-derived which cells are sealed would be a second
// implementation of the one rule, free to drift from the compiler that wrote
// the dungeon in the first place — so the list rides the blob and Load reads
// it, exactly as the regions do.
func (s *FootingSuite) TestSegmentsAndSealedRideThePersistence() {
	const rows = 6
	field := s.hugField(rows, true)
	field.Sealed = []spatial.Position{{X: 5, Y: 2}}

	enc := s.open(field, findsNothing{},
		encounter.MemberInput{ID: watcher, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}})
	before, err := enc.Atlas()
	s.Require().NoError(err)

	data := enc.ToData()
	s.Require().Len(data.Field.Segments, 1, "the wall's line is written out")
	s.Equal([]encounter.DoorID{"panel"}, data.Field.Segments[0].DoorIDs)
	s.Require().Len(data.Field.Sealed, 1, "and so is what it sealed")

	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
	})
	s.Require().NoError(err)

	after, err := loaded.Atlas()
	s.Require().NoError(err)
	s.Equal(before.Segments, after.Segments, "the same lines")
	s.Equal(before.Sealed, after.Sealed, "and the same cells nobody stands on")
	s.Equal(before.Cells, after.Cells)

	scoped, err := loaded.AtlasFor(watcher)
	s.Require().NoError(err)
	fresh, err := enc.AtlasFor(watcher)
	s.Require().NoError(err)
	s.Equal(fresh, scoped, "and the projection over them is the same too")
}
