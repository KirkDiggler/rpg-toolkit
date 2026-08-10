// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package interrupt is the ledger of open windows: suspension-as-value
// custody for resolutions that stop and wait for an outside decision.
// Pose opens a window (one audience entity, offered option tokens, an
// opaque frozen payload); Answer validates, closes it, and returns the
// envelope for the rulebook to resume. The module never interprets an
// option, a choice, or a payload byte — custody, not execution. Human
// and machine deciders are indistinguishable here: an auto-taken
// reaction is an ordinary Pose answered immediately by composition.
//
// Design contract: docs/ideas/play/interrupt/design.md (R1–R10). Leaf
// module: depends only on core, no context.Context, deltas returned as
// values, never published.
package interrupt
