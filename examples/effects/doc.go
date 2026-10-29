// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package effects is the composer. It builds a condition from a recipe by
// running a machine (package effects/machine) and attaching the current
// state's outputs to the chains while the machine sits in that state.
//
// The machine decides when a condition moves. This package decides what the
// condition contributes in each state. The rulebook plugs words into both.
// The design is design.md on the idea branch docs/idea-composable-conditions,
// in https://github.com/KirkDiggler/rpg-toolkit/tree/docs/idea-composable-conditions/docs/ideas/composable-conditions
//
// # Composed
//
// Composed is the condition a registry builds. It owns one machine, the
// outputs of every state compiled up front, and the detach functions of the
// outputs attached now. It has Ref, IsApplied, Apply, Remove and ToJSON, so
// it satisfies a rulebook's condition behaviour structurally while
// importing none of it. ToJSON writes the machine's flat blob, so a
// composed Rage persists the same keys that today's Rage does.
//
// # Output
//
// An Output is one contribution a state makes. It has four parts:
//
//   - Kind names the Go code that knows the chain and the stage.
//   - When is an optional guard over the frame, written in the machine's
//     grammar.
//   - Amount is optional.
//   - Params holds the kind's own fields, which only the kind reads and
//     validates.
//
// A recipe carries outputs as opaque JSON on its states, and Compose decodes
// them.
//
//	{kind: damage_bonus, when: all(melee, weapon pool, ability = str),
//	 amount: by_class_level barbarian {1: 2, 9: 3, 16: 4}, params: {stage: features}}
//	{kind: resist, params: {types: [bludgeoning, piercing, slashing]}}
//	{kind: advantage, when: ability = str, params: {on: saving_throws}}
//	{kind: advantage, when: ability = str, params: {on: ability_checks}}
//
// # Amount
//
// An Amount sets exactly one field: fixed, dice, by_class_level or
// ability_modifier. The field's name is the AmountName that the vocabulary
// must hold, and the field's value is that function's parameters. Rage uses
// by_class_level.
//
// # OutputVocabulary
//
// OutputVocabulary is the rulebook's plug into the composer. It has four
// parts:
//
//   - Kinds maps a KindName to a KindSpec. Check validates the params at
//     load. Attach subscribes the output to its chain and returns a Detach.
//     Assess answers the output's information row.
//   - Amounts maps an AmountName to the function that computes it from a
//     frame.
//   - End is called when the machine enters its terminal state.
//   - Changed is called when the bag changes.
//
// The plug supplies End because removing a condition is announced on a
// rulebook topic. The machine and the composer both stay ignorant of that
// topic, but the rulebook's collectors depend on the announcement happening.
//
// # Bound
//
// A Bound is one output bound to its condition. It is what Attach and Assess
// receive.
//
//   - ID is unique per ref, state and index, which makes it fit for a chain
//     modifier.
//   - Host returns the holder, ref and source.
//   - Params returns the kind's fields.
//   - Answer evaluates the output's guard over a frame and the live bag.
//   - Amount computes the output's amount from a frame.
//
// A kind never sees the machine, the recipe or another output.
//
// # Lifecycle
//
// Compose validates everything that can be checked at load. It refuses a
// kind, amount, fact or flag that is not in a vocabulary or not declared,
// and an amount that sets other than one field. It passes the bag keys that
// output guards read to the machine's Validate. It fills the instance's bag
// from parameters. Apply attaches the machine and then the current state's
// outputs, undoing everything if any step fails.
//
// On every transition, the old state's outputs detach and the new state's
// attach. On the terminal state, End runs first, while the outputs still
// answer, and only then does everything detach (design E5: publish, then
// detach). Remove detaches without publishing, because an outside removal
// is announced by whoever removes.
//
// # Rows
//
// Rows is the sum of the current state's outputs. It returns one row per
// output, in recipe order, from each kind's Assess. Information and
// execution ask the same guard. A guard that depends gives a row whose
// Verdict is Depends, not an error. So Rage answers four rows (damage,
// resistance and two advantages) where the hand-written condition answers
// one.
//
// # What effects does not know
//
// The composer knows no rulebook. It has no chain types, stages, removal
// topic or fact names of its own. Every one of them arrives through the
// machine's Vocabulary and the OutputVocabulary.
//
// Contract (design E9): this package imports only the standard library,
// core, core/chain, events and effects/machine, and never a rulebook.
// TestImportsStayInsideTheSeam fails on any other import.
package effects
