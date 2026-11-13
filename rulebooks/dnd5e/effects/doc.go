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
// bonuses are the closed meanings used by the first consumer. Attack advantage
// additionally declares by_holder or against_holder bearing. Target-side execution
// requires exact local identity and captured configuration in authoritative facts;
// it does not relabel the target as the actor. ProjectPartOutput and
// AssessConfiguredOutput carry owned definitions through that same meaning.
//
// PartTemplate describes unbound authored meaning; ValidatePartTemplate reports
// its kind-owned benefit/maintenance class and capabilities without runtime IDs.
// Damage and save/check advantage are independent payload parts alongside
// resistance. Authored guards use supported frame facts, never private state;
// unsupported fields are refused. Templates/declarations own detached copies
// through Clone when crossing binding or snapshot boundaries.
//
// BindPart selects reusable local recipes from that same kind table. Concentration
// requests a CON follow-up from positive applied damage and has no duration counter;
// duration owns its counter/grace separately. End-on and until-empty policies are
// independent lifecycle parts. Combat-activity upkeep owns attack/applied-damage
// flags and optional first-end grace, not a clock or concentration. End-on selects
// rest, combat-end, next received turn-start or dropped-to-zero for its holder.
// Actual last-member loss reports the source-neutral members_empty cause.
// Resistance reuses its existing incoming fold.
// AddToRoll is query-only and describes signed dice plus explicit selection
// policy; it never subscribes a second roll handler. Dice lives only in Amount,
// with polarity/domains/selection in params. Unbound validation uses the same
// homogeneous-dice grammar as the native resolver. DescribeRoll and Assess share the kind's
// applicability; RollQuery names an actual recipient roll, not a caster's outgoing
// save-shaped action. InspectPart returns maintenance facts without exposing bag
// interpretation to callers. Registry/transport integration is owned above this
// vocabulary; parts neither select targets nor remove another holder's conditions.
package effects
