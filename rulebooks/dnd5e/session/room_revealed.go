// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EventRoomRevealed adds newly known fixed room data for its recipient.
const EventRoomRevealed EventKind = "room_revealed"

// RoomRevealedBody is fixed layout, not mutable observation or a whole-atlas
// replacement. Scenery is additional unowned floor (for example wall footing).
type RoomRevealedBody struct {
	Region     AtlasRegion        `json:"region"`
	Scenery    []spatial.Position `json:"scenery"`
	Props      []AtlasProp        `json:"props"`
	Boundaries []AtlasBoundary    `json:"boundaries"`
	Segments   []AtlasSegment     `json:"segments"`
	Sealed     []spatial.Position `json:"sealed"`
	Doorways   []AtlasDoorway     `json:"doorways"`
	Placed     []AtlasPlacedProp  `json:"placed"`
	Exits      []AtlasExit        `json:"exits"`

	// StructuralWalls and StructuralDoors are the optional new-or-changed
	// fixed structural rows this room brought (rpg-project#169, P2E). Each is
	// the COMPLETE projected record for its identity, so applying it by id
	// onto a cached atlas yields the fresh answer; both are absent on a legacy
	// beat and on a beat where the recipient's structural layout did not
	// change.
	StructuralWalls []AtlasStructuralWall `json:"structural_walls,omitempty"`
	StructuralDoors []AtlasStructuralDoor `json:"structural_doors,omitempty"`
}

func (RoomRevealedBody) isEventBody() {}

func roomRevealedBody(payload []byte) EventBody {
	var p struct {
		RoomRevealedBody
		Placed []struct {
			ID                string                  `json:"id"`
			Placement         encounter.PlacementData `json:"placement"`
			Cells             []spatial.Position      `json:"cells"`
			BlocksMovement    bool                    `json:"blocks_movement"`
			BlocksLineOfSight bool                    `json:"blocks_line_of_sight"`
		} `json:"placed"`
	}
	if json.Unmarshal(payload, &p) != nil || p.Region.ID == "" {
		return nil
	}
	// THE STRUCTURAL ROWS ARE DECODED ONCE, SHARED WITH THE CONCEALMENT BEAT.
	// A row missing its identity refuses the whole beat rather than handing a
	// client a half-patched cache; an absent key is the legacy payload and
	// leaves both lists empty.
	walls, doors, ok := structuralRowsFromPayload(payload)
	if !ok {
		return nil
	}
	p.StructuralWalls = walls
	p.StructuralDoors = doors
	for _, prop := range p.Placed {
		if prop.ID == "" {
			return nil
		}
		d := prop.Placement
		p.RoomRevealedBody.Placed = append(p.RoomRevealedBody.Placed, AtlasPlacedProp{
			ID: prop.ID, Cells: prop.Cells, BlocksMovement: prop.BlocksMovement, BlocksLineOfSight: prop.BlocksLineOfSight,
			Placement: FootprintPlacement{Width: d.Footprint.W, Depth: d.Footprint.D, Origin: FootprintPoint{X: d.Origin.X, Y: d.Origin.Y}, Facing: d.Facing, LocalOffset: FootprintPoint{X: d.LocalOffset.X, Y: d.LocalOffset.Y}},
		})
	}
	return p.RoomRevealedBody
}
