// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EventRoomRevealed was the whole-room reveal. Discovery is by cell now
// (rpg-project#586), so nothing produces it; the kind and its body remain so a
// stored story naming it still decodes as what it was.
//
// Deprecated: superseded by [EventCellsRevealed].
const EventRoomRevealed EventKind = "room_revealed"

// RoomRevealedBody is the deprecated [EventRoomRevealed]'s body: fixed layout, not mutable observation or a whole-atlas
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
		Placed json.RawMessage `json:"placed"`
	}
	if json.Unmarshal(payload, &p) != nil || p.Region.ID == "" {
		return nil
	}
	// A row missing its identity refuses the whole beat rather than handing a
	// client a half-patched cache; an absent key is the legacy payload and
	// leaves its list empty.
	fixed, ok := revealedFixedRows(payload)
	if !ok {
		return nil
	}
	body := p.RoomRevealedBody
	body.Placed = fixed.placed
	body.PropPresentations = fixed.presentations
	body.StructuralWalls = fixed.rows.Walls
	body.StructuralDoors = fixed.rows.Doors
	body.StructuralWallOpeningsReplacements = fixed.rows.Replacements
	return body
}
