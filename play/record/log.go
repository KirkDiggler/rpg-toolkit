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

// AllInput is the input to the All query.
type AllInput struct {
	// FromSeq is the inclusive lower bound of sequences to return.
	FromSeq uint64
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
		return nil, fmt.Errorf("%w", ErrNilInput)
	}

	// Step 2: validate audience (no empty IDs, no duplicates)
	if err := l.validateAudience(in.Audience); err != nil {
		return nil, err
	}

	// Step 3: validate tags (no empty keys)
	if err := l.validateTags(in.Tags); err != nil {
		return nil, err
	}

	// Step 4: validate payload (must be non-nil)
	if in.Payload == nil {
		return nil, fmt.Errorf("%w", ErrNoPayload)
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

// All returns every retained entry with Seq >= in.FromSeq, in Seq order.
// Nil guard → ErrNilInput. Copy-out: returned entries MUST NOT alias
// internal state (nil stays nil, non-nil is deep-copied).
func (l *Log) All(in *AllInput) ([]Entry, error) {
	if in == nil {
		return nil, fmt.Errorf("%w", ErrNilInput)
	}

	var result []Entry
	for _, e := range l.entries {
		if e.seq >= in.FromSeq {
			result = append(result, l.entryToCopy(e))
		}
	}
	return result, nil
}

// entryToCopy converts an internal entry to an exported Entry with deep copies.
func (l *Log) entryToCopy(e entry) Entry {
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
func (l *Log) validateAudience(aud []core.EntityID) error {
	seen := make(map[core.EntityID]bool)
	for _, id := range aud {
		if id == "" {
			return fmt.Errorf("%w", ErrBadAudience)
		}
		if seen[id] {
			return fmt.Errorf("%w", ErrBadAudience)
		}
		seen[id] = true
	}
	return nil
}

// validateTags checks that all keys are non-empty.
func (l *Log) validateTags(tags map[string]string) error {
	for k := range tags {
		if k == "" {
			return fmt.Errorf("%w", ErrBadTag)
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
