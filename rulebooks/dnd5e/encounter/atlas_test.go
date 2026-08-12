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
// room has a distinct, nonzero-on-both-axes Origin (r1: (-50,7), r2:
// (-46,6)) so a sign-flipped bridge (Subtract instead of Add, or vice
// versa) produces a wrong, non-accidental-match absolute value.
func validAtlasOrderingSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "atlas-r3", Width: 4, Height: 3, Origin: spatial.Position{X: 9950, Y: 7}},
				{ID: "atlas-r1", Width: 4, Height: 3, Origin: spatial.Position{X: -50, Y: 7},
					Occluders: []spatial.Position{{X: 1, Y: 1}}},
				{ID: "atlas-r4", Width: 6, Height: 5, Origin: spatial.Position{X: 9954, Y: 6}},
				{ID: "atlas-r2", Width: 6, Height: 5, Origin: spatial.Position{X: -46, Y: 6}},
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

// singleRoomSetup builds a minimal one-room encounter for the enumeration
// property test — Origin zero, so a room's Atlas Cells equal its local
// cells exactly, letting the test compare against spatial's own
// IsValidPosition without any Origin arithmetic in the way.
func singleRoomSetup(shape spatial.GridShape, width, height int) *encounter.SetupInput {
	return &encounter.SetupInput{
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
// mutant inside Absolute/Locate, pinned separately).
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

	s.Require().Contains(r1.Cells, spatial.Position{X: -50, Y: 7}, "local (0,0) projected through Origin")
	s.Require().Contains(r1.Cells, spatial.Position{X: -47, Y: 9}, "local (3,2), the far corner, projected through Origin")
	s.Require().Equal([]spatial.Position{{X: -49, Y: 8}}, r1.Occluders, "occluder must be offset by Origin too")
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
	s.Require().Equal(spatial.Position{X: -47, Y: 8}, connA.FromCell)
	s.Require().Equal("atlas-r2", connA.To)
	s.Require().Equal(spatial.Position{X: -46, Y: 8}, connA.ToCell)
}

// TestAtlasUnaffectedByLiveState pins ruling 3: Atlas is construction
// truth, never live truth. Placing (already done by Setup), moving a
// member, and pumping a clock tick must never change it.
func (s *EncounterTestSuite) TestAtlasUnaffectedByLiveState() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	before, err := enc.Atlas()
	s.Require().NoError(err)

	_, err = enc.Move(&encounter.MoveInput{Member: "atlas-p1", To: spatial.Position{X: 3, Y: 0}})
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
	s.Require().Equal([]spatial.Position{{X: -49, Y: 8}}, r1.Occluders,
		"mutating the first snapshot's Occluders must not corrupt internal state")
	s.Require().Contains(r1.Cells, spatial.Position{X: -50, Y: 7},
		"mutating the first snapshot's Cells must not corrupt internal state")
	s.Require().Equal(spatial.Position{X: -47, Y: 8}, atlas2.Doorways[0].FromCell,
		"mutating the first snapshot's Doorways must not corrupt internal state")
}

// TestLocateAbsoluteRoundTripHex pins the round-trip law over EVERY
// in-bounds cell of BOTH rooms in the canonical asymmetric hex fixture:
// Locate(Absolute(r,p)) == (r,p).
func (s *EncounterTestSuite) TestLocateAbsoluteRoundTripHex() {
	enc, err := encounter.NewEncounter(validAnchoredHexSetup())
	s.Require().NoError(err)

	rooms := []struct {
		id            string
		width, height int
	}{
		{"hex-big", 10, 4},
		{"hex-small", 3, 9},
	}
	for _, room := range rooms {
		for _, local := range bruteForceLocalCells(spatial.GridShapeHex, room.width, room.height) {
			abs, err := enc.Absolute(&encounter.AbsoluteInput{Room: room.id, Position: local})
			s.Require().NoError(err, "room %s cell %v", room.id, local)

			loc, err := enc.Locate(&encounter.LocateInput{Position: abs.Position})
			s.Require().NoError(err, "room %s cell %v absolute %v", room.id, local, abs.Position)
			s.Require().Equal(room.id, loc.Room, "round trip must return to the same room")
			s.Require().Equal(local, loc.Position, "round trip must return to the same local position")
		}
	}
}

// TestLocateAbsoluteRoundTripFractionalSquare pins ruling 2: a fractional
// SQUARE position is legal and round-trips exactly, subtract-then-bounds.
func (s *EncounterTestSuite) TestLocateAbsoluteRoundTripFractionalSquare() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	local := spatial.Position{X: 1.5, Y: 0.5}
	abs, err := enc.Absolute(&encounter.AbsoluteInput{Room: "atlas-r1", Position: local})
	s.Require().NoError(err)
	s.Require().Equal(spatial.Position{X: -48.5, Y: 7.5}, abs.Position)

	loc, err := enc.Locate(&encounter.LocateInput{Position: abs.Position})
	s.Require().NoError(err)
	s.Require().Equal("atlas-r1", loc.Room)
	s.Require().Equal(local, loc.Position)
}

// TestLocateOwnershipAtKissingDoorway pins the case that makes W2's
// uniqueness meaningful: each doorway cell — immediately adjacent to a
// cell in the OTHER room — must locate to its OWN room, not its
// neighbor's.
func (s *EncounterTestSuite) TestLocateOwnershipAtKissingDoorway() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	fromAbs, err := enc.Absolute(&encounter.AbsoluteInput{Room: "atlas-r1", Position: spatial.Position{X: 3, Y: 1}})
	s.Require().NoError(err)
	toAbs, err := enc.Absolute(&encounter.AbsoluteInput{Room: "atlas-r2", Position: spatial.Position{X: 0, Y: 2}})
	s.Require().NoError(err)

	fromLoc, err := enc.Locate(&encounter.LocateInput{Position: fromAbs.Position})
	s.Require().NoError(err)
	s.Require().Equal("atlas-r1", fromLoc.Room, "the doorway cell in atlas-r1 must locate to atlas-r1, not atlas-r2")

	toLoc, err := enc.Locate(&encounter.LocateInput{Position: toAbs.Position})
	s.Require().NoError(err)
	s.Require().Equal("atlas-r2", toLoc.Room, "the doorway cell in atlas-r2 must locate to atlas-r2, not atlas-r1")
}

// TestAbsoluteLegalOnOccludedCell and TestLocateOwnsOccludedCell pin
// ruling 1: occlusion is walkability, not ownership — both bridges treat
// an occluded cell exactly like an empty one.
func (s *EncounterTestSuite) TestAbsoluteLegalOnOccludedCell() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	out, err := enc.Absolute(&encounter.AbsoluteInput{Room: "atlas-r1", Position: spatial.Position{X: 1, Y: 1}})
	s.Require().NoError(err, "an occluder must not bar Absolute from projecting the cell")
	s.Require().Equal(spatial.Position{X: -49, Y: 8}, out.Position)
}

// TestLocateOwnsOccludedCell is the Locate-side sibling of
// TestAbsoluteLegalOnOccludedCell.
func (s *EncounterTestSuite) TestLocateOwnsOccludedCell() {
	enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
	s.Require().NoError(err)

	loc, err := enc.Locate(&encounter.LocateInput{Position: spatial.Position{X: -49, Y: 8}})
	s.Require().NoError(err, "an occluder must not bar Locate from resolving the cell's owner")
	s.Require().Equal("atlas-r1", loc.Room)
	s.Require().Equal(spatial.Position{X: 1, Y: 1}, loc.Position)
}

// TestAbsoluteRejections pins Absolute's R5 rejection classes.
func (s *EncounterTestSuite) TestAbsoluteRejections() {
	s.Run("nil input", func() {
		enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
		s.Require().NoError(err)
		_, err = enc.Absolute(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("unknown room", func() {
		enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
		s.Require().NoError(err)
		_, err = enc.Absolute(&encounter.AbsoluteInput{Room: "nowhere", Position: spatial.Position{X: 0, Y: 0}})
		s.Require().ErrorIs(err, encounter.ErrNoField)
		s.Require().Contains(err.Error(), `unknown room "nowhere"`)
	})

	s.Run("out of bounds", func() {
		enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
		s.Require().NoError(err)
		_, err = enc.Absolute(&encounter.AbsoluteInput{Room: "atlas-r1", Position: spatial.Position{X: 100, Y: 100}})
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Require().Contains(err.Error(), "out of bounds")
	})

	s.Run("hex fractional position", func() {
		enc, err := encounter.NewEncounter(validAnchoredHexSetup())
		s.Require().NoError(err)
		_, err = enc.Absolute(&encounter.AbsoluteInput{Room: "hex-big", Position: spatial.Position{X: 0.5, Y: 0}})
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Require().Contains(err.Error(), "not an integral axial cell")
	})
}

// TestLocateRejections pins Locate's R5 rejection classes.
func (s *EncounterTestSuite) TestLocateRejections() {
	s.Run("nil input", func() {
		enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
		s.Require().NoError(err)
		_, err = enc.Locate(nil)
		s.Require().ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("void position owned by no room", func() {
		enc, err := encounter.NewEncounter(validAtlasOrderingSetup())
		s.Require().NoError(err)
		_, err = enc.Locate(&encounter.LocateInput{Position: spatial.Position{X: 99999, Y: 99999}})
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Require().Contains(err.Error(), "owned by no room")
	})

	s.Run("fractional hex absolute position resolves to no room", func() {
		// (0.5,0) is not a real cell of any hex room — must be void, not
		// silently truncated into hex-big's ownership.
		enc, err := encounter.NewEncounter(validAnchoredHexSetup())
		s.Require().NoError(err)
		_, err = enc.Locate(&encounter.LocateInput{Position: spatial.Position{X: 0.5, Y: 0}})
		s.Require().ErrorIs(err, encounter.ErrBadPlacement)
		s.Require().Contains(err.Error(), "owned by no room")
	})
}
