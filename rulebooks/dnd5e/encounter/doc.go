// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package encounter implements the free-roam encounter composition.
//
// An encounter is a composition with an outcome (Setup → play → Outcome).
// This module is the courier between play/clock, play/intel, play/record,
// and tools/spatial: it surveils percepts into intel, lets deciders act on
// their own intel, and appends the story to record.
// Members exit, encounters close; player activity pumps the clock, the world
// thinks on the tick. A member the rulebook reports down keeps their place on
// the map and in the roster, and stops acting: no turn, no side in a contact,
// no tick action, and a beat in the story saying so.
//
// The composition holds no rules of its own that it could hold instead: two
// capabilities are SUPPLIED at construction and consulted during play, never
// defaulted (rpg-toolkit#1033). InitiativeRoller says what order a fight goes
// in; Standing says who is down. Both are the same move — this module cannot
// import the rulebook (C1), so randomness and hit points are facts it asks for
// rather than facts it knows. Neither has a default answer, because a default
// would be this module quietly deciding a rule it is not allowed to know, and
// both are refused at Setup AND Load rather than guarded where they are used.
//
// Every member is on exactly one clock (R6). The world tick is the default —
// free roam is not a mode, it is where you are when no fight has pulled you
// elsewhere. A fight is a turn bubble: Form pulls its members off the world
// clock into a caller-rolled order (R7 — initiative arrives from outside),
// Transfer moves a straggler in or out mid-round, EndTurn advances the fight,
// and Dissolve re-homes everyone to the tick. A fight also ENDS ITSELF when a
// side runs out of members standing in it — [ByDefeat], with no caller, the
// mirror of sight starting one. Everyone not in the fight keeps
// free-roaming while it runs; everyone in it is the fight's alone — Step,
// Move, Traverse, and Pump are world-clock verbs and will not act for a
// fight member. Which clock somebody is on is always askable, per member, via
// ClockOf.
//
// # Atomicity, and what R5 does and does not promise
//
// Verbs validate before they mutate, and the first validation failure wins
// (R5). That covers the common case completely: a rejected verb never touched
// anything.
//
// It does NOT mean a verb's MUTATE phase is atomic. Join and Exit each perform
// several fallible steps after their first mutation — refreshSight and
// appendBeat both come after placement and member registration — so a failure
// late in a verb can leave the in-memory encounter partially changed.
//
// The clock verbs are the same: Form moves members off the world clock one at a
// time and Dissolve re-homes them one at a time, so a failure mid-verb can
// leave a member between clocks — a state ClockOf reports as a defect rather
// than guessing (see its on-no-clock check).
//
// Record is the same shape for a reason worth naming, since the verb looks
// atomic: it consults Standing AFTER appending its outcome beat, because that
// beat is the cause the consult reads (see [Encounter.Record]). A rulebook that
// cannot answer therefore leaves an outcome recorded and its consequences
// unworked.
//
// That is safe because of how this module is used, not by accident: every
// caller loads, acts, and saves, so a verb returning an error means the
// encounter is DISCARDED UNSAVED and the persisted world is untouched. There
// is no long-lived encounter to repair. Rolling back individual steps would
// buy nothing and would imply an atomicity the mutate phase does not have.
//
// The obligation this places on a caller is the whole of it: on error, drop
// the encounter. Do not save it, and do not keep using it.
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
//     coordinates appear only where this module REPORTS a cell, never in a
//     rule's own logic. That set has grown as the seam converged on one map:
//     the coordinate queries it started as (Atlas, Absolute, Locate), then
//     placement reads and movement beats (#1040), sight payloads (#1044),
//     the pump's monster moves (#1062), and finally the outcome (#1068).
//     Everything a caller is told a cell for now speaks the same one.
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
