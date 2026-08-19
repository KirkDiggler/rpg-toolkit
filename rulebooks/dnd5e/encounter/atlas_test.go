// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// validAtlasOrderingSetup is the asymmetric fixture for #929 T3: two
// independent, far-apart kissing room pairs (atlas-r1/atlas-r2 near the
// absolute origin, atlas-r3/atlas-r4 shifted +10000 on X so W2 is
// trivially satisfied between the pairs), declared with BOTH rooms and
// connections out of ID order (Rooms: r3,r1,r4,r2 — sorted order is
// r1,r2,r3,r4; Connections: connB,connA — sorted order is connA,connB),
// so a missing/removed sort is observable through the public API. Every
// room has a distinct, nonzero-on-both-axes Origin (r1: (0,7), r2:
// (4,6)) so a sign-flipped projection (Subtract instead of Add, or vice
// versa) produces a wrong, non-accidental-match absolute value. atlas-r1
// also carries one boundary (#929 hardening round A) so AtlasRoom.Boundaries
// projection, copy-out, reload survival, and live-state independence are
// all exercised by the same tests that already cover Cells/Occluders.
func validAtlasOrderingSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "atlas-r3", Width: 4, Height: 3, Origin: spatial.Position{X: 9950, Y: 7}},
				{ID: "atlas-r1", Width: 4, Height: 3, Origin: spatial.Position{X: 0, Y: 7},
					Occluders: []spatial.Position{{X: 1, Y: 1}},
					// (0,2)-(1,2): the far row from atlas-p1's Y=0 move path
					// below (TestAtlasUnaffectedByLiveState moves it (0,0) to
					// (3,0)) — a movement-blocking boundary on that row would
					// reject the very move this fixture is also used to pin.
					Boundaries: []spatial.Boundary{
						{From: spatial.Position{X: 0, Y: 2}, To: spatial.Position{X: 1, Y: 2}, BlocksMovement: true, BlocksLineOfSight: true},
					}},
				{ID: "atlas-r4", Width: 6, Height: 5, Origin: spatial.Position{X: 9954, Y: 6}},
				{ID: "atlas-r2", Width: 6, Height: 5, Origin: spatial.Position{X: 4, Y: 6}},
			},
			Connections: []encounter.ConnectionInput{
				{ID: "atlas-connB", From: "atlas-r3", To: "atlas-r4",
					FromPosition: spatial.Position{X: 3, Y: 1},
					ToPosition:   spatial.Position{X: 0, Y: 2}},
				{ID: "atlas-connA", From: "atlas-r1", To: "atlas-r2",
					FromPosition: spatial.Position{X: 3, Y: 1},
					ToPosition:   spatial.Position{X: 0, Y: 2}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "atlas-p1", Kind: encounter.KindPlayer, Room: "atlas-r1", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
}

// validAtlasVoidGapSetup is two square rooms close together but NOT
// touching — a genuine 3-cell void gap between them (gap-a absolute
// X∈[0,2], gap-b absolute X∈[6,8], same Y band) — the realistic shape of
// "void is not floor": a corridor-width gap between two nearby rooms, not
// a point off in empty space (#929 T3 fix round item 5).
func validAtlasVoidGapSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "gap-a", Width: 3, Height: 3, Origin: spatial.Position{X: 0, Y: 0}},
				{ID: "gap-b", Width: 3, Height: 3, Origin: spatial.Position{X: 6, Y: 0}},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "gap-a", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
}

// singleRoomSetup builds a minimal one-room encounter for the enumeration
// property test — Origin zero, so a room's Atlas Cells equal its local
// cells exactly, letting the test compare against spatial's own
// IsValidPosition without any Origin arithmetic in the way.
func singleRoomSetup(shape spatial.GridShape, width, height int) *encounter.SetupInput {
	return &encounter.SetupInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{{ID: "solo", Width: width, Height: height, Grid: shape}},
		},
		Members: []encounter.MemberInput{
			{ID: "p1", Kind: encounter.KindPlayer, Room: "solo", Position: spatial.Position{X: 0, Y: 0}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	}
}

// bruteForceLocalCells independently enumerates a room's valid integer
// local cells using ONLY spatial's public API (NewSquareGrid/
// NewAxialHexGrid + IsValidPosition) — no dependency on this module's
// own enumeration, so it is a genuine independent oracle for
// TestAtlasCellsMatchIsValidPosition (#929 T3 ruling 4).
func bruteForceLocalCells(shape spatial.GridShape, width, height int) []spatial.Position {
	var grid spatial.Grid
	if shape == spatial.GridShapeHex {
		grid = spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: float64(width), SpanHeight: float64(height)})
	} else {
		grid = spatial.NewSquareGrid(spatial.SquareGridConfig{Width: float64(width), Height: float64(height)})
	}

	pad := width + height + 4 // generous — bigger than any valid span in either family
	var cells []spatial.Position
	for x := -pad; x <= pad; x++ {
		for y := -pad; y <= pad; y++ {
			pos := spatial.Position{X: float64(x), Y: float64(y)}
			if grid.IsValidPosition(pos) {
				cells = append(cells, pos)
			}
		}
	}
	return cells
}

// TestAtlasCellsMatchIsValidPosition is the ruling-4 property test: for a
// spread of dimension parities (odd, even, and 1xN thin rooms) across
// both grid families, Atlas's room Cells (Origin zero, so absolute ==
// local) must be EXACTLY the set spatial's own IsValidPosition accepts —
// not a subset, not a superset. A one-cell span drift in the enumerator
// fails this for some parity in the spread.
func (s *EncounterTestSuite) TestAtlasCellsMatchIsValidPosition() {
	dims := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	shapes := map[string]spatial.GridShape{"square": spatial.GridShapeSquare, "hex": spatial.GridShapeHex}

	for shapeName, shape := range shapes {
		for _, w := range dims {
			for _, h := range dims {
				s.Run(fmt.Sprintf("%s-%dx%d", shapeName, w, h), func() {
					enc, err := encounter.NewEncounter(singleRoomSetup(shape, w, h))
					s.Require().NoError(err)

					atlas, err := enc.Atlas()
					s.Require().NoError(err)
					s.Require().Len(atlas.Rooms, 1)

					got := make(map[spatial.Position]bool, len(atlas.Rooms[0].Cells))
					for _, c := range atlas.Rooms[0].Cells {
						got[c] = true
					}

					want := make(map[spatial.Position]bool)
					for _, c := range bruteForceLocalCells(shape, w, h) {
						want[c] = true
					}

					s.Require().Equal(want, got,
						"atlasCells must agree exactly with spatial's own IsValidPosition")
				})
			}
		}
	}
}

// TestAtlasDeterministicOrdering pins C8: Rooms sorted by room ID and
// Doorways sorted by connection ID, regardless of declaration order —
// validAtlasOrderingSetup declares both scrambled.
func (s *EncounterTestSuite) TestAtlasDeterministicOrdering() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	atlas, err := enc.Atlas()
	s.Require().NoError(err)

	gotRoomIDs := make([]string, len(atlas.Rooms))
	for i, r := range atlas.Rooms {
		gotRoomIDs[i] = r.ID
	}
	s.Require().Equal([]string{"atlas-r1", "atlas-r2", "atlas-r3", "atlas-r4"}, gotRoomIDs,
		"Rooms must be sorted by ID regardless of declaration order")

	gotConnIDs := make([]string, len(atlas.Doorways))
	for i, d := range atlas.Doorways {
		gotConnIDs[i] = d.Connection
	}
	s.Require().Equal([]string{"atlas-connA", "atlas-connB"}, gotConnIDs,
		"Doorways must be sorted by connection ID regardless of declaration order")
}

// TestAtlasRoomCellsAndOccludersAreAbsolute pins exact absolute values —
// local cell + Origin, element-wise — and would die under a sign-flipped
// Add-vs-Subtract mutant inside Atlas itself (distinct from the same
// mutant inside Absolute/Locate, pinned separately). Also pins
// AtlasBoundary's absolute projection (#929 hardening round A): both
// endpoints offset by Origin, independently — a mutant that projects only
// From, or swaps From/To, changes one or both expected values here.
func (s *EncounterTestSuite) TestAtlasRoomCellsAndOccludersAreAbsolute() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	atlas, err := enc.Atlas()
	s.Require().NoError(err)

	var r1 encounter.AtlasRoom
	found := false
	for _, r := range atlas.Rooms {
		if r.ID == "atlas-r1" {
			r1 = r
			found = true
		}
	}
	s.Require().True(found, "atlas-r1 must be present")

	s.Require().Contains(r1.Cells, spatial.Position{X: 0, Y: 7}, "local (0,0) projected through Origin")
	s.Require().Contains(r1.Cells, spatial.Position{X: 3, Y: 9}, "local (3,2), the far corner, projected through Origin")
	s.Require().Equal([]spatial.Position{{X: 1, Y: 8}}, r1.Occluders, "occluder must be offset by Origin too")

	// #929 T3 fix round item 3: an occluder's cell is floor AND blockage —
	// it must appear in BOTH Cells and Occluders (ruling 1: occlusion is
	// walkability, not ownership). This is the property hosts actually
	// render by: floor from Cells, blockage layered from Occluders.
	s.Require().Contains(r1.Cells, spatial.Position{X: 1, Y: 8}, "an occluded cell is still floor — it must appear in Cells too")

	// #929 hardening round A: local (0,0)-(1,0) projected through Origin
	// (0,7) — BOTH endpoints offset, flags carried through unchanged.
	s.Require().Equal([]encounter.AtlasBoundary{
		{From: spatial.Position{X: 0, Y: 9}, To: spatial.Position{X: 1, Y: 9}, BlocksMovement: true, BlocksLineOfSight: true},
	}, r1.Boundaries, "boundary endpoints must each be offset by Origin, flags preserved")
}

// TestAtlasDoorwaysAreAbsolute pins exact FromCell/ToCell values, each
// offset by its OWN room's Origin (asymmetric: atlas-r1 and atlas-r2
// have different, nonzero origins) — dies under a sign flip or a
// wrong-room-Origin cross-wiring mutant.
func (s *EncounterTestSuite) TestAtlasDoorwaysAreAbsolute() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.Doorways, 2)

	connA := atlas.Doorways[0] // sorted: connA precedes connB
	s.Require().Equal("atlas-connA", connA.Connection)
	s.Require().Equal("atlas-r1", connA.From)
	s.Require().Equal(spatial.Position{X: 3, Y: 8}, connA.FromCell)
	s.Require().Equal("atlas-r2", connA.To)
	s.Require().Equal(spatial.Position{X: 4, Y: 8}, connA.ToCell)
}

// TestAtlasUnaffectedByLiveState pins ruling 3: Atlas is construction
// truth, never live truth. Placing (already done by Setup), moving a
// member, and pumping a clock tick must never change it.
func (s *EncounterTestSuite) TestAtlasUnaffectedByLiveState() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	before, err := enc.Atlas()
	s.Require().NoError(err)

	// atlas-r1 is anchored at (0,7), so its own (3,0) is (3,7) on the map.
	_, err = enc.Step(&encounter.StepInput{Member: "atlas-p1", To: spatial.Position{X: 3, Y: 7}})
	s.Require().NoError(err)

	_, err = enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)

	after, err := enc.Atlas()
	s.Require().NoError(err)

	s.Require().Equal(before, after,
		"Atlas must be computed from construction data only — live member placement and clock state must never leak in")
}

// TestAtlasCopyOutImmunity mutates every slice a returned Atlas exposes
// and re-fetches — the second Atlas must be unaffected, proving nothing
// aliases internal storage (MUTATION-PROOF: verified by temporarily
// aliasing Occluders directly to fieldInput and observing this fail,
// per #929 T3 mutation-evidence protocol).
func (s *EncounterTestSuite) TestAtlasCopyOutImmunity() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	atlas1, err := enc.Atlas()
	s.Require().NoError(err)

	for i := range atlas1.Rooms {
		for j := range atlas1.Rooms[i].Cells {
			atlas1.Rooms[i].Cells[j] = spatial.Position{X: -999, Y: -999}
		}
		for j := range atlas1.Rooms[i].Occluders {
			atlas1.Rooms[i].Occluders[j] = spatial.Position{X: -999, Y: -999}
		}
		for j := range atlas1.Rooms[i].Boundaries {
			atlas1.Rooms[i].Boundaries[j] = encounter.AtlasBoundary{
				From: spatial.Position{X: -999, Y: -999}, To: spatial.Position{X: -999, Y: -999},
			}
		}
	}
	for i := range atlas1.Doorways {
		atlas1.Doorways[i].FromCell = spatial.Position{X: -999, Y: -999}
		atlas1.Doorways[i].ToCell = spatial.Position{X: -999, Y: -999}
	}

	atlas2, err := enc.Atlas()
	s.Require().NoError(err)

	var r1 encounter.AtlasRoom
	for _, r := range atlas2.Rooms {
		if r.ID == "atlas-r1" {
			r1 = r
		}
	}
	s.Require().Equal([]spatial.Position{{X: 1, Y: 8}}, r1.Occluders,
		"mutating the first snapshot's Occluders must not corrupt internal state")
	s.Require().Contains(r1.Cells, spatial.Position{X: 0, Y: 7},
		"mutating the first snapshot's Cells must not corrupt internal state")
	s.Require().Equal(spatial.Position{X: 3, Y: 8}, atlas2.Doorways[0].FromCell,
		"mutating the first snapshot's Doorways must not corrupt internal state")
	s.Require().Equal([]encounter.AtlasBoundary{
		{From: spatial.Position{X: 0, Y: 9}, To: spatial.Position{X: 1, Y: 9}, BlocksMovement: true, BlocksLineOfSight: true},
	}, r1.Boundaries, "mutating the first snapshot's Boundaries must not corrupt internal state (#929 hardening round A)")
}

// TestAtlasIdenticalAfterReload pins the wave's reload-behavior-identity
// property (established at T2 for traversal — TestReloadedAnchoredEncounterAcceptsSameTraverse
// in data_test.go) for Atlas specifically: an anchored multi-room
// encounter's Atlas, captured before ToData/LoadEncounter and again after,
// must be deep-equal (#929 T3 fix round item 2).
//
// This proves Cells ordering is DETERMINISTIC across a reload — not that
// the order is any PARTICULAR order (#929 hardening round, test-gap
// closure item 2 — corrects this comment's earlier "including Cells
// ordering" claim, which overclaimed what a same-enumerator comparison
// can show): both atlas1 and atlas2 are produced by calling the exact
// SAME atlasCells enumerator, so a mutant that reordered atlasCells
// itself (e.g. swapping its Q-outer/R-inner nesting to R-outer/Q-inner)
// would reorder BOTH calls identically and still pass here. The exact,
// intended order is pinned separately, against a hand-computed fixture
// independent of atlasCells' own implementation, by
// TestAtlasCellsExactOrder.
func (s *EncounterTestSuite) TestAtlasIdenticalAfterReload() {
	enc1, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	atlas1, err := enc1.Atlas()
	s.Require().NoError(err)

	data := enc1.ToData()
	enc2, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Standing: everyoneStanding{}, Initiative: orderAsGiven{}, Data: data})
	s.Require().NoError(err)

	atlas2, err := enc2.Atlas()
	s.Require().NoError(err)

	s.Require().Equal(atlas1, atlas2, "Atlas must be identical after a ToData/LoadEncounter round trip — deterministic, though this alone cannot prove the order is the INTENDED one (see TestAtlasCellsExactOrder)")
}

// TestAtlasCellsExactOrder pins atlasCells' Q-outer/R-inner nesting order
// exactly, against a fixture independent of atlasCells' own
// implementation (#929 hardening round, test-gap closure item 2):
// TestAtlasCellsMatchIsValidPosition only proves SET membership (both
// sides are compared as maps), and TestAtlasIdenticalAfterReload only
// proves the SAME enumerator is deterministic across two calls — neither
// can catch a reordering that both calls agree on, such as swapping
// atlasCells' outer/inner loop nesting. A small, hand-computed 2x3
// square room (Origin zero, so absolute == local) makes the full
// expected order easy to verify by inspection: Q outer over X∈[0,1], R
// inner over Y∈[0,2].
func (s *EncounterTestSuite) TestAtlasCellsExactOrder() {
	enc, err := encounter.NewEncounter(singleRoomSetup(spatial.GridShapeSquare, 2, 3))
	s.Require().NoError(err)

	atlas, err := enc.Atlas()
	s.Require().NoError(err)
	s.Require().Len(atlas.Rooms, 1)

	s.Require().Equal([]spatial.Position{
		{X: 0, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: 2},
		{X: 1, Y: 0}, {X: 1, Y: 1}, {X: 1, Y: 2},
	}, atlas.Rooms[0].Cells, "atlasCells must iterate Q (X) outer, R (Y) inner, in exactly this order")
}

// TestChamberOwnershipAtKissingDoorway pins the case that makes W2's
// uniqueness meaningful: each doorway cell — immediately adjacent to a cell in
// the OTHER chamber — belongs to its OWN chamber, not its neighbour's.
//
// It used to ask the Locate bridge, which is gone. The question survives it,
// and it is asked the way it is asked now: a member standing on a cell is
// reported in the chamber whose authored footprint holds it (rpg-toolkit#1106).
// The two members here stand one cell apart, on opposite sides of a doorway —
// exactly the pair a room-membership answer has to get right.
func (s *EncounterTestSuite) TestChamberOwnershipAtKissingDoorway() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	// atlas-r1 local (3,1) is absolute (3,8); atlas-r2 local (0,2) is (4,8).
	_, err = enc.Join(&encounter.JoinInput{
		Member: "near", Kind: encounter.KindPlayer, Cell: spatial.Position{X: 3, Y: 8}})
	s.Require().NoError(err)
	_, err = enc.Join(&encounter.JoinInput{
		Member: "far", Kind: encounter.KindPlayer, Cell: spatial.Position{X: 4, Y: 8}})
	s.Require().NoError(err)

	rooms := map[encounter.MemberID]string{}
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		rooms[m.ID] = m.Room
	}
	s.Require().Equal("atlas-r1", rooms["near"], "the doorway cell in atlas-r1 belongs to atlas-r1, not atlas-r2")
	s.Require().Equal("atlas-r2", rooms["far"], "the doorway cell in atlas-r2 belongs to atlas-r2, not atlas-r1")
}

// TestAnOccludedCellIsStillFloor pins ruling 1 in the terms that survive the
// bridges: occlusion is walkability, not ownership. A member standing on an
// occluder's own cell is reported in the chamber that owns it, exactly as one
// on an empty cell is — the Atlas already reports occluders as a SUBSET of
// Cells (TestAtlasRoomCellsAndOccludersAreAbsolute), and this is the runtime
// half of the same claim.
func (s *EncounterTestSuite) TestAnOccludedCellIsStillFloor() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	// atlas-r1's occluder is local (1,1), absolute (1,8).
	_, err = enc.Join(&encounter.JoinInput{
		Member: "onTheRubble", Kind: encounter.KindPlayer, Cell: spatial.Position{X: 1, Y: 8}})
	s.Require().NoError(err, "an occluder must not bar a placement — it blocks sight, not floor")

	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == "onTheRubble" {
			s.Require().Equal("atlas-r1", m.Room)
			s.Require().Equal(spatial.Position{X: 1, Y: 8}, m.Position)
			return
		}
	}
	s.Require().Fail("the member is not on the roster")
}

// TestVoidIsNotFloor is what is left of Locate's rejection classes, asked of
// the verb that still needs the answer: the canvas spans the field's bounding
// box, so a cell can be on the map and belong to no chamber, and arriving
// there is refused.
func (s *EncounterTestSuite) TestVoidIsNotFloor() {
	enc, err := encounter.NewEncounter(validAtlasVoidGapSetup())
	s.Require().NoError(err)

	// The 3-cell corridor of void between gap-a (X in [0,2]) and gap-b
	// (X in [6,8]) — on the canvas, owned by nothing.
	_, err = enc.Join(&encounter.JoinInput{
		Member: "nowhere", Kind: encounter.KindPlayer, Cell: spatial.Position{X: 4, Y: 1}})
	s.Require().ErrorIs(err, encounter.ErrBadPlacement)
	s.Require().Contains(err.Error(), "not floor")
}
