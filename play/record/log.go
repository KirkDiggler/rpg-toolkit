// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Log is the retained story: an append-only, sequence-ordered, audience-projected
// log of opaque entries. It provides storage and query only.
//
// Not safe for concurrent use (design R10). The zero value is not usable;
// construct via NewLog or LoadLog.
type Log struct {
	entries []entry
	nextSeq uint64
}

// entry is the internal representation of a log entry, with deep-copied
// fields and nil-normalized empty containers.
type entry struct {
	seq         uint64
	at          uint64
	correlation string
	audience    []core.EntityID
	tags        map[string]string
	payload     []byte
}

// Entry is the read-side value returned from queries.
type Entry struct {
	// Seq is the monotonic, gapless, strictly-increasing sequence assigned
	// by Append and never renumbered by TrimBefore.
	Seq uint64
	// At is the optional caller-supplied provenance timestamp (e.g., wall
	// clock). The log's order is Seq; At is informational.
	At uint64
	// Correlation is an opaque caller token grouping cause and effects
	// across entries. Empty is legal (an uncorrelated beat).
	Correlation string
	// Audience is the roster of viewers; materialized and duplicate-free.
	// Empty (nil or empty slice) means no viewer — the GM/debug beat.
	// "Everyone" is an explicit roster, never implicit.
	Audience []core.EntityID
	// Tags is the caller-chosen question-surface: queryable metadata.
	// Typical keys: "kind" (e.g., "intel.first_contact", "clock.turn_started"),
	// "observer", "subject", "actor". Record never interprets keys or values;
	// keys MUST be non-empty (enforced at Append). Empty values are legal
	// (flags); nil/empty tags are legal (an unaskable beat).
	Tags map[string]string
	// Payload is opaque composition-encoded story beats (leaf deltas, outcomes).
	// Record never interprets.
	Payload []byte
}

// AppendInput is the input to the Append verb.
type AppendInput struct {
	// At is the optional caller-supplied provenance timestamp.
	At uint64
	// Correlation is an opaque caller token grouping cause and effects.
	Correlation string
	// Audience is the roster of viewers. MUST be duplicate-free with no empty
	// IDs. Empty-non-nil is legal and normalized to nil on store.
	Audience []core.EntityID
	// Tags is the caller-chosen question-surface. Keys MUST be non-empty;
	// values may be anything including empty. Empty-non-nil is legal and
	// normalized to nil on store.
	Tags map[string]string
	// Payload is opaque composition-encoded story beats. MUST be non-nil
	// (empty non-nil is legal).
	Payload []byte
}

// AppendOutput is the output of the Append verb.
type AppendOutput struct {
	// Seq is the assigned sequence number, monotonic and gapless from 1.
	Seq uint64
}

// SliceForInput is the input to the SliceFor query.
type SliceForInput struct {
	// Viewer is the audience member whose story is requested. MUST be non-empty.
	Viewer core.EntityID
	// FromSeq is the inclusive lower bound of sequences to return.
	FromSeq uint64
	// Tags is an optional AND-exact-match filter. Nil/empty = no filter; keys
	// must be non-empty. Every tag key must be present in the entry with exactly
	// the given value; an entry with nil tags fails any non-empty filter.
	Tags map[string]string
}

// AllInput is the input to the All query.
type AllInput struct {
	// FromSeq is the inclusive lower bound of sequences to return.
	FromSeq uint64
	// Tags is an optional AND-exact-match filter. Nil/empty = no filter; keys
	// must be non-empty. Every tag key must be present in the entry with exactly
	// the given value; an entry with nil tags fails any non-empty filter.
	Tags map[string]string
}

// TrimBeforeInput is the input to the TrimBefore verb.
type TrimBeforeInput struct {
	// Seq is the exclusive upper bound of sequences to remove.
	// Entries with Seq < in.Seq are dropped. Trimming at or below the oldest
	// retained Seq is a no-op; in.Seq == NextSeq is legal and empties the log;
	// in.Seq > NextSeq errors ErrBadSeq.
	Seq uint64
}

// TrimBeforeOutput is the output of the TrimBefore verb.
type TrimBeforeOutput struct {
	// Removed is the count of entries dropped by this call.
	Removed int
}

// NewLog returns a fresh Log ready for use.
// Returns (*Log, error) per family law (cannot fail today; error slot reserved).
func NewLog() (*Log, error) {
	return &Log{nextSeq: 1}, nil
}

// Append appends one immutable entry to the log, assigning the next Seq.
// Validates per the design's Append verb, first-failure-wins in order:
// nil guard, Audience (no empty IDs or duplicates), Tags (no empty keys),
// Payload (not nil, empty non-nil legal). On error, no state changed (R5).
// Audience and tags are defensively copied, with empty-non-nil normalized
// to nil on store (family nil-container convention).
func (l *Log) Append(in *AppendInput) (*AppendOutput, error) {
	// Step 1: nil guard
	if in == nil {
		return nil, fmt.Errorf("append: %w", ErrNilInput)
	}

	// Step 2: validate audience (no empty IDs, no duplicates)
	if err := validateAudience(in.Audience); err != nil {
		return nil, fmt.Errorf("append: %w", err)
	}

	// Step 3: validate tags (no empty keys)
	if err := validateTags(in.Tags); err != nil {
		return nil, fmt.Errorf("append: %w", err)
	}

	// Step 4: validate payload (must be non-nil)
	if in.Payload == nil {
		return nil, fmt.Errorf("append: %w", ErrNoPayload)
	}

	// All validation passed; now mutate state (R5 atomicity)
	seq := l.nextSeq
	l.nextSeq++

	// Deep copy and normalize audience
	var aud []core.EntityID
	if len(in.Audience) > 0 {
		aud = make([]core.EntityID, len(in.Audience))
		copy(aud, in.Audience)
	}

	// Deep copy and normalize tags
	var tgs map[string]string
	if len(in.Tags) > 0 {
		tgs = make(map[string]string, len(in.Tags))
		for k, v := range in.Tags {
			tgs[k] = v
		}
	}

	// Copy payload
	p := make([]byte, len(in.Payload))
	copy(p, in.Payload)

	// Append entry
	l.entries = append(l.entries, entry{
		seq:         seq,
		at:          in.At,
		correlation: in.Correlation,
		audience:    aud,
		tags:        tgs,
		payload:     p,
	})

	return &AppendOutput{Seq: seq}, nil
}

// NextSeq returns the sequence number the next Append will assign.
// Zero-arg read, never errs today; the error slot is the law's.
func (l *Log) NextSeq() (uint64, error) {
	return l.nextSeq, nil
}

// TrimBefore drops all entries with Seq < in.Seq. Retention is the
// composition's policy made visible (design brainstorm §4). Trimming at or
// below the oldest retained Seq is a no-op (Removed: 0), not an error;
// in.Seq == NextSeq is legal and empties the log; in.Seq > NextSeq errors
// ErrBadSeq (you cannot forget the future). Never renumbers entries.
// Nil guard → ErrNilInput; bad Seq → ErrBadSeq (R5 atomicity: on error, no
// state changed).
func (l *Log) TrimBefore(in *TrimBeforeInput) (*TrimBeforeOutput, error) {
	// Step 1: nil guard
	if in == nil {
		return nil, fmt.Errorf("trim before: %w", ErrNilInput)
	}

	// Step 2: validate Seq (cannot trim beyond NextSeq)
	if in.Seq > l.nextSeq {
		return nil, fmt.Errorf("trim before: seq %d beyond next %d: %w", in.Seq, l.nextSeq, ErrBadSeq)
	}

	// All validation passed; find the cut point (first entry with seq >= in.Seq)
	cut := len(l.entries) // default: all entries are removed (if log is empty, cut stays 0)
	for i, e := range l.entries {
		if e.seq >= in.Seq {
			cut = i
			break
		}
	}

	// Count how many entries will be removed
	removed := cut

	// Trim entries by re-slicing; defensively copy the tail to avoid holding
	// the old backing array in memory
	if cut < len(l.entries) {
		l.entries = append([]entry(nil), l.entries[cut:]...)
	} else {
		l.entries = nil
	}

	return &TrimBeforeOutput{Removed: removed}, nil
}

// All returns every retained entry with Seq >= in.FromSeq matching the optional
// tag filter, in Seq order. This is the GM/debug/host view.
// Nil guard → ErrNilInput; filter-key validation → ErrBadTag. Copy-out: returned
// entries MUST NOT alias internal state (nil stays nil, non-nil is deep-copied).
func (l *Log) All(in *AllInput) ([]Entry, error) {
	if in == nil {
		return nil, fmt.Errorf("all: %w", ErrNilInput)
	}

	// Validate filter keys (no empty keys)
	if err := validateTags(in.Tags); err != nil {
		return nil, fmt.Errorf("all: tag key: %w", err)
	}

	var result []Entry
	for _, e := range l.entries {
		if e.seq >= in.FromSeq && matchTags(e.tags, in.Tags) {
			result = append(result, copyEntryOut(e))
		}
	}
	return result, nil
}

// SliceFor returns entries with Seq >= in.FromSeq whose audience contains Viewer
// AND which carry every given tag key with exactly the given value (AND semantics;
// nil/empty Tags = no filter). This is the reconnect/replay call for a specific viewer.
// Validation first-failure-wins: nil → ErrNilInput, empty viewer → ErrNoViewer,
// filter keys non-empty → ErrBadTag. Copy-out: returned entries MUST NOT alias
// internal state (nil stays nil, non-nil is deep-copied).
func (l *Log) SliceFor(in *SliceForInput) ([]Entry, error) {
	if in == nil {
		return nil, fmt.Errorf("slice for: %w", ErrNilInput)
	}

	if in.Viewer == "" {
		return nil, fmt.Errorf("slice for: %w", ErrNoViewer)
	}

	// Validate filter keys (no empty keys)
	if err := validateTags(in.Tags); err != nil {
		return nil, fmt.Errorf("slice for: tag key: %w", err)
	}

	var result []Entry
	for _, e := range l.entries {
		if e.seq >= in.FromSeq && containsViewer(e.audience, in.Viewer) && matchTags(e.tags, in.Tags) {
			result = append(result, copyEntryOut(e))
		}
	}
	return result, nil
}

// matchTags checks whether every tag key in the filter is present in entryTags
// with exactly the given value. Empty filter → true (no filter means everything matches).
// Note: an entry with nil tags fails any non-empty filter.
func matchTags(entryTags, filter map[string]string) bool {
	// No filter means everything matches
	if len(filter) == 0 {
		return true
	}

	// Every filter key must be present in entryTags with exactly the value
	for k, v := range filter {
		if entryTags[k] != v {
			return false
		}
	}
	return true
}

// containsViewer checks whether the audience slice contains the given viewer.
func containsViewer(audience []core.EntityID, viewer core.EntityID) bool {
	for _, id := range audience {
		if id == viewer {
			return true
		}
	}
	return false
}

// copyEntryOut converts an internal entry to an exported Entry with deep copies.
func copyEntryOut(e entry) Entry {
	return Entry{
		Seq:         e.seq,
		At:          e.at,
		Correlation: e.correlation,
		Audience:    copyAudience(e.audience),
		Tags:        copyTags(e.tags),
		Payload:     copyPayload(e.payload),
	}
}

// validateAudience checks that audience is duplicate-free with no empty IDs.
func validateAudience(aud []core.EntityID) error {
	seen := make(map[core.EntityID]bool)
	for _, id := range aud {
		if id == "" {
			return fmt.Errorf("audience: %w", ErrBadAudience)
		}
		if seen[id] {
			return fmt.Errorf("audience: duplicate %q: %w", id, ErrBadAudience)
		}
		seen[id] = true
	}
	return nil
}

// validateTags checks that all keys are non-empty.
func validateTags(tags map[string]string) error {
	for k := range tags {
		if k == "" {
			return fmt.Errorf("tag key: empty: %w", ErrBadTag)
		}
	}
	return nil
}

// copyAudience returns a deep copy of the audience, or nil if empty.
func copyAudience(aud []core.EntityID) []core.EntityID {
	if len(aud) == 0 {
		return nil
	}
	result := make([]core.EntityID, len(aud))
	copy(result, aud)
	return result
}

// copyTags returns a deep copy of the tags map, or nil if empty.
func copyTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	result := make(map[string]string, len(tags))
	for k, v := range tags {
		result[k] = v
	}
	return result
}

// copyPayload returns a deep copy of the payload bytes.
func copyPayload(p []byte) []byte {
	if len(p) == 0 {
		return make([]byte, 0)
	}
	result := make([]byte, len(p))
	copy(result, p)
	return result
}
