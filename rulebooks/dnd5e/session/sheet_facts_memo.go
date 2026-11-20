// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// sheetFactsMemo is one verb's sheet facts, each member's read at most once
// (rpg-toolkit#2493).
//
// The encounter asks for every member's sheet facts on its hot path — in the
// castle, every creature that walks in a world round asks — and a read is a
// repository read and a load of the sheet with its held effects. Within one
// verb the sheet does not change unless the verb writes it, so the second ask
// is answered from the first.
//
// # Nothing survives the verb, nothing is saved
//
// [Manager.standingFor] builds a new memo for every verb (S1, S4), exactly as
// it builds the [participationMemo] beside it, and nothing here is written
// anywhere. What follows from a creature's effects is still worked out when
// read (rpg-project#990 comment 7034098169): this keeps one verb from
// working it out twice from the same sheet.
//
// # A sheet written is a sheet re-read
//
// The memo forgets a member at every write of its sheet, through the only two
// doors a verb writes sheets by:
//   - a character record, through the verb's [sheetStore] save, which every
//     character write passes (TestOnlyTheStoreCallsTheCharacterRepository) and
//     which reports into [writeScope.noteSheetTouched], fenced or not;
//   - a monster record, through [writeScope.replaceMonsterSheet].
//
// A hold given by a line, ended by a fight's start or by leaving the run is a
// sheet written through one of them, so the next ask reads it afresh. A read
// that failed is not kept.
//
// A continued walk (a Move that answers before the world runs) keeps its
// memo across the wait for the host, which is safe only because it holds the
// session guard for that whole time: a host writing a record outside the SDK
// in between would break it, as it already breaks the participation memo.
//
// The sneaker's halved speed is NOT here: it is asked of the live encounter
// at every read ([sheetSeam.factsOf]).
type sheetFactsMemo struct {
	kept map[encounter.MemberID]encounter.SheetFacts
}

// newSheetFactsMemo returns an empty memo for one verb.
func newSheetFactsMemo() *sheetFactsMemo {
	return &sheetFactsMemo{kept: make(map[encounter.MemberID]encounter.SheetFacts)}
}

// read answers id's facts from the memo, or reads them with fresh and keeps
// them. A nil memo keeps nothing. The caller gets its own copy of the lists.
func (m *sheetFactsMemo) read(
	id encounter.MemberID, fresh func(encounter.MemberID) (encounter.SheetFacts, error),
) (encounter.SheetFacts, error) {
	if m != nil {
		if kept, ok := m.kept[id]; ok {
			return copyFacts(kept), nil
		}
	}
	facts, err := fresh(id)
	if err != nil {
		return encounter.SheetFacts{}, err
	}
	if m != nil {
		m.kept[id] = copyFacts(facts)
	}
	return facts, nil
}

// forget drops what the memo holds for id, after its sheet was written.
func (m *sheetFactsMemo) forget(id string) {
	if m == nil {
		return
	}
	delete(m.kept, encounter.MemberID(id))
}

// copyFacts is facts with its lists copied.
func copyFacts(facts encounter.SheetFacts) encounter.SheetFacts {
	facts.Actions = slices.Clone(facts.Actions)
	facts.CannotMove = slices.Clone(facts.CannotMove)
	return facts
}
