// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record

import (
	"fmt"
	"strconv"
	"strings"

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
	// Audiences is the audience table: each distinct audience among the
	// retained entries, stored once, in the order an entry first names it
	// (rpg-project#896, F20: on a 100-member keep the per-entry roster was
	// 73% of the session blob). A row is the exact ordered id list, non-empty
	// and duplicate-free; no two rows are the same list. An entry names its
	// row by EntryData.AudienceRef.
	Audiences [][]core.EntityID `json:"audiences,omitempty"`
	// Entries are the retained log entries, deep-copied from the runtime log.
	Entries []EntryData `json:"entries,omitempty"`
	// LastID is the last id the log issued, written only when it is above
	// the last retained entry's id — a MintID after the last append, or a log
	// trimmed to nothing — so an id issued after the load still sorts after
	// every id issued before it. Absent when the last id issued is the last
	// entry's.
	LastID string `json:"last_id,omitempty"`
}

// EntryData is the persistent representation of an Entry.
type EntryData struct {
	// Seq is the monotonic, gapless sequence assigned at Append.
	Seq uint64 `json:"seq"`
	// ID is the entry's wire id (id.go). Empty only on an entry persisted
	// before ids existed, which LoadLog gives a deterministic legacy id.
	ID string `json:"id,omitempty"`
	// At is the optional caller-supplied provenance timestamp.
	At uint64 `json:"at,omitempty"`
	// Correlation is an opaque caller token grouping cause and effects.
	Correlation string `json:"correlation,omitempty"`
	// AudienceRef names the entry's roster of viewers in LogData.Audiences,
	// 1-based: ref n is Audiences[n-1]. Zero is the empty audience (no
	// viewer, the GM/debug beat), so the zero value tells the truth and an
	// empty audience never takes a table row. The JSON key is "audience": a
	// blob from before the table, which held the inline array there, fails
	// to decode instead of loading every entry audience-less.
	AudienceRef uint32 `json:"audience,omitempty"`
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
//
// ToData is DataFrom(0): every retained entry is at or after seq 0.
func (l *Log) ToData() LogData {
	return l.DataFrom(0)
}

// LoadLog reconstructs a Log from persisted LogData.
// Validates per design R9: rejects non-contiguous/zero Seqs, mismatched
// NextSeq-to-lastSeq relationship, stored NextSeq 1 (fresh encodes as 0),
// invalid audience table rows (empty rows, empty IDs, duplicate IDs, a row
// stored twice), audience references past the table, invalid tags (empty keys),
// and nil payloads. Deep-copies all data; empty→nil normalization preserved.
// Returns ErrInvalidData for any R9 rejection with verb-context wrapping.
//
// IDS (id.go): an entry with no id — persisted before ids existed — takes the
// deterministic legacy id of its seq, which sorts below every minted id. Ids
// must be well formed and strictly increasing in seq order, so a minted id
// followed by a legacy one is refused. Options configure the loaded log as
// they do NewLog.
func LoadLog(data LogData, opts ...Option) (*Log, error) {
	l := &Log{ids: wallClock{}}
	for _, opt := range opts {
		opt(l)
	}

	var last ulid
	if data.LastID != "" {
		u, err := parseULID(data.LastID)
		if err != nil {
			return nil, fmt.Errorf("load log: last_id: %v: %w", err, ErrInvalidData)
		}
		last = u
	}

	// Empty data (zero LogData) → fresh log (nextSeq 1)
	if data.NextSeq == 0 && len(data.Entries) == 0 {
		l.nextSeq = 1
		l.last = last
		return l, nil
	}

	// Validate empty-entries case
	if len(data.Entries) == 0 {
		// Stored NextSeq 1 is never written (fresh encodes as 0, trimmed-empty >= 2)
		if data.NextSeq == 1 {
			return nil, fmt.Errorf("load log: empty entries with next_seq 1 unreachable: %w", ErrInvalidData)
		}
		// Trimmed-empty: NextSeq >= 2 is legal
		l.nextSeq = data.NextSeq
		l.last = last
		return l, nil
	}

	// Non-empty: validate contiguity, NextSeq, and per-entry constraints

	// Reject NextSeq 0 on non-empty path (fresh encodes as 0; 0 with entries is unreachable)
	if data.NextSeq == 0 {
		return nil, fmt.Errorf("load log: next seq 0 with entries: %w", ErrInvalidData)
	}

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

	// Validate the audience table: each row a legal, non-empty audience,
	// and no audience stored twice (ToData writes each distinct one once).
	rows := make([][]core.EntityID, len(data.Audiences))
	seenRow := make(map[string]int, len(data.Audiences))
	for i, row := range data.Audiences {
		if len(row) == 0 {
			return nil, fmt.Errorf("load log: audiences[%d] empty (the empty audience is ref 0): %w",
				i, ErrInvalidData)
		}
		if err := validateAudience(row); err != nil {
			return nil, fmt.Errorf("load log: audiences[%d] %v: %w", i, err, ErrInvalidData)
		}
		key := audienceKey(row)
		if prev, ok := seenRow[key]; ok {
			return nil, fmt.Errorf("load log: audiences[%d] repeats audiences[%d]: %w", i, prev, ErrInvalidData)
		}
		seenRow[key] = i
		// One materialised copy per row, shared by every entry naming it:
		// entries are immutable and every read copies out.
		rows[i] = copyAudience(row)
	}

	// Validate per-entry constraints
	for i, ed := range data.Entries {
		// Validate the audience reference (0 = no viewer, n = rows[n-1])
		if int(ed.AudienceRef) > len(rows) {
			return nil, fmt.Errorf("load log: entry[%d] audience %d past the table of %d: %w",
				i, ed.AudienceRef, len(rows), ErrInvalidData)
		}

		// Validate tags (no empty keys)
		if err := validateTags(ed.Tags); err != nil {
			// Include validation detail in message without double-wrapping
			return nil, fmt.Errorf("load log: entry[%d] %v: %w", i, err, ErrInvalidData)
		}

		// Validate payload (not nil)
		if ed.Payload == nil {
			return nil, fmt.Errorf("load log: entry[%d] payload nil: %w", i, ErrInvalidData)
		}
	}

	// Ids: fill legacy ones, then require well-formed and strictly
	// increasing (an id is the order a wire carries).
	ids := make([]ulid, len(data.Entries))
	for i, ed := range data.Entries {
		if ed.ID == "" {
			ids[i] = legacyULID(ed.Seq)
		} else {
			u, err := parseULID(ed.ID)
			if err != nil {
				return nil, fmt.Errorf("load log: entry[%d] %v: %w", i, err, ErrInvalidData)
			}
			ids[i] = u
		}
		if i > 0 && ids[i].String() <= ids[i-1].String() {
			return nil, fmt.Errorf("load log: entry[%d] id %s does not increase: %w",
				i, ids[i].String(), ErrInvalidData)
		}
	}
	if top := ids[len(ids)-1]; top.String() > last.String() {
		last = top
	}

	// All validation passed; deep-copy entries
	entries := make([]entry, len(data.Entries))
	for i, ed := range data.Entries {
		entries[i] = entry{
			seq:         ed.Seq,
			id:          ids[i].String(),
			at:          ed.At,
			correlation: ed.Correlation,
			audience:    resolveAudience(rows, ed.AudienceRef),
			tags:        copyTags(ed.Tags),
			payload:     copyPayload(ed.Payload),
		}
	}

	l.entries = entries
	l.nextSeq = data.NextSeq
	l.last = last
	return l, nil
}

// audienceTable builds LogData.Audiences: each distinct audience once, in
// first-seen order, keyed by the exact ordered id list.
type audienceTable struct {
	rows  [][]core.EntityID
	index map[string]uint32
	// last is the previous entry's audience and its ref: consecutive entries
	// usually share one backing slice (Append and LoadLog share it), so the
	// common case skips building the key.
	last    []core.EntityID
	lastRef uint32
}

// ref returns the 1-based table reference of aud, adding a deep-copied row
// on first sight; the empty audience is ref 0 and takes no row.
func (t *audienceTable) ref(aud []core.EntityID) uint32 {
	if len(aud) == 0 {
		return 0
	}
	if len(t.last) == len(aud) && &t.last[0] == &aud[0] {
		return t.lastRef
	}
	key := audienceKey(aud)
	r, ok := t.index[key]
	if !ok {
		if t.index == nil {
			t.index = make(map[string]uint32)
		}
		t.rows = append(t.rows, copyAudience(aud))
		r = uint32(len(t.rows)) //nolint:gosec // rows never outnumber the retained entries
		t.index[key] = r
	}
	t.last, t.lastRef = aud, r
	return r
}

// audienceKey is the exact ordered id list as a map key; each id is length
// prefixed so no two different lists share a key.
func audienceKey(aud []core.EntityID) string {
	var b strings.Builder
	for _, id := range aud {
		b.WriteString(strconv.Itoa(len(id)))
		b.WriteByte(':')
		b.WriteString(string(id))
	}
	return b.String()
}

// resolveAudience returns the row a validated reference names, nil for 0.
func resolveAudience(rows [][]core.EntityID, ref uint32) []core.EntityID {
	if ref == 0 {
		return nil
	}
	return rows[ref-1]
}
