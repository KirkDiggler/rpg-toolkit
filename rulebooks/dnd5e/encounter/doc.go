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
// LOCATION KNOWLEDGE IS ENCOUNTER-OWNED. play/intel stores channel-
// sourced testimony opaquely; this composition gives sight payloads their
// strict Known(position) or Unknown meaning. New payloads are tagged, legacy
// untagged coordinates remain readable as known, and malformed or current-
// unknown sight testimony is refused on load. Other Intel channel payloads
// remain uninterpreted.
//
// A fight-time [MonsterView] keeps current sight in Seen and held known
// locations in Remembered. Remembered members carry no concealed standing or
// reach fact, are never attackable, and have paths ending on the exact
// remembered cell. Held unknown testimony persists but is not actionable. A
// view is rebuilt after each driven move, so a visible-first driver such as
// behavior.Basic can abandon remembered pursuit for new sight on its next
// call.
//
// Only a successful fight-time driver move performs arrival correction: after
// sight refresh, the composition compares the mover's prior held Intel and
// arrival cell with that lawful complete percept. An absent subject remembered
// at the exact cell becomes Held + Unknown without exposing its concealed live
// position. Encounter-owned [IntelDelta] values surface the correction for
// persistence; public Step and free-roam Pump do not independently correct
// location testimony.
//
// The composition holds no rules of its own that it could hold instead: three
// capabilities are SUPPLIED at construction and consulted during play, never
// defaulted (rpg-toolkit#1033). InitiativeRoller says what order a fight goes
// in; Standing says who is down; Sight says how far each member can see. All
// three are the same move — this module cannot import the rulebook (C1), so
// randomness, hit points and light are facts it asks for rather than facts it
// knows. None has a default answer, because a default would be this module
// quietly deciding a rule it is not allowed to know, and all three are refused
// at Setup AND Load rather than guarded where they are used.
//
// Every member is on exactly one clock (R6). The world tick is the default —
// free roam is not a mode, it is where you are when no fight has pulled you
// elsewhere. A fight is a turn bubble: Form pulls its members off the world
// clock into a caller-rolled order (R7 — initiative arrives from outside),
// Transfer moves a straggler in or out mid-round, EndTurn advances the fight,
// and Dissolve re-homes everyone to the tick. A fight also ENDS ITSELF when a
// side runs out of members standing in it — [ByDefeat], with no caller, the
// mirror of sight starting one. Everyone not in the fight keeps
// free-roaming while it runs; everyone in it is the fight's alone — Step and
// Pump are world-clock verbs and will not act for a fight member. Which clock somebody is on is always askable, per member, via
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
// Every cell of a field lives in one dungeon-absolute space (docs/ideas/
// encounter-anchoring/design.md; regions since rpg-project#256, ADR-0044),
// governed by these laws:
//
//   - W1 (one geometry per field) — every field is hex, under ONE declared
//     orientation ([CanvasInput.Orientation], required). The square family
//     left with the room chain (#256): a region is painted on a hex grid,
//     and a second family would be a second frame for every coordinate to
//     be wrong in.
//   - W2 (regions never overlap) — the floor is the union of the regions'
//     cells, and a cell belongs to exactly one region. A cell listed twice,
//     in one region or across two, is refused (ErrRegionOverlap); touching
//     is legal, sharing a cell is not.
//   - W3 (a door edge joins two adjacent floor cells) — every authored
//     edge, wall or door, has both endpoints on the floor and adjacent
//     under the declared orientation (ErrEdgeOffFloor, ErrEdgeNotAdjacent).
//     The envelope is implied, never written: a crossing from floor into
//     void is a crossing nobody can make, and [Void] already says whether
//     sight crosses it.
//   - W4 (projection is a read) — RETIRED by #1106, and worth stating as
//     history because the whole shape of this module used to follow from it.
//     Rules and verbs stayed room-local and absolute coordinates appeared
//     only where the module REPORTED a cell; that reporting set grew until
//     it was everything, at which point the room-local frame underneath had
//     no readers left. #1106 compiled the authored rooms into one canvas;
//     #256 deleted the rooms. There is one frame, and ONE conversion into
//     it: every authored [col,row] pair goes through [HexCellAt] exactly
//     once, at construction (compileField), and no caller ever adds an
//     origin — the room-local seam (#1139) ceased to exist rather than
//     getting fixed.
//   - W5 (world facts are construction data) — a region's archetype and
//     lighting are authored, REQUIRED, carried unread, and persisted; an
//     archetype NEVER decides a mechanic (ErrRegionArchetypeMissing,
//     ErrRegionLightingMissing). Validated identically at Setup and Load
//     through the one shared compileField.
//   - W6 (the field is one canvas) — the floor's bounding box fits in a
//     single origin-centred hex grid, which always widens to hold it.
//
// A REGION IS A NAMED SET OF CELLS (#1108, #256). It is how a dungeon is
// AUTHORED — [RegionInput] lists the cells it owns — and what the runtime
// answers in: [Encounter.RegionAt] says which region holds a cell,
// [Encounter.MembersIn] says who is standing in one, and [Encounter.Atlas]
// reports every region's cells, archetype and lighting beside the flat lists
// of props, walls and doorways. Membership is DERIVED from a member's cell
// wherever it is reported, never stored beside it.
//
// AND THE CANVAS ITSELF IS READABLE (#1114). Everything above DESCRIBES the
// map; [Encounter.Canvas] hands out the map, to read. It is the live room
// rather than a snapshot, so it goes out behind a view that refuses every
// write by name — see its own doc for why a copy is not an option and why a
// silent no-op would be worse than a refusal.
//
// Field size is bounded (maxFieldCells): a region is its cells, so the bound
// is on how many a field may list, which is also the size of the owner map
// this package keeps and the list the Atlas hands out.
package encounter
