// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// EventCellsRevealed adds the fixed layout its recipient newly learned, by
// cell (rpg-project#586 R1). It is the encounter's [encounter.BeatCellsRevealed]
// delivered unchanged: one per member per learning moment, on that member's
// own stream only. It replaces [EventRoomRevealed], which nothing produces.
const EventCellsRevealed EventKind = "cells_revealed"

// CellsRevealedBody is fixed layout newly known to one recipient, never a
// whole-atlas replacement and never mutable observation. Every record is the
// encounter's difference of the recipient's atlas before and after the cells
// were learned; this package decodes it and computes nothing.
//
// Regions IS PARTIAL. Each entry carries only the newly known cells of that
// region. A client merges cells into its cached region by id, by union — a
// region first seen one hex at a time arrives as one-hex entries.
//
// Every other field reads as [RoomRevealedBody]'s does: Scenery is additional
// unowned floor (wall footing), and the structural rows and prop presentations
// introduce complete permitted records by identity.
type CellsRevealedBody struct {
	Regions    []AtlasRegion      `json:"regions"`
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
	StructuralWalls []AtlasStructuralWall `json:"structural_walls,omitempty"`
	StructuralDoors []AtlasStructuralDoor `json:"structural_doors,omitempty"`
	// StructuralWallOpeningsReplacements replaces cuts on known walls in the
	// same atomic structural update. An empty/default list is a clear.
	StructuralWallOpeningsReplacements []StructuralWallOpeningsReplacement `json:"structural_wall_openings_replacements,omitempty"`
}

func (CellsRevealedBody) isEventBody() {}

// cellsRevealedBody decodes the beat, or answers nil. A region, placed prop
// or structural row missing its identity refuses the whole body rather than
// handing a client a half-patched cache.
func cellsRevealedBody(payload []byte) EventBody {
	var p struct {
		CellsRevealedBody
		Placed json.RawMessage `json:"placed"`
	}
	if json.Unmarshal(payload, &p) != nil {
		return nil
	}
	for _, region := range p.Regions {
		if region.ID == "" {
			return nil
		}
	}
	fixed, ok := revealedFixedRows(payload)
	if !ok {
		return nil
	}
	body := p.CellsRevealedBody
	body.Placed = fixed.placed
	body.PropPresentations = fixed.presentations
	body.StructuralWalls = fixed.rows.Walls
	body.StructuralDoors = fixed.rows.Doors
	body.StructuralWallOpeningsReplacements = fixed.rows.Replacements
	return body
}

// revealedFixed is what a room or cells reveal decodes beyond its plain
// fields: placed props from the composition's placement shape, and the shared
// structural rows and prop presentations.
type revealedFixed struct {
	placed        []AtlasPlacedProp
	presentations []PropPresentation
	rows          structuralRows
}

// revealedFixedRows decodes the shared rows of a reveal payload. False when
// any row is malformed or missing its identity; an absent key leaves its list
// empty.
func revealedFixedRows(payload []byte) (revealedFixed, bool) {
	var p struct {
		Placed []struct {
			ID                string                  `json:"id"`
			Placement         encounter.PlacementData `json:"placement"`
			Cells             []spatial.Position      `json:"cells"`
			BlocksMovement    bool                    `json:"blocks_movement"`
			BlocksLineOfSight bool                    `json:"blocks_line_of_sight"`
		} `json:"placed"`
	}
	if json.Unmarshal(payload, &p) != nil {
		return revealedFixed{}, false
	}
	// THE STRUCTURAL ROWS ARE DECODED ONCE, SHARED WITH THE CONCEALMENT BEAT.
	rows, ok := structuralRowsFromPayload(payload)
	if !ok {
		return revealedFixed{}, false
	}
	presentations, ok := propPresentationsFromPayload(payload)
	if !ok {
		return revealedFixed{}, false
	}
	out := revealedFixed{presentations: presentations, rows: rows}
	for _, prop := range p.Placed {
		if prop.ID == "" {
			return revealedFixed{}, false
		}
		d := prop.Placement
		out.placed = append(out.placed, AtlasPlacedProp{
			ID: prop.ID, Cells: prop.Cells, BlocksMovement: prop.BlocksMovement, BlocksLineOfSight: prop.BlocksLineOfSight,
			Placement: FootprintPlacement{
				Width: d.Footprint.W, Depth: d.Footprint.D, Origin: FootprintPoint{X: d.Origin.X, Y: d.Origin.Y},
				Facing: d.Facing, LocalOffset: FootprintPoint{X: d.LocalOffset.X, Y: d.LocalOffset.Y},
			},
		})
	}
	return out, true
}
