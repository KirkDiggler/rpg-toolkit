// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// generated is one default-density keep, every stage kept so the tests can
// read the placements against what the loader compiled.
type generated struct {
	opts     Options
	layout   *layout
	site     *site
	doc      []byte
	report   Report
	compiled dungeonspec.Compiled
}

var (
	generatedOnce sync.Map // hexes -> *sync.Once
	generatedAt   sync.Map // hexes -> *generated
)

// keepAt generates the embedded plan at hexes with the default options, once
// per test binary: the 30k keep takes a full compile, and under -race the
// package's tests would pay it once per test otherwise.
func keepAt(t *testing.T, hexes int) *generated {
	t.Helper()
	once, _ := generatedOnce.LoadOrStore(hexes, &sync.Once{})
	once.(*sync.Once).Do(func() {
		g := &generated{opts: DefaultOptions()}
		g.opts.Hexes = hexes
		g.opts.CommandLine = "keepgen"
		p := DefaultPlan()
		var err error
		g.layout, err = layoutOf(p, hexes)
		if err != nil {
			panic(err)
		}
		g.site, err = placeAll(p, g.layout, g.opts)
		if err != nil {
			panic(err)
		}
		g.doc, g.report, err = Generate(p, g.opts)
		if err != nil {
			panic(err)
		}
		g.compiled, err = dungeonspec.Load(g.doc)
		if err != nil {
			panic(err)
		}
		generatedAt.Store(hexes, g)
	})
	g, ok := generatedAt.Load(hexes)
	require.True(t, ok, "the keep at %d hexes generated", hexes)
	return g.(*generated)
}

// floorCells is every compiled floor cell and the region that owns it.
func floorCells(c dungeonspec.Compiled) map[cell]string {
	out := map[cell]string{}
	for _, r := range c.Field.Regions {
		for _, p := range r.Cells {
			out[cell{int(p.X), int(p.Y)}] = r.ID
		}
	}
	return out
}
