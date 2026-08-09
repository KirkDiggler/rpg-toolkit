// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package clock provides scheduling policies for live play: who may act,
// and what advances time. Two concrete clocks — Turn (a localized
// initiative bubble) and Tick (the player-driven world clock) — plus the
// Milestone vocabulary their verbs return and the Transfer helper that
// moves an entity between clocks atomically.
//
// Design contract: docs/ideas/play/clock/design.md (R1–R10). This is a
// leaf module: it depends only on core, takes no context.Context, returns
// milestones as values, and never publishes.
package clock
