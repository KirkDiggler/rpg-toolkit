// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// DataFromInput is the input to [Log.DataFrom].
type DataFromInput struct {
	// Since is the first seq the caller wants: for a store, the first seq it
	// does not hold yet. Zero (or anything at or below the oldest retained
	// seq) is every retained entry.
	Since uint64
}

// DataFrom returns the persistent representation of the retained entries at
// or after in.Since: a SEGMENT, the unit a store appends (doc.go).
//
// The result is a legal LogData on its own. Its audience table is built
// first-seen over its own entries only, so it never leans on a table stored
// before it. NextSeq is the Log's, and LastID is written exactly as ToData
// writes it relative to the result's own entries: whenever no entry in the
// result carries the last id issued. The fresh-log convention holds: a
// never-appended log returns the zero LogData (but for a minted LastID).
//
// Entries older than Since are never copied. Since at or below the oldest
// retained seq returns every retained entry (Since 0 is ToData); Since at or
// above NextSeq returns no entries, which is the HEAD a store keeps beside
// its segments (JoinData). A nil input is refused with ErrNilInput.
func (l *Log) DataFrom(in *DataFromInput) (LogData, error) {
	if in == nil {
		return LogData{}, ErrNilInput
	}
	return l.dataFrom(in.Since), nil
}

// dataFrom is DataFrom past its input check: the one body behind DataFrom
// and ToData.
func (l *Log) dataFrom(since uint64) LogData {
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

// JoinDataInput is the input to [JoinData].
type JoinDataInput struct {
	// Head is the newest DataFrom at NextSeq: NextSeq and LastID, no entries,
	// no table.
	Head LogData
	// Floor is the seq below which the store may hold nothing: the oldest seq
	// the log still retained at the last save (its trim floor; NextSeq when
	// it retained none). Entries below Floor are dropped from every segment,
	// and the store may already have dropped them. Zero (or 1) keeps
	// everything and tolerates no missing seq.
	Floor uint64
	// Segments are the segments the store holds, oldest first: DataFrom
	// outputs. Empty segments (a save that appended nothing) are skipped.
	Segments []LogData
}

// JoinData assembles one LogData from the head and segments a store holds.
// The result is what ToData wrote at the last save: the entries at or above
// Floor, one audience table re-interned first-seen over them (each segment
// row is remapped once, not per entry), the head's NextSeq, and the head's
// LastID unless the last joined entry carries it.
//
// THE STORE TRIMS, THE JOIN TOLERATES. A log trims to its retention window
// on save, so a segment can start above the store's last seq + 1 (a verb
// appended more than the window), and the store drops segments wholly below
// the floor while the one that straddles it keeps older entries. Below Floor
// the join drops what it is handed and forgives what is missing; at or above
// Floor every seq through NextSeq-1 must be present.
//
// JoinData refuses, wrapping ErrInvalidData and naming the reason: a head
// with entries or a table; a Floor past the head's NextSeq; a segment whose
// entries are not gapless, whose NextSeq is not its last seq + 1, whose table
// holds an empty, illegal or repeated row, or whose entry names a row past
// its table; a missing seq at or above Floor (gap); a segment that overlaps
// the one before it at or above Floor; a head NextSeq that is not the last
// joined seq + 1; a head LastID older than the last joined id. A nil input is
// refused with ErrNilInput.
//
// The result's entries share Tags and Payload with the segments; LoadLog
// deep-copies them, and so must any other caller that keeps the segments.
func JoinData(in *JoinDataInput) (LogData, error) {
	if in == nil {
		return LogData{}, ErrNilInput
	}
	head := in.Head
	if len(head.Entries) > 0 {
		return LogData{}, fmt.Errorf("join data: head holds %d entries, want none: %w",
			len(head.Entries), ErrInvalidData)
	}
	if len(head.Audiences) > 0 {
		return LogData{}, fmt.Errorf("join data: head holds %d audience rows, want none: %w",
			len(head.Audiences), ErrInvalidData)
	}
	// The first seq that must be present: everything below it may be gone.
	floor := max(in.Floor, 1)
	next := max(head.NextSeq, 1) // fresh encodes NextSeq 0
	if floor > next {
		return LogData{}, fmt.Errorf("join data: floor %d is past the head's next_seq %d: %w",
			in.Floor, next, ErrInvalidData)
	}

	total := 0
	for si, seg := range in.Segments {
		if len(seg.Entries) == 0 {
			continue
		}
		if err := checkSegment(si, seg); err != nil {
			return LogData{}, err
		}
		total += len(seg.Entries)
	}

	out := LogData{NextSeq: head.NextSeq, Entries: make([]EntryData, 0, total)}
	var table audienceTable
	want := floor // the next seq the join needs
	for si, seg := range in.Segments {
		kept := seg.Entries
		for len(kept) > 0 && kept[0].Seq < want {
			// Below the floor, or already joined: an overlap above the
			// floor is refused, one below it is the straddler's past.
			if kept[0].Seq >= floor {
				return LogData{}, fmt.Errorf("join data: segment[%d] repeats seq %d (overlap): %w",
					si, kept[0].Seq, ErrInvalidData)
			}
			kept = kept[1:]
		}
		if len(kept) == 0 {
			continue
		}
		if kept[0].Seq > want {
			return LogData{}, fmt.Errorf("join data: segment[%d] starts at seq %d, seq %d is missing at or above floor %d (gap): %w",
				si, kept[0].Seq, want, floor, ErrInvalidData)
		}
		remap := make([]uint32, len(seg.Audiences)) // 0 = not yet interned
		for _, ed := range kept {
			if ed.AudienceRef != 0 {
				r := &remap[ed.AudienceRef-1]
				if *r == 0 {
					*r = table.ref(seg.Audiences[ed.AudienceRef-1])
				}
				ed.AudienceRef = *r
			}
			out.Entries = append(out.Entries, ed)
		}
		want = kept[len(kept)-1].Seq + 1
	}

	if len(out.Entries) == 0 {
		if want != next {
			return LogData{}, fmt.Errorf("join data: seqs %d through %d are missing at or above floor %d (gap): %w",
				want, next-1, floor, ErrInvalidData)
		}
		return head, nil
	}
	if head.NextSeq != want {
		return LogData{}, fmt.Errorf("join data: head next_seq %d != last joined seq+1 (%d): %w",
			head.NextSeq, want, ErrInvalidData)
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
