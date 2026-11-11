// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package selection captures an explainable pool of choices and selects from it.
//
// The caller supplies ordered identities, named requirement verdicts and integer
// weight factors. Evaluate checks and snapshots these values; Inspect returns
// detached evidence. Unknown is not false, and an eligible zero-share row is
// distinct from an ineligible row. Selection owns no fact reader or game policy.
//
// A weighted Draw uses one supplied die; an explicit Choose uses none. Neither
// operation re-evaluates inputs, changes progress, consumes a key or executes a
// result. A captured pool is not a current authorization token: the execution
// caller revalidates whatever its own world requires. Diagnostic evidence may
// contain private facts; disclosure is also the caller's responsibility.
//
// Weights are nonnegative integer shares. Named factors multiply without
// normalization, rounding or implicit percentage conversion. Overflow is an
// error rather than an invented distribution. Nested policy is composition of
// pools at the caller, not flattened rows or an internal workflow interpreter.
//
// Contract: docs/ideas/behavior-selection/design.md, R1–R7 in the toolkit repo.
// This package imports no parent behavior, event, actor, clock or storage API.
package selection
