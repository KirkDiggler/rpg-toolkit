// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package intel stores per-observer intel: channel-sourced, possibly
// false, possibly stale holdings about opaque subjects.
//
// A holding is a record of three parts: who holds it (the observer), whom
// or what it is about (the subject), and what it is about them (the
// topic). Holder and subject sit at the outside edges; the topic carries
// the relationship. One observer keeps one holding per subject per topic,
// so a ledger clerk can hold where account "acct-7"'s papers are filed (the
// default, empty topic) and that it was signed off (topic "audited")
// without either overwriting the other. The empty topic is the default, and
// a caller that never names one gets one holding per subject. Two testimony
// verbs — Surveil (sustained collection, complete-percept contract) and
// Report (discrete testimony), each scoped to one topic per call — plus read
// queries (HeldBy, About, OnTopic, On) and persistence. The
// module never sees the world and cannot verify anything: illusions,
// disguises, and planted lies are ordinary testimony. Deciders consult
// HeldBy(themselves) and nothing else — monsters act on their intel,
// not the world.
//
// Design contract: docs/ideas/play/intel/design.md (R1–R10). Leaf
// module: depends only on core, no context.Context, deltas returned as
// values, never published.
package intel
