// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// exportreach_test.go is the test-only door into the oracle and the placement
// index (rpg-project#753), for the external package's keep and castle
// fixtures, which an internal test cannot import. In a _test.go file, so
// nothing outside this package reaches it at build time.

// ExportedShownTo is [Encounter.shownTo] under the member's own mask, built
// once for the call.
func (e *Encounter) ExportedShownTo(member MemberID, cells []spatial.Position) bool {
	return e.shownTo(e.undiscoveredMask(member), cells)
}

// ExportedAtlasProjections is how many times [Encounter.Atlas] has been built
// on this encounter's field, every AtlasFor's included.
func (e *Encounter) ExportedAtlasProjections() int { return e.field.atlasProjections }

// ExportedIndexedStands is the placement index's own answer for an authored
// placed contributor: the cells it was built standing on.
func (e *Encounter) ExportedIndexedStands(id PropID) []spatial.Position {
	if e.field.index == nil {
		return nil
	}
	return slices.Clone(e.field.index.stands[id])
}

// ExportedPlacementIndexAgrees holds the bucketed reads to the whole-floor
// walk they replace: for every structural wall's presence and a seeded
// sample of the other placed contributors (all of them when sample is zero
// or covers them), the index's cells and support against the walk's, and the
// bucketed placedCells of the same rectangle dropped on drops seeded cells of
// the floor against the walk's. It returns the first disagreement, or nil.
//
// SAMPLED BECAUSE THE WALK IS THE COST BEING REMOVED: every walk is the
// whole floor, and on the keep under -race all of them together outran the
// package's CI budget.
func (e *Encounter) ExportedPlacementIndexAgrees(sample, drops int, seed int64) error {
	f := e.field
	walk := func(read func() ([]spatial.Position, error)) ([]spatial.Position, error) {
		centres := f.centres
		f.centres = nil
		defer func() { f.centres = centres }()
		return read()
	}
	rng := rand.New(rand.NewPCG(uint64(seed), 0)) //nolint:gosec // a test's sample, not a secret
	walls := make(map[PropID]bool, len(f.structuralWalls))
	for i := range f.structuralWalls {
		walls[f.structuralWalls[i].id] = true
	}
	var picked, others []int
	for i := range f.placed {
		if walls[f.placed[i].id] {
			picked = append(picked, i)
		} else {
			others = append(others, i)
		}
	}
	rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	if sample > 0 && sample < len(others) {
		others = others[:sample]
	}
	picked = append(picked, others...)
	for _, i := range picked {
		p := &f.placed[i]
		want, _ := walk(func() ([]spatial.Position, error) { return f.placedCells(p.placement), nil })
		if got := f.index.stands[p.id]; !slices.Equal(got, want) {
			return fmt.Errorf("%q stands: index %v, walk %v", p.id, got, want)
		}
		wantSupport, err := walk(func() ([]spatial.Position, error) { return f.footprintObservationCells(p.placement) })
		if err != nil {
			return fmt.Errorf("%q support walk: %w", p.id, err)
		}
		if got := f.index.support[p.id]; !slices.Equal(got, wantSupport) {
			return fmt.Errorf("%q support: index %v, walk %v", p.id, got, wantSupport)
		}
		for range drops {
			cell := f.cells[rng.IntN(len(f.cells))]
			moved := p.placement
			moved.Origin = f.plane.CellCentre(cell)
			want, _ := walk(func() ([]spatial.Position, error) { return f.placedCells(moved), nil })
			if got := f.placedCells(moved); !slices.Equal(got, want) {
				return fmt.Errorf("%q dropped on %v: bucketed %v, walk %v", p.id, cell, got, want)
			}
		}
	}
	return nil
}

// ExportedCellsWithin is [Encounter.cellsWithin]: the hex disk of reach
// around a cell.
func (e *Encounter) ExportedCellsWithin(from spatial.Position, reach int) []spatial.Position {
	return e.cellsWithin(from, reach)
}

// ExportedUndiscoveredAgrees holds [Encounter.undiscoveredFrom], now the
// mask materialized, to the whole-floor walk it was before rpg-project#753
// (undiscoveredFromBefore, kept verbatim below as the reference): the same
// hidden cells, unexplored cells and withheld doors for this member.
func (e *Encounter) ExportedUndiscoveredAgrees(member MemberID) error {
	got, want := e.undiscoveredFrom(member), e.undiscoveredFromBefore(member)
	for _, pair := range []struct {
		name      string
		got, want map[spatial.Position]bool
	}{{"cells", got.cells, want.cells}, {"unexplored", got.unexploredCells, want.unexploredCells}} {
		if !maps.Equal(pair.got, pair.want) {
			return fmt.Errorf("%s: %s differ: mask %d, walk %d", member, pair.name, len(pair.got), len(pair.want))
		}
	}
	if !maps.Equal(got.doors, want.doors) || !maps.Equal(got.props, want.props) {
		return fmt.Errorf("%s: doors or props differ", member)
	}
	return nil
}

// undiscoveredFromBefore is undiscoveredFrom as it stood at encounter
// v0.157.0, verbatim: the reference the mask is held to.
func (e *Encounter) undiscoveredFromBefore(member MemberID) hiddenView {
	hidden := e.hiddenFrom(member)
	hidden.unexploredCells = make(map[spatial.Position]bool)
	known := e.world.knownCells(member)
	// Existing explicit concealment discovery teaches its cells without the
	// sight pass seeing them. Preserve that knowledge; ordinary cell
	// discovery is not an authority to forget separately learned geometry.
	revealed := make(map[spatial.Position]bool)
	for i := range e.field.concealments {
		c := &e.field.concealments[i]
		if e.world.knowsConcealment(member, c.id) {
			for _, cell := range e.hiddenCellsOf(c) {
				revealed[cell] = true
			}
		}
	}
	for _, cell := range e.field.cells {
		if !known[cell] && !revealed[cell] {
			hidden.cells[cell] = true
			hidden.unexploredCells[cell] = true
		}
	}
	// Unowned scenery directly bordering learned floor belongs to its visible
	// boundary. A presented wall adds no footing of its own (fog-of-war R9).
	for _, cell := range e.field.cells {
		if _, owned := e.field.regionOf(cell); owned || !hidden.unexploredCells[cell] {
			continue
		}
		for _, neighbor := range adjacencyGrid.GetNeighbors(cell) {
			_, owned := e.field.regionOf(neighbor)
			if owned && known[neighbor] && !hidden.cells[neighbor] {
				// Knowing a boundary's ordinary footing does not beat an
				// explicit concealment on that floor. Keep the two masks
				// independent so unlisted props do not inherit floor secrecy.
				if _, concealed := e.field.concealmentOfCell[cell]; !concealed || revealed[cell] {
					delete(hidden.cells, cell)
				}
				delete(hidden.unexploredCells, cell)
				break
			}
		}
	}
	// An authored boundary brings its unowned footing, but not the adjoining
	// room's interior. Read the pre-boundary set so long chains do not flood.
	footing := make(map[spatial.Position]bool)
	for _, wall := range e.field.walls {
		from, to := e.field.cellAt(wall.From), e.field.cellAt(wall.To)
		if hidden.cells[from] && hidden.cells[to] {
			continue
		}
		for _, cell := range []spatial.Position{from, to} {
			if _, owned := e.field.regionOf(cell); !owned {
				footing[cell] = true
			}
		}
	}
	for cell := range footing {
		delete(hidden.cells, cell)
		delete(hidden.unexploredCells, cell)
	}
	for _, door := range e.doors {
		if hidden.doors[door.id] {
			continue
		}
		if id, concealed := e.hiddenDoorConcealment(door.id); concealed && e.world.knowsConcealment(member, id) {
			continue
		}
		visible := false
		for _, edge := range door.edges {
			if !hidden.cells[edge.From] || !hidden.cells[edge.To] {
				visible = true
				break
			}
		}
		// Footprint doors are projected with their fixed footprint rather than edges.
		if len(door.edges) > 0 && !visible {
			hidden.doors[door.id] = true
		}
	}
	return hidden
}

// ExportedRebuildPlacementIndex builds the field's centres and placement
// index again, as compileField does: what the index costs a load.
func (e *Encounter) ExportedRebuildPlacementIndex() {
	e.field.centres = newCellCentres(e.field)
	e.field.buildPlacementIndex()
}
