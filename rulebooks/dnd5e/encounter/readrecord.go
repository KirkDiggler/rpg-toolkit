// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"strings"

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
// THE TRIM EDGE RESTARTS. An AfterID equal to the newest trimmed entry's id
// — a caller that missed nothing — is also below the oldest retained entry,
// and is told to restart: the record does not keep the id it trimmed
// through, so it cannot tell that caller from one that missed beats. The
// cost is a caller replacing rows it legitimately held. It is inert under
// RetentionUnbounded, which every session run uses; the fix, when retention
// matters, is for play/record to persist its trim floor id.
//
// AN AfterID IS AN ID OR NOTHING. Anything but empty or 26 upper-case
// Crockford base32 characters is refused, never read as "caught up": a
// lower-cased id sorts above every real one and would silently answer
// nothing (R28).
//
// Errors: ErrNilInput; ErrInvalidData for a malformed AfterID; a record
// read failure.
func (e *Encounter) ReadRecord(in *ReadRecordInput) (*ReadRecordOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("read record: %w", ErrNilInput)
	}
	if in.AfterID != "" && !isRecordID(in.AfterID) {
		return nil, fmt.Errorf("read record: after id %q is not a record id: %w", in.AfterID, ErrInvalidData)
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

// isRecordID reports whether id has the record's wire-id form: 26 upper-case
// Crockford base32 characters, the first no wider than 3 bits — the rule
// play/record applies to the ids it loads.
func isRecordID(id string) bool {
	const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	if len(id) != 26 || id[0] > '7' {
		return false
	}
	for i := 0; i < len(id); i++ {
		if strings.IndexByte(crockford, id[i]) < 0 {
			return false
		}
	}
	return true
}
