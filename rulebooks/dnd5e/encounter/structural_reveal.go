// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// structural_reveal.go is THE STRUCTURAL HALF OF A REVEAL BEAT (rpg-project#169,
// P2E): full introductions and typed opening-list replacements carried by
// room_revealed or concealment_revealed when a recipient's layout changes.
//
// # What it is, and what it deliberately is not
//
// It is a DIFFERENCE BY IDENTITY between the recipient's own prior projection
// and their scoped projection now — never a reuse of [Encounter.Atlas], which is
// the whole field's truth, and never a lookup of current or future world state
// while an old event is being replayed. Both sides come from [Encounter.AtlasFor]
// with that recipient's knowledge as it stood at the two moments, exactly as the
// segment delta beside it does.
//
// Newly permitted identities arrive as complete records. An already-known wall
// whose cut list changed receives only a structural_wall_openings_replacements
// record. Applying those introductions and replacements atomically yields the
// fresh projected answer without resending the wall's unchanged layout fields.
// An unchanged independently known door is not repeated when its parent arrives.
//
// NOTHING RIDES HERE THAT IS NOT FIXED LAYOUT: no DoorState, no lock, no placed
// id, no parent association. Mutable state has its own existing event fields and
// is not authority to add it to a fixed row. Payload keys are omitted entirely when
// there is nothing new or changed, so a legacy beat's bytes are untouched.

// structuralRevealPayload computes introductions and known-wall opening
// replacements from the recipient's before/after projections.
//
// Each returned slice is empty when nothing of that kind changed. Callers
// attach a key only for a non-empty slice, which is what keeps an unchanged or
// legacy beat byte-identical.
func structuralRevealPayload(before, after Atlas) (walls, doors, replacements []map[string]interface{}) {
	return changedStructuralWalls(before.StructuralWalls, after.StructuralWalls),
		changedStructuralDoors(before.StructuralDoors, after.StructuralDoors),
		changedStructuralWallOpenings(before.StructuralWalls, after.StructuralWalls)
}

// addStructuralReveal attaches the optional structural keys to a reveal payload
// when — and only when — the recipient's structural layout changed. An empty
// wall or door delta adds no key at all.
func addStructuralReveal(payload map[string]interface{}, before, after Atlas) {
	walls, doors, replacements := structuralRevealPayload(before, after)
	if len(walls) > 0 {
		payload["structural_walls"] = walls
	}
	if len(doors) > 0 {
		payload["structural_doors"] = doors
	}
	if len(replacements) > 0 {
		payload["structural_wall_openings_replacements"] = replacements
	}
}

// changedStructuralWalls introduces newly permitted walls as complete records.
// Layout definitions are fixed during a run; a known wall's only projected
// component change is its permitted opening list, emitted separately below.
func changedStructuralWalls(before, after []AtlasStructuralWall) []map[string]interface{} {
	had := make(map[PropID]bool, len(before))
	for _, w := range before {
		had[w.ID] = true
	}
	out := make([]map[string]interface{}, 0)
	for _, w := range after {
		if !had[w.ID] {
			out = append(out, structuralWallRow(w))
		}
	}
	return out
}

// changedStructuralWallOpenings replaces a known wall's permitted cuts without
// revealing or repeating its fixed fields. A present row with an empty list is
// a clear, not a no-op; absent rows leave the component unchanged.
func changedStructuralWallOpenings(before, after []AtlasStructuralWall) []map[string]interface{} {
	had := make(map[PropID]AtlasStructuralWall, len(before))
	for _, w := range before {
		had[w.ID] = w
	}
	out := make([]map[string]interface{}, 0)
	for _, w := range after {
		prior, known := had[w.ID]
		if !known || sameStructuralOpenings(prior.Openings, w.Openings) {
			continue
		}
		out = append(out, map[string]interface{}{
			"wall_id":  w.ID,
			"openings": structuralOpeningRows(w.Openings),
		})
	}
	return out
}

// changedStructuralDoors is [changedStructuralWalls] on the independent door
// collection: every door in after that is new or different by canonical id.
func changedStructuralDoors(before, after []AtlasStructuralDoor) []map[string]interface{} {
	had := make(map[DoorID]AtlasStructuralDoor, len(before))
	for _, d := range before {
		had[d.ID] = d
	}
	out := make([]map[string]interface{}, 0)
	for _, d := range after {
		if prior, known := had[d.ID]; known && prior == d {
			continue
		}
		out = append(out, structuralDoorRow(d))
	}

	return out
}

// sameStructuralOpenings compares complete permitted lists in snapshot order.
func sameStructuralOpenings(a, b []AtlasStructuralOpening) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// structuralWallRow renders one projected wall as the wire row: its identity,
// its opaque ref, its canonical-feet line and assembled dimensions, and its
// permitted cuts — each opening id, position and width ONLY. No door id, no
// state, no placed/parent association is carried through the opening.
func structuralWallRow(w AtlasStructuralWall) map[string]interface{} {
	return map[string]interface{}{
		"id":        w.ID,
		"ref":       w.Ref,
		"from":      w.From,
		"to":        w.To,
		"height":    w.Height,
		"thickness": w.Thickness,
		"elevation": w.Elevation,
		"openings":  structuralOpeningRows(w.Openings),
	}
}

// structuralOpeningRows encodes only permitted cut facts. Even an empty list
// is allocated so a clearing replacement has an explicit array in stored JSON.
func structuralOpeningRows(openings []AtlasStructuralOpening) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(openings))
	for _, o := range openings {
		out = append(out, map[string]interface{}{
			"id": o.ID, "position": o.Position, "width": o.Width,
		})
	}
	return out
}

// structuralDoorRow renders one independent door as the wire row: its actual
// canonical gameplay id, its opaque ref, its resolved opening endpoints and the
// assembled dimensions it fits. Deliberately no state, lock or parent id.
func structuralDoorRow(d AtlasStructuralDoor) map[string]interface{} {
	return map[string]interface{}{
		"id":        d.ID,
		"ref":       d.Ref,
		"from":      d.From,
		"to":        d.To,
		"height":    d.Height,
		"thickness": d.Thickness,
		"elevation": d.Elevation,
	}
}
