// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// LogData is the persistent representation of a Log.
// The zero value marshals to {} (design R8); the fresh-log encoding convention
// stores NextSeq 0 for a never-appended log (runtime nextSeq 1), so LoadLog(LogData{})
// yields a fresh log with NextSeq() == 1. Trimmed-empty logs store their real
// NextSeq (always >= 2). Stored NextSeq 1 is never written and is rejected by LoadLog.
type LogData struct {
	// NextSeq is the sequence number the next Append will assign (or 0 for fresh).
	NextSeq uint64 `json:"next_seq,omitempty"`
	// Entries are the retained log entries, deep-copied from the runtime log.
	Entries []EntryData `json:"entries,omitempty"`
}

// EntryData is the persistent representation of an Entry.
type EntryData struct {
	// Seq is the monotonic, gapless sequence assigned at Append.
	Seq uint64 `json:"seq"`
	// At is the optional caller-supplied provenance timestamp.
	At uint64 `json:"at,omitempty"`
	// Correlation is an opaque caller token grouping cause and effects.
	Correlation string `json:"correlation,omitempty"`
	// Audience is the roster of viewers.
	Audience []core.EntityID `json:"audience,omitempty"`
	// Tags is the caller-chosen question-surface.
	Tags map[string]string `json:"tags,omitempty"`
	// Payload is the opaque composition-encoded story beats.
	Payload []byte `json:"payload"`
}

// ToData returns the persistent representation of the Log.
// The fresh-log encoding convention (design R8): a never-appended log
// (nextSeq == 1, no entries) returns the zero LogData (NextSeq 0, Entries nil).
// A trimmed-empty log (nextSeq >= 2, no entries) stores the real NextSeq.
// A populated log stores NextSeq and deep-copied entries.
func (l *Log) ToData() LogData {
	// Fresh log: nextSeq == 1 && no entries → return zero LogData
	if l.nextSeq == 1 && len(l.entries) == 0 {
		return LogData{}
	}

	// Populated or trimmed-empty: store the real NextSeq and deep-copied entries
	data := LogData{
		NextSeq: l.nextSeq,
	}

	if len(l.entries) > 0 {
		data.Entries = make([]EntryData, len(l.entries))
		for i, e := range l.entries {
			data.Entries[i] = EntryData{
				Seq:         e.seq,
				At:          e.at,
				Correlation: e.correlation,
				Audience:    copyAudience(e.audience),
				Tags:        copyTags(e.tags),
				Payload:     copyPayload(e.payload),
			}
		}
	}

	return data
}

// LoadLog reconstructs a Log from persisted LogData.
// Validates per design R9: rejects non-contiguous/zero Seqs, mismatched
// NextSeq-to-lastSeq relationship, stored NextSeq 1 (fresh encodes as 0),
// invalid audiences (empty IDs, duplicates), invalid tags (empty keys),
// and nil payloads. Deep-copies all data; empty→nil normalization preserved.
// Returns ErrInvalidData for any R9 rejection with verb-context wrapping.
func LoadLog(data LogData) (*Log, error) {
	// Empty data (zero LogData) → fresh log (nextSeq 1)
	if data.NextSeq == 0 && len(data.Entries) == 0 {
		return &Log{nextSeq: 1}, nil
	}

	// Validate empty-entries case
	if len(data.Entries) == 0 {
		// Stored NextSeq 1 is never written (fresh encodes as 0, trimmed-empty >= 2)
		if data.NextSeq == 1 {
			return nil, fmt.Errorf("load log: empty entries with next_seq 1 unreachable: %w", ErrInvalidData)
		}
		// Trimmed-empty: NextSeq >= 2 is legal
		return &Log{nextSeq: data.NextSeq}, nil
	}

	// Non-empty: validate contiguity, NextSeq, and per-entry constraints

	// Check contiguity: first seq >= 1, each seq == prev + 1
	for i, ed := range data.Entries {
		if ed.Seq == 0 {
			return nil, fmt.Errorf("load log: entry[%d] seq 0: %w", i, ErrInvalidData)
		}
		if i > 0 {
			prevSeq := data.Entries[i-1].Seq
			if ed.Seq != prevSeq+1 {
				msg := fmt.Sprintf(
					"load log: entry[%d] seq %d not contiguous (prev %d)",
					i, ed.Seq, prevSeq,
				)
				return nil, fmt.Errorf("%s: %w", msg, ErrInvalidData)
			}
		}
	}

	// Check NextSeq == lastSeq + 1 exactly
	lastSeq := data.Entries[len(data.Entries)-1].Seq
	if data.NextSeq != lastSeq+1 {
		msg := fmt.Sprintf(
			"load log: next_seq %d != last_seq+1 (%d+1=%d)",
			data.NextSeq, lastSeq, lastSeq+1,
		)
		return nil, fmt.Errorf("%s: %w", msg, ErrInvalidData)
	}

	// Validate per-entry constraints
	for i, ed := range data.Entries {
		// Validate audience (no empty IDs, no duplicates)
		if err := validateAudience(ed.Audience); err != nil {
			return nil, fmt.Errorf("load log: entry[%d] audience: invalid data: %w", i, ErrInvalidData)
		}

		// Validate tags (no empty keys)
		if err := validateTags(ed.Tags); err != nil {
			return nil, fmt.Errorf("load log: entry[%d] tags: invalid data: %w", i, ErrInvalidData)
		}

		// Validate payload (not nil)
		if ed.Payload == nil {
			return nil, fmt.Errorf("load log: entry[%d] payload nil: %w", i, ErrInvalidData)
		}
	}

	// All validation passed; deep-copy entries
	entries := make([]entry, len(data.Entries))
	for i, ed := range data.Entries {
		entries[i] = entry{
			seq:         ed.Seq,
			at:          ed.At,
			correlation: ed.Correlation,
			audience:    copyAudience(ed.Audience),
			tags:        copyTags(ed.Tags),
			payload:     copyPayload(ed.Payload),
		}
	}

	return &Log{
		entries: entries,
		nextSeq: data.NextSeq,
	}, nil
}
