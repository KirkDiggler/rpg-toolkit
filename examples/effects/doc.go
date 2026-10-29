// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package effects is the composer: it builds a condition from a recipe by
// running a machine and attaching the current state's outputs to the chains
// while the machine sits in that state.
//
// The machine (package effects/machine) owns states, transitions, trackers,
// guards and the bag. This package owns outputs: each output names a kind
// the rulebook supplied in an OutputVocabulary, an optional guard over the
// frame, and an optional amount. On every transition the composer detaches
// the old state's outputs and attaches the new state's; on entry to the
// terminal state it calls the vocabulary's End hook first, so the removal is
// published while the condition still owns its outputs, and then detaches
// everything (E5). A Composed satisfies a rulebook's condition behaviour
// structurally (Ref, IsApplied, Apply, Remove, ToJSON) without importing it.
//
// Contract (design E9 of docs/ideas/composable-conditions): this package
// imports the standard library, core, core/chain, events and effects/machine
// only, and never a rulebook. TestImportsStayInsideTheSeam fails on any other
// import.
package effects
