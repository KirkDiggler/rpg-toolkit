// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// single_room_placement_test.go pins THE APPROVED SOURCE-TO-CANONICAL MAPPING
// (single-room-play.md §3) term by term, against the committed world-builder
// fixture's own table: scale k = FeetPerCell/sqrt(3), the negative Three yaw
// degrees, the D/W name swap, and the owner-local offset. What a canonical
// placement DOES once compiled is TestPlacedPropsSuite's subject; this suite
// pins that the construction boundary produces exactly the geometry the
// approved frame names, deterministically, and refuses ineligible sources.
type SingleRoomPlacementSuite struct {
	suite.Suite
	raw []byte
}

func TestSingleRoomPlacementSuite(t *testing.T) {
	suite.Run(t, new(SingleRoomPlacementSuite))
}

func (s *SingleRoomPlacementSuite) SetupTest() {
	var err error
	s.raw, err = os.ReadFile("testdata/world-builder-v3.yaml")
	s.Require().NoError(err)
}

func (s *SingleRoomPlacementSuite) decode() *SingleRoomSpec {
	out, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)

	return out.Spec
}

func (s *SingleRoomPlacementSuite) TestCanonicalPlacedPropsMapTheApprovedFrame() {
	out, err := s.decode().Room.CanonicalPlacedProps()
	s.Require().NoError(err)
	s.Require().Len(out, 1, "the fixture declares exactly one prop")

	k := encounter.FeetPerCell / math.Sqrt(3)
	table := out[0]
	s.Equal("table", table.ID)
	s.Equal(-2.25*k, table.Placement.Origin.X, "origin x is transform.x in feet")
	s.Equal(1.3*k, table.Placement.Origin.Y, "origin y is transform.z in feet — the X/Z plane")
	s.Equal(-0.37*180/math.Pi, table.Placement.Facing, "positive Three Y yaw turns +X toward -Z")
	s.Require().NotNil(table.Placement.Footprint.Box)
	s.Equal(1.2*k, table.Placement.Footprint.Box.D, "spatial D lies along the facing: the width")
	s.Equal(0.5*k, table.Placement.Footprint.Box.W, "spatial W lies across it: the depth")
	s.Equal(0.1*k, table.Placement.LocalOffset.X, "the local rectangle rides its own axes")
	s.Equal(-0.2*k, table.Placement.LocalOffset.Y)
	s.True(table.BlocksMovement)
	s.False(table.BlocksLineOfSight)
}

func (s *SingleRoomPlacementSuite) TestCanonicalPlacedPropsSortByID() {
	yes, no := true, false
	spec := RoomSource{
		Scene: encounter.RoomVisualScene{Items: []encounter.RoomSceneItem{
			{ID: "b", Kind: propKind, Transform: encounter.RoomSceneTransform{X: 1}},
			{ID: "a", Kind: propKind, Transform: encounter.RoomSceneTransform{X: 2}},
		}},
		Gameplay: RoomGameplaySource{PropDeclarations: map[string]RoomPropDeclaration{
			"b": {BlocksMovement: &yes, BlocksLineOfSight: &no, Footprint: RoomFootprint{Width: 1, Depth: 1}},
			"a": {BlocksMovement: &no, BlocksLineOfSight: &yes, Footprint: RoomFootprint{Width: 1, Depth: 1}},
		}},
	}

	out, err := spec.CanonicalPlacedProps()
	s.Require().NoError(err)
	s.Require().Len(out, 2)
	s.Equal("a", out[0].ID, "map iteration order never reaches geometry")
	s.Equal(2.0*feetPerSourceUnit, out[0].Placement.Origin.X)
	s.Equal("b", out[1].ID)
	s.Equal(1.0*feetPerSourceUnit, out[1].Placement.Origin.X)
}

func (s *SingleRoomPlacementSuite) TestCanonicalPlacedPropsAreAbsentWithoutDeclarations() {
	out, err := s.decode().Room.CanonicalPlacedProps()
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Len(out, 1)

	empty, err := (&RoomSource{}).CanonicalPlacedProps()
	s.Require().NoError(err)
	s.Nil(empty, "no declarations is no contributors, not an empty non-nil list")
}

func (s *SingleRoomPlacementSuite) TestCanonicalPlacedPropsRefuseIneligibleSources() {
	yes := true
	dangling := &RoomSource{
		Scene: encounter.RoomVisualScene{Items: []encounter.RoomSceneItem{
			{ID: "table", Kind: propKind, Transform: encounter.RoomSceneTransform{X: 1}},
		}},
		Gameplay: RoomGameplaySource{PropDeclarations: map[string]RoomPropDeclaration{
			"ghost": {BlocksMovement: &yes, BlocksLineOfSight: &yes,
				Footprint: RoomFootprint{Width: 1, Depth: 1}},
		}},
	}
	_, err := dangling.CanonicalPlacedProps()
	s.Require().Error(err, "a declaration naming no live prop is refused, not skipped")

	flagless := &RoomSource{
		Scene: encounter.RoomVisualScene{Items: []encounter.RoomSceneItem{
			{ID: "table", Kind: propKind},
		}},
		Gameplay: RoomGameplaySource{PropDeclarations: map[string]RoomPropDeclaration{
			"table": {Footprint: RoomFootprint{Width: 1, Depth: 1}},
		}},
	}
	_, err = flagless.CanonicalPlacedProps()
	s.Require().Error(err, "a declaration that says neither blocking answer is refused")
}
