// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package contributions holds the inert values a rule answers with and the
// frame it answers from. It contains no content registry, event bus,
// character, randomness or lifecycle.
//
// A [Frame] holds typed facts, each known or unknown; unknown is never read as
// false, and a known false, zero or empty value stays known. An
// [ActionAssessor] reads only the frame and its own persisted state and
// returns an [Answer]: applies, does not apply, or depends, with its reason
// and what it contributes as data. Execution and information ask the same
// rule; at execution a Depends answer fails the action with an error wrapping
// [ErrRuleCannotAnswer].
//
// A frame also says what members hold and what they see. [Frame.Held] lists
// each known member's conditions; a member it does not list is unknown, never
// one holding nothing. [PairFacts.Sees] says its From member can see its To
// member. A rule for an effect a target holds reads these, keyed by the
// condition's reference, and contributes advantage or disadvantage as an
// [AttackMode] on its answer.
//
// An [Effect] is one row of information about an action. Rows are listed, not
// folded: nothing here produces a total, and no row grants or refuses an
// action.
//
// Source and unresolved dice values also underlie the event-facing aliases, so
// information and resolved traces share one identity vocabulary.
package contributions
