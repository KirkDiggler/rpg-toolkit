// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package record is the retained story: an append-only, sequence-ordered,
// audience-projected, tag-queryable log of opaque entries. Storage and
// query only — streaming the appends is the host's business; payloads and
// tag vocabularies belong to the composition; record never interprets.
//
// Every entry carries two orders: Seq, the internal gapless sequence, and ID,
// a ULID-style wire id that is an ordering key, not a counter (id.go,
// rpg-project ideas/event-stream R27). Ids draw their time and entropy from
// an IDSource the caller may supply; the log holds no randomness of its own.
//
// SEGMENTS: a store need not rewrite the whole record on every save. A
// segment is the unit a store appends: Log.DataFrom from Since, the first seq
// the store does not yet hold, returns the entries at or after it as a legal
// LogData with its own audience table, built over its own entries only (a
// stored table is never leaned on: any TrimBefore renumbers a rebuilt one).
// The head, DataFrom at NextSeq, carries NextSeq and LastID with no entries.
//
// A log trimmed to a retention window moves its floor, the oldest seq it
// retains. The store keeps the floor of its last save beside the head, drops
// the segments wholly below it, and may keep the one that straddles it.
// JoinData with that Floor drops every entry below it, tolerates what the
// store already dropped, re-interns the kept rows into one table and returns
// what ToData wrote at the save; a seq missing at or above Floor, an overlap
// above it, or a head that disagrees with the last segment is refused by name
// (rpg-project#897).
//
// Design contract: docs/ideas/play/record/design.md (R1–R10). Leaf module:
// depends only on core, takes no context.Context, returns results as
// values, and never publishes.
package record
