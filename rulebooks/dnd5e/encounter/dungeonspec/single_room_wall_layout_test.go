// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// single_room_wall_layout_test.go is THE NUMERIC SOURCE WITNESS for the fixed
// structural layout (rpg-project#169, P1): one authored wall lowers to
// canonical feet once, its opening resolves the attached door's visual
// endpoints, and the INDEPENDENT physical blocker keeps its own depth and
// lateral offset untouched.
//
// It is an INTERNAL test because the witness is the lowering function itself,
// named term by term against `feetPerSourceUnit`; a full compile would need a
// corridor for the party to stand in and could not use the exact source line
// the contract names. The full picture is committed next door in
// contentgolden_test.go, and the projection is pinned in the encounter
// package's structural_walls_test.go.
type SingleRoomWallLayoutSuite struct {
	suite.Suite
}

func TestSingleRoomWallLayoutSuite(t *testing.T) {
	suite.Run(t, new(SingleRoomWallLayoutSuite))
}

// layoutWitnessWall is the contract's own source wall: a ten-unit line along
// the X axis, a three-by-0.3 appearance at the floor, a blocker 0.5 deep and
// shifted 0.4 across, and a two-unit opening at position seven carrying a
// door.
func (s *SingleRoomWallLayoutSuite) layoutWitnessWall(openings ...RoomWallOpening) RoomWall {
	movement, sight := true, true

	return RoomWall{
		ID:    "wall-1",
		Label: "witness",
		Line: RoomWallLine{
			Start: RoomWallPoint{X: 0, Z: 0},
			End:   RoomWallPoint{X: 10, Z: 0},
		},
		Appearance: RoomWallAppearance{
			AssetRef: "dnd5e:env:test:wall", Height: 3, Thickness: 0.3, Elevation: 0,
		},
		Blocker: RoomPropDeclaration{
			BlocksMovement:    &movement,
			BlocksLineOfSight: &sight,
			Footprint:         RoomFootprint{Width: 4, Depth: 0.5, OffsetX: 0, OffsetZ: 0.4},
		},
		Openings: openings,
	}
}

func (s *SingleRoomWallLayoutSuite) witnessOpening() RoomWallOpening {
	return RoomWallOpening{
		ID: "doorway", Position: 7, Width: 2,
		Door: &RoomWallDoor{ID: "gate", AssetRef: "dnd5e:env:test:door"},
	}
}

// TestTheSourceLineLowersToCanonicalFeetOnce pins the arithmetic term by
// term: every point and every length is converted by exactly one factor, and
// nothing is converted twice.
func (s *SingleRoomWallLayoutSuite) TestTheSourceLineLowersToCanonicalFeetOnce() {
	k := feetPerSourceUnit
	s.InDelta(5/math.Sqrt(3), k, 1e-15, "k is the approved across-the-flats scale")

	lowered, err := canonicalStructuralWalls("vault-key", []RoomWall{s.layoutWitnessWall(s.witnessOpening())})
	s.Require().NoError(err)
	s.Require().Len(lowered, 1)

	wall := lowered[0]
	s.Equal(encounter.PropID("wall-1"), wall.ID)
	s.Equal("dnd5e:env:test:wall", wall.Ref, "the appearance ref is carried verbatim")
	s.InDelta(0, wall.From.X, 1e-12)
	s.InDelta(0, wall.From.Y, 1e-12)
	s.InDelta(10*k, wall.To.X, 1e-12, "line end 0 -> 10k")
	s.InDelta(0, wall.To.Y, 1e-12)
	s.InDelta(10*k, math.Hypot(wall.To.X-wall.From.X, wall.To.Y-wall.From.Y), 1e-12, "the line's canonical length is 10k")
	s.InDelta(3*k, wall.Height, 1e-12, "visual height 3 -> 3k")
	s.InDelta(0.3*k, wall.Thickness, 1e-12, "visual thickness 0.3 -> 0.3k")
	s.InDelta(0, wall.Elevation, 1e-12)

	s.Require().Len(wall.Openings, 1)
	opening := wall.Openings[0]
	s.Equal("doorway", opening.ID)
	s.InDelta(7*k, opening.Position, 1e-12, "opening centre 7 -> 7k")
	s.InDelta(2*k, opening.Width, 1e-12, "opening width 2 -> 2k")

	s.Require().NotNil(opening.Door, "an opening with a door carries a binding")
	door := opening.Door
	s.Equal(encounter.PropID("gate"), door.PlacedID, "the raw attached id is the placed presence id")
	s.Equal(encounter.DoorID("vault-key/gate"), door.DoorID, "the door id is the gameplay `<key>/<id>` minting")
	s.Equal("dnd5e:env:test:door", door.Ref)
	s.InDelta(6*k, door.From.X, 1e-12, "opening 7 width 2: the visual door runs 6k -> 8k")
	s.InDelta(8*k, door.To.X, 1e-12)
	s.InDelta(0, door.From.Y, 1e-12)
	s.InDelta(0, door.To.Y, 1e-12)
	s.InDelta(2*k, math.Hypot(door.To.X-door.From.X, door.To.Y-door.From.Y), 1e-12, "the door's width is its endpoints' distance")
}

// TestThePhysicalBlockerKeepsItsOwnDepthAndOffset is the independence witness:
// the assembled thickness is 0.3, and the blocker's depth and lateral offset
// stay their own values (0.5 and 0.4), converted by the same scale but never
// replaced by the visual dimensions.
func (s *SingleRoomWallLayoutSuite) TestThePhysicalBlockerKeepsItsOwnDepthAndOffset() {
	k := feetPerSourceUnit
	wall := s.layoutWitnessWall(s.witnessOpening())

	lowered, err := canonicalStructuralWalls("vault-key", []RoomWall{wall})
	s.Require().NoError(err)
	s.Require().Len(lowered, 1)
	s.InDelta(0.3*k, lowered[0].Thickness, 1e-12)

	physical, err := wallLoweringFor(&wall, 0)
	s.Require().NoError(err)
	s.Require().NotNil(physical.presence.Placement.Footprint.Box)
	s.InDelta(0.5*k, physical.presence.Placement.Footprint.Box.W, 1e-12,
		"the blocker's depth stays 0.5k, not the visual thickness 0.3k")
	s.InDelta(0.4*k, physical.presence.Placement.LocalOffset.Y, 1e-12,
		"the blocker's lateral offset stays 0.4k")
	s.NotEqual(lowered[0].Thickness, physical.presence.Placement.Footprint.Box.W,
		"the visual thickness is not the physical depth")
	s.Require().NotEmpty(physical.spans)
	for _, span := range physical.spans {
		s.InDelta(0.5*k, span.Placement.Footprint.Box.W, 1e-12, "every blocking span keeps the blocker's depth")
		s.InDelta(0.4*k, span.Placement.LocalOffset.Y, 1e-12, "every blocking span keeps the blocker's offset")
	}
}

// TestABareOpeningCarriesNoDoor pins the nil binding: absence is an ordinary
// gap, not an open or hidden door.
func (s *SingleRoomWallLayoutSuite) TestABareOpeningCarriesNoDoor() {
	opening := s.witnessOpening()
	opening.Door = nil
	lowered, err := canonicalStructuralWalls("vault-key", []RoomWall{s.layoutWitnessWall(opening)})
	s.Require().NoError(err)
	s.Require().Len(lowered, 1)
	s.Require().Len(lowered[0].Openings, 1)
	s.Nil(lowered[0].Openings[0].Door, "a bare opening binds no door")
}

// TestTheDoorEndpointsFollowTheOpening pins the resolved endpoints for a door
// offset along a diagonal line, so the projection is not just an X-axis
// coincidence.
func (s *SingleRoomWallLayoutSuite) TestTheDoorEndpointsFollowTheOpening() {
	wall := s.layoutWitnessWall(RoomWallOpening{
		ID: "gap", Position: 5, Width: 2,
		Door: &RoomWallDoor{ID: "gate", AssetRef: "dnd5e:env:test:door"},
	})
	// A diagonal line of length 10 along +X/+Z, so the direction is (1,1)/sqrt(2).
	wall.Line = RoomWallLine{
		Start: RoomWallPoint{X: 0, Z: 0},
		End:   RoomWallPoint{X: 10 / math.Sqrt(2), Z: 10 / math.Sqrt(2)},
	}
	lowered, err := canonicalStructuralWalls("vault-key", []RoomWall{wall})
	s.Require().NoError(err)
	s.Require().Len(lowered, 1)
	s.Require().Len(lowered[0].Openings, 1)
	door := lowered[0].Openings[0].Door
	s.Require().NotNil(door)
	// Centre at 5 along a unit diagonal from the origin, opening half-width 1:
	// the door runs 4 -> 6 scene units along the diagonal, then scaled by k.
	k := feetPerSourceUnit
	s.InDelta(4/math.Sqrt(2)*k, door.From.X, 1e-12)
	s.InDelta(4/math.Sqrt(2)*k, door.From.Y, 1e-12)
	s.InDelta(6/math.Sqrt(2)*k, door.To.X, 1e-12)
	s.InDelta(6/math.Sqrt(2)*k, door.To.Y, 1e-12)
	s.InDelta(2*k, math.Hypot(door.To.X-door.From.X, door.To.Y-door.From.Y), 1e-12)
}

// TestALineWithNoLengthRefusesAHandAssembledSource pins the hand-assembled
// guard: the decode refuses this, but the exported lowering still returns an
// error rather than panicking.
func (s *SingleRoomWallLayoutSuite) TestALineWithNoLengthRefusesAHandAssembledSource() {
	wall := s.layoutWitnessWall()
	wall.Line.End = wall.Line.Start
	_, err := canonicalStructuralWalls("vault-key", []RoomWall{wall})
	s.Require().Error(err)
}

// TestAnEmptyWallListLowersToNothing keeps a wall-less room byte-identical:
// no records, so no key is ever written.
func (s *SingleRoomWallLayoutSuite) TestAnEmptyWallListLowersToNothing() {
	lowered, err := canonicalStructuralWalls("vault-key", nil)
	s.Require().NoError(err)
	s.Nil(lowered)
}

// TestTheWitnessGeometryHasNoAssetDependency is a compile-shape guard: the
// lowering needs neither a scene item nor a catalog, and the ref it carries is
// exactly the string the author wrote.
func (s *SingleRoomWallLayoutSuite) TestTheWitnessGeometryHasNoAssetDependency() {
	wall := s.layoutWitnessWall(s.witnessOpening())
	lowered, err := canonicalStructuralWalls("vault-key", []RoomWall{wall})
	s.Require().NoError(err)
	s.Equal(wall.Appearance.AssetRef, lowered[0].Ref)
	s.Equal(wall.Openings[0].Door.AssetRef, lowered[0].Openings[0].Door.Ref)
}

// TestAppearanceChangesTheLayoutButNotTheBlocker pins the new boundary: the
// assembled appearance IS the delivered layout record now, so a changed asset,
// height, thickness or elevation shows up there — while the mechanical blocker
// spans it stands beside are byte-identical.
func (s *SingleRoomWallLayoutSuite) TestAppearanceChangesTheLayoutButNotTheBlocker() {
	plain := s.layoutWitnessWall(s.witnessOpening())
	dressed := plain
	dressed.Appearance = RoomWallAppearance{
		AssetRef: "a-completely-different-asset", Height: 40, Thickness: 9, Elevation: -12,
	}

	plainLayout, err := canonicalStructuralWalls("vault-key", []RoomWall{plain})
	s.Require().NoError(err)
	dressedLayout, err := canonicalStructuralWalls("vault-key", []RoomWall{dressed})
	s.Require().NoError(err)
	s.NotEqual(plainLayout, dressedLayout, "the assembled appearance is delivered layout now")
	s.Equal("a-completely-different-asset", dressedLayout[0].Ref)
	s.InDelta(40*feetPerSourceUnit, dressedLayout[0].Height, 1e-12)
	s.InDelta(9*feetPerSourceUnit, dressedLayout[0].Thickness, 1e-12)
	s.InDelta(-12*feetPerSourceUnit, dressedLayout[0].Elevation, 1e-12)

	plainSpans, err := wallLoweringFor(&plain, 0)
	s.Require().NoError(err)
	dressedSpans, err := wallLoweringFor(&dressed, 0)
	s.Require().NoError(err)
	s.Equal(plainSpans.presence.Placement, dressedSpans.presence.Placement,
		"appearance never reaches the blocker")
	s.Require().Len(plainSpans.spans, len(dressedSpans.spans))
	for i := range plainSpans.spans {
		s.Equal(plainSpans.spans[i].Placement, dressedSpans.spans[i].Placement)
	}
}
