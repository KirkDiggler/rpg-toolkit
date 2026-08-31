// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package journal is the world's memory: an append-only log of attributed,
// audience-scoped facts.
//
// It depends on nothing outside the standard library, and it defines the base
// vocabulary — fact, audience, attribution, outcome — that flows up to graph
// and quest. Nothing here interprets a fact. The journal cannot tell a lie
// from the truth; it records who claimed what, in front of whom, and in what
// order. Meaning is the fold's business, one layer up.
//
// Three ideas carry the whole package:
//
//   - A [Fact] is attributed. Every fact names an Actor. There are no
//     anonymous events, so every derived consequence can be traced to someone.
//
//   - A [Fact] is audience-scoped. The [Audience] is the set of entities that
//     witnessed it. This is the entire knowledge model: an entity's beliefs
//     are the fold over the facts it witnessed, so stealth is controlling the
//     audience of your own events and disguise is planting a fact in someone
//     else's feed. There is no belief database anywhere.
//
//   - The log is append-only. A [Journal] hands out copies and offers no way
//     to edit or remove what it holds. Present state is a pure function of the
//     declaration and the facts, which is why nothing downstream stores it.
//
// [Outcome] is the one piece of resolution vocabulary that lives here, because
// facts carry it: a fact records whether the attempt behind it landed. Who
// decides that — and whether a d20, a coin, or a model does the deciding — is
// the composer's seam, one layer up. The journal never learns what got rolled.
package journal
