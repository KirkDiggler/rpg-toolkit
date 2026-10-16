// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// thresholdSpec is TestKnownWallKeepsFloorUnderAClosedDoor's room: a boundary
// wall with a gap and a closed door bound to it, the door's prop concealed
// when secret, and the solid twin when !door (no opening, no binding, no
// secret).
func (s *SingleRoomWallDoorSuite) thresholdSpec(door bool) *dungeonspec.SingleRoomSpec {
	spec := s.gapSpec(dungeonspec.RoomDoorBinding{Closed: true})
	spec.Room.Gameplay.WalkableHexes = nil
	for q := 0; q <= 3; q++ {
		for r := -1; r <= 1; r++ {
			spec.Room.Gameplay.WalkableHexes = append(spec.Room.Gameplay.WalkableHexes, dungeonspec.RoomCell{Q: q, R: r})
		}
	}
	x := boundaryWallX(5)
	spec.Room.Gameplay.Walls = []dungeonspec.RoomWall{roomWall("boundary", x, -6, x, 6, wallBlocks(true, true, 12, 0.2, 0, 0),
		dungeonspec.RoomWallOpening{ID: "gap", Position: 6, Width: 2, Door: &dungeonspec.RoomWallDoor{ID: "gap-door", AssetRef: "dnd5e:env:test:door"}})}
	spec.Concealments = map[string]dungeonspec.ConcealmentSpec{"door-only": {Checks: dungeonspec.CheckSpec{{Ability: "perception", DC: 20}}, Props: []string{"gap-door"}}}
	if !door {
		spec.Room.Gameplay.Walls[0].Openings = nil
		spec.Room.Gameplay.DoorBindings = nil
		spec.Concealments = nil
	}

	return spec
}

// TestUnfoundSecretDoorRegionsEqualTheSolidTwin is the region
// half of the masquerade on compiled content (fog-of-war R9). The compiler
// partitions a room by geometry, so the hex a closed door encloses is its own
// one-hex space; the solid twin's hex there is ownerless footing. A masked
// threshold keeps its region only when that region is otherwise on the map,
// so the non-knower is not handed a region named after the room exactly at
// the secret door.
func (s *SingleRoomWallDoorSuite) TestUnfoundSecretDoorRegionsEqualTheSolidTwin() {
	secret, err := s.play(s.load(s.thresholdSpec(true))).AtlasFor("walker")
	s.Require().NoError(err)
	solid, err := s.play(s.load(s.thresholdSpec(false))).AtlasFor("walker")
	s.Require().NoError(err)

	s.Contains(secret.Cells, axial(1, 0), "fixture: the threshold is learned and on the map")
	s.Equal(solid.Cells, secret.Cells)
	s.Equal(solid.Sealed, secret.Sealed)
	s.Equal(solid.Regions, secret.Regions, "no region marks the secret door: the threshold is ownerless, as the twin's footing")
	for _, r := range secret.Regions {
		for _, c := range r.Cells {
			s.NotEqual(axial(1, 0), c, "the threshold belongs to no region on the non-knower's map")
		}
	}

	// And once found, the threshold is the room's floor again.
	full, err := s.play(s.load(s.thresholdSpec(true))).Atlas()
	s.Require().NoError(err)
	owned := false
	for _, r := range full.Regions {
		for _, c := range r.Cells {
			owned = owned || c == axial(1, 0)
		}
	}
	s.True(owned, "fixture: the door world's threshold has a region of its own")
}
