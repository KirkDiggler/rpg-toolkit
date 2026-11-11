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
//
// BindPart selects reusable local recipes from that same kind table. Concentration
// requests a CON follow-up from positive applied damage and has no duration counter;
// duration owns its counter/grace separately. End-on and until-empty policies are
// independent lifecycle parts. Resistance reuses its existing incoming fold.
// Roll modifiers are query-only, describe signed dice with selection metadata, and
// never subscribe a second roll handler. DescribeRoll and Assess share the kind's
// applicability; RollQuery names an actual recipient roll, not a caster's outgoing
// save-shaped action. InspectPart returns maintenance facts without exposing bag
// interpretation to callers. Registry/transport integration is owned above this
// vocabulary; parts neither select targets nor remove another holder's conditions.
package effects
