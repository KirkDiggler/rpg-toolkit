// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BeatCellsRevealed adds one recipient's newly learned fixed layout, by cell
// (rpg-project#586 R1). It carries no creature, movable-prop placement, or
// unobserved door state. One per member per learning moment — a sight pass, a
// door opening — and never before scene-opened, whose snapshot already holds
// the initial map.
const BeatCellsRevealed = "cells_revealed"

// BeatRoomRevealed was the whole-room reveal. Discovery is by cell now, so
// nothing produces it; the constant remains so a stored story naming it still
// reads as what it was.
//
// Deprecated: superseded by [BeatCellsRevealed] (rpg-project#586).
const BeatRoomRevealed = "room_revealed"

// appendCellsRevealedBeat derives delivery from the same projection as the
// read: everything is a difference of [Encounter.AtlasFor] before and after
// the learning fact landed. Already known shared walls stay in the
// recipient's atlas; the event adds what is new, never a replacement of the
// whole atlas.
//
// `regions` IS PARTIAL. Each entry is a region the recipient gained cells of,
// carrying ONLY those newly known cells — a client merges cells by region id,
// by union. A region they first see one hex of arrives as a one-hex entry.
//
// THE FIXED STRUCTURAL LAYOUT RIDES ALONG WHEN IT CHANGED (rpg-project#169,
// structural_reveal.go): `structural_walls` and `structural_doors` carry the
// new-or-changed projected rows by id, so a client draws the authored walls
// and independently permitted doors without a second read. Both keys are
// absent when nothing changed, and the rows are a difference against the same
// recipient's own prior projection — never the whole field's truth.
func (e *Encounter) appendCellsRevealedBeat(member MemberID, before Atlas) error {
	after, err := e.AtlasFor(member)
	if err != nil {
		return err
	}
	had := make(map[spatial.Position]bool, len(before.Cells))
	for _, cell := range before.Cells {
		had[cell] = true
	}
	hadRegion := make(map[RegionID]map[spatial.Position]bool, len(before.Regions))
	for _, region := range before.Regions {
		cells := make(map[spatial.Position]bool, len(region.Cells))
		for _, cell := range region.Cells {
			cells[cell] = true
		}
		hadRegion[region.ID] = cells
	}
	// cells is every cell new to this recipient's atlas: region floor,
	// scenery and footing alike — what every other record is judged against.
	cells := make(map[spatial.Position]bool)
	regions := make([]AtlasRegion, 0)
	for _, region := range after.Regions {
		fresh := make([]spatial.Position, 0)
		for _, cell := range region.Cells {
			if !hadRegion[region.ID][cell] {
				fresh = append(fresh, cell)
				cells[cell] = true
			}
		}
		if len(fresh) == 0 {
			continue
		}
		entry := region
		entry.Cells = fresh
		regions = append(regions, entry)
	}
	scenery := make([]spatial.Position, 0)
	for _, cell := range after.Cells {
		if !had[cell] && !cells[cell] {
			scenery = append(scenery, cell)
			cells[cell] = true
		}
	}
	// EVERY LIST IS A DIFFERENCE against the recipient's own prior map, sent
	// once and never again (rpg-api-protos 69a68f3e): a record the recipient
	// already holds is not repeated because a cell beside it became known.
	// AtlasBoundary has no id to upsert by, so a repeat would be a duplicate.
	// A FIXED PROP'S ID IS OPTIONAL ([AtlasProp.ID]), so it cannot key the
	// difference: every id-less prop would share "" and the second one would
	// never be sent. The whole record is the key, counted, so even two
	// identical id-less props on one cell arrive as two. (Placements, exits
	// and doors carry required ids; doorways and sealed cells key by value.)
	hadProp := make(map[AtlasProp]int, len(before.Props))
	for _, prop := range before.Props {
		hadProp[prop]++
	}
	props := make([]AtlasProp, 0)
	for _, prop := range after.Props {
		if hadProp[prop] > 0 {
			hadProp[prop]--
			continue
		}
		props = append(props, prop)
	}
	hadBoundary := make(map[AtlasBoundary]bool, len(before.Boundaries))
	for _, boundary := range before.Boundaries {
		hadBoundary[boundary] = true
	}
	boundaries := make([]AtlasBoundary, 0)
	for _, boundary := range after.Boundaries {
		if !hadBoundary[boundary] {
			boundaries = append(boundaries, boundary)
		}
	}
	hadDoorway := make(map[AtlasDoorway]bool, len(before.Doorways))
	for _, doorway := range before.Doorways {
		hadDoorway[doorway] = true
	}
	doorways := make([]map[string]interface{}, 0)
	for _, doorway := range after.Doorways {
		if !hadDoorway[doorway] {
			doorways = append(doorways, map[string]interface{}{"door": doorway.Door, "from": doorway.From, "to": doorway.To})
		}
	}
	hadPlaced := make(map[PropID]bool, len(before.Placed))
	for _, prop := range before.Placed {
		hadPlaced[prop.ID] = true
	}
	placed := make([]map[string]interface{}, 0)
	for _, prop := range after.Placed {
		// A newly permitted boundary can have its standing support on the
		// still-unknown side. Introduce the same row the snapshot permits,
		// once; do not substitute a region-cell test for that answer.
		if !hadPlaced[prop.ID] {
			placed = append(placed, map[string]interface{}{
				"id": prop.ID, "placement": placementDataFrom(prop.Placement), "cells": prop.Cells,
				"blocks_movement": prop.BlocksMovement, "blocks_line_of_sight": prop.BlocksLineOfSight, "holdable": false,
			})
		}
	}
	hadSealed := make(map[spatial.Position]bool, len(before.Sealed))
	for _, cell := range before.Sealed {
		hadSealed[cell] = true
	}
	sealed := make([]spatial.Position, 0)
	for _, cell := range after.Sealed {
		if !hadSealed[cell] {
			sealed = append(sealed, cell)
		}
	}
	hadExit := make(map[string]bool, len(before.Exits))
	for _, exit := range before.Exits {
		hadExit[string(exit.ID)] = true
	}
	exits := make([]map[string]interface{}, 0)
	for _, exit := range after.Exits {
		if !hadExit[string(exit.ID)] {
			exits = append(exits, map[string]interface{}{"id": exit.ID, "at": exit.At})
		}
	}
	segments := newSegments(before.Segments, after.Segments)
	payload := map[string]interface{}{
		"beat": BeatCellsRevealed, "regions": revealRegionsPayload(regions),
		"scenery": scenery, "props": revealPropsPayload(props), "boundaries": revealBoundariesPayload(boundaries),
		"segments": revealSegmentsPayload(segments), "sealed": sealed,
		"doorways": doorways, "placed": placed, "exits": exits,
	}
	base := len(payload)
	// THE FIXED STRUCTURAL LAYOUT the cells brought, when it changed: new or
	// updated wall rows by id, and independent door rows that were not
	// already the recipient's. Omitted when nothing changed.
	addStructuralReveal(payload, before, after)
	addPropPresentationReveal(payload, before, after)
	// A cell can be learned that the recipient's map already drew — scenery
	// already presented beside known floor. Learning it
	// changes no record, and an empty patch is not a beat.
	if len(payload) == base && len(regions) == 0 && len(scenery) == 0 && len(props) == 0 &&
		len(boundaries) == 0 && len(segments) == 0 && len(sealed) == 0 && len(doorways) == 0 &&
		len(placed) == 0 && len(exits) == 0 {
		return nil
	}
	_, err = e.appendRevealBeat(member, payload, uint64(e.clock.ToData().HighWater))
	return err
}
