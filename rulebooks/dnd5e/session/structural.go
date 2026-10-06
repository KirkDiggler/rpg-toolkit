// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// structural.go is the shared half of the structural DTO (rpg-project#169):
// the projection the snapshot path uses, and the payload decode both reveal
// beats use. Keeping it in one file means the two routes cannot drift.
//
// WHAT IT DOES NOT DO is the whole point. The composition has already decided
// which walls and doors a recipient may know and which cuts survive; this seam
// copies complete records and typed replacements, refusing malformed identity
// sets rather than yielding a partial event. It never
// re-reads the world, never infers a door's state, never reconstructs a parent
// link, and never fills a missing identity in.

// projectStructuralWall copies one composition wall into this package's own
// type (S2). Every field is carried; the two endpoints are rewritten into the
// local [FootprintPoint], and the opening slice is copied so a caller mutating
// the projection cannot reach the composition's snapshot.
func projectStructuralWall(w encounter.AtlasStructuralWall) AtlasStructuralWall {
	openings := make([]AtlasStructuralOpening, 0, len(w.Openings))
	for _, o := range w.Openings {
		openings = append(openings, AtlasStructuralOpening{ID: o.ID, Position: o.Position, Width: o.Width})
	}

	return AtlasStructuralWall{
		ID:        string(w.ID),
		Ref:       w.Ref,
		From:      FootprintPoint{X: w.From.X, Y: w.From.Y},
		To:        FootprintPoint{X: w.To.X, Y: w.To.Y},
		Height:    w.Height,
		Thickness: w.Thickness,
		Elevation: w.Elevation,
		Openings:  openings,
	}
}

// projectStructuralDoor copies one composition door into this package's own
// type (S2). No parent id and no state exist on the inner record, so none can
// be invented here.
func projectStructuralDoor(d encounter.AtlasStructuralDoor) AtlasStructuralDoor {
	return AtlasStructuralDoor{
		ID:        string(d.ID),
		Ref:       d.Ref,
		From:      FootprintPoint{X: d.From.X, Y: d.From.Y},
		To:        FootprintPoint{X: d.To.X, Y: d.To.Y},
		Height:    d.Height,
		Thickness: d.Thickness,
		Elevation: d.Elevation,
	}
}

// structuralRows is the shared structural payload on both reveal kinds.
// Absent collections are legacy no-ops; a present replacement remains present
// even when its opening list has the empty/default value.
type structuralRows struct {
	Walls        []AtlasStructuralWall               `json:"structural_walls"`
	Doors        []AtlasStructuralDoor               `json:"structural_doors"`
	Replacements []StructuralWallOpeningsReplacement `json:"structural_wall_openings_replacements"`
}

// structuralRowsFromPayload decodes the optional structural keys of a reveal
// payload and refuses rather than returning a PARTIAL record.
//
// A malformed value — the wrong JSON type, or a row missing its identity — is
// not a row a client could apply, and applying the sibling rows anyway would
// leave a cache half-patched. So the whole decode is refused (ok false) and
// the caller produces no body, exactly as the existing required-identity
// guards do. An absent key is not malformed: legacy payloads decode with nil
// slices and a true verdict.
func structuralRowsFromPayload(payload []byte) (structuralRows, bool) {
	var rows structuralRows
	if err := json.Unmarshal(payload, &rows); err != nil || !validStructuralRows(rows) {
		return structuralRows{}, false
	}
	return rows, true
}

// validStructuralRows checks identities without consulting the live world.
// Duplicate replacement IDs, repeated opening IDs and full-row/replacement
// collisions refuse the entire body. A default empty replacement remains valid;
// deciding whether its baseline wall is present belongs to the receiving cache.
func validStructuralRows(rows structuralRows) bool {
	walls := make(map[string]bool)
	openings := make(map[string]bool)
	validOpenings := func(list []AtlasStructuralOpening) bool {
		for _, o := range list {
			if o.ID == "" || openings[o.ID] {
				return false
			}
			openings[o.ID] = true
		}
		return true
	}
	for _, w := range rows.Walls {
		if w.ID == "" || walls[w.ID] || !validOpenings(w.Openings) {
			return false
		}
		walls[w.ID] = true
	}
	for _, replacement := range rows.Replacements {
		if replacement.WallID == "" || walls[replacement.WallID] || !validOpenings(replacement.Openings) {
			return false
		}
		walls[replacement.WallID] = true
	}
	for _, d := range rows.Doors {
		if d.ID == "" {
			return false
		}
	}
	return true
}
