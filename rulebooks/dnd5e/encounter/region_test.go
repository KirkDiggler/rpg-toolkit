// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// region_test.go is THE REGIONS ARE THE FLOOR (rpg-project#256), pinned at
// both construction seams: a region is a named set of absolute cells, the
// floor is their union, and every way a region can be wrong is refused by
// name at Setup AND at Load.

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type RegionSetupSuite struct {
	suite.Suite
}

func TestRegionSetupSuite(t *testing.T) {
	suite.Run(t, new(RegionSetupSuite))
}

// threeRegions is an L-shaped hall between two rectangles, anchored away from
// the origin so an authored pair read as an axial cell is visibly wrong.
func threeRegions() []encounter.RegionInput {
	hall := rectRegion("hall", 10, 4, 4, 2)
	hall.Cells = append(hall.Cells, rectCells(10, 6, 2, 3)...) // the L's foot
	hall.Lighting = &encounter.Lighting{Intensity: 0.4}
	hall.Archetype = "cavern"
	return []encounter.RegionInput{
		rectRegion("entrance", 6, 4, 4, 5),
		hall,
		rectRegion("tomb", 14, 4, 3, 5),
	}
}

func (s *RegionSetupSuite) setupWith(regions []encounter.RegionInput) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field:   encounter.FieldInput{Canvas: pointyCanvas(), Regions: regions},
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 4}}},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
}

// loadWith opens the valid field, then hands Load a blob with one region
// edited — so the SAME defect is asked of the other seam.
func (s *RegionSetupSuite) loadWith(edit func(*encounter.RegionData)) error {
	enc, err := s.setupWith(threeRegions())
	s.Require().NoError(err)
	data := enc.ToData()
	edit(&data.Field.Regions[1])
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	return err
}

// TestSetup_RegionsMakeTheFloor: three regions, and the floor is exactly
// their union — every authored cell answers to its owner, sorted once, and
// a cell nobody painted is void.
func (s *RegionSetupSuite) TestSetup_RegionsMakeTheFloor() {
	regions := threeRegions()
	enc, err := s.setupWith(regions)
	s.Require().NoError(err)

	var want []spatial.Position
	owner := map[spatial.Position]string{}
	for _, r := range regions {
		for _, c := range r.Cells {
			cell := cellAt(int(c.X), int(c.Y))
			want = append(want, cell)
			owner[cell] = r.ID
		}
	}
	sortPositions(want)

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Equal(want, atlas.Cells, "this field authors no scenery, so its floor is the regions' cells, sorted")
	s.Len(atlas.Cells, 20+14+15)

	for cell, id := range owner {
		got, ok := enc.RegionAt(cell)
		s.Require().True(ok, "cell %v is floor", cell)
		s.Equal(encounter.RegionID(id), got, "cell %v", cell)
	}

	_, ok := enc.RegionAt(cellAt(5, 4))
	s.False(ok, "one column west of the entrance is void")
	_, ok = enc.RegionAt(cellAt(12, 7))
	s.False(ok, "the notch of the L is void too: there is no other floor")

	// And the Atlas agrees region by region.
	s.Require().Len(atlas.Regions, 3)
	s.Equal([]string{"entrance", "hall", "tomb"},
		[]string{atlas.Regions[0].ID, atlas.Regions[1].ID, atlas.Regions[2].ID}, "sorted by ID (C8)")
	s.Len(atlas.Regions[1].Cells, 14)
	s.Equal("cavern", atlas.Regions[1].Archetype)
	s.Equal(0.4, atlas.Regions[1].Lighting.Intensity)
}

func (s *RegionSetupSuite) TestSetup_RefusesOverlap() {
	regions := threeRegions()
	regions[2].Cells = append(regions[2].Cells, spatial.Position{X: 11, Y: 5}) // a hall cell, painted twice
	_, err := s.setupWith(regions)
	s.Require().ErrorIs(err, encounter.ErrRegionOverlap)
	s.Contains(err.Error(), `"hall"`, "names the region that already owns it")
	s.Contains(err.Error(), "cells[15]", "and the cell by its index in the offending region")

	s.Run("within one region", func() {
		regions := threeRegions()
		regions[0].Cells = append(regions[0].Cells, regions[0].Cells[3])
		_, err := s.setupWith(regions)
		s.Require().ErrorIs(err, encounter.ErrRegionOverlap)
		s.Contains(err.Error(), "listed twice")
	})

	s.Run("and at Load", func() {
		err := s.loadWith(func(r *encounter.RegionData) {
			r.Cells = append(r.Cells, encounter.PositionData{X: 6, Y: 4})
		})
		s.Require().ErrorIs(err, encounter.ErrRegionOverlap)
		s.ErrorIs(err, encounter.ErrInvalidData)
	})
}

func (s *RegionSetupSuite) TestSetup_RefusesEmptyRegion() {
	regions := threeRegions()
	regions[1].Cells = nil
	_, err := s.setupWith(regions)
	s.Require().ErrorIs(err, encounter.ErrRegionEmpty)
	s.Contains(err.Error(), `"hall"`)

	err = s.loadWith(func(r *encounter.RegionData) { r.Cells = nil })
	s.Require().ErrorIs(err, encounter.ErrRegionEmpty)
	s.ErrorIs(err, encounter.ErrInvalidData)
}

func (s *RegionSetupSuite) TestSetup_RefusesMissingLighting() {
	regions := threeRegions()
	regions[1].Lighting = nil
	_, err := s.setupWith(regions)
	s.Require().ErrorIs(err, encounter.ErrRegionLightingMissing)
	s.Contains(err.Error(), `"hall"`)

	s.Run("at Load, a missing block", func() {
		err := s.loadWith(func(r *encounter.RegionData) { r.Lighting = nil })
		s.Require().ErrorIs(err, encounter.ErrRegionLightingMissing)
	})
	s.Run("at Load, a block that omits its intensity", func() {
		err := s.loadWith(func(r *encounter.RegionData) { r.Lighting.Intensity = nil })
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "intensity")
	})
	s.Run("an intensity outside [0,1]", func() {
		regions := threeRegions()
		regions[1].Lighting = &encounter.Lighting{Intensity: 1.2}
		_, err := s.setupWith(regions)
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "1.2")

		regions[1].Lighting = &encounter.Lighting{Intensity: math.NaN()}
		_, err = s.setupWith(regions)
		s.Require().ErrorIs(err, encounter.ErrNoField, "NaN is not in [0,1] either")
	})
	s.Run("zero is an answer, not an absence", func() {
		regions := threeRegions()
		regions[1].Lighting = &encounter.Lighting{Intensity: 0}
		enc, err := s.setupWith(regions)
		s.Require().NoError(err)
		data := enc.ToData()
		s.Require().NotNil(data.Field.Regions[1].Lighting.Intensity)
		s.Equal(0.0, *data.Field.Regions[1].Lighting.Intensity, "persisted as an explicit 0")
	})
}

func (s *RegionSetupSuite) TestSetup_RefusesMissingArchetype() {
	regions := threeRegions()
	regions[1].Archetype = ""
	_, err := s.setupWith(regions)
	s.Require().ErrorIs(err, encounter.ErrRegionArchetypeMissing)

	err = s.loadWith(func(r *encounter.RegionData) { r.Archetype = "" })
	s.Require().ErrorIs(err, encounter.ErrRegionArchetypeMissing)
	s.ErrorIs(err, encounter.ErrInvalidData)
}

// TestAnArchetypeNeverDecidesAMechanic is the law stated as a test: two
// fields identical but for every region's archetype produce the same atlas
// (minus the word), the same floor, and the same sightlines. v1's archetype
// chose where the party stood; nothing here may read it.
func (s *RegionSetupSuite) TestAnArchetypeNeverDecidesAMechanic() {
	crypt, err := s.setupWith(threeRegions())
	s.Require().NoError(err)

	regions := threeRegions()
	for i := range regions {
		regions[i].Archetype = "open-air-ruin"
	}
	ruin, err := s.setupWith(regions)
	s.Require().NoError(err)

	a, err := crypt.Atlas()
	s.Require().NoError(err)
	b, err := ruin.Atlas()
	s.Require().NoError(err)
	for i := range a.Regions {
		a.Regions[i].Archetype, b.Regions[i].Archetype = "", ""
	}
	s.Equal(a, b, "the archetype is carried and never read")

	ca, err := crypt.Canvas()
	s.Require().NoError(err)
	cb, err := ruin.Canvas()
	s.Require().NoError(err)
	s.Equal(ca.IsLineOfSightBlocked(cellAt(6, 4), cellAt(16, 8)), cb.IsLineOfSightBlocked(cellAt(6, 4), cellAt(16, 8)))
}

func (s *RegionSetupSuite) TestSetup_RefusesRegionDefectsByName() {
	s.Run("no regions", func() {
		_, err := s.setupWith(nil)
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "no regions")
	})
	s.Run("empty id", func() {
		regions := threeRegions()
		regions[1].ID = ""
		_, err := s.setupWith(regions)
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "regions[1]")
	})
	s.Run("duplicate id", func() {
		regions := threeRegions()
		regions[1].ID = "tomb"
		_, err := s.setupWith(regions)
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), `duplicate region "tomb"`)
	})
	s.Run("a fractional cell", func() {
		regions := threeRegions()
		regions[1].Cells[0] = spatial.Position{X: 10.5, Y: 4}
		_, err := s.setupWith(regions)
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "cells[0]")
	})
	s.Run("more cells than the field budget", func() {
		// 2049 x 2049 is 4,198,401 cells — one past 1<<22. The budget is
		// checked before any cell is converted, so this costs the input
		// slice and nothing else.
		_, err := s.setupWith([]encounter.RegionInput{rectRegion("vast", 0, 0, 2049, 2049)})
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Contains(err.Error(), "more than 4194304 cells")
	})
	s.Run("a cell past the coordinate bound", func() {
		regions := threeRegions()
		regions[1].Cells[0] = spatial.Position{X: 1 << 31, Y: 4}
		_, err := s.setupWith(regions)
		s.Require().ErrorIs(err, encounter.ErrNoField)
	})
}

// TestAFieldSaysWhichWayItsHexesPoint — required, at both seams, because an
// offset pair means nothing until the orientation is known.
func (s *RegionSetupSuite) TestAFieldSaysWhichWayItsHexesPoint() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field:   encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()}, Regions: threeRegions()},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Contains(strings.ToLower(err.Error()), "orientation")

	enc, err := s.setupWith(threeRegions())
	s.Require().NoError(err)
	for _, word := range []string{"", "sideways"} {
		data := enc.ToData()
		data.Field.Canvas.Orientation = word
		_, lerr := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data: data, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
			Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		})
		s.Require().ErrorIs(lerr, encounter.ErrNoField, "orientation %q", word)
		s.Contains(lerr.Error(), "orientation")
	}
}

// TestTheOrientationSurvivesASave — construction truth; a reload that forgot
// it would read every stored cell in the wrong frame.
func (s *RegionSetupSuite) TestTheOrientationSurvivesASave() {
	flat := encounter.HexesAreFlatTop()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field:   encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: flat}, Regions: threeRegions()},
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 15, Y: 8}}},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	data := enc.ToData()
	s.Equal("flat", data.Field.Canvas.Orientation)

	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)

	region, floor := back.RegionAt(encounter.HexCellAt(flat, 15, 8))
	s.True(floor)
	s.Equal("tomb", region)
	_, floor = back.RegionAt(encounter.HexCellAt(encounter.HexesArePointyTop(), 15, 8))
	s.False(floor, "read under the other layout, the same pair is somewhere else — which is the whole reason it persists")

	atlas, err := back.Atlas()
	s.Require().NoError(err)
	s.Equal(encounter.OrientationFlatTop, atlas.Orientation.Kind())
}

// TestEdges_MustBeAdjacentUnderOrientation is the discriminator (the
// rpg-toolkit#1141/#1150 lesson): the SAME authored [col,row] pair is a
// crossing under one layout and not the other, and the oracle is a pixel
// formula per orientation — two formulas that are NOT a swapped pair of
// each other — rather than anything the composition computes.
//
// Under pointy-top (odd-r) the centre of [col,row] is
// (sqrt3 * (col + (row&1)/2), 1.5 * row); under flat-top (odd-q) it is
// (1.5 * col, sqrt3 * (row + (col&1)/2)). Two cells are neighbours iff their
// centres are exactly sqrt3 apart. Every pair in a window is asked of both
// formulas and of compileField, through a wall, and the three must agree.
func (s *RegionSetupSuite) TestEdges_MustBeAdjacentUnderOrientation() {
	const size = 4
	centre := func(o encounter.Orientation, col, row int) (float64, float64) {
		if o.Kind() == encounter.OrientationPointyTop {
			return math.Sqrt(3) * (float64(col) + 0.5*float64(row&1)), 1.5 * float64(row)
		}
		return 1.5 * float64(col), math.Sqrt(3) * (float64(row) + 0.5*float64(col&1))
	}
	pixelAdjacent := func(o encounter.Orientation, a, b [2]int) bool {
		ax, ay := centre(o, a[0], a[1])
		bx, by := centre(o, b[0], b[1])
		return math.Abs(math.Hypot(bx-ax, by-ay)-math.Sqrt(3)) < 1e-9
	}
	build := func(o encounter.Orientation, a, b [2]int) error {
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			Field: encounter.FieldInput{
				Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: o},
				Regions: []encounter.RegionInput{rectRegion("window", 0, 0, size, size)},
				Walls:   []encounter.WallInput{wall(a[0], a[1], b[0], b[1])},
			},
			Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		})
		return err
	}

	// The two named pairs first, so the discriminator is legible: each is
	// a crossing under exactly one layout.
	pointy, flat := encounter.HexesArePointyTop(), encounter.HexesAreFlatTop()
	s.Require().NoError(build(pointy, [2]int{0, 1}, [2]int{1, 2}), "[0,1]-[1,2] touch under pointy-top")
	s.Require().ErrorIs(build(flat, [2]int{0, 1}, [2]int{1, 2}), encounter.ErrEdgeNotAdjacent, "and not under flat-top")
	s.Require().ErrorIs(build(pointy, [2]int{1, 0}, [2]int{2, 1}), encounter.ErrEdgeNotAdjacent, "[1,0]-[2,1] do not touch under pointy-top")
	s.Require().NoError(build(flat, [2]int{1, 0}, [2]int{2, 1}), "and do under flat-top")

	// Then every pair in the window, against the pixel formula.
	for _, o := range []encounter.Orientation{pointy, flat} {
		s.Run(string(o.Kind()), func() {
			accepted, refused := 0, 0
			for ac := 0; ac < size; ac++ {
				for ar := 0; ar < size; ar++ {
					for bc := 0; bc < size; bc++ {
						for br := 0; br < size; br++ {
							a, b := [2]int{ac, ar}, [2]int{bc, br}
							if a == b {
								continue
							}
							err := build(o, a, b)
							if pixelAdjacent(o, a, b) {
								s.Require().NoError(err, "%v-%v are neighbours on the screen", a, b)
								accepted++
							} else {
								s.Require().ErrorIs(err, encounter.ErrEdgeNotAdjacent, "%v-%v are not", a, b)
								refused++
							}
						}
					}
				}
			}
			s.Positive(accepted)
			s.Positive(refused)
		})
	}
}

// TestEdgesMustStandOnTheFloor — the envelope is implied, never written.
func (s *RegionSetupSuite) TestEdgesMustStandOnTheFloor() {
	build := func(walls []encounter.WallInput, doors []encounter.DoorInput) error {
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			Field:   encounter.FieldInput{Canvas: pointyCanvas(), Regions: threeRegions(), Walls: walls, Doors: doors},
			Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		})
		return err
	}

	err := build([]encounter.WallInput{wall(6, 4, 5, 4)}, nil)
	s.Require().ErrorIs(err, encounter.ErrEdgeOffFloor)
	s.Contains(err.Error(), "walls[0]")
	s.Contains(err.Error(), "[5,4]")

	err = build(nil, []encounter.DoorInput{openDoorway("rim", 6, 4, 5, 4)})
	s.Require().ErrorIs(err, encounter.ErrDoorEdgeOffFloor)
	s.ErrorIs(err, encounter.ErrBadDoor)

	err = build(nil, []encounter.DoorInput{openDoorway("far", 6, 4, 8, 4)})
	s.Require().ErrorIs(err, encounter.ErrEdgeNotAdjacent)
	s.ErrorIs(err, encounter.ErrBadDoor)

	err = build([]encounter.WallInput{wall(6, 4, 7, 4), wall(7, 4, 6, 4)}, nil)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Contains(err.Error(), "listed twice", "the same edge either way round is the same edge")

	s.Require().NoError(build(nil, []encounter.DoorInput{openDoorway("inside", 7, 5, 8, 5)}),
		"a door inside a region, on no seam, is legal (TestCompile_DoorInsideARegionIsLegal's encounter half)")
}

func sortPositions(cells []spatial.Position) {
	for i := 1; i < len(cells); i++ {
		for j := i; j > 0 && (cells[j].X < cells[j-1].X || (cells[j].X == cells[j-1].X && cells[j].Y < cells[j-1].Y)); j-- {
			cells[j], cells[j-1] = cells[j-1], cells[j]
		}
	}
}
