// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math/rand/v2"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// exportsweep_test.go is the test-only door into the discovery sweep
// (rpg-project#753, S3) for the external package's keep and castle fixtures,
// which an internal test cannot import. In a _test.go file, so nothing
// outside this package reaches it at build time.

// ExportedRosterIDs is every member, sorted: who a verb's look is asked of.
func (e *Encounter) ExportedRosterIDs() []MemberID { return e.rosterIDs() }

// ExportedCellsMeasured is how many times the per-placement floor measures
// ran on this field, the index's own build included.
func (e *Encounter) ExportedCellsMeasured() int { return e.field.cellsMeasured }

// SweepAgreement is what one [Encounter.ExportedSweepAgrees] call compared.
type SweepAgreement struct {
	// Sweeps is how many observer positions were compared.
	Sweeps int
	// Learned is how many cells the bounded sweeps answered, all positions.
	Learned int
	// Rim is how many of those stand exactly one past the observer's sight:
	// a cover's face, the cells the "+ 1" exists for.
	Rim int
}

// ExportedSweepAgrees holds the bounded sweep ([field.sweepCells], what
// [Encounter.discoverCells] walks) to the whole-floor sweep it replaced
// (sweepFullBefore, below, the v0.157.0 walk): for each observer, from its
// own cell or the one at names for it, the cells the sight test answers for
// are the same set. The geometry is the pass's own — [Encounter.sightNow]
// read once, [Encounter.newSightReach] over every placed member, inside a
// sight pass — so the answers are the ones a refresh would learn, before
// the "already known" filter both sweeps apply alike.
func (e *Encounter) ExportedSweepAgrees(observers []MemberID, at map[MemberID]spatial.Position) (SweepAgreement, error) {
	var out SweepAgreement
	sight, err := e.sightNow()
	if err != nil {
		return out, err
	}
	positions := map[MemberID]spatial.Position{}
	for id := range e.members {
		if cell, placed := e.canvas.GetEntityPosition(string(id)); placed {
			positions[id] = cell
		}
	}
	for id, cell := range at {
		positions[id] = cell
	}
	geometry := e.newSightReach(positions, sight)
	defer e.field.beginSightPass()()
	for _, observer := range observers {
		own, placed := positions[observer]
		if !placed {
			continue
		}
		hidden := e.hiddenFrom(observer)
		sees := func(cell spatial.Position) bool { return cell == own || geometry.seesCell(observer, cell, hidden) }
		bounded := map[spatial.Position]bool{}
		for _, cell := range e.field.sweepCells(own, sight[observer]) {
			if sees(cell) {
				bounded[cell] = true
			}
		}
		full := e.sweepFullBefore(own, sees)
		for cell := range full {
			if !bounded[cell] {
				return out, fmt.Errorf("%s at %v: the full sweep learns %v, %g away, the bounded one does not",
					observer, own, cell, e.Distance(own, cell))
			}
		}
		for cell := range bounded {
			if !full[cell] {
				return out, fmt.Errorf("%s at %v: the bounded sweep learns %v, the full one does not", observer, own, cell)
			}
			if e.Distance(own, cell) == float64(sight[observer]+1) {
				out.Rim++
			}
		}
		out.Sweeps++
		out.Learned += len(bounded)
	}
	return out, nil
}

// sweepFullBefore is the v0.157.0 discovery walk, kept verbatim as the
// reference: every floor cell of the field asked the sight test.
func (e *Encounter) sweepFullBefore(own spatial.Position, sees func(spatial.Position) bool) map[spatial.Position]bool {
	out := map[spatial.Position]bool{}
	for _, cell := range e.field.cells {
		if cell == own || sees(cell) {
			out[cell] = true
		}
	}
	return out
}

// ExportedPlacedReachAgrees holds the reach index ([field.placedNear]) to the
// walk it prefilters: for lanes between seeded pairs of floor cells up to
// span apart, and for every sampled cell's centre, every standing placement
// whose trace touches the lane — interior or contact — is among the index's
// candidates, and the candidates are ascending and distinct. It returns how
// many touching placements it checked, so a caller can see the lanes met
// something.
func (e *Encounter) ExportedPlacedReachAgrees(lanes, span int, seed int64) (int, error) {
	f := e.field
	rng := rand.New(rand.NewPCG(uint64(seed), 1))
	now := f.placedNow()
	touched := 0
	check := func(a, b spatial.Point) error {
		near := f.placedNear(a, b)
		in := make(map[int32]bool, len(near))
		for i, at := range near {
			if i > 0 && near[i-1] >= at {
				return fmt.Errorf("candidates for %v-%v are not ascending and distinct: %v", a, b, near)
			}
			in[at] = true
		}
		for i := range f.placed {
			p := &f.placed[i]
			placement, standing := now.stands(p)
			// mayTouch is the walk's own rejection, trusted here as it is
			// there (TestMayTouchAgreesWithSpatial holds it to spatial).
			if !standing || !p.reach.mayTouch(placement.Origin, a, b) {
				continue
			}
			trace, err := spatial.TraceFootprint(spatial.FootprintTraceInput{Placement: placement, From: a, To: b})
			if err != nil || !(trace.Interior || trace.Contact) {
				continue
			}
			touched++
			if !in[int32(i)] {
				return fmt.Errorf("placement %q touches %v-%v and the index does not name it", p.id, a, b)
			}
		}
		return nil
	}
	for i := 0; i < lanes; i++ {
		from := f.cells[rng.IntN(len(f.cells))]
		disk := e.cellsWithin(from, span)
		to := disk[rng.IntN(len(disk))]
		a, b := f.plane.CellCentre(from), f.plane.CellCentre(to)
		if err := check(a, b); err != nil {
			return touched, err
		}
		if err := check(a, a); err != nil {
			return touched, err
		}
	}
	return touched, nil
}
