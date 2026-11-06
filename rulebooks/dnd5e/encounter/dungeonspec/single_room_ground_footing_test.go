// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func (s *SingleRoomWallDoorSuite) TestGroundPaintLeavesWithOpaqueWallFooting() {
	spec := s.footingSpec()
	spec.Room.Gameplay.Regions = []dungeonspec.RoomRegion{
		dungeonspec.OneRegion("room-1-region", "West", []dungeonspec.RoomCell{{Q: 0, R: 0}}),
		dungeonspec.OneRegion("east", "East", []dungeonspec.RoomCell{{Q: 1, R: 0}, {Q: 2, R: 0}, {Q: 3, R: 0}, {Q: 4, R: 0}}),
	}
	spec.Room.Gameplay.Regions[1].Ground = &dungeonspec.RoomGround{BaseRef: "grass", Paint: []dungeonspec.RoomGroundPaint{{Ref: "dirt", Cells: []dungeonspec.RoomCellRow{{R: 0, Q: [][2]int{{1, 2}}}}}}}
	compiled := s.load(spec)
	s.Require().NotNil(compiled.Field.Regions[1].Ground)
	s.Require().Len(compiled.Field.Regions[1].Ground.Paint, 1)
	s.Equal([]spatial.Position{{X: 2, Y: 0}}, compiled.Field.Regions[1].Ground.Paint[0].Cells)
	s.Equal([]spatial.Position{{X: 1, Y: 0}}, compiled.Field.Scenery)
	s.Equal([]dungeonspec.RoomCellRow{{R: 0, Q: [][2]int{{1, 2}}}}, spec.Room.Gameplay.Regions[1].Ground.Paint[0].Cells, "lowering does not mutate the editor's source")
}
