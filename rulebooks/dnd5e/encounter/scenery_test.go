// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// scenery_test.go is FLOOR NOBODY STANDS ON (rpg-project#360, wall-geometry
// design §1 and §4.1) — the composition's half of slice 1.
//
// The composition's mask used to answer one question: does a region own this
// cell. Everything else fell out of that one answer — floor, standability,
// what a wall may stand on, whether a sightline stops. Scenery splits it:
//
//   - FLOOR is owner ∪ scenery. Walls stand on it, props sit on it, and it is
//     in every atlas.
//   - STANDABLE is owner alone. Feet never touch scenery, so a step onto it is
//     refused exactly as a step into the void always was.
//   - TRANSPARENT REGARDLESS. Sight crosses scenery whatever the field
//     declared its void to be, because scenery is floor and the void
//     declaration is about the space between the floor.
//
// Every fixture is one row of hexes, where consecutive columns are neighbours
// under pointy-top, so what a scene is about is the only thing in it.

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type ScenerySuite struct {
	suite.Suite
}

func TestScenerySuite(t *testing.T) {
	suite.Run(t, new(ScenerySuite))
}

// sceneryCells is an authored strip of scenery on row 0, columns [from,to].
func sceneryCells(from, to int) []spatial.Position {
	var out []spatial.Position
	for c := from; c <= to; c++ {
		out = append(out, spatial.Position{X: float64(c), Y: 0})
	}
	return out
}

// theGap is the sight fixture: two rooms on one row with a scenery strip
// between them and NOTHING else — no wall, no door, and an OPAQUE void, which
// is what makes the scene about scenery rather than about the declaration.
//
//	west [0,0] [1,0] [2,0] | scenery [3,0] | east [4,0] [5,0] [6,0]
func theGap() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("west", 0, 0, 3, 1),
			rectRegion("east", 4, 0, 3, 1),
		},
		Scenery: sceneryCells(3, 3),
	}
}

func (s *ScenerySuite) open(field encounter.FieldInput, members ...encounter.MemberInput) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
		Field:   field,
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// sceneryHolds reports whether the observer is CURRENTLY seeing the subject —
// sight_test.go's own reading, repeated here so this file does not depend on
// that suite's receiver.
func (s *ScenerySuite) seesNow(enc *encounter.Encounter, observer, subject core.EntityID) bool {
	view, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range view {
		if h.Subject == intel.Subject(subject) {
			return h.Status == intel.Current
		}
	}
	return false
}

// TestA5_AStepOntoSceneryIsRefused — feet never touch scenery. The refusal is
// the one the void always gave, unchanged: standability is what a step asks
// about, and scenery has none.
func (s *ScenerySuite) TestA5_AStepOntoSceneryIsRefused() {
	enc := s.open(theGap(), encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 0},
	})

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(3, 0)})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement, "nobody stands on scenery")

	// And the cell one further, which IS a room, is still reachable — so what
	// refused above was the scenery and not the distance.
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(4, 0)})
	s.Require().NoError(err, "the room beyond the strip is ordinary floor")
}

// TestA5_SightCrossesSceneryUnderAnOpaqueVoid is the other half of A5, and the
// one the design calls out: the field declares its void OPAQUE, so an
// ownerless cell used to stop a sightline dead. Scenery is floor, so it does
// not — and it does not because of what it IS, not because of what the field
// declared.
func (s *ScenerySuite) TestA5_SightCrossesSceneryUnderAnOpaqueVoid() {
	field := theGap()
	s.Require().Equal(encounter.VoidOpaque, field.Canvas.Void.Kind(),
		"the scene is only about scenery if the void would otherwise block")

	enc := s.open(field,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 0}},
	)

	s.True(s.seesNow(enc, alice, bob), "the only thing between them is scenery, and scenery is transparent")
	s.True(s.seesNow(enc, bob, alice), "and geometry is mutual")

	// The handed-out canvas answers the same way, which is what keeps a rule
	// installed on it from disagreeing with the percept loop (void.go's own
	// law).
	canvas, err := enc.Canvas()
	s.Require().NoError(err)
	s.False(canvas.IsLineOfSightBlocked(cellAt(0, 0), cellAt(6, 0)))
}

// TestSceneryIsFloorNotVoid — with the strip made void instead (nothing
// authored there at all), the same two members under the same opaque
// declaration cannot see each other. The contrast is what proves the scene
// above is about scenery rather than about a ray that never left the floor.
func (s *ScenerySuite) TestSceneryIsFloorNotVoid() {
	bare := theGap()
	bare.Scenery = nil

	enc := s.open(bare,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 0}},
	)

	s.False(s.seesNow(enc, alice, bob), "an ownerless cell nobody painted is void, and this void is opaque")
}

// TestA4_APropStandsOnSceneryAndAMemberMayNot — a prop drops on scenery; a
// member seated there is refused at construction.
func (s *ScenerySuite) TestA4_APropStandsOnSceneryAndAMemberMayNot() {
	yes, no := true, false
	field := theGap()
	field.Props = []encounter.PropInput{{
		Ref: "dnd5e:props:rubble", At: spatial.Position{X: 3, Y: 0},
		BlocksMovement: &no, BlocksLineOfSight: &no,
	}}

	enc := s.open(field)
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.Props, 1)
	s.Equal(cellAt(3, 0), atlas.Props[0].At, "the prop is on the map where it was authored")

	seated := theGap()
	seated.Props = []encounter.PropInput{{
		Ref: "dnd5e:props:altar", At: spatial.Position{X: 3, Y: 0},
		BlocksMovement: &yes, BlocksLineOfSight: &yes,
	}}
	_, err = encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field:   seated,
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 0}}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement, "a seat on scenery is not a seat")
}

// TestSceneryIsInTheAtlas — the flat cell list is owner ∪ scenery, and the
// region lists are unchanged: a scenery cell is floor that belongs to no
// region, which is exactly what the wire already says (design §5.1).
func (s *ScenerySuite) TestSceneryIsInTheAtlas() {
	atlas, err := s.open(theGap()).Atlas()
	s.Require().NoError(err)

	s.Contains(atlas.Cells, cellAt(3, 0), "scenery is floor, and the floor list is the floor")
	s.Len(atlas.Cells, 7, "three cells west, three east, and the strip between them")

	owned := 0
	for _, r := range atlas.Regions {
		owned += len(r.Cells)
		s.NotContains(r.Cells, cellAt(3, 0), "scenery belongs to no region")
	}
	s.Equal(6, owned, "the regions own everything but the strip")
}

// TestScenerySurvivesTheBlob — the strip is construction truth, so it rides
// ToData like the regions do and comes back the same floor.
func (s *ScenerySuite) TestScenerySurvivesTheBlob() {
	enc := s.open(theGap())

	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: nobodyPerceives{},
	})
	s.Require().NoError(err)

	before, err := enc.Atlas()
	s.Require().NoError(err)
	after, err := back.Atlas()
	s.Require().NoError(err)
	s.Equal(before.Cells, after.Cells, "the reloaded floor is the floor that was saved")
}

// theYardstick is A2's dungeon: a visible hall, a WALL along its far edge, a
// strip of scenery behind the wall, and a secret vault behind the strip.
//
//	hall [0,0] [1,0] [2,0] |wall| scenery [3,0] | vault [4,0]
//
// The wall is what makes it authorable at all — without it the strip is a way
// anyone can walk into the secret, and the compiler says so (dungeonspec's
// C4). With it, what a non-knower must see is a hall, a wall, and a strip of
// floor beyond: exactly the dungeon in theYardstickTwin.
func theYardstick() encounter.FieldInput {
	field := encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 3, 1),
			{
				ID: "vault", Name: "vault", Archetype: testArchetype, Lighting: fullLight(),
				Cells: []spatial.Position{{X: 4, Y: 0}}, Concealed: true,
			},
		},
		Scenery: sceneryCells(3, 3),
		Walls:   []encounter.WallInput{wall(2, 0, 3, 0)},
	}
	return field
}

// theYardstickTwin is the same dungeon HONESTLY AUTHORED as what a non-knower
// believes: the secret deleted, the strip kept.
func theYardstickTwin() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas:  pointyCanvas(),
		Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 3, 1)},
		Scenery: sceneryCells(3, 3),
		Walls:   []encounter.WallInput{wall(2, 0, 3, 0)},
	}
}

// TestA2_TheYardstickHoldsAcrossScenery is acceptance A2.
//
// It is the C5 and C6 pins in one scene. C5: scenery has no owner, so no
// member's atlas withholds it. C6: the masquerade never stands a wall on
// scenery's far side — a bare crossing from scenery into hidden space gets no
// synthesized wall, because scenery is not visible SPACE and there is nobody
// standing in it for the wall to lie to.
func (s *ScenerySuite) TestA2_TheYardstickHoldsAcrossScenery() {
	enc := s.open(theYardstick(), encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0},
	})
	scoped, err := enc.AtlasFor(alice)
	s.Require().NoError(err)

	twin, err := s.open(theYardstickTwin(), encounter.MemberInput{
		ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 0},
	}).Atlas()
	s.Require().NoError(err)

	s.Equal(twin.Cells, scoped.Cells, "cells byte-identical to never-authored, strip included")
	s.Equal(twin.Regions, scoped.Regions, "regions byte-identical to never-authored")
	s.Equal(twin.Props, scoped.Props)
	s.Equal(twin.Doorways, scoped.Doorways)
	s.Equal(twin.Boundaries, scoped.Boundaries,
		"C6: no wall is synthesized on the strip's far side — scenery is not visible space")

	// Stated as its own assertion too, because the equality above would also
	// hold if BOTH atlases grew the same phantom wall.
	s.Contains(scoped.Cells, cellAt(3, 0), "C5: scenery has no owner, so nobody's map withholds it")
	s.NotContains(scoped.Cells, cellAt(4, 0), "and the vault itself is still a secret")
	for _, b := range scoped.Boundaries {
		s.False(b.From == cellAt(3, 0) && b.To == cellAt(4, 0) || b.From == cellAt(4, 0) && b.To == cellAt(3, 0),
			"the strip|vault crossing is not presented as a wall")
	}
}
