// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// BeatRoomRevealed adds one recipient's newly learned fixed room layout.
// It carries no creature, movable-prop placement, or unobserved door state.
const BeatRoomRevealed = "room_revealed"

// appendRoomRevealedBeat derives delivery from the same projection as the read.
// Already known shared walls stay in the recipient's atlas; the event adds the
// room and the newly presented walls, not a replacement of the whole atlas.
//
// THE FIXED STRUCTURAL LAYOUT RIDES ALONG WHEN IT CHANGED (rpg-project#169,
// structural_reveal.go): `structural_walls` and `structural_doors` carry the
// new-or-changed projected rows by id, so a client draws the room's authored
// walls and independently permitted doors without a second read. Both keys are
// absent when nothing changed, and the rows are a difference against the same
// recipient's own prior projection — never the whole field's truth.
func (e *Encounter) appendRoomRevealedBeat(member MemberID, id RegionID, before Atlas) error {
	after, err := e.AtlasFor(member)
	if err != nil {
		return err
	}
	for _, region := range after.Regions {
		if region.ID != id {
			continue
		}
		cells := make(map[spatial.Position]bool, len(region.Cells))
		for _, cell := range region.Cells {
			cells[cell] = true
		}
		had := make(map[spatial.Position]bool, len(before.Cells))
		for _, cell := range before.Cells {
			had[cell] = true
		}
		scenery := make([]spatial.Position, 0)
		for _, cell := range after.Cells {
			if !had[cell] && !cells[cell] {
				scenery = append(scenery, cell)
				cells[cell] = true
			}
		}
		props := make([]AtlasProp, 0)
		for _, prop := range after.Props {
			if cells[prop.At] {
				props = append(props, prop)
			}
		}
		boundaries := make([]AtlasBoundary, 0)
		for _, boundary := range after.Boundaries {
			if cells[boundary.From] || cells[boundary.To] {
				boundaries = append(boundaries, boundary)
			}
		}
		doorways := make([]map[string]interface{}, 0)
		for _, doorway := range after.Doorways {
			if cells[doorway.From] || cells[doorway.To] {
				doorways = append(doorways, map[string]interface{}{"door": doorway.Door, "from": doorway.From, "to": doorway.To})
			}
		}
		placed := make([]map[string]interface{}, 0)
		for _, prop := range after.Placed {
			for _, cell := range prop.Cells {
				if !cells[cell] {
					continue
				}
				placed = append(placed, map[string]interface{}{
					"id": prop.ID, "placement": placementDataFrom(prop.Placement), "cells": prop.Cells,
					"blocks_movement": prop.BlocksMovement, "blocks_line_of_sight": prop.BlocksLineOfSight, "holdable": false,
				})
				break
			}
		}
		sealed := make([]spatial.Position, 0)
		for _, cell := range after.Sealed {
			if cells[cell] {
				sealed = append(sealed, cell)
			}
		}
		exits := make([]map[string]interface{}, 0)
		for _, exit := range after.Exits {
			if cells[exit.At] {
				exits = append(exits, map[string]interface{}{"id": exit.ID, "at": exit.At})
			}
		}
		payload := map[string]interface{}{
			"beat": BeatRoomRevealed, "region": revealRegionsPayload([]AtlasRegion{region})[0],
			"scenery": scenery, "props": revealPropsPayload(props), "boundaries": revealBoundariesPayload(boundaries),
			"segments": revealSegmentsPayload(newSegments(before.Segments, after.Segments)), "sealed": sealed,
			"doorways": doorways, "placed": placed, "exits": exits,
		}
		// THE FIXED STRUCTURAL LAYOUT the room brought, when it changed:
		// new or updated wall rows by id, and independent door rows that were
		// not already the recipient's. Omitted when nothing changed.
		addStructuralReveal(payload, before, after)
		_, err := e.appendRevealBeat(member, payload, uint64(e.clock.ToData().HighWater))
		return err
	}
	return fmt.Errorf("revealed room %q is absent from its recipient's atlas: %w", id, ErrInvalidData)
}
