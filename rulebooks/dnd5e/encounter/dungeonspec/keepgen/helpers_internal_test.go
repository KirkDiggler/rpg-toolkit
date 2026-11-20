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
		// Generate's own two steps, kept apart so the site is compiled once:
		// the proof's compile is the one the tests read.
		g.doc, g.report, err = write(p, g.opts)
		if err != nil {
			panic(err)
		}
		g.compiled, err = Prove(g.doc)
		if err != nil {
			panic(err)
		}
		generatedAt.Store(hexes, g)
	})
	g, ok := generatedAt.Load(hexes)
	require.True(t, ok, "the keep at %d hexes generated", hexes)
	return g.(*generated)
}

// written is the bytes and report [Generate] would return for o, without the
// proof: for a test whose claim is about the bytes alone (a golden, a header,
// determinism). The proof compiles the whole site, which is nearly all of a
// generation's time; [keepAt] pays it once for the tests that read the
// compiled site.
func written(t *testing.T, o Options) ([]byte, Report) {
	t.Helper()
	doc, r, err := write(DefaultPlan(), o)
	require.NoError(t, err)
	return doc, r
}

// defaultWritten is the default keep's options at hexes, as [keepAt] makes
// it: command line "keepgen".
func defaultWritten(hexes int) Options {
	o := DefaultOptions()
	o.Hexes = hexes
	o.CommandLine = "keepgen"
	return o
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
