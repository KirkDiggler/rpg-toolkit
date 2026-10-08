// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

func (s *SingleRoomWallDoorSuite) TestKnownWallKeepsFloorUnderAClosedDoor() {
	for _, secret := range []bool{false, true} {
		s.Run(fmt.Sprintf("secret=%t", secret), func() {
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
			spec.Room.Scene = sceneNode(s.T(), `
version: 1
id: scene-1
name: Threshold
items: [{id: threshold-prop, kind: prop, assetRef: 'dnd5e:props:books', transform: {x: `+fmtFloat(x)+`, y: 0, z: 0, rotationY: 0}}]
groups: []
`)
			spec.Room.Gameplay.PropDeclarations = map[string]dungeonspec.RoomPropDeclaration{
				"threshold-prop": wallBlocks(false, false, 0.1, 0.1, 0, 0),
			}
			if secret {
				spec.Concealments = map[string]dungeonspec.ConcealmentSpec{"door-only": {Checks: dungeonspec.CheckSpec{{Ability: "perception", DC: 20}}, Props: []string{"gap-door"}}}
			}
			enc := s.play(s.load(spec))
			before, err := enc.AtlasFor("walker")
			s.Require().NoError(err)
			s.Require().NotEmpty(before.StructuralWalls, "the bordering wall is already known")
			s.Contains(before.Cells, axial(1, 0), "the known wall needs floor beneath its closed door, independently of door concealment")
			s.Contains(before.Sealed, axial(1, 0), "unknown threshold ownership is not permission to stand through a closed leaf")
			s.NotContains(before.Cells, axial(3, 0), "footing does not discover the other room")
			for _, p := range before.Placed {
				s.NotEqual("threshold-prop", p.ID, "footing alone does not grant object permission")
			}
			if secret {
				s.Empty(before.StructuralDoors)
				s.Empty(before.StructuralWalls[0].Openings)
			}

			// A secret cut must not punch a hole that the solid-wall twin lacks.
			spec.Room.Gameplay.Walls[0].Openings = nil
			spec.Room.Gameplay.DoorBindings = nil
			spec.Concealments = nil
			solid, err := s.play(s.load(spec)).AtlasFor("walker")
			s.Require().NoError(err)
			s.Equal(solid.Cells, before.Cells)
			s.Equal(solid.Sealed, before.Sealed)

			if !secret {
				_, err = enc.OpenDoor(&encounter.OpenDoorInput{Actor: "walker", Door: "wall-doors-room/gap-door"})
				s.Require().NoError(err)
				after, err := enc.AtlasFor("walker")
				s.Require().NoError(err)
				s.Contains(after.Cells, axial(1, 0))
				s.NotContains(after.Sealed, axial(1, 0), "opening teaches ownership; threshold was not made permanent scenery")
				s.Contains(after.Cells, axial(3, 0))
				restored, err := s.reload(enc).AtlasFor("walker")
				s.Require().NoError(err)
				s.Equal(after, restored)
			}
		})
	}
}
