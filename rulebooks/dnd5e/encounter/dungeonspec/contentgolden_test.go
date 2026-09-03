// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// contentgolden_test.go is ACCEPTANCE A1 for the scenery slice
// (rpg-project#360, wall-geometry design §7): EVERY CONTENT FILE THIS
// REPOSITORY SHIPS COMPILES TO THE ATLAS IT ALREADY COMPILED TO.
//
// Scenery is additive — a file that authors none must be untouched by the
// feature existing — and the only honest way to say that is a golden captured
// from the build BEFORE the feature. testdata/*.compiled.json was written by a
// throwaway generator run on origin/main at 304e0645, the commit this branch
// was cut from, and is never regenerated from the new code: a golden a feature
// wrote about itself proves the feature agrees with itself.
//
// It covers more than golden_test.go's does, deliberately. That one is the
// version-1 atlas — cells, walls, doorways, props — and stops where version 1
// stopped. This one carries the regions with their archetypes and lighting,
// the party's seats in their computed order, the monsters with their derived
// regions, and the canvas's two declarations: everything [dungeonspec.Load]
// produces that a host can observe.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// contentGolden is everything a compile of one content file produces that a
// host can observe, in concrete types: [encounter.Atlas] holds its orientation
// behind an interface that marshals to `{}`, so the two declarations are
// carried as the words they are.
type contentGolden struct {
	Orientation string                         `json:"orientation"`
	Void        string                         `json:"void"`
	Cells       []spatial.Position             `json:"cells"`
	Regions     []encounter.AtlasRegion        `json:"regions"`
	Props       []encounter.AtlasProp          `json:"props"`
	Boundaries  []encounter.AtlasBoundary      `json:"boundaries"`
	Doorways    []encounter.AtlasDoorway       `json:"doorways"`
	PartyStart  []dungeonspec.Seat             `json:"party_start"`
	Monsters    []dungeonspec.MonsterPlacement `json:"monsters"`
}

func contentGoldenOf(t *testing.T, path string) contentGolden {
	t.Helper()
	compiled, atlas := compiledAtlas(t, path)
	return contentGolden{
		Orientation: string(atlas.Orientation.Kind()),
		Void:        string(compiled.Field.Canvas.Void.Kind()),
		Cells:       atlas.Cells,
		Regions:     atlas.Regions,
		Props:       atlas.Props,
		Boundaries:  atlas.Boundaries,
		Doorways:    atlas.Doorways,
		PartyStart:  compiled.PartyStart,
		Monsters:    compiled.Monsters,
	}
}

// TestA1_EveryContentFileCompilesToItsPreSceneryGolden walks every authored
// dungeon in testdata and compares the whole compile against the golden
// captured before scenery existed. A file with no `scenery:` block must be
// bit-for-bit the dungeon it was.
func TestA1_EveryContentFileCompilesToItsPreSceneryGolden(t *testing.T) {
	files, err := filepath.Glob("testdata/*.yaml")
	require.NoError(t, err)
	require.Len(t, files, 3, "the three authored dungeons this package ships")

	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			goldenPath := strings.TrimSuffix(path, ".yaml") + ".compiled.json"
			want, err := os.ReadFile(goldenPath)
			require.NoError(t, err, "the golden was captured on origin/main before scenery")

			got, err := json.Marshal(contentGoldenOf(t, path))
			require.NoError(t, err)

			require.JSONEq(t, string(want), string(got),
				"%s compiles to a different world than it did before scenery existed", path)
		})
	}
}
