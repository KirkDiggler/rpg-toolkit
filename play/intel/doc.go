// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package intel stores per-observer intel: channel-sourced, possibly
// false, possibly stale holdings about opaque subjects. Two testimony
// verbs — Surveil (sustained collection, complete-percept contract) and
// Report (discrete testimony) — plus read queries and persistence. The
// module never sees the world and cannot verify anything: illusions,
// disguises, and planted lies are ordinary testimony. Deciders consult
// HeldBy(themselves) and nothing else — monsters act on their intel,
// not the world.
//
// Design contract: docs/ideas/play/intel/design.md (R1–R10). Leaf
// module: depends only on core, no context.Context, deltas returned as
// values, never published.
package intel
