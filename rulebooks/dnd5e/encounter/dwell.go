// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// dwell.go is A PATROL STANDING AT ITS WAYPOINT (rpg-project#624; Kirk,
// 2026-10-19: "the patrol walks to an area, STANDS for N turns, then goes
// on").
//
// # The route stays stateless; the creature carries the dwell
//
// A route never remembers how far along a creature is: which waypoint is next
// is derived from where it stands (patrol.go). How long it has STOOD cannot be
// derived from anything, so it is the one thing a patrol keeps, and it is
// kept on the member, beside its facing: the waypoint's cell and the turns
// left to stand there ([memberRecord.Dwell]), persisted.
//
// # How a dwell runs
//
//   - A walk that ends standing on a waypoint whose [WaypointPause.Wait] is
//     above zero starts one: the creature turns to the waypoint's facing
//     when one is stated ([Encounter.faceWalkEnd]), and stands for the rest
//     of that turn.
//   - Each FRESH turn of the creature's after that spends one of the turns
//     left ([Encounter.spendDwell]); a turn resumed after a reaction window
//     is the same turn and spends none (R18). When none are left the dwell
//     ends and the patrol walks on.
//   - While it stands, [MonsterView.Dwelling] says so, and the table
//     driver's patrol row holds instead of walking. The creature's other
//     rows still fire above it — an `enemy: seen` row interrupts the dwell,
//     because the dwell is only the patrol's own leg.
//   - A creature that is not on the dwell's cell when its turn comes has
//     left its post — a row above the patrol walked it off, or something
//     moved it — and the dwell ends.

// dwell is a creature standing out a waypoint's wait.
type dwell struct {
	// Cell is the waypoint it stands on, dungeon-absolute.
	Cell spatial.Position

	// Rounds is how many more of its fresh turns it stands there. Zero is its
	// last: the next fresh turn ends the dwell and the patrol walks on.
	Rounds int
}

// DwellData is the persistent representation of a member's dwell.
type DwellData struct {
	Cell   PositionData `json:"cell"`
	Rounds int          `json:"rounds"`
}

// dwellDataFrom renders a dwell, nil staying nil.
func dwellDataFrom(d *dwell) *DwellData {
	if d == nil {
		return nil
	}

	return &DwellData{Cell: PositionData{X: d.Cell.X, Y: d.Cell.Y}, Rounds: d.Rounds}
}

// dwellFromData resolves a persisted dwell, nil staying nil.
func dwellFromData(d *DwellData) *dwell {
	if d == nil {
		return nil
	}

	return &dwell{Cell: spatial.Position{X: d.Cell.X, Y: d.Cell.Y}, Rounds: d.Rounds}
}

// validateDwellData refuses a stored dwell with negative turns left.
func validateDwellData(d *DwellData) error {
	if d != nil && d.Rounds < 0 {
		return fmt.Errorf("dwell of %d rounds is negative", d.Rounds)
	}

	return nil
}

// dwelling reports whether a member stands out a dwell now: it has one, and
// it is on the dwell's cell.
func (e *Encounter) dwelling(m *memberRecord) bool {
	if m.Dwell == nil {
		return false
	}
	here, placed := e.canvas.GetEntityPosition(string(m.ID))

	return placed && here == m.Dwell.Cell
}

// spendDwell is a FRESH turn's first act for a dwelling creature: one of the
// turns it stands is this one, and a dwell with none left, or one whose
// creature has left the cell, ends.
func (e *Encounter) spendDwell(m *memberRecord) {
	if m.Dwell == nil {
		return
	}
	if !e.dwelling(m) || m.Dwell.Rounds == 0 {
		m.Dwell = nil

		return
	}
	m.Dwell.Rounds--
}
