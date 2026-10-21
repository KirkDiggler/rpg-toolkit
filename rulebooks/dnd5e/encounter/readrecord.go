// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// readrecord.go is THE RECORD READ (rpg-project#628, ideas/event-stream R5,
// R13, R15, R25): every retained beat, as written, after an id. It is the
// GM and debug view — every audience, per-observer beats included — which is
// what the record stream delivers before release (R26). [Encounter.Story]
// stays the per-member read.

// ReadRecordInput names where the read resumes.
type ReadRecordInput struct {
	// AfterID is the last record id the caller holds; the read returns
	// entries with a greater id. Empty reads from the oldest retained entry.
	AfterID string
}

// ReadRecordOutput is the retained record after the requested id.
type ReadRecordOutput struct {
	// Entries are the retained entries after AfterID, in id order (which is
	// seq order).
	Entries []record.Entry

	// Restarted is true when AfterID is older than the oldest retained entry
	// and something has aged out of the retention window: Entries are then
	// every retained entry, and the caller replaces what it holds rather
	// than appending — a short answer would otherwise look complete.
	Restarted bool
}

// ReadRecord returns the retained record after an id.
//
// RETENTION IS AS IT IS TODAY (R25): the window is enforced at the storage
// boundary, so a read on a live encounter sees everything appended since its
// load. An id below the oldest retained entry, when anything has been
// trimmed, cannot be resumed from and restarts the read instead; an id below
// it when nothing has been trimmed is an ordinary read from the start.
//
// Errors: ErrNilInput; a record read failure.
func (e *Encounter) ReadRecord(in *ReadRecordInput) (*ReadRecordOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("read record: %w", ErrNilInput)
	}
	all, err := e.story.All(&record.AllInput{})
	if err != nil {
		return nil, fmt.Errorf("read record: %w", err)
	}
	if in.AfterID == "" || len(all) == 0 {
		return &ReadRecordOutput{Entries: all}, nil
	}
	if trimmed := all[0].Seq > 1; trimmed && in.AfterID < all[0].ID {
		return &ReadRecordOutput{Entries: all, Restarted: true}, nil
	}
	cut := len(all)
	for i, entry := range all {
		if entry.ID > in.AfterID {
			cut = i
			break
		}
	}
	return &ReadRecordOutput{Entries: all[cut:]}, nil
}
