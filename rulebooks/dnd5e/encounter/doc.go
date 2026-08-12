// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package encounter implements the free-roam encounter composition.
//
// An encounter is a composition with an outcome (Setup → play → Outcome).
// This module is the courier between play/clock, play/intel, play/record,
// and tools/spatial: it surveils percepts into intel, lets deciders act on
// their own intel, and appends the story to record.
// Members exit, encounters close; player activity pumps the clock, the world
// thinks on the tick.
//
// Design contract: docs/ideas/encounter/design.md (composition laws C1–C8).
// This is not a play/ leaf: the module composes published pieces and exposes
// one aggregate persistence pair at the host seam.
//
// v0.3 anchors every room in one dungeon-absolute space (docs/ideas/
// encounter-anchoring/design.md), governed by five more laws:
//
//   - W1 (one geometry per field) — every room in a field shares the same
//     grid family; a mixed field has no coherent absolute space.
//   - W2 (rooms never overlap) — distinct rooms' absolute footprints are
//     disjoint; touching is legal, sharing a cell is not.
//   - W3 (doorways kiss) — a connection's two endpoints, once anchored to
//     their rooms' origins, are adjacent absolute cells.
//   - W4 (projection is a read) — rules and verbs stay room-local; absolute
//     coordinates appear only in query outputs (Atlas, Absolute, Locate),
//     never in a rule's own logic.
//   - W5 (anchors are construction data) — Origin's LEGALITY (bounds,
//     integrality) is validated identically at Setup and Load, never
//     derived or inferred; PRESENCE is structurally Load-only — Origin is
//     a plain value at Setup (RoomInput) but a pointer at Load
//     (RoomData), so only Load can distinguish a missing Origin from a
//     declared zero one.
//
// Room and field size are allocation-bounded (a legal-but-absurd room or
// field could otherwise demand an allocation Atlas cannot safely make);
// see maxRoomCells/maxFieldCells for the exact figures and why.
package encounter
