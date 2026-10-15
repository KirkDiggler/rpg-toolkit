// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// VisibleCellsInput selects the member whose seen-now cells are read.
type VisibleCellsInput struct {
	Member MemberID
}

// VisibleCellsOutput is one member's seen-now set.
type VisibleCellsOutput struct {
	// Cells is every cell the member's sight reaches at the moment of the
	// read, sorted in the map's coordinate order. Never nil. A placed member
	// always has at least the cell they stand on; an unplaced member has none.
	Cells []spatial.Position
}

// VisibleCellsFor is SEEN NOW, PER SEAT, NEVER STORED (rpg-project#586 R1):
// the known cells this member's sight reaches at the moment of the read.
//
// It is derived from current positions and the [Sight] capability's current
// answer through the same [sightReach.reachesCell] the sight pass learns
// cells with — one sight test (R5), built by the same [Encounter.newSightReach]
// — so a cell seen now is always one the pass has taught, and the answer is a
// subset of [Encounter.AtlasFor]'s cells. It is never a party union: no other
// member's position or sight enters it. Nothing about it is persisted; a
// stored visible set would outlive the pass whose readings made it.
//
// Refusals: a cell the member does not know is never in the answer, nor is a
// cell a concealment still hides from them. A sight-obscuring area between
// the member and a cell drops it from seen-now while it stays remembered.
//
// THE CELL THE MEMBER STANDS ON IS ALWAYS IN IT. A blind member, or one inside
// an obscuring area, still knows where they stand — the same presence rule
// the sight pass learns by — so a placed member's answer is never empty, which
// is what lets a host read an empty list as "not sent" (rpg-project#586 plan,
// Task 4).
//
// ErrNilInput for a nil input, ErrNotMember for a stranger; an unplaced
// member answers an empty, non-nil list. The [Sight] capability is asked about
// this member alone, so its refusals concern this member alone.
func (e *Encounter) VisibleCellsFor(in *VisibleCellsInput) (*VisibleCellsOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("visible cells for %q: %w", in.Member, ErrNotMember)
	}
	out := &VisibleCellsOutput{Cells: make([]spatial.Position, 0)}
	own, placed := e.canvas.GetEntityPosition(string(in.Member))
	if !placed {
		return out, nil
	}
	// ONE VIEWER, ONE RANGE: only this member's sight enters seen-now, so only
	// this member is asked about. Another member's unanswerable sheet is not
	// this read's business.
	reach, err := e.sightOf([]MemberID{in.Member})
	if err != nil {
		return nil, fmt.Errorf("visible cells for %q: %w", in.Member, err)
	}
	geometry := e.newSightReach(map[MemberID]spatial.Position{in.Member: own}, reach)
	known := e.world.knownCells(in.Member)
	hidden := e.hiddenFrom(in.Member)
	// e.field.cells is sorted by cellBefore, so the answer is too.
	for _, cell := range e.field.cells {
		if cell == own || (known[cell] && !hidden.cells[cell] && geometry.seesCell(in.Member, cell, hidden)) {
			out.Cells = append(out.Cells, cell)
		}
	}
	return out, nil
}
