// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// sight_pass_memo_internal_test.go is the sight pass memo (rpg-toolkit#2097):
// within one pass, every observer reads the same answer for a cell's origin
// opacity and a placement's cells, computed once; and the memo dies with the
// pass, so the next pass reads the field as it now stands.
//
// White-box because the memo and its counts are the field's own business:
// nothing outside the package can tell one computation from forty but the
// clock.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sightPassWatchers are the four members of the sight pass scene.
var sightPassWatchers = []MemberID{"north-west", "north-east", "south-west", "south-east"}

// passSight is everybody seeing sixty cells, recording whether each time it
// is asked a sight pass's memo is live.
type passSight struct {
	field  **field
	inPass *[]bool
}

func (s passSight) Sight(members []MemberID) (map[MemberID]int, error) {
	*s.inPass = append(*s.inPass, *s.field != nil && (*s.field).sightPass != nil)
	out := make(map[MemberID]int, len(members))
	for _, m := range members {
		out[m] = 60
	}
	return out, nil
}

// sightPassScene is a hall with a sight-blocking pillar and a shut footprint
// door standing in its middle, and a watcher in each corner.
func sightPassScene(t *testing.T) (*Encounter, *[]bool) {
	t.Helper()
	regions := []RegionInput{rectRegion("hall", 0, 0, 12, 10)}
	probe := prefilterEncounter(t, FieldInput{Canvas: openAir(), Regions: regions}, spatial.Position{})
	at := func(col, row int) spatial.Position {
		return probe.field.cellAt(spatial.Position{X: float64(col), Y: float64(row)})
	}
	centre := func(col, row int) spatial.Point { return probe.field.plane.CellCentre(at(col, row)) }

	pillar := centre(6, 5)
	door := centre(3, 3)
	hall := FieldInput{
		Canvas: openAir(), Regions: regions,
		Placed: []PlacedPropInput{{ID: "pillar", BlocksMovement: true, BlocksLineOfSight: true,
			Placement: boxPlacement(4, 4, 0, pillar, spatial.Point{})}},
		Doors: []DoorInput{{ID: "door/hall", State: DoorIsClosed(),
			Placement: &spatial.FootprintPlacement{
				Footprint: spatial.Footprint{Box: &spatial.Box{W: 1, D: 6}},
				Origin:    door,
			}}},
	}
	corners := [][2]int{{1, 1}, {10, 1}, {1, 8}, {10, 8}}
	members := make([]MemberInput, 0, len(sightPassWatchers))
	for i, id := range sightPassWatchers {
		members = append(members, MemberInput{ID: id, Kind: KindPlayer, Position: spatial.Position{X: float64(corners[i][0]), Y: float64(corners[i][1])}})
	}

	var enc *Encounter
	var f *field
	inPass := &[]bool{}
	enc, err := NewEncounter(&SetupInput{
		Field:   hall,
		Members: members,
		Endings: []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:         passSight{field: &f, inPass: inPass},
			Equipment:     UnobservedEquipment{},
			Conditions:    UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors:        Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
		},
	})
	require.NoError(t, err)
	f = enc.field

	return enc, inPass
}

// TestASightPassComputesItsSharedAnswersOnceForEveryWatcher: the pass asks
// the Sight capability with its memo live, and inside one pass four watchers
// each ask the sight question of every floor cell and build their atlas
// twice, as an observer that learns cells does. Each cell's origin opacity
// is computed at most once. The pillar's cells are never computed in the
// pass at all: an authored placement's cells are the placement index's,
// derived when the field was built (placement_index.go, rpg-project#753).
// Without the memo the opacity is computed for every lane end of every
// watcher's every cell; without the index the pillar's cells were the
// memo's, computed once per pass.
func TestASightPassComputesItsSharedAnswersOnceForEveryWatcher(t *testing.T) {
	enc, inPass := sightPassScene(t)
	f := enc.field

	*inPass = nil
	_, _, err := enc.refreshSight(sightPassWatchers)
	require.NoError(t, err)
	require.NotEmpty(t, *inPass, "precondition: the pass asked the Sight capability")
	for _, live := range *inPass {
		require.True(t, live, "the pass asks sight with its memo live")
	}
	require.Nil(t, f.sightPass, "and drops the memo when it returns")

	positions := map[MemberID]spatial.Position{}
	cells := map[MemberID]int{}
	for _, id := range sightPassWatchers {
		positions[id], _ = enc.canvas.GetEntityPosition(string(id))
		cells[id] = 60
	}
	reach := enc.newSightReach(positions, cells)

	opacity, placements := f.originOpacityComputed, f.placedCellsComputed
	end := f.beginSightPass()
	asked := 0
	for _, id := range sightPassWatchers {
		for _, cell := range f.cells {
			reach.reachesCell(id, cell)
			asked++
		}
		for range 2 {
			_, err := enc.AtlasFor(id)
			require.NoError(t, err)
		}
	}
	end()

	require.Greater(t, asked, len(f.cells), "precondition: the watchers ask more cells than the floor holds")
	require.Positive(t, f.originOpacityComputed-opacity, "precondition: the pass asked origin opacity")
	require.LessOrEqual(t, f.originOpacityComputed-opacity, len(f.cells),
		"each cell's origin opacity is computed at most once in the pass")
	require.Equal(t, 0, f.placedCellsComputed-placements,
		"the pillar's cells are read from the placement index for the four watchers' eight atlases")
}

// TestTheSightPassMemoDiesWithThePass: in one pass the cell under the shut
// door is opaque; the pass ends, the door opens, and the next pass computes
// the cell afresh and finds it clear. A memo that outlived its pass would
// answer the second pass from the first, with the door still shut.
func TestTheSightPassMemoDiesWithThePass(t *testing.T) {
	enc, _ := sightPassScene(t)
	f := enc.field
	require.Len(t, f.doorFootprints, 1, "precondition: the hall has one footprint door")
	door := f.doorFootprints[0]
	var under spatial.Position
	found := false
	for _, cell := range f.cells {
		if f.plane.CellCentre(cell) == door.placement.Origin {
			under, found = cell, true
		}
	}
	require.True(t, found, "precondition: the door stands on a cell centre")

	end := f.beginSightPass()
	covered, err := f.sightBlocksOriginAt(under)
	require.NoError(t, err)
	end()
	require.True(t, covered, "precondition: the shut door covers the cell under it")

	door.state = DoorIsOpen()

	end = f.beginSightPass()
	before := f.originOpacityComputed
	covered, err = f.sightBlocksOriginAt(under)
	require.NoError(t, err)
	end()
	require.Equal(t, 1, f.originOpacityComputed-before, "the next pass computes the cell afresh")
	require.False(t, covered, "and finds it clear with the door open")
}
