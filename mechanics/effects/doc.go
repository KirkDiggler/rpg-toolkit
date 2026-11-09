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
// # Construction
//
// Compose takes one ComposeInput. Recipe, MachineVocabulary and OutputVocabulary
// name the authored structure and the consumer's words; HolderID and SourceRef
// name this instance's recipient and applying content. Parameters fills its bag
// over the recipe defaults. State selects a saved state; empty means initial.
// Nil Parameters uses the defaults, and nil SourceRef means no source.
//
// Compose returns a *Composed and an error; it does not attach to a bus. Named
// fields keep fresh construction and restoration readable at the call site.
//
// # Composed
//
// Composed is the condition a registry builds. It owns one machine, the
// outputs of every state compiled up front, and the detach functions of the
// outputs attached now. It has Ref, IsApplied, Apply, Remove and ToJSON, so
// it satisfies a rulebook's condition behaviour structurally while
// importing none of it. ToJSON writes the machine's flat blob. Snapshot
// returns typed lifecycle state for a consumer that owns a richer saved-instance
// format, without an intermediate JSON encode/decode.
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
// by_class_level. Evaluation returns an AmountResult: either a present scalar
// or unresolved dice, never an implicit roll. ScalarValue refuses a dice result;
// a row with no answered amount uses nil rather than an invented zero.
//
// # OutputVocabulary
//
// OutputVocabulary is the rulebook's plug into the composer. It has four
// parts:
//
//   - Kinds maps a KindName to a KindSpec. Check validates the complete Output
//     at construction, including amount/parameter requirements. A subscribed kind
//     requires Attach and returns a Detach. A query-only kind forbids Attach;
//     the consumer selects its contribution through description instead.
//     Both kinds require Assess for their information row.
//   - Amounts maps an AmountName to the function that computes it from a
//     frame.
//   - End is called when the machine enters its terminal state.
//   - Changed is called when the bag changes or a nonterminal transition
//     successfully attaches its new outputs. Terminal transitions use End.
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
//   - ID names ref, state and index, prefixed by InstanceKey when supplied.
//     Assemblies can therefore separate equal recipes from different instances.
//   - Host returns the holder, ref and source with detached ref values.
//   - Params returns an owned copy of the kind's fields.
//   - Answer evaluates the output's guard over a frame and the live bag.
//   - Amount returns an owned, validated scalar or unresolved dice description.
//   - CanReact distinguishes permission to request new work from a contribution
//     that still answers during End; it is false while unattached or ending.
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
// # Observations
//
// Snapshot returns an owned state/bag value. CurrentOutputs returns a defensive
// slice of observational Bound handles for the current state. These queries do
// not attach, publish or mutate. Handles read the live bag through Answer, so
// callers fetch current outputs again after transitions; Snapshot retains state.
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
// # Local assemblies
//
// Assemble binds ordered, independently reacting Composed parts to one holder.
// Each part has a stable key and explicit EndPart, EndAttachment or EndApplication
// scope: complete one part, end this recipient attachment, or end the shared
// application. It
// supplies no nested lifecycle hooks or instance namespace: the assembly owns
// both. The same beat can update multiple parts without flattening their rules
// into one first-match transition list. Application end is reported once while
// subscribed and query contributions still answer, including pre-terminal
// outputs. During that report, the assembly gates beat/ tracker dispatch before
// sibling reactions can mutate state or detach outputs. Then all local parts
// detach. BeginEnd exposes that local gate to an external lifetime owner without
// firing another callback. Remove only detaches.
//
// AssemblySnapshot stores every part, including locally completed ones.
// RestoreAssembly refuses missing state and terminal required maintenance, and
// never reattaches completed local parts. Lifecycle.Changed reports local
// completion as well as nonterminal changes, so a save cannot revive a part.
// CurrentOutputs carries part keys and instance-qualified output identities.
//
// # Shared lifetime values
//
// Lifetime is the neutral value an application coordinator owns. NewLifetime
// binds opaque ordered member keys and authorized ender keys. MemberRemoved
// reports actual before/after membership, so empty creation is not confused
// with losing the last member. RequestEnd freezes membership and yields one
// ordered plan; repeated requests retain the original reason and yield no plan.
// CompleteEnd follows successful delivery. Nothing here publishes or interprets
// a key: the rulebook delivers the plan through its own exact-address protocol.
// Snapshot and returned plans own their slices. RestoreLifetime requires an
// explicitly live subset and never revives members from fresh defaults.
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
