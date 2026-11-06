// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package behavior composes independent behavior instances and evaluates
// authored weighted policy. The composer uses mind/machine for private progress;
// the table remains the real selector, with the vocabulary supplied by its caller.
//
// # Composition
//
// [NewComposer] takes materialized [InstanceSpec] values in authored priority
// order. Each instance owns a machine blob. [Composer.Observe] delivers data-only
// inputs, broadcast or addressed to one instance; an action request is refused,
// not silently dropped. [Composer.Decide] previews the instances and rolls only
// the first requesting one's table. Losing requests retain their progress.
// Data-only transitions can commit alongside the winner. A no-candidate hold
// retains its pending step; an explicitly selected Hold row can progress.
//
// Results are values. The caller executes the selected word and sends completion
// back to its owning instance, rather than rolling again or broadcasting another
// behavior's completion. No geometry, actor registry, bus or clock is added here.
// Input time remains the caller's; retained progress is not a virtual paused clock.
//
// [Composer.ToData] saves private progress plus configuration identity. A
// [LoadComposer] caller supplies the original materialized configuration; changed
// definitions/order and malformed progress are refused, never silently rebound.
// [CheckParameters] validates producer-supplied scalar values from mechanical
// metadata. It does not bind templates, sample values or define a quality profile.
//
// # Bind-time finite sequences
//
// [ExpandSequence] is a producer utility, separate from runtime execution. It
// expands a [SequenceDefinition]'s data-authored recipe/table body over finite
// supplied items and returns an ordinary [InstanceSpec], a detached vocabulary,
// and completion/source mappings. Item-local state, table and bag references are
// qualified. The existing scalar checker validates named values, and the real
// composer validates the result. Binding never rolls or runs a fact reader.
// Catalogs, defaults, domain/coordinate checks and actual completion delivery
// stay with the producer/caller. This introduces no new studio source field,
// runtime interpreter or asynchronous completion/deduplication protocol.
//
// # Policy and testimony
//
// mind/perception says what a creature KNOWS. This module says what it DOES
// with that — by rolling on an authored weighted table, loaded by the
// creature's temperament and filtered by what it has seen and suffered. A
// rulebook supplies the facts and executes the words. Neither of the two
// modules knows a game.
//
//	[Pick] — a table, a temperament, some facts and a die, in;
//	         the entry that fired and the whole arithmetic, out.
//	         An entry is on the table when its condition holds AND the
//	         creature can afford it; either way it is absent, not weighted
//	         zero, so the candidate list is the honest account of what could
//	         have happened.
//	[Deal] — a faction's spread and a die, in; one creature's nerve, out.
//
// The deeds channel is the other half, and it is what a table's `when` reads:
// [deed] for what a deed is and how it is encoded, and [stage] for landing one
// on the people who witnessed it. [sound] is its sibling for noise: a kind, a
// place and a time landed on whoever heard it, and never who made it. All
// three are about what a creature COMES TO HOLD. None decides anything.
//
// # It knows no game, which is the point
//
// This module holds no board, no clock, no rulebook rules and supplies no
// behavioral policy of its own; the instance order and tables are authored.
// It names exactly one trigger, [KeyTime] — a creature
// having time is the one thing every game has — and every other key is the
// caller's to name, seal and refuse. It seals no temperament vocabulary: any
// word that arrives with a profile is multiplied by it. It carries a
// [Selector] and resolves nothing; it carries a cell and never reads it.
//
// A spoken answer may also carry an opaque [ActivityCue]. This primitive
// preserves the selected leaf's reference and positive sustain count, but
// resolves no assets, runs no animation clock and changes no roll arithmetic.
// The caller captures it with the spoken answer; presentation owns playback.
//
// That is the composability claim being tested. The table came out of a D&D
// composition, and what was D&D about it — building the facts from a board,
// executing the words through a striker and a router, the authoring dialect,
// the persistence, the beats — all stayed there. What is here is what any
// rulebook would have had to write for itself.
//
// # What was here, and where it went
//
// This package was ALSO the mind: a Mind of four judgments — which holdings are
// one thing, what to call it, which to deal with first, how close to let it
// get — and a fixed ladder over them that answered what a creature does with
// its turn. It is deleted (rpg-project#465, ideas/creature-table/design.md
// §7), not deprecated and not left beside.
//
// A creature decides by rolling on an authored weighted table now — the one
// above — and four things load that die: the rulebook's default table for its
// kind, the author's orders, its own temperament, and what it has seen and
// suffered. The reason is not that the ladder was wrong. It is that the ladder
// was a black box with three words on the lid and a streamer could not open
// it, while a table is a tool an author can hold — which is the product.
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
// [stage] still keep. The table is rpg-project#465,
// ideas/creature-table/design.md, ruled by Kirk 2026-09-18: "that it is not
// dnd specific makes it a welcome addition to the mind package which can be
// used in any rulebook and is good validation of its composability."
package behavior
