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
	// PropPresentations introduce permitted fixed renderer input by identity.
	PropPresentations []PropPresentation `json:"prop_presentations,omitempty"`

	// StructuralWalls and StructuralDoors introduce complete permitted records.
	// Historical full changed-wall records retain whole-record upsert semantics.
	StructuralWalls []AtlasStructuralWall `json:"structural_walls,omitempty"`
	StructuralDoors []AtlasStructuralDoor `json:"structural_doors,omitempty"`
	// StructuralWallOpeningsReplacements replaces cuts on known walls in the
	// same atomic structural update. An empty/default list is a clear.
	StructuralWallOpeningsReplacements []StructuralWallOpeningsReplacement `json:"structural_wall_openings_replacements,omitempty"`
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
	rows, ok := structuralRowsFromPayload(payload)
	if !ok {
		return nil
	}
	presentations, valid := propPresentationsFromPayload(payload)
	if !valid {
		return nil
	}
	p.PropPresentations = presentations
	p.StructuralWalls = rows.Walls
	p.StructuralDoors = rows.Doors
	p.StructuralWallOpeningsReplacements = rows.Replacements
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
