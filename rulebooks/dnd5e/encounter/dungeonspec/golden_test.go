// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// golden_test.go is THE FORCING CASE for dungeonspec version 2 (rpg-project#256,
// design §5): the reference tomb, re-authored by hand in the new dialect, must
// compile to the SAME atlas the version-1 room chain produced — same cells,
// same walls, same doorways, same props. testdata/reference-tomb.atlas.json is
// that atlas, captured from the v1 compile BEFORE v1 was deleted (the first
// commit of the PR), in the flat, sorted shape the session seam projects.
//
// Regenerate with UPDATE_GOLDEN=1 only when the dungeon itself is meant to
// change — never to make a failing compile pass.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const goldenPath = "testdata/reference-tomb.atlas.json"

// goldenAtlas is the atlas flattened the way rulebooks/dnd5e/session projects
// it: every list sorted by coordinate so nothing about how the field was
// authored leaks through the order.
type goldenAtlas struct {
	Layout     string           `json:"layout"`
	Cells      []goldenCell     `json:"cells"`
	Props      []goldenProp     `json:"props"`
	Boundaries []goldenBoundary `json:"boundaries"`
	Doorways   []goldenDoorway  `json:"doorways"`
}

type goldenCell struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type goldenProp struct {
	Ref               string     `json:"ref"`
	At                goldenCell `json:"at"`
	BlocksMovement    bool       `json:"blocks_movement"`
	BlocksLineOfSight bool       `json:"blocks_line_of_sight"`
}

type goldenBoundary struct {
	From              goldenCell `json:"from"`
	To                goldenCell `json:"to"`
	BlocksMovement    bool       `json:"blocks_movement"`
	BlocksLineOfSight bool       `json:"blocks_line_of_sight"`
}

type goldenDoorway struct {
	From goldenCell `json:"from"`
	To   goldenCell `json:"to"`
}

func cellOf(p spatial.Position) goldenCell { return goldenCell{X: p.X, Y: p.Y} }

func cellBefore(a, b goldenCell) bool {
	if a.X != b.X {
		return a.X < b.X
	}
	return a.Y < b.Y
}

func normalizeGolden(g *goldenAtlas) {
	sort.Slice(g.Cells, func(i, j int) bool { return cellBefore(g.Cells[i], g.Cells[j]) })
	sort.Slice(g.Props, func(i, j int) bool {
		if g.Props[i].At != g.Props[j].At {
			return cellBefore(g.Props[i].At, g.Props[j].At)
		}
		return g.Props[i].Ref < g.Props[j].Ref
	})
	for i, b := range g.Boundaries {
		if cellBefore(b.To, b.From) {
			g.Boundaries[i].From, g.Boundaries[i].To = b.To, b.From
		}
	}
	sort.Slice(g.Boundaries, func(i, j int) bool {
		if g.Boundaries[i].From != g.Boundaries[j].From {
			return cellBefore(g.Boundaries[i].From, g.Boundaries[j].From)
		}
		return cellBefore(g.Boundaries[i].To, g.Boundaries[j].To)
	})
	for i, d := range g.Doorways {
		if cellBefore(d.To, d.From) {
			g.Doorways[i].From, g.Doorways[i].To = d.To, d.From
		}
	}
	sort.Slice(g.Doorways, func(i, j int) bool {
		if g.Doorways[i].From != g.Doorways[j].From {
			return cellBefore(g.Doorways[i].From, g.Doorways[j].From)
		}
		return cellBefore(g.Doorways[i].To, g.Doorways[j].To)
	})
}

// goldenFromAtlas flattens the composition's atlas into the golden shape. On
// the version-1 (room chain) shape a region is a rectangle it describes rather
// than enumerates, so the cells are walked through HexCellAt exactly as the
// session seam walks them.
func goldenFromAtlas(atlas encounter.Atlas) goldenAtlas {
	g := goldenAtlas{Layout: string(atlas.Orientation.Kind())}
	for _, r := range atlas.Regions {
		for row := 0; row < r.Height; row++ {
			for col := 0; col < r.Width; col++ {
				g.Cells = append(g.Cells, cellOf(encounter.HexCellAt(
					atlas.Orientation, col+int(r.Origin.X), row+int(r.Origin.Y))))
			}
		}
		for _, p := range r.Props {
			g.Props = append(g.Props, goldenProp{
				Ref: p.Ref, At: cellOf(p.At),
				BlocksMovement: p.BlocksMovement, BlocksLineOfSight: p.BlocksLineOfSight,
			})
		}
		for _, b := range r.Boundaries {
			g.Boundaries = append(g.Boundaries, goldenBoundary{
				From: cellOf(b.From), To: cellOf(b.To),
				BlocksMovement: b.BlocksMovement, BlocksLineOfSight: b.BlocksLineOfSight,
			})
		}
	}
	for _, d := range atlas.Doorways {
		g.Doorways = append(g.Doorways, goldenDoorway{From: cellOf(d.FromCell), To: cellOf(d.ToCell)})
	}
	normalizeGolden(&g)
	return g
}

func compiledTombAtlas(t *testing.T) encounter.Atlas {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(tombYAML))
	require.NoError(t, err)
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: noAttacksExpected{},
		Field:   compiled.Field,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	atlas, err := enc.Atlas()
	require.NoError(t, err)
	return atlas
}

func TestGolden_ReferenceTombAtlas(t *testing.T) {
	got := goldenFromAtlas(compiledTombAtlas(t))
	raw, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)

	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, append(raw, '\n'), 0o644))
		t.Logf("wrote %s (%d cells, %d boundaries, %d doorways, %d props)",
			goldenPath, len(got.Cells), len(got.Boundaries), len(got.Doorways), len(got.Props))
		return
	}

	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "the golden must exist before the dialect it pins is touched")
	var wantAtlas goldenAtlas
	require.NoError(t, json.Unmarshal(want, &wantAtlas))
	require.Equal(t, wantAtlas, got, "the compiled tomb moved under the fixtures that use it")
	require.Len(t, got.Cells, 224, "the reference tomb is 224 cells (design §5)")
}
