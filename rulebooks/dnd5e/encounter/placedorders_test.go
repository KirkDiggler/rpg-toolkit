// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// placedorders_test.go is A PLACED FOOTPRINT CAN BE HELD AND CAN ARRIVE
// (rpg-toolkit#1854, rpg-project#488 R1) — the three orders a rectangle now
// carries, proved through the exported verbs rather than through the compile.
//
// # What each claim is, and what would break it
//
//   - REACH IS DERIVED. A placement has no anchor cell, so [Encounter.Hold]
//     applies the LEGACY reach rule — grid distance, Range 0 meaning adjacent
//     — to every cell the footprint stands on. Two fixtures carry that: a
//     fifteen-foot slab covering three cells, taken from beside the far one
//     rather than only from beside its origin; and a scroll SMALLER THAN A
//     HEX covering no cell centre at all, which stands in the hex it lies in.
//     The second is the World Builder's own case — a 0.3-unit item is a
//     0.87ft square on a 5ft cell — and a rule that only read covered centres
//     would make every such thing unreachable from everywhere.
//
//   - ARRIVING IS ABSENT. Until its predicate holds, a reserved placement is
//     on no map, closes no cell and obstructs no lane: the never-authored
//     yardstick, asked of a rectangle. All three are asserted BEFORE and
//     AFTER, through [Encounter.Atlas], [Encounter.CellAt] and the canvas's
//     own sight, so a claim that only checked the atlas could not pass.
//
//   - ONE STORE. Held, dropped and what it teaches go through the holdings
//     journal exactly as a legacy prop's do, which is why the fact a placed
//     letter carries can bring a LEGACY prop in from reserve in the same
//     breath as a placed one: a fact is a fact, whichever kind of thing
//     taught it.
//
// NOTHING HERE RE-IMPLEMENTS THE GEOMETRY. Every expected cell comes from
// spatial's own embedding through [centreOf], the same frame the field
// compiles, and every distance the reach claims depends on is asserted with
// [Encounter.Distance] rather than assumed from a picture.

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	// theSlab is a fifteen-foot table covering three cells in a row —
	// holdable, so "any covered cell" has something to be about.
	theSlab = "slab"

	// theScroll is smaller than a hex and covers no cell centre: the
	// fallback clause of [field.placedCells], and the World Builder's
	// authored letter.
	theScroll = "scroll"

	// theBarricade waits in reserve and blocks both facts when it comes.
	theBarricade = "barricade"

	// theTag is a sub-hex prop whose LOCAL OFFSET puts its box in a
	// different hex from its origin — the case that tells "where the
	// rectangle is" apart from "where its origin is".
	theTag = "tag"

	// theNotes is what the scroll says; theWord is the fact it teaches.
	theNotes = "scroll-notes"
	theWord  = "the-word"

	// thePrize is a LEGACY prop waiting on the same fact — the control that
	// makes "exactly as for a legacy prop" an assertion rather than a claim.
	thePrize = "prize"

	// theWayOut is the exit the recovery ending is bound to.
	theWayOut = "way-out"
)

// slabOrigin, scrollOrigin and barricadeOrigin are the three placements'
// poses, named so the assertions can talk about where each thing is.
var (
	// The slab is centred on cell (2,4) and reaches five feet either side of
	// it, so the centres of (1,4) and (3,4) fall inside it too.
	slabCentre = cellAt(2, 4)

	// The scroll lies a foot and a bit off cell (2,2)'s centre: inside that
	// hex, on no centre at all.
	scrollHex = cellAt(2, 2)

	// The barricade stands across cell (1,0) — thirty feet of it, which is
	// every lane between the two sides of the hall.
	barricadeHex = cellAt(1, 0)

	// The tag's ORIGIN is cell (2,2)'s centre; its box sits 6.2 feet along
	// the facing from there, which is inside cell (3,2) and covers no centre
	// at all.
	tagOriginHex = cellAt(2, 2)
	tagBoxHex    = cellAt(3, 2)
)

// tagPlacement is the offset sub-hex prop: a ten-inch square pushed out of
// the hex its origin sits in.
func tagPlacement() spatial.FootprintPlacement {
	return spatial.FootprintPlacement{
		Footprint:   spatial.Footprint{Box: &spatial.Box{W: 0.87, D: 0.87}},
		Origin:      centreOf(tagOriginHex),
		LocalOffset: spatial.Point{X: 6.2},
	}
}

// PlacedOrdersSuite runs every scene on the one hall below.
type PlacedOrdersSuite struct {
	suite.Suite
}

func TestPlacedOrdersSuite(t *testing.T) {
	suite.Run(t, new(PlacedOrdersSuite))
}

// slabPlacement is the three-cell table: five feet across the row, fifteen
// along it, centred on (2,4).
func slabPlacement() spatial.FootprintPlacement {
	centre := centreOf(slabCentre)

	return spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 5, D: 15}},
		Origin:    centre,
	}
}

// scrollPlacement is a ten-inch square sitting a foot off cell (2,2)'s
// centre — the shape a World Builder item actually compiles to.
func scrollPlacement() spatial.FootprintPlacement {
	centre := centreOf(scrollHex)

	return coveredBox(0.87, spatial.Point{X: centre.X + 1.2, Y: centre.Y})
}

// ordersField is the hall: a slab, a scroll that teaches a fact, a barricade
// and a prize both waiting on that fact, and one way out.
func ordersField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas:  openAir(),
		Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 6)},
		Intel: []encounter.IntelRecord{
			{ID: theNotes, Reveals: encounter.RevealTargets{Fact: theWord}},
		},
		Props: []encounter.PropInput{
			func() encounter.PropInput {
				p := holdableProp(thePrize, "dnd5e:props:chest", spatial.Position{X: 5, Y: 5})
				p.Arrives = encounter.TriggerFact{Fact: theWord}

				return p
			}(),
		},
		Placed: []encounter.PlacedPropInput{
			{ID: theSlab, Placement: slabPlacement(), Holdable: true},
			{ID: theScroll, Placement: scrollPlacement(), Holdable: true, Holds: []encounter.IntelID{theNotes}},
			{ID: theTag, Placement: tagPlacement(), Holdable: true},
			{
				ID: theBarricade, Placement: thinWall(0.2, 30, 0, centreOf(barricadeHex)),
				BlocksMovement: true, BlocksLineOfSight: true,
				Arrives: encounter.TriggerFact{Fact: theWord},
			},
		},
		Exits: []encounter.FieldExit{{ID: theWayOut, At: spatial.Position{X: 0, Y: 5}}},
	}
}

// authoredAt is one AUTHORED offset seat — the frame [encounter.MemberInput]
// and [encounter.FieldExit] speak, as against the absolute cells every verb
// and every read speak ([cellAt] converts).
func authoredAt(col, row int) spatial.Position {
	return spatial.Position{X: float64(col), Y: float64(row)}
}

// open builds the hall with the members standing where the scene wants them,
// in authored seats.
func (s *PlacedOrdersSuite) open(
	at map[encounter.MemberID]spatial.Position, endings ...encounter.EndingInput,
) *encounter.Encounter {
	members := make([]encounter.MemberInput, 0, len(at))
	for _, id := range []encounter.MemberID{alice, bella} {
		cell, standing := at[id]
		if !standing {
			continue
		}
		members = append(members, encounter.MemberInput{ID: id, Kind: encounter.KindPlayer, Position: cell})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   ordersField(),
		Members: members,
		Endings: append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, endings...),
	})
	s.Require().NoError(err)

	return enc
}

// build opens the hall on an edited field and hands back whatever
// construction answered — the seam the refusal scenes are about.
func (s *PlacedOrdersSuite) build(field encounter.FieldInput) (*encounter.Encounter, error) {
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   field,
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: authoredAt(2, 2)}},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
}

// placedIDs is every footprint on the truth-grain atlas, by id.
func (s *PlacedOrdersSuite) placedIDs(enc *encounter.Encounter) []encounter.PropID {
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	out := make([]encounter.PropID, 0, len(atlas.Placed))
	for _, p := range atlas.Placed {
		out = append(out, p.ID)
	}

	return out
}

// placedNamed is one footprint on the atlas, required to be there.
func (s *PlacedOrdersSuite) placedNamed(enc *encounter.Encounter, id encounter.PropID) encounter.AtlasPlacedProp {
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	for _, p := range atlas.Placed {
		if p.ID == id {
			return p
		}
	}
	s.Require().FailNowf("no such placement", "%q is not on the atlas", id)

	return encounter.AtlasPlacedProp{}
}

// propIDsOn is every legacy prop on the truth-grain atlas, by id.
func (s *PlacedOrdersSuite) propIDsOn(enc *encounter.Encounter) []encounter.PropID {
	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	out := make([]encounter.PropID, 0, len(atlas.Props))
	for _, p := range atlas.Props {
		out = append(out, p.ID)
	}

	return out
}

// --- (1) Reach: the legacy rule, applied to every cell the footprint stands on ---

// A slab covering three cells is in reach from beside ANY of them, and the
// far one is two cells from its origin — which is what makes this a claim
// about the footprint rather than about where the author put its centre.
func (s *PlacedOrdersSuite) TestASlabIsTakenFromBesideAnyCellItCovers() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(4, 4)})
	reach := cellAt(4, 4)

	s.Require().Equal(float64(1), enc.Distance(reach, cellAt(3, 4)),
		"the taker is beside the slab's far cell")
	s.Require().Equal(float64(2), enc.Distance(reach, slabCentre),
		"and two cells from the one its origin sits on, which the legacy rule alone would refuse")

	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theSlab})
	s.Require().NoError(err, "a covered cell within reach is within reach")
	s.NotContains(s.placedIDs(enc), encounter.PropID(theSlab), "and it leaves the floor for everyone")
}

// Standing ON a cell the footprint covers is distance zero, which the legacy
// rule already admits: a holdable thing blocks no movement, so you walk onto
// it to take it.
func (s *PlacedOrdersSuite) TestASlabIsTakenFromOnTopOfIt() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(1, 4)})
	s.Require().Equal(float64(0), enc.Distance(cellAt(1, 4), cellAt(1, 4)),
		"they are standing on a cell the slab covers")

	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theSlab})
	s.Require().NoError(err)
}

// And a cell that is beside NO covered cell is out of range, by name.
func (s *PlacedOrdersSuite) TestASlabIsRefusedFromOutsideTheReachOfEveryCoveredCell() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(5, 1)})
	far := cellAt(5, 1)

	for _, covered := range []spatial.Position{cellAt(1, 4), slabCentre, cellAt(3, 4)} {
		s.Require().Greater(enc.Distance(far, covered), float64(1),
			"the taker is beyond reach of every cell the slab stands on")
	}

	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theSlab})
	s.Require().ErrorIs(err, encounter.ErrOutOfRange)
}

// A thing SMALLER THAN A HEX covers no cell centre and still stands
// somewhere: the hex it lies in. Taken from that hex and from beside it,
// refused from two cells away — the same three answers the slab gives.
func (s *PlacedOrdersSuite) TestAScrollTooSmallToCoverACentreStandsInItsOwnHex() {
	s.Run("from the hex it lies in", func() {
		enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(2, 2)})
		_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theScroll})
		s.Require().NoError(err)
	})

	s.Run("from beside it", func() {
		enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(3, 2)})
		s.Require().Equal(float64(1), enc.Distance(cellAt(3, 2), scrollHex))
		_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theScroll})
		s.Require().NoError(err)
	})

	s.Run("and not from two cells away", func() {
		enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(4, 2)})
		s.Require().Equal(float64(2), enc.Distance(cellAt(4, 2), scrollHex))
		_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theScroll})
		s.Require().ErrorIs(err, encounter.ErrOutOfRange)
	})
}

// A placement's cells follow its RECTANGLE, not its origin: a local offset
// puts the box in a different hex, and that is the hex it is taken from.
//
// The pair is the whole claim. Beside the box's hex takes it; beside the
// ORIGIN'S hex — two cells from the box — does not. A rule that measured from
// the origin would answer the other way round on both.
func (s *PlacedOrdersSuite) TestAPlacementStandsWhereItsRectangleIsNotWhereItsOriginIs() {
	s.Require().NotEqual(tagOriginHex, tagBoxHex, "the fixture puts the two apart")

	s.Run("beside the hex the box is in", func() {
		enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(4, 2)})
		s.Require().Equal(float64(1), enc.Distance(cellAt(4, 2), tagBoxHex))
		s.Require().Equal(float64(2), enc.Distance(cellAt(4, 2), tagOriginHex),
			"and two cells from the origin, which a rule reading the origin would refuse")

		_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theTag})
		s.Require().NoError(err)
	})

	s.Run("and not from beside the hex its origin is in", func() {
		enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(1, 2)})
		s.Require().Equal(float64(1), enc.Distance(cellAt(1, 2), tagOriginHex))
		s.Require().Equal(float64(2), enc.Distance(cellAt(1, 2), tagBoxHex))

		_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theTag})
		s.Require().ErrorIs(err, encounter.ErrOutOfRange)
	})
}

// The centre this module computes is the one SPATIAL measures — the pin on
// two lines that had to be repeated because spatial does not export them.
//
// NOT A TAUTOLOGY. The tag's rectangle stands 6.2 feet from its origin and is
// ten inches across, so the origin is nowhere near it: a centre computed
// without the local offset, or with the rotation the other way round, lands
// outside the box and spatial's own contact query says so.
func (s *PlacedOrdersSuite) TestAPlacementsCentreIsTheOneSpatialMeasures() {
	placement := tagPlacement()
	placement.Facing = 37 // an angle no axis snaps to

	centre := encounter.ExportedPlacedCentre(placement)
	inside, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
		Placement: placement, From: centre, To: centre,
	})
	s.Require().NoError(err)
	s.True(inside.Contact, "the point this module calls the centre is inside spatial's own rectangle")

	atOrigin, err := spatial.TraceFootprint(spatial.FootprintTraceInput{
		Placement: placement, From: placement.Origin, To: placement.Origin,
	})
	s.Require().NoError(err)
	s.False(atOrigin.Contact, "and the origin is not, which is why the offset has to be applied")
}

// A placement nobody declared holdable is scenery, and says so by name
// rather than by the probe law's evasive refusal: there is no secret in a
// barricade the member can see.
func (s *PlacedOrdersSuite) TestAPlacementNobodyDeclaredHoldableIsRefusedByName() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(2, 2)})
	s.learnTheWord(enc)

	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theBarricade})
	s.Require().ErrorIs(err, encounter.ErrNotHoldable)
}

// Taking it twice is ErrAlreadyHeld, the legacy answer for a thing that has
// left the floor.
func (s *PlacedOrdersSuite) TestTakingAPlacementTwiceIsRefused() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(2, 2)})
	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theScroll})
	s.Require().NoError(err)

	_, err = enc.Hold(&encounter.HoldInput{Member: alice, Target: theScroll})
	s.Require().ErrorIs(err, encounter.ErrAlreadyHeld)
}

// --- (2) Arriving: absent from the atlas, from the cell fold and from sight ---

// learnTheWord takes the scroll, which teaches the fact both reserved things
// are waiting on.
func (s *PlacedOrdersSuite) learnTheWord(enc *encounter.Encounter) {
	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theScroll})
	s.Require().NoError(err)
}

func (s *PlacedOrdersSuite) TestAReservedPlacementIsNowhereUntilItsPredicateHolds() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(2, 2)})
	canvas, err := enc.Canvas()
	s.Require().NoError(err)

	s.Run("before: on no map, closing no cell, obstructing no lane", func() {
		s.NotContains(s.placedIDs(enc), encounter.PropID(theBarricade))
		s.Require().Equal(encounter.PassageStandable,
			enc.CellAt(encounter.CellAtInput{Cell: barricadeHex}).Passage,
			"a rectangle that has not come closes nothing")
		s.False(canvas.IsLineOfSightBlocked(cellAt(-2, 0), cellAt(2, 0)),
			"and obstructs nothing")
	})

	s.learnTheWord(enc)

	s.Run("after: on the map, closing its cells, obstructing the lane", func() {
		s.Contains(s.placedIDs(enc), encounter.PropID(theBarricade))
		fact := enc.CellAt(encounter.CellAtInput{Cell: barricadeHex})
		s.Require().Equal(encounter.PassageBlocked, fact.Passage)
		named := false
		for _, contrib := range fact.Contribs {
			if contrib.Kind == encounter.ContribProp && contrib.ID == theBarricade {
				named = true
			}
		}
		s.True(named, "and the cell fold says which rectangle closed it")
		s.True(canvas.IsLineOfSightBlocked(cellAt(-2, 0), cellAt(2, 0)),
			"thirty feet of it blocks every lane")
	})
}

// The arrival writes the SAME journal kind and the SAME beat a legacy prop's
// does — one predicate, one fact, both kinds of thing, in one pass.
func (s *PlacedOrdersSuite) TestAPlacementArrivesWithTheSameBeatALegacyPropDoes() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(2, 2)})
	s.Require().NotContains(s.propIDsOn(enc), encounter.PropID(thePrize))

	s.learnTheWord(enc)

	s.Contains(s.propIDsOn(enc), encounter.PropID(thePrize), "the legacy prop came")
	s.Contains(s.placedIDs(enc), encounter.PropID(theBarricade), "and so did the footprint")

	arrived := beatsOfKindFor(s.T(), enc, alice, "arrived")
	kinds := map[string]string{}
	for _, beat := range arrived {
		id, _ := beat["id"].(string)
		kind, _ := beat["kind"].(string)
		kinds[id] = kind
	}
	s.Equal(encounter.ArrivedProp, kinds[thePrize], "a legacy prop arrives as a prop")
	s.Equal(encounter.ArrivedProp, kinds[theBarricade], "and a footprint arrives as the same kind of thing")
}

// A reserved placement is refused as an id that names NOTHING, never as
// "not yet" — the probe law, which is the only reason the reserved refusal
// is not ErrNotHoldable.
func (s *PlacedOrdersSuite) TestAReservedPlacementIsRefusedAsAnIdThatNamesNothing() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(1, 0)})

	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theBarricade})
	s.Require().ErrorIs(err, encounter.ErrNoProp)
	s.Equal(`hold: "barricade": no such prop`, err.Error())

	_, unknown := enc.Hold(&encounter.HoldInput{Member: alice, Target: "no-such-thing"})
	s.Require().ErrorIs(unknown, encounter.ErrNoProp)
	s.Equal(`hold: "no-such-thing": no such prop`, unknown.Error(),
		"byte for byte the answer for an id that names nothing, but for the id")
}

// --- (3) What it carries is learned on taking it, exactly as a prop's is ---

// The scroll's record reaches the taker: the proof is that the fact it
// reveals brought BOTH reserved things in, which only happens if the
// placement's Holds went through the same routine a legacy prop's does.
func (s *PlacedOrdersSuite) TestWhatAPlacementCarriesIsAppliedOnTakingIt() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(2, 2)})

	s.Require().NotContains(s.propIDsOn(enc), encounter.PropID(thePrize),
		"nothing has taught the fact yet")

	s.learnTheWord(enc)

	s.Contains(s.propIDsOn(enc), encounter.PropID(thePrize),
		"the record the placement carries was applied to whoever took it")
}

// --- (4) Dropped: back on the floor, at the cell it was dropped on ---

// A carrier who leaves from anywhere but a bound exit drops what they held
// (R9), and a dropped rectangle lands with its ORIGIN on that cell — same
// shape, same facing, new place.
func (s *PlacedOrdersSuite) TestADroppedPlacementLandsOnTheCellItWasDroppedOn() {
	standing := cellAt(5, 1)
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(1, 4), bella: authoredAt(5, 5)})

	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theSlab})
	s.Require().NoError(err)
	s.Require().NotContains(s.placedIDs(enc), encounter.PropID(theSlab))

	_, err = enc.Step(&encounter.StepInput{Member: alice, To: standing})
	s.Require().NoError(err)
	out, err := enc.Exit(&encounter.ExitInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Nil(out.Closed, "they left through no bound exit, so the run goes on")

	dropped := s.placedNamed(enc, theSlab)
	s.Equal(centreOf(standing), dropped.Placement.Origin,
		"the rectangle stands where they put it down")
	s.Equal(slabPlacement().Facing, dropped.Placement.Facing, "turned no differently")
	s.Equal(*slabPlacement().Footprint.Box, *dropped.Placement.Footprint.Box, "and the same shape")

	// AND IT REPORTS THE CELLS IT STANDS ON NOW, not the ones it was
	// authored over — the claim [AtlasPlacedProp.Cells] makes about being
	// asked of the fold's placement rather than the compiled one.
	//
	// The two sets are DISJOINT here, which is what makes this an assertion
	// rather than a restatement: a derivation reading the authored placement
	// would answer the row the slab was drawn on, four rows away. The
	// dropped set is smaller because the slab's far end now hangs off the
	// edge of the hall, which is the floor answering rather than the
	// rectangle.
	s.ElementsMatch([]spatial.Position{cellAt(4, 1), standing}, dropped.Cells,
		"the cells its rectangle covers where it was put down")
	for _, authored := range []spatial.Position{cellAt(1, 4), slabCentre, cellAt(3, 4)} {
		s.NotContains(dropped.Cells, authored,
			"and not one cell of the row it was authored over")
	}
}

// --- (5) A scenario's artifact may be a placement ---

// The recovery ending names a PLACED artifact, and the run ends when it is
// carried out through the bound exit — the tomb-heirloom scene with the
// prize authored as a footprint.
func (s *PlacedOrdersSuite) TestCarryingAPlacedArtifactOutEndsTheRun() {
	const recovered = "recovered"
	wayOut := cellAt(0, 5)
	enc := s.open(
		map[encounter.MemberID]spatial.Position{alice: authoredAt(1, 4)},
		encounter.EndingInput{Key: recovered, Trigger: encounter.TriggerExitedHolding{
			Exit: theWayOut, Item: theSlab,
		}},
	)

	_, err := enc.Hold(&encounter.HoldInput{Member: alice, Target: theSlab})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: wayOut})
	s.Require().NoError(err)

	out, err := enc.Exit(&encounter.ExitInput{Member: alice})
	s.Require().NoError(err)
	s.Require().NotNil(out.Closed, "they carried the artifact out")
	s.Equal(recovered, out.Closed.Ending)
}

// An ending naming a placement nobody declared holdable is refused at
// construction, by the same sentence a legacy prop earns: the one holdable
// index answers for both kinds of thing.
func (s *PlacedOrdersSuite) TestAnEndingOnAPlacementNobodyCanTakeIsRefused() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field:   ordersField(),
		Members: []encounter.MemberInput{{ID: alice, Kind: encounter.KindPlayer, Position: authoredAt(1, 4)}},
		Endings: []encounter.EndingInput{{Key: "recovered", Trigger: encounter.TriggerExitedHolding{
			Exit: theWayOut, Item: theBarricade,
		}}},
	})
	s.Require().ErrorIs(err, encounter.ErrNoEnding)
}

// --- Construction: the two things a placement's orders may not say ---

// A placement whose arrival predicate can never hold is refused at
// construction, by the liveness rule every prop's arrival meets and in the
// sentence a legacy prop earns — "prop %q's arrival", because an author who
// wrote `arrives:` under propBindings wrote the same key.
func (s *PlacedOrdersSuite) TestAPlacementWaitingOnAPredicateThatCannotHoldIsRefused() {
	field := ordersField()
	for i := range field.Placed {
		if field.Placed[i].ID == theBarricade {
			field.Placed[i].Arrives = encounter.TriggerRound{Round: 0}
		}
	}

	_, err := s.build(field)
	s.Require().ErrorIs(err, encounter.ErrNoField)
	s.Require().ErrorContains(err, `prop "barricade"'s arrival`,
		"named as the prop it is, not as a second kind of thing")
	s.Require().ErrorContains(err, "a round is counted from 1",
		"and judged by the one predicate validator")
}

// A placement carrying a record this field does not declare is refused at
// construction, the same mistake a legacy prop's dangling record is —
// knowledge the author thinks they placed and did not.
func (s *PlacedOrdersSuite) TestAPlacementHoldingARecordNobodyDeclaredIsRefused() {
	field := ordersField()
	for i := range field.Placed {
		if field.Placed[i].ID == theScroll {
			field.Placed[i].Holds = []encounter.IntelID{"no-such-record"}
		}
	}

	_, err := s.build(field)
	s.Require().ErrorIs(err, encounter.ErrNoIntel)
	s.Require().ErrorContains(err, `prop "scroll" holds intel "no-such-record"`,
		"named by its id, because a placement has no content ref to name it by")
}

// --- Persistence: the three orders survive the blob ---

// reloadOf saves the run and loads it back — the round trip every verb makes
// at the host seam, where a blob is checked before it is trusted.
func (s *PlacedOrdersSuite) reloadOf(enc *encounter.Encounter) *encounter.Encounter {
	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:      enc.ToData(),
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err, "the run this module just wrote is one it can read")

	return reloaded
}

// A RUN IN WHICH A PLACEMENT WAS TAKEN, BROUGHT SOMETHING IN AND WAS PUT
// DOWN AGAIN RELOADS — all three holdings facts about a footprint, across the
// boundary that decides whether a blob is trusted.
//
// FOUND ON THE WALK, not here (rpg-toolkit#1854). The load boundary checks
// that every `held:`, `dropped:` and `arrived:` fact names a prop the field
// places, and it knew only the legacy list — so the first save after somebody
// picked up a placed letter was a save the next verb refused to read. Nothing
// in this file caught it, because every reload here was of a run where
// nothing had happened yet; the stack caught it the moment a held letter
// turned a camp and the dissolve reloaded the world.
func (s *PlacedOrdersSuite) TestARunThatHeldDroppedAndArrivedAPlacementReloads() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(2, 2), bella: authoredAt(5, 5)})

	s.learnTheWord(enc)
	s.Require().Contains(s.placedIDs(enc), encounter.PropID(theBarricade), "the arrival happened")

	s.Run("a held placement reloads", func() {
		held := s.reloadOf(enc)
		s.NotContains(s.placedIDs(held), encounter.PropID(theScroll), "still in somebody's hands")
		s.Contains(s.placedIDs(held), encounter.PropID(theBarricade), "and still on the floor")
	})

	out, err := enc.Exit(&encounter.ExitInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Nil(out.Closed)
	dropped := s.placedNamed(enc, theScroll)

	s.Run("and so does a dropped one, where it was dropped", func() {
		reloaded := s.reloadOf(enc)
		s.Equal(dropped.Placement.Origin, s.placedNamed(reloaded, theScroll).Placement.Origin,
			"the rectangle comes back standing where it was put down")
	})
}

// A reload rebuilds the same answers from the same bytes: what is holdable,
// what it carries, and what is still in reserve.
func (s *PlacedOrdersSuite) TestTheThreeOrdersSurviveASaveAndLoad() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(2, 2)})
	reloaded := s.reloadOf(enc)

	s.NotContains(s.placedIDs(reloaded), encounter.PropID(theBarricade),
		"a reserved rectangle is still in reserve")
	s.True(s.placedNamed(reloaded, theScroll).Holdable, "and the scroll can still be picked up")

	s.learnTheWord(reloaded)

	s.Contains(s.placedIDs(reloaded), encounter.PropID(theBarricade),
		"the predicate still brings it in")
	s.Contains(s.propIDsOn(reloaded), encounter.PropID(thePrize),
		"and the record it carried still teaches the fact")
}

// --- (5) The atlas SAYS where a rectangle stands, in cells ---

// A placement reports the cells it stands on, and they are the cells the
// derivation names: the covered ones for a thing bigger than a hex, and the
// one hex it lies in for a thing smaller than one (rpg-api-protos#351).
//
// THE POINT IS THE CLIENT. A footprint has no anchor cell, so every consumer
// asking "what is this next to?" had to re-run this module's geometry against
// this module's plane to find out. The atlas answers instead.
//
// THE SET, NOT ITS LENGTH: each claim names the cells it is about, so a
// fourth cell arriving fails as loudly as one going missing.
func (s *PlacedOrdersSuite) TestTheAtlasReportsEveryCellAPlacementStandsOn() {
	enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(0, 0)})

	s.Run("a fifteen-foot slab stands on all three cells it covers", func() {
		s.ElementsMatch([]spatial.Position{cellAt(1, 4), slabCentre, cellAt(3, 4)},
			s.placedNamed(enc, theSlab).Cells,
			"the same three cells the reach scenes above take it from")
	})

	s.Run("a scroll smaller than a hex stands in exactly the hex it lies in", func() {
		s.Equal([]spatial.Position{scrollHex}, s.placedNamed(enc, theScroll).Cells,
			"covering no cell centre at all, so only the centre clause puts it anywhere")
	})

	s.Run("and it is the hex the RECTANGLE lies in, never the one its origin sits in", func() {
		s.Require().NotEqual(tagOriginHex, tagBoxHex, "the fixture puts the two apart")
		s.Equal([]spatial.Position{tagBoxHex}, s.placedNamed(enc, theTag).Cells,
			"a rule reading the origin would name the other hex")
	})

	s.Run("in the atlas's own coordinate order", func() {
		cells := s.placedNamed(enc, theSlab).Cells
		s.True(sort.SliceIsSorted(cells, func(i, j int) bool {
			if cells[i].X != cells[j].X {
				return cells[i].X < cells[j].X
			}

			return cells[i].Y < cells[j].Y
		}), "by X then Y, the one order every list on this snapshot is in (C8)")
	})
}

// EVERY CELL THE ATLAS NAMES IS A CELL THE VERB GRANTS FROM — the claim the
// field exists for. A client offering Hold where this list says the member is
// standing on the thing, and [Encounter.Hold] refusing it, would be two
// derivations of one geometry disagreeing across the wire; there is one
// derivation ([field.placedCells]), and reach reads it (hold.go, holdPlaced).
//
// Driven off the ATLAS'S OWN LIST rather than off a written-down set, so a
// cell the snapshot starts reporting has to be one reach grants from too.
// The converse — a cell beside none of them is refused — is
// TestASlabIsRefusedFromOutsideTheReachOfEveryCoveredCell above.
func (s *PlacedOrdersSuite) TestTheCellsTheAtlasNamesAreTheCellsReachJudges() {
	listed := s.placedNamed(
		s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(0, 0)}), theSlab).Cells
	s.Require().NotEmpty(listed, "the slab stands somewhere")

	for _, cell := range listed {
		// A fresh hall per cell: taking it is what proves reach, and a
		// thing that has been taken is on no map to take again.
		enc := s.open(map[encounter.MemberID]spatial.Position{alice: authoredAt(0, 0)})
		_, err := enc.Step(&encounter.StepInput{Member: alice, To: cell})
		s.Require().NoErrorf(err, "standing on %v, a cell the atlas says the slab is on", cell)

		_, err = enc.Hold(&encounter.HoldInput{Member: alice, Target: theSlab})
		s.Require().NoErrorf(err, "the verb grants what the atlas offered at %v", cell)
	}
}

// beatsOfKindFor reads one member's story and keeps the beats of one kind —
// the read [HoldingsSuite.beatsOfKind] makes, available to this suite too.
func beatsOfKindFor(t *testing.T, enc *encounter.Encounter, member core.EntityID, kind string) []map[string]any {
	t.Helper()
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	require.NoError(t, err)
	var out []map[string]any
	for _, entry := range story {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}

	return out
}
