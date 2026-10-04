// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package contributions holds inert facts and source-bearing rule decisions
// shared by information and execution. It contains no content registry, event
// bus, character, randomness or lifecycle. Rules own eligibility; consumers must
// preserve unknown facts rather than turn them into a negative answer.
//
// Source and unresolved dice values also underlie the event-facing aliases, so
// information and resolved traces do not acquire different identity vocabularies.
// An eligibility decision is not calculation availability, stacking selection,
// an executable offer, or permission to act.
package contributions
