// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package record is the retained story: an append-only, sequence-ordered,
// audience-projected, tag-queryable log of opaque entries. Storage and
// query only — streaming the appends is the host's business; payloads and
// tag vocabularies belong to the composition; record never interprets.
//
// Design contract: docs/ideas/play/record/design.md (R1–R10). Leaf module:
// depends only on core, takes no context.Context, returns results as
// values, and never publishes.
package record
