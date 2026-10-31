// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// TestGenerate_Loads: both default sizes load through the dialect's loader
// with one region per room, the courtyard (which is also the implicit
// region) and both grounds, the floor the requested count, and the party
// seated in the gatehouse.
func TestGenerate_Loads(t *testing.T) {
	p := DefaultPlan()
	for _, hexes := range []int{10000, 30000} {
		t.Run(fmt.Sprint(hexes), func(t *testing.T) {
			g := keepAt(t, hexes)
			compiled, err := dungeonspec.Load(g.doc)
			require.NoError(t, err)
			require.Len(t, compiled.Field.Regions, len(p.Rooms)+3)
			cells := 0
			for _, r := range compiled.Field.Regions {
				cells += len(r.Cells)
			}
			require.Equal(t, hexes, cells, "every hex is floor: no wall turned one to footing")
			require.Equal(t, hexes, g.report.Hexes)
			require.Equal(t, len(p.Rooms)+3, g.report.Regions)
			require.GreaterOrEqual(t, len(compiled.PartyStart), 4)
			require.Equal(t, "gatehouse", compiled.PartyStart[0].Region)
			require.Equal(t, len(p.Doors), g.report.Doors)
			require.Len(t, compiled.Monsters, g.report.Monsters)
			require.Equal(t, "bench-keep", compiled.Key)
			t.Logf("%d hexes: %s, %d bytes", hexes, g.report, len(g.doc))
		})
	}
}

// TestGenerate_RefusesOptions: hexes below the plan, a negative density, a
// key the shelf could not use, a command line that would break the header.
func TestGenerate_RefusesOptions(t *testing.T) {
	for _, c := range []struct {
		edit func(*Options)
		want string
	}{
		{func(o *Options) { o.Hexes = 5000 }, "the plan needs at least 10000 hexes"},
		{func(o *Options) { o.PropDensity = -1 }, "densities must be zero or more"},
		{func(o *Options) { o.Key = "Bench Keep" }, `key "Bench Keep" must be lowercase letters, digits and dashes`},
		{func(o *Options) { o.CommandLine = "a\nb" }, "the command line must be one line"},
	} {
		o := DefaultOptions()
		c.edit(&o)
		doc, _, err := Generate(DefaultPlan(), o)
		require.ErrorContains(t, err, c.want)
		require.Nil(t, doc)
	}
}

// TestGenerate_ZeroDensityMeansNone: no props, no monsters, still a keep.
func TestGenerate_ZeroDensityMeansNone(t *testing.T) {
	o := DefaultOptions()
	o.PropDensity, o.GroundsDensity, o.MonsterDensity = 0, 0, 0
	_, r, err := Generate(DefaultPlan(), o)
	require.NoError(t, err)
	require.Zero(t, r.Props)
	require.Zero(t, r.Monsters)
	require.Equal(t, 10000, r.Hexes)
}
