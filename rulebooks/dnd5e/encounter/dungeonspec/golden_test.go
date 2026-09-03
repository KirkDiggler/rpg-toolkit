// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// golden_test.go is THE FORCING CASE, twice over.
//
// It began as version 2's (rpg-project#256, design §5): the reference tomb,
// re-authored by hand as painted regions, must compile to the SAME atlas the
// version-1 room chain produced. testdata/reference-tomb.atlas.json is that
// atlas, captured from the v1 compile BEFORE v1 was deleted, in the flat,
// sorted shape the session seam projects. It is never regenerated — it is the
// last thing the chain said, and the whole point is that later dialects agree
// with it.
//
// It is now ACCEPTANCE A6 as well (rpg-project#360, wall-geometry design §7).
// The tomb has been re-authored a second time, its two seams as LINES rather
// than as 28 loose crossings, and the same golden holds it: the same 224
// cells, the same 15 props, and THE SAME SEAM — every crossing the chain
// walled or doored is walled or doored still.
//
// One thing moved, and A6 is where it is recorded. Each seam is a quarter
// line, which passes through the slanted side midpoints of the cells it cuts
// and not through the flat-side midpoint the old doors stood at; no thin line
// does (design F16). So each door moved one row, onto a crossing between the
// same two rooms. The union of walls and doorways is unchanged; which one of
// the two crossings by the door is the opening is not, and this file says so
// in as many words rather than quietly relaxing the comparison. Kirk walks the
// result: "we built the thing, verify it works, then rewrite that and commit
// it."

import (
	"encoding/json"
	"os"
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
// authored leaks through the order. Doorways carry no ID: version 1 named
// them `<key>:<west>-<east>` and version 2 names them `<key>/<door id>`, and
// the golden is about WHERE the openings are.
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

// goldenFromAtlas flattens the composition's atlas into the golden shape. The
// atlas is flat and sorted already since rpg-project#256; normalizing again
// is what keeps this comparison independent of that.
func goldenFromAtlas(atlas encounter.Atlas) goldenAtlas {
	g := goldenAtlas{Layout: string(atlas.Orientation.Kind())}
	for _, c := range atlas.Cells {
		g.Cells = append(g.Cells, cellOf(c))
	}
	for _, p := range atlas.Props {
		g.Props = append(g.Props, goldenProp{
			Ref: p.Ref, At: cellOf(p.At),
			BlocksMovement: p.BlocksMovement, BlocksLineOfSight: p.BlocksLineOfSight,
		})
	}
	for _, b := range atlas.Boundaries {
		g.Boundaries = append(g.Boundaries, goldenBoundary{
			From: cellOf(b.From), To: cellOf(b.To),
			BlocksMovement: b.BlocksMovement, BlocksLineOfSight: b.BlocksLineOfSight,
		})
	}
	for _, d := range atlas.Doorways {
		g.Doorways = append(g.Doorways, goldenDoorway{From: cellOf(d.From), To: cellOf(d.To)})
	}
	normalizeGolden(&g)
	return g
}

// compiledAtlas compiles a file and builds the encounter its field describes,
// with nobody in it, and hands back the map.
func compiledAtlas(t *testing.T, path string) (dungeonspec.Compiled, encounter.Atlas) {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	compiled, err := dungeonspec.Load(raw)
	require.NoError(t, err)
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: noAttacksExpected{}, Announcer: quietAnnouncer{},
		Field:   compiled.Field,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	atlas, err := enc.Atlas()
	require.NoError(t, err)
	return compiled, atlas
}

// tombDoorMoves is A6's one recorded difference: for each door, the crossing
// the pair form opened and the crossing the line form opens instead. Both join
// the same two rooms; the second is the one the quarter line actually passes
// through.
var tombDoorMoves = []struct {
	door       string
	was, isNow [2][2]int // in the author's own [col,row], as the file writes them
}{
	{"entrance-hall", [2][2]int{{5, 4}, {6, 4}}, [2][2]int{{5, 3}, {6, 4}}},
	{"hall-tomb", [2][2]int{{15, 4}, {16, 4}}, [2][2]int{{15, 5}, {16, 4}}},
}

// doorwayAt is one crossing named the way the FILE names it — an offset
// [col,row] pair each end — since an atlas answers in absolute axial and a
// reader checking this test against the tomb has the file open, not the axial
// map.
func doorwayAt(ends [2][2]int) goldenDoorway {
	o := encounter.HexesArePointyTop()
	a := cellOf(encounter.HexCellAt(o, ends[0][0], ends[0][1]))
	b := cellOf(encounter.HexCellAt(o, ends[1][0], ends[1][1]))
	if cellBefore(b, a) {
		a, b = b, a
	}

	return goldenDoorway{From: a, To: b}
}

// TestGolden_ReferenceTombV2MatchesV1Atlas is the forcing case, and A6 with
// it.
func TestGolden_ReferenceTombV2MatchesV1Atlas(t *testing.T) {
	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "the golden was captured from version 1 before version 1 was deleted")
	var wantAtlas goldenAtlas
	require.NoError(t, json.Unmarshal(want, &wantAtlas))
	normalizeGolden(&wantAtlas)

	_, atlas := compiledAtlas(t, tombPath)
	got := goldenFromAtlas(atlas)

	require.Equal(t, wantAtlas.Layout, got.Layout)
	require.Equal(t, wantAtlas.Cells, got.Cells, "the 224 cells the chain drew, painted by hand")
	require.Equal(t, wantAtlas.Props, got.Props, "the 15 props, at absolute cells")
	require.Len(t, got.Cells, 224, "the reference tomb is 224 cells (design §5)")

	// A6: THE SAME SEAM. Everything the chain walled or doored, the two
	// authored lines wall or door — same 30 crossings, derived rather than
	// listed. Compared as the union so the two doors moving inside it is a
	// difference this test states deliberately below, not one it hides.
	require.Equal(t, seamOf(wantAtlas), seamOf(got),
		"every crossing the pair form stood in, the two lines stand in")
	require.Len(t, seamOf(got), 30, "fourteen walls and one door per seam, twice over")

	// And the move itself, named. Each door opens a crossing between the same
	// two rooms, one row from where the pair form put it.
	for _, move := range tombDoorMoves {
		require.NotContains(t, got.Doorways, doorwayAt(move.was),
			"%s no longer opens the flat-side crossing no thin line passes through", move.door)
		require.Contains(t, got.Doorways, doorwayAt(move.isNow),
			"%s opens the slanted crossing its wall does pass through", move.door)
		require.Contains(t, wantAtlas.Doorways, doorwayAt(move.was),
			"and the pair form really did open the other one")
	}
	require.Len(t, got.Doorways, 2, "two doors, one crossing each")

	// The rooms either side of each door are the rooms they always were: the
	// opening moved within the seam, not through it.
	compiled, _ := compiledAtlas(t, tombPath)
	owner := map[goldenCell]string{}
	for _, r := range atlas.Regions {
		for _, c := range r.Cells {
			owner[cellOf(c)] = string(r.ID)
		}
	}
	for _, move := range tombDoorMoves {
		was, isNow := doorwayAt(move.was), doorwayAt(move.isNow)
		require.Equal(t, owner[was.From], owner[isNow.From],
			"%s still leaves the room it left", move.door)
		require.Equal(t, owner[was.To], owner[isNow.To],
			"%s still arrives in the room it arrived in", move.door)
	}

	// NOTHING IS SEALED. Both seams are quarter lines: they shave five
	// twenty-fourths off the cells either side and leave nineteen, which
	// clears MinStandable with room to spare — so the tomb has exactly the
	// floor it had, all of it standable.
	require.Empty(t, atlas.Sealed, "a quarter line seals nothing")
	require.Len(t, compiled.Field.Segments, 2, "two authored lines, two segments")

	// And what version 2 adds, compared beside the golden rather than
	// folded into it: three regions whose cells union to the floor.
	require.Len(t, atlas.Regions, 3)
	var union []goldenCell
	byID := map[string]encounter.AtlasRegion{}
	for _, r := range atlas.Regions {
		byID[r.ID] = r
		for _, c := range r.Cells {
			union = append(union, cellOf(c))
		}
	}
	sort.Slice(union, func(i, j int) bool { return cellBefore(union[i], union[j]) })
	require.Equal(t, got.Cells, union, "the regions' cells ARE the floor")
	require.Equal(t, "crypt", byID["entrance"].Archetype)
	require.Equal(t, 0.6, byID["entrance"].Lighting.Intensity)
	require.Equal(t, 0.4, byID["hall"].Lighting.Intensity)
	require.Equal(t, 0.15, byID["tomb"].Lighting.Intensity)
	require.Equal(t, 48, len(byID["entrance"].Cells))
	require.Equal(t, 80, len(byID["hall"].Cells))
	require.Equal(t, 96, len(byID["tomb"].Cells))

	// rpg-project#261: the reference tomb authors neither field on any prop,
	// so the additive fields must change nothing absent — every prop's
	// Facing and Offset are the zero value, the same fact "said nothing"
	// always means.
	for _, p := range atlas.Props {
		require.Equal(t, "", p.Facing, "the reference tomb authors no facing on %q", p.Ref)
		require.Equal(t, [3]float64{0, 0}, p.Offset, "nor an offset on %q", p.Ref)
	}
}

// seamOf is every crossing a wall stands in OR a door opens, as one set: what
// A6 compares, because "the same seam" is a fact about where the stone runs
// and not about which hex of it is the hole.
func seamOf(g goldenAtlas) map[goldenDoorway]bool {
	out := map[goldenDoorway]bool{}
	for _, b := range g.Boundaries {
		out[goldenDoorway{From: b.From, To: b.To}] = true
	}
	for _, d := range g.Doorways {
		out[d] = true
	}

	return out
}

// TestSecondSkeletonFixtureCompiles is rpg-project#254's fixture, so slice 2
// keeps running: the same tomb with a wall across the hall, and the second
// skeleton behind it. The wall is the only difference, and the golden says
// so: cells, props and doorways identical, 15 more boundaries — one more
// authored line, and the crossings it derives.
func TestSecondSkeletonFixtureCompiles(t *testing.T) {
	compiled, atlas := compiledAtlas(t, secondSkeletonPath)
	got := goldenFromAtlas(atlas)

	_, tomb := compiledAtlas(t, tombPath)
	want := goldenFromAtlas(tomb)

	require.Equal(t, want.Cells, got.Cells)
	require.Equal(t, want.Props, got.Props)
	require.Equal(t, want.Doorways, got.Doorways)
	require.Len(t, got.Boundaries, len(want.Boundaries)+15, "a third line across the hall: 8 straight crossings and 7 staggered")

	require.Len(t, compiled.Monsters, 3)
	require.Equal(t, "hall", compiled.Monsters[1].Region, "the second skeleton is in the hall")
	require.Equal(t, spatial.Position{X: 13, Y: 5}, compiled.Monsters[1].At, "behind the wall")
}
