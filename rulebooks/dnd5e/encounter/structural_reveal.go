// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// structural_reveal.go is THE STRUCTURAL HALF OF A REVEAL BEAT (rpg-project#169,
// P2E): the optional `structural_walls` and `structural_doors` a room_revealed
// or concealment_revealed payload gains when a recipient's fixed structural
// layout actually changed.
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
// A row is the COMPLETE projected record for its identity, so applying it by id
// onto the recipient's cached atlas yields the fresh answer. A wall whose cut
// list changed is emitted again under the SAME id; a door already present and
// unchanged is not emitted at all, so a parent becoming known does not duplicate
// an independent door the recipient already had.
//
// NOTHING RIDES HERE THAT IS NOT FIXED LAYOUT: no DoorState, no lock, no placed
// id, no parent association. Mutable state has its own existing event fields and
// is not authority to add it to a fixed row. Both keys are omitted entirely when
// there is nothing new or changed, so a legacy beat's bytes are untouched.

// structuralRevealPayload computes the new-or-changed wall and door rows for one
// reveal, in the recipient-scoped before/after atlases the beat was built from.
//
// Either returned slice is empty when nothing of that kind changed. Callers
// attach a key only for a non-empty slice, which is what keeps an unchanged or
// legacy beat byte-identical.
func structuralRevealPayload(before, after Atlas) (walls, doors []map[string]interface{}) {
	return changedStructuralWalls(before.StructuralWalls, after.StructuralWalls),
		changedStructuralDoors(before.StructuralDoors, after.StructuralDoors)
}

// addStructuralReveal attaches the optional structural keys to a reveal payload
// when — and only when — the recipient's structural layout changed. An empty
// wall or door delta adds no key at all.
func addStructuralReveal(payload map[string]interface{}, before, after Atlas) {
	walls, doors := structuralRevealPayload(before, after)
	if len(walls) > 0 {
		payload["structural_walls"] = walls
	}
	if len(doors) > 0 {
		payload["structural_doors"] = doors
	}
}

// changedStructuralWalls is every wall in after that is NEW or DIFFERENT by id
// from before, each as its complete projected row. A wall the recipient already
// had, unchanged, is not news; a wall whose opening list grew a newly permitted
// cut is emitted whole under the same id.
func changedStructuralWalls(before, after []AtlasStructuralWall) []map[string]interface{} {
	had := make(map[PropID]AtlasStructuralWall, len(before))
	for _, w := range before {
		had[w.ID] = w
	}
	out := make([]map[string]interface{}, 0)
	for _, w := range after {
		if prior, known := had[w.ID]; known && sameStructuralWall(prior, w) {
			continue
		}
		out = append(out, structuralWallRow(w))
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

// sameStructuralWall reports whether two projected walls are the same fixed
// record. Openings are compared by value in authored order, which is part of the
// row a client applies.
func sameStructuralWall(a, b AtlasStructuralWall) bool {
	if a.ID != b.ID || a.Ref != b.Ref || a.From != b.From || a.To != b.To ||
		a.Height != b.Height || a.Thickness != b.Thickness || a.Elevation != b.Elevation {
		return false
	}
	if len(a.Openings) != len(b.Openings) {
		return false
	}
	for i := range a.Openings {
		if a.Openings[i] != b.Openings[i] {
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
	openings := make([]map[string]interface{}, 0, len(w.Openings))
	for _, o := range w.Openings {
		openings = append(openings, map[string]interface{}{
			"id":       o.ID,
			"position": o.Position,
			"width":    o.Width,
		})
	}

	return map[string]interface{}{
		"id":        w.ID,
		"ref":       w.Ref,
		"from":      w.From,
		"to":        w.To,
		"height":    w.Height,
		"thickness": w.Thickness,
		"elevation": w.Elevation,
		"openings":  openings,
	}
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
