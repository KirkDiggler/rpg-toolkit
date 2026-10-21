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
// Design contract: docs/ideas/play/record/design.md (R1–R10). Leaf module:
// depends only on core, takes no context.Context, returns results as
// values, and never publishes.
package record
