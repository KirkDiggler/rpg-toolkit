// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package effects supplies the D&D vocabulary consumed by mechanics/effects.
// It owns mechanical kind meanings, facts, beats, amounts and typed assessment
// contexts, not named conditions, budgets, inventory or persistence.
//
// Bind supplies generic machine/output vocabularies using the same private kind
// table as Assess. A kind validates its complete output before attachment, and
// execution and information use the same domain/guard/amount evaluation. Unknown
// required facts are Depends in information and ErrRuleCannotAnswer in execution;
// a known different domain is not applicable. An absent query describes only.
//
// Origin is immutable provenance from the caller; it is not a stacking address.
// The owning condition supplies End/Changed hooks and decides how to serialize
// its bindings and runtime snapshot. No item package or condition registry is
// imported here, and no Raging implementation or per-source behavior is hidden
// in the vocabulary. Save/check advantage, resistance and fixed/class damage
// bonuses are the closed meanings used by the first consumer.
package effects
