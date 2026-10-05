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
// copies the bytes and refuses a row that is not a whole record. It never
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

// structuralRows is the optional structural half of a reveal payload: the two
// keys P2E adds to room_revealed and concealment_revealed, and nothing else.
// An absent key decodes as nil, which is the legacy payload's own reading.
type structuralRows struct {
	Walls []AtlasStructuralWall `json:"structural_walls"`
	Doors []AtlasStructuralDoor `json:"structural_doors"`
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
func structuralRowsFromPayload(payload []byte) (walls []AtlasStructuralWall, doors []AtlasStructuralDoor, ok bool) {
	var rows structuralRows
	if err := json.Unmarshal(payload, &rows); err != nil {
		return nil, nil, false
	}
	if !validStructuralRows(rows.Walls, rows.Doors) {
		return nil, nil, false
	}

	return rows.Walls, rows.Doors, true
}

// validStructuralRows reports whether every declared row names itself. A wall
// names its wall id and every one of its cuts; a door names its canonical door
// id. A row that names nothing is not a change a client can apply, so its
// presence refuses the body rather than being silently dropped.
func validStructuralRows(walls []AtlasStructuralWall, doors []AtlasStructuralDoor) bool {
	for _, w := range walls {
		if w.ID == "" {
			return false
		}
		for _, o := range w.Openings {
			if o.ID == "" {
				return false
			}
		}
	}
	for _, d := range doors {
		if d.ID == "" {
			return false
		}
	}

	return true
}
