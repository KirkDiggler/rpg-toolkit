// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// contentgolden_test.go is THE WHOLE PICTURE, COMMITTED: every content file
// this repository ships, compiled, in one reviewable file each.
//
// It began as ACCEPTANCE A1 for the scenery slice — every content file
// compiles to the atlas it already compiled to — against a golden captured
// before scenery existed. Scenery was additive and that claim was exactly
// right for it. THE WALL SLICE IS NOT ADDITIVE (rpg-project#360): the pair
// form is deleted and every fixture is re-authored as lines, so byte-identity
// is not something the new dialect could honestly promise. The design says so
// itself — A6 "is the forcing case and the regression net that replaces
// byte-identity."
//
// So these goldens were regenerated once, on this branch, and what they are
// now is a CHARACTERIZATION: any later change to the compiler shows up here as
// a diff a reviewer reads, rather than as a surprise in the game. That is
// worth having and it is not proof of anything on its own — a golden a feature
// wrote about itself proves only that the feature agrees with itself. The
// independent claim lives next door in golden_test.go, against the atlas
// version 1 produced before any of this existed.
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
	Exits       []encounter.AtlasExit          `json:"exits"`
	Start       *encounter.AtlasStart          `json:"start"`
	PartyStart  []dungeonspec.Seat             `json:"party_start"`
	Monsters    []dungeonspec.MonsterPlacement `json:"monsters"`

	// Scenarios is what this slice added that a host can observe and the
	// atlas does not carry (rpg-project#368). Exits, prop ids and
	// holdability all ride in the atlas above, which is already here.
	Intel     []encounter.IntelRecord      `json:"intel"`
	Scenarios map[string]map[string]string `json:"scenarios"`

	// Factions and Dispositions are the sides (rpg-project#375). Omitted
	// when a file declares none, so the four dungeons authored before
	// factions existed picture byte-identically.
	Factions     []encounter.FactionInput     `json:"factions,omitempty"`
	Dispositions []encounter.DispositionInput `json:"dispositions,omitempty"`
}

func contentGoldenOf(t *testing.T, path string) contentGolden {
	t.Helper()
	compiled, atlas := compiledAtlas(t, path)
	return contentGolden{
		Orientation:  string(atlas.Orientation.Kind()),
		Void:         string(compiled.Field.Canvas.Void.Kind()),
		Cells:        atlas.Cells,
		Regions:      atlas.Regions,
		Props:        atlas.Props,
		Boundaries:   atlas.Boundaries,
		Doorways:     atlas.Doorways,
		Exits:        atlas.Exits,
		Start:        atlas.Start,
		PartyStart:   compiled.PartyStart,
		Monsters:     compiled.Monsters,
		Intel:        compiled.Intel,
		Scenarios:    compiled.Scenarios,
		Factions:     compiled.Factions,
		Dispositions: compiled.Dispositions,
	}
}

// TestEveryContentFileCompilesToItsCommittedPicture walks every authored
// dungeon in testdata and compares the whole compile against the picture
// committed beside it.
//
// Regenerate deliberately, never reflexively: `CONTENT_GOLDEN=write go test
// ./dungeonspec/ -run TestEveryContentFile`, then READ THE DIFF. A golden that
// is rewritten whenever it disagrees is a test that cannot fail.
func TestEveryContentFileCompilesToItsCommittedPicture(t *testing.T) {
	files, err := filepath.Glob("testdata/*.yaml")
	require.NoError(t, err)
	require.Len(t, files, 5, "the five authored dungeons this package ships")

	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			goldenPath := strings.TrimSuffix(path, ".yaml") + ".compiled.json"

			got, err := json.Marshal(contentGoldenOf(t, path))
			require.NoError(t, err)
			if os.Getenv("CONTENT_GOLDEN") == "write" {
				require.NoError(t, os.WriteFile(goldenPath, got, 0o600))
				t.Log("golden rewritten — read the diff before committing it")
				return
			}

			want, err := os.ReadFile(goldenPath)
			require.NoError(t, err, "the compiled picture is committed beside the file")

			require.JSONEq(t, string(want), string(got),
				"%s compiles to a different world than the picture committed beside it", path)
		})
	}
}
