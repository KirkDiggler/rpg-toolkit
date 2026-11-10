// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// DataFrom returns the persistent representation of the retained entries at
// or after since: a SEGMENT, the unit a store appends (doc.go).
//
// The result is a legal LogData on its own. Its audience table is built
// first-seen over its own entries only, so it never leans on a table stored
// before it. NextSeq is the Log's, and LastID is written exactly as ToData
// writes it relative to the result's own entries: whenever no entry in the
// result carries the last id issued. The fresh-log convention holds: a
// never-appended log returns the zero LogData (but for a minted LastID).
//
// Entries older than since are never copied. since at or below the oldest
// retained seq returns every retained entry (DataFrom(0) is ToData); since at
// or above NextSeq returns no entries, which is the HEAD a store keeps beside
// its segments (JoinData).
func (l *Log) DataFrom(since uint64) LogData {
	from := l.indexOf(since)
	entries := l.entries[from:]

	// The last id issued is written whenever no entry in the result carries it.
	var lastID string
	if !l.last.isZero() && (len(entries) == 0 || l.last.String() > entries[len(entries)-1].id) {
		lastID = l.last.String()
	}

	// Fresh log: nextSeq == 1 && no entries → zero LogData (but for an id
	// minted before any append, which must survive the load)
	if l.nextSeq == 1 && len(entries) == 0 {
		return LogData{LastID: lastID}
	}

	// Populated or trimmed-empty: store the real NextSeq and deep-copied entries
	data := LogData{
		NextSeq: l.nextSeq,
		LastID:  lastID,
	}

	if len(entries) > 0 {
		var table audienceTable
		data.Entries = make([]EntryData, len(entries))
		for i, e := range entries {
			data.Entries[i] = EntryData{
				Seq:         e.seq,
				ID:          e.id,
				At:          e.at,
				Correlation: e.correlation,
				AudienceRef: table.ref(e.audience),
				Tags:        copyTags(e.tags),
				Payload:     copyPayload(e.payload),
			}
		}
		data.Audiences = table.rows
	}

	return data
}

// indexOf returns the index of the first retained entry with seq >= since.
// Retained seqs are gapless, so it is arithmetic, not a search.
func (l *Log) indexOf(since uint64) int {
	if len(l.entries) == 0 || since <= l.entries[0].seq {
		return 0
	}
	off := since - l.entries[0].seq
	if off >= uint64(len(l.entries)) {
		return len(l.entries)
	}
	return int(off) //nolint:gosec // bounded by len(l.entries) above
}

// JoinData assembles one LogData from a head and the segments a store holds,
// oldest first. The result is what ToData would have written for a log
// holding the segments' entries: one audience table, re-interned first-seen
// over the joined entries (each segment row is remapped once, not per entry),
// the head's NextSeq, and the head's LastID unless the last joined entry
// carries it.
//
// The head is the newest DataFrom(NextSeq): NextSeq and LastID, no entries,
// no table. A segment is a DataFrom output. Empty segments (a save that
// appended nothing) are skipped.
//
// JoinData refuses, wrapping ErrInvalidData and naming the reason: a head
// with entries or a table; a segment whose entries are not gapless, whose
// NextSeq is not its last seq + 1, whose table holds an empty, illegal or
// repeated row, or whose entry names a row past its table; a segment that
// leaves a gap after, or overlaps, the one before it; a head NextSeq that is
// not the last joined seq + 1; a head LastID older than the last joined id.
//
// The result's entries share Tags and Payload with the segments; LoadLog
// deep-copies them, and so must any other caller that keeps the segments.
func JoinData(head LogData, segments ...LogData) (LogData, error) {
	if len(head.Entries) > 0 {
		return LogData{}, fmt.Errorf("join data: head holds %d entries, want none: %w",
			len(head.Entries), ErrInvalidData)
	}
	if len(head.Audiences) > 0 {
		return LogData{}, fmt.Errorf("join data: head holds %d audience rows, want none: %w",
			len(head.Audiences), ErrInvalidData)
	}

	total := 0
	for _, seg := range segments {
		total += len(seg.Entries)
	}
	if total == 0 {
		return head, nil
	}

	out := LogData{NextSeq: head.NextSeq, Entries: make([]EntryData, 0, total)}
	var table audienceTable
	var prev uint64 // last joined seq, 0 before the first
	for si, seg := range segments {
		if len(seg.Entries) == 0 {
			continue
		}
		if err := checkSegment(si, seg); err != nil {
			return LogData{}, err
		}
		first := seg.Entries[0].Seq
		if prev != 0 && first > prev+1 {
			return LogData{}, fmt.Errorf("join data: segment[%d] starts at seq %d after seq %d (gap): %w",
				si, first, prev, ErrInvalidData)
		}
		if prev != 0 && first <= prev {
			return LogData{}, fmt.Errorf("join data: segment[%d] starts at seq %d at or before seq %d (overlap): %w",
				si, first, prev, ErrInvalidData)
		}
		remap := make([]uint32, len(seg.Audiences)) // 0 = not yet interned
		for _, ed := range seg.Entries {
			if ed.AudienceRef != 0 {
				r := &remap[ed.AudienceRef-1]
				if *r == 0 {
					*r = table.ref(seg.Audiences[ed.AudienceRef-1])
				}
				ed.AudienceRef = *r
			}
			out.Entries = append(out.Entries, ed)
		}
		prev = seg.Entries[len(seg.Entries)-1].Seq
	}

	if head.NextSeq != prev+1 {
		return LogData{}, fmt.Errorf("join data: head next_seq %d != last joined seq+1 (%d+1=%d): %w",
			head.NextSeq, prev, prev+1, ErrInvalidData)
	}
	lastEntryID := out.Entries[len(out.Entries)-1].ID
	switch {
	case head.LastID == "" || head.LastID == lastEntryID:
	case head.LastID < lastEntryID:
		return LogData{}, fmt.Errorf("join data: head last_id %s is older than the last joined id %s: %w",
			head.LastID, lastEntryID, ErrInvalidData)
	default:
		out.LastID = head.LastID
	}
	out.Audiences = table.rows
	return out, nil
}

// checkSegment validates the parts of segment si that JoinData relies on:
// gapless seqs, NextSeq after the last, a legal table, refs inside it.
// LoadLog validates the rest of each entry on the joined result.
func checkSegment(si int, seg LogData) error {
	for i, ed := range seg.Entries {
		if ed.Seq == 0 {
			return fmt.Errorf("join data: segment[%d] entry[%d] seq 0: %w", si, i, ErrInvalidData)
		}
		if i > 0 && ed.Seq != seg.Entries[i-1].Seq+1 {
			return fmt.Errorf("join data: segment[%d] entry[%d] seq %d not contiguous (prev %d): %w",
				si, i, ed.Seq, seg.Entries[i-1].Seq, ErrInvalidData)
		}
		if int(ed.AudienceRef) > len(seg.Audiences) {
			return fmt.Errorf("join data: segment[%d] entry[%d] audience %d past the table of %d: %w",
				si, i, ed.AudienceRef, len(seg.Audiences), ErrInvalidData)
		}
	}
	if last := seg.Entries[len(seg.Entries)-1].Seq; seg.NextSeq != last+1 {
		return fmt.Errorf("join data: segment[%d] next_seq %d != last_seq+1 (%d+1=%d): %w",
			si, seg.NextSeq, last, last+1, ErrInvalidData)
	}
	seen := make(map[string]int, len(seg.Audiences))
	for i, row := range seg.Audiences {
		if err := checkRow(row); err != nil {
			return fmt.Errorf("join data: segment[%d] audiences[%d] %v: %w", si, i, err, ErrInvalidData)
		}
		key := audienceKey(row)
		if p, ok := seen[key]; ok {
			return fmt.Errorf("join data: segment[%d] audiences[%d] repeats audiences[%d]: %w",
				si, i, p, ErrInvalidData)
		}
		seen[key] = i
	}
	return nil
}

// checkRow reports why a stored audience row is illegal, or nil.
func checkRow(row []core.EntityID) error {
	if len(row) == 0 {
		return fmt.Errorf("empty (the empty audience is ref 0)")
	}
	return validateAudience(row)
}
