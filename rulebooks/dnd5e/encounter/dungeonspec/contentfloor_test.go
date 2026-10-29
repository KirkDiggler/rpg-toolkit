// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// contentfloor_test.go is THE CONVERSION'S PROOF (rpg-project#725, R6 and the
// design's Equality law): every content file converted from the walkable list
// to authored regions compiles to the same world it compiled to before —
// against the pictures captured at the base commit, kept in testdata/pre-v5
// for exactly this comparison — and the only thing that moves is how the
// floor is grouped into regions.
//
// Key by key: every key of the picture but three is byte-identical. The three:
//
//   - regions: the UNION of the regions' cells is identical, while the
//     regions themselves regroup from the walls' partitions to the authored
//     regions; and for a single-room file the encounter's derived partitions
//     are exactly the regions the old compile reported, so the mechanical
//     division did not move either;
//   - monsters and party_start: the same members and seats on the same cells,
//     each now reporting the authored region it stands in.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// preV5 is where the pictures captured at the base commit are kept.
const preV5 = "testdata/pre-v5"

func TestConvertedContentKeepsItsFloor(t *testing.T) {
	pictures, err := filepath.Glob(filepath.Join(preV5, "*.compiled.json"))
	require.NoError(t, err)
	require.NotEmpty(t, pictures, "the pre-v5 pictures are committed for this comparison")
	for _, before := range pictures {
		name := strings.TrimSuffix(filepath.Base(before), ".compiled.json")
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", name+".yaml")
			compiled, enc := compiledEncounter(t, path)
			gotBytes, err := json.Marshal(contentGoldenOf(t, path))
			require.NoError(t, err)
			wantBytes, err := os.ReadFile(before)
			require.NoError(t, err)

			var got, want map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(gotBytes, &got))
			require.NoError(t, json.Unmarshal(wantBytes, &want))
			keys := map[string]bool{}
			for k := range got {
				keys[k] = true
			}
			for k := range want {
				keys[k] = true
			}
			for k := range keys {
				switch k {
				case "regions", "monsters", "party_start":
					continue
				}
				require.JSONEq(t, string(orNull(want[k])), string(orNull(got[k])), "%s: %q moved", name, k)
			}

			var wantRegions, gotRegions []encounter.AtlasRegion
			require.NoError(t, json.Unmarshal(want["regions"], &wantRegions))
			require.NoError(t, json.Unmarshal(got["regions"], &gotRegions))
			require.ElementsMatch(t, unionOf(wantRegions), unionOf(gotRegions), "%s: the floor is the same cells", name)
			src := string(mustRead(t, path))
			single := strings.HasPrefix(src, "version: 5\n") || strings.Contains(src, "\nversion: 5\n")
			if single {
				for _, r := range gotRegions {
					require.Equal(t, 1.0, r.Lighting.Intensity, "%s: %s is lit as nothing authored", name, r.ID)
				}
				// THE MECHANICAL DIVISION DID NOT MOVE: the old compile's
				// regions WERE the walls' partitions, and the encounter
				// derives the same ones now.
				require.Equal(t, groupsOf(wantRegions), partitionsOf(t, enc, gotRegions),
					"%s: the walls make the same rooms they made", name)
			} else {
				require.JSONEq(t, string(want["regions"]), string(got["regions"]), "%s: a v2 document is untouched", name)
			}
			t.Logf("%s: %d regions before, %d after", name, len(wantRegions), len(gotRegions))

			owner := map[spatial.Position]string{}
			for _, r := range gotRegions {
				for _, c := range r.Cells {
					owner[c] = r.ID
				}
			}
			orientation := compiled.Field.Canvas.Orientation
			sameButRegion(t, name, "monsters", want["monsters"], got["monsters"], owner, orientation)
			sameButRegion(t, name, "party_start", want["party_start"], got["party_start"], owner, orientation)
		})
	}
}

// sameButRegion asserts two lists of placements are the same entries with
// every key but `Region` identical, and that each entry's region is the
// authored region owning its cell. ([dungeonspec.Seat] and
// [dungeonspec.MonsterPlacement] carry no JSON tags, so the picture's keys
// are the Go field names.)
func sameButRegion(t *testing.T, name, key string, want, got json.RawMessage, owner map[spatial.Position]string, o encounter.Orientation) {
	t.Helper()
	var w, g []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(orNull(want), &w))
	require.NoError(t, json.Unmarshal(orNull(got), &g))
	require.Len(t, g, len(w), "%s: %s", name, key)
	for i := range w {
		for k := range w[i] {
			if k != "Region" {
				require.JSONEq(t, string(w[i][k]), string(g[i][k]), "%s: %s[%d].%s moved", name, key, i, k)
			}
		}
		var at spatial.Position
		require.NoError(t, json.Unmarshal(g[i]["At"], &at))
		var region string
		require.NoError(t, json.Unmarshal(g[i]["Region"], &region))
		require.Equal(t, owner[encounter.HexCellAt(o, int(at.X), int(at.Y))], region,
			"%s: %s[%d] reports the authored region it stands in", name, key, i)
	}
}

// unionOf is every cell of every region.
func unionOf(regions []encounter.AtlasRegion) []spatial.Position {
	var out []spatial.Position
	for _, r := range regions {
		out = append(out, r.Cells...)
	}
	return out
}

// groupsOf is each region's cells as a sorted group, the groups sorted.
func groupsOf(regions []encounter.AtlasRegion) [][]spatial.Position {
	out := make([][]spatial.Position, 0, len(regions))
	for _, r := range regions {
		out = append(out, sortedCells(r.Cells))
	}
	return sortedGroups(out)
}

// partitionsOf is the encounter's wall partitions over the regions' cells, as
// sorted groups, the groups sorted.
func partitionsOf(t *testing.T, enc *encounter.Encounter, regions []encounter.AtlasRegion) [][]spatial.Position {
	t.Helper()
	by := map[encounter.PartitionID][]spatial.Position{}
	for _, c := range unionOf(regions) {
		p, ok := enc.PartitionAt(c)
		require.True(t, ok, "%v is in a partition", c)
		by[p] = append(by[p], c)
	}
	out := make([][]spatial.Position, 0, len(by))
	for _, cells := range by {
		out = append(out, sortedCells(cells))
	}
	return sortedGroups(out)
}

func sortedCells(cells []spatial.Position) []spatial.Position {
	out := append([]spatial.Position(nil), cells...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

func sortedGroups(groups [][]spatial.Position) [][]spatial.Position {
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i][0], groups[j][0]
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.X < b.X
	})
	return groups
}

func orNull(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return json.RawMessage("null")
	}
	return raw
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return raw
}
