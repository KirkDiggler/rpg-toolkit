// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// reach_door_test.go is THE DOOR AND THE OFFER MEASURE ALIKE (rpg-project#753,
// S3): [encounter.Encounter.PropWithinReach] is Speak's own prop door,
// exported for a host's pre-charge check; it admits every prop
// [encounter.Encounter.WithinReach] offers, a footprint by any cell it stands
// on, and refuses a shown prop beyond reach as out of range and a hidden one
// as no such thing. And the projection reads the placement index, measuring
// no authored placement's floor.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// requireDoorAdmitsTheOffer asks PropWithinReach of every prop WithinReach
// offers each member, and returns how many it asked.
func requireDoorAdmitsTheOffer(t *testing.T, enc *encounter.Encounter, members ...encounter.MemberID) int {
	t.Helper()
	asked := 0
	for _, member := range members {
		_, props, err := enc.WithinReach(member, speakReach)
		require.NoError(t, err)
		for _, prop := range props {
			require.NoError(t, enc.PropWithinReach(member, prop, speakReach), "%s is offered %s", member, prop)
			asked++
		}
	}
	return asked
}

// TestPropWithinReach_AgreesWithWithinReach: on a hall built for the three
// answers, and on the castle and the keep for the door admitting every offer.
func TestPropWithinReach_AgreesWithWithinReach(t *testing.T) {
	const (
		hero   = encounter.MemberID("hero")
		table  = encounter.PropID("table")  // a wide one: an edge within reach, its origin far
		altar  = encounter.PropID("altar")  // shown, ten hexes off
		secret = encounter.PropID("secret") // two hexes off, under an unfound secret
	)
	box := func(w, d float64, cell spatial.Position) spatial.FootprintPlacement {
		return spatial.FootprintPlacement{Footprint: spatial.Footprint{Box: &spatial.Box{W: w, D: d}}, Origin: centreOf(cell)}
	}
	enc := openHall(t,
		[]encounter.MemberInput{{ID: hero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 10}}},
		[]encounter.PlacedPropInput{
			{ID: table, Placement: box(30, 30, cellAt(4, 15))},
			{ID: altar, Placement: box(1, 1, cellAt(12, 10))},
			{ID: secret, Placement: box(1, 1, cellAt(4, 10))},
		},
		nil,
		[]encounter.ConcealmentInput{{
			ID: "the-secret", Checks: []encounter.CheckApproach{{Ability: "perception", DC: 30}}, Props: []encounter.PropID{secret},
		}},
		findsNothing{},
	)
	at := positionOfMember(t, enc, hero)
	atlas, err := enc.AtlasFor(hero)
	require.NoError(t, err)
	var tableCells []spatial.Position
	for _, p := range atlas.Placed {
		if p.ID == table {
			tableCells = p.Cells
		}
	}
	require.Greater(t, enc.Distance(at, cellAt(4, 15)), float64(speakReach), "precondition: the table's origin is beyond reach")
	nearest := enc.Distance(at, tableCells[0])
	for _, cell := range tableCells {
		nearest = min(nearest, enc.Distance(at, cell))
	}
	require.LessOrEqual(t, nearest, float64(speakReach), "precondition: one cell of it is within reach")

	_, offered, err := enc.WithinReach(hero, speakReach)
	require.NoError(t, err)
	require.Equal(t, []encounter.PropID{table}, offered, "precondition: the table alone is offered")
	require.Equal(t, 1, requireDoorAdmitsTheOffer(t, enc, hero))

	require.ErrorIs(t, enc.PropWithinReach(hero, altar, speakReach), encounter.ErrOutOfRange, "a shown altar ten off")
	require.NoError(t, enc.PropWithinReach(hero, altar, 10), "and within a reach of ten")
	require.ErrorIs(t, enc.PropWithinReach(hero, secret, speakReach), encounter.ErrNoProp,
		"a secret within reach is no such thing, never out of range")
	require.ErrorIs(t, enc.PropWithinReach(hero, "nothing", speakReach), encounter.ErrNoProp)
	require.ErrorIs(t, enc.PropWithinReach("nobody", table, speakReach), encounter.ErrNotMember)

	castle, _ := openSite(t, 0, true)
	requireDoorAdmitsTheOffer(t, castle, keepPlayers...)
	requireDoorAdmitsTheOffer(t, sharedKeep10k(t), keepPlayers[:2]...)
}

// TestWithinReach_CellPropsOffTheFloor: the two cell-prop guards in
// WithinReach's record loop. A cell prop in reserve within reach is not
// offered, and neither is one in somebody's hands — whose record names no
// cell at all, so without its guard it would be offered at the origin, which
// the hero stands beside.
func TestWithinReach_CellPropsOffTheFloor(t *testing.T) {
	const (
		hero   = encounter.MemberID("hero")
		porter = encounter.MemberID("porter")
		pin    = encounter.PropID("pin")   // holdable, the porter takes it
		early  = encounter.PropID("early") // reserved, never arrives
	)
	reserved := holdableProp(early, "test:props:early", spatial.Position{X: 2, Y: 1})
	reserved.Holdable = false
	reserved.Arrives = encounter.TriggerRound{Round: 99}
	enc := openHall(t,
		[]encounter.MemberInput{
			{ID: hero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: porter, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 3}},
		},
		nil,
		[]encounter.PropInput{holdableProp(pin, "test:props:pin", spatial.Position{X: 1, Y: 4}), reserved},
		nil, findsNothing{},
	)
	at := positionOfMember(t, enc, hero)
	origin := spatial.Position{}
	require.LessOrEqual(t, enc.Distance(at, origin), float64(speakReach), "precondition: the origin is within reach")
	visible, err := enc.VisibleCellsFor(&encounter.VisibleCellsInput{Member: hero})
	require.NoError(t, err)
	require.Contains(t, visible.Cells, origin, "precondition: and seen")
	require.Contains(t, visible.Cells, cellAt(2, 1), "precondition: the reserved prop's cell is seen")

	_, props, err := enc.WithinReach(hero, speakReach)
	require.NoError(t, err)
	require.Equal(t, []encounter.PropID{pin}, props, "the pin on the floor; the reserved prop has not arrived")

	_, err = enc.Hold(&encounter.HoldInput{Member: porter, Target: pin, Range: 1})
	require.NoError(t, err)
	_, props, err = enc.WithinReach(hero, speakReach)
	require.NoError(t, err)
	require.Empty(t, props, "a held pin stands nowhere, the origin included")
}

// TestAtlas_ReadsTheIndex: the whole-field projection and a member's read the
// placement index for every authored placement and wall, on the 10k keep —
// no per-placement floor measure runs to build either.
func TestAtlas_ReadsTheIndex(t *testing.T) {
	enc := sharedKeep10k(t)
	before := enc.ExportedCellsMeasured()
	full, err := enc.Atlas()
	require.NoError(t, err)
	require.NotEmpty(t, full.Placed, "precondition: the keep has placements")
	require.NotEmpty(t, full.StructuralWalls, "precondition: and walls")
	_, err = enc.AtlasFor(keepPlayers[0])
	require.NoError(t, err)
	require.Equal(t, before, enc.ExportedCellsMeasured(), "no placement's or wall's cells are measured")
}

// TestPlacedReach_AgreesWithTheWalk: every placement a lane or a centre
// touches is one the reach index names, on the castle and the 10k keep —
// seeded lanes up to sight length, and their start cells' centres.
func TestPlacedReach_AgreesWithTheWalk(t *testing.T) {
	castle, _ := openSite(t, 0, true)
	touched, err := castle.ExportedPlacedReachAgrees(2000, 25, 1)
	require.NoError(t, err, "castle")
	require.Positive(t, touched, "precondition: castle lanes meet placements")
	touched, err = sharedKeep10k(t).ExportedPlacedReachAgrees(2000, 25, 2)
	require.NoError(t, err, "keep")
	require.Positive(t, touched, "precondition: keep lanes meet placements")
}

// TestPlacedReach_ADroppedBlockerIsAskedWhereItLies: a crate that blocks
// movement and sight is carried off and dropped. Where it lies now closes
// that cell to a step and hides what is behind it; the cell it was authored
// on is open floor. The index files only what cannot move, so the crate is
// asked of every query from its record.
func TestPlacedReach_ADroppedBlockerIsAskedWhereItLies(t *testing.T) {
	const (
		hero   = encounter.MemberID("hero")
		porter = encounter.MemberID("porter")
		crate  = encounter.PropID("crate")
	)
	enc := openHall(t,
		[]encounter.MemberInput{
			{ID: hero, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: porter, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 4}},
		},
		[]encounter.PlacedPropInput{{
			ID: crate, Holdable: true, BlocksMovement: true, BlocksLineOfSight: true,
			Placement: spatial.FootprintPlacement{
				Footprint: spatial.Footprint{Box: &spatial.Box{W: 4, D: 4}}, Origin: centreOf(cellAt(5, 5)),
			},
		}},
		nil, nil, findsNothing{},
	)
	_, err := enc.Hold(&encounter.HoldInput{Member: porter, Target: crate, Range: 1})
	require.NoError(t, err)
	dropAt := cellAt(12, 2)
	walkMember(t, enc, porter, dropAt)
	_, err = enc.Exit(&encounter.ExitInput{Member: porter})
	require.NoError(t, err)

	walkMember(t, enc, hero, cellAt(11, 2))
	_, err = enc.Step(&encounter.StepInput{Member: hero, To: dropAt})
	require.Error(t, err, "the crate closes the cell it was dropped on")
	walkMember(t, enc, hero, cellAt(5, 5))
	require.Equal(t, cellAt(5, 5), positionOfMember(t, enc, hero), "and the cell it was authored on is open floor")

	touched, err := enc.ExportedPlacedReachAgrees(3000, 12, 3)
	require.NoError(t, err)
	require.Positive(t, touched, "precondition: lanes meet the crate where it lies")
}
