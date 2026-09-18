// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package behavior is the module root, and holds nothing.
//
// What is left of this module is the DEEDS CHANNEL and the two packages that
// own it: [deed] for what a deed is and how it is encoded, and [stage] for
// landing one on the people who witnessed it. Both are about what a creature
// COMES TO HOLD. Neither decides anything.
//
// # What was here, and where it went
//
// This package was the mind: a [Mind] of four judgments — which holdings are
// one thing, what to call it, which to deal with first, how close to let it
// get — and a fixed ladder over them that answered what a creature does with
// its turn. It is deleted (rpg-project#465, ideas/creature-table/design.md
// §7), not deprecated and not left beside.
//
// A creature decides by rolling on an authored weighted table now, and four
// things load that die: the rulebook's default table for its kind, the
// author's orders, its own temperament, and what it has seen and suffered.
// The reason is not that the ladder was wrong. It is that the ladder was a
// black box with three words on the lid and a streamer could not open it,
// while a table is a tool an author can hold — which is the product.
//
// What the mind got right is kept, and is read from the table rather than
// reimplemented: the outcome of anything is TESTIMONY a creature holds and
// never a flag, and who a creature believes is where comes from perception,
// per observer. That is exactly what [deed] and [stage] still serve.
//
// So the vocabulary for reasoning about what a creature holds went with the
// ladder that read it — Contact, Reading, Reader, Name, Self, Situation,
// Space, Verb, Intent, and the Game that composed them. Nothing imported them
// but the ladder and the presets, and a projection of holdings belongs beside
// whoever is deciding from them: the composition builds its own, per pick,
// against a board it actually has. Two projections of one truth would be two
// truths to keep in step.
//
// # The arrow still points one way
//
// This module depends on mind/perception and perception never learns anything
// about it (R1). What is left of that layering is [stage.Land], which writes
// through the store's own door and runs no pass of its own.
//
// Design contract: docs/ideas/mind/behavior/design.md, R1–R13 — R7 (the
// ladder) and R11 (the Space) are retired with the code they bound; R2 (a
// payload is the caller's to read, except the deeds channel this module
// wrote) and R9 (a deed lands in each witness's own terms) are what [deed] and
// [stage] still keep.
package behavior
