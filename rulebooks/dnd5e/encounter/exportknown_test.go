// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// exportknown_test.go is the test-only door into the `known:cells` fold and
// its two spellings (rpg-project#753, S4) for the external package's
// fixtures. In a _test.go file, so nothing outside this package reaches it.

// ExportedCellsKnownKind is the journal kind of a member's learned cells.
const ExportedCellsKnownKind = string(cellsKnownKind)

// ExportedKnownCells is a copy of the cells member's facts fold to.
func (e *Encounter) ExportedKnownCells(member MemberID) map[spatial.Position]bool {
	out := make(map[spatial.Position]bool)
	for cell := range e.world.knownCells(member) {
		out[cell] = true
	}
	return out
}

// ExportedDecodeCellsKnown decodes one fact's Detail as the fold does.
func ExportedDecodeCellsKnown(detail string) ([]spatial.Position, error) {
	return decodeCellsKnown(detail, maxFieldCells)
}
