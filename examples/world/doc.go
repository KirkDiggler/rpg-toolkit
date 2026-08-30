// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package world is the composer: it assembles declared content with an injected
// rulebook, and owns the one door that writes.
//
// Three packages beneath it each hold one thing — journal the memory, graph the
// structure and the present, quest the goals. None of them knows about the
// others' concerns and none of them can act. This is where they meet, and it is
// deliberately small: the act loop and the two doors, nothing else.
//
// # Declare, inject, subscribe
//
// A rulebook builds its world through exactly three touchpoints, and the types
// here are those three:
//
//   - **Declare.** [Scenario] is what a content package hands over: graph
//     declarations, verbs, quest templates. All data. A scenario says who
//     exists, what anyone may try, and what the jobs are.
//
//   - **Inject.** [Resolver] is what content deliberately withholds. [Config]
//     is a Scenario plus a Resolver, and the split in that struct is the whole
//     boundary: the same camp runs under a different rulebook by changing one
//     field.
//
//   - **Subscribe.** [Result] carries the fact that was written and what the
//     jobs made of it, returned as values. Nothing is published; there is no
//     bus here and none is wanted. A host decides that finishing a contract is
//     worth three hundred experience points, and the world never learns it did.
//
// # The act loop
//
// [World.Act] is the only thing in this module that writes a fact:
//
//	verb lookup → resolver, if the verb is contested → the outcome band the
//	margin reached → subject and audience → append → the jobs observe
//
// Every consequence in the world is downstream of that one function, which is
// why audience discipline holds: there is no second path by which a fact could
// reach the log with the wrong people watching.
//
// [World.View] is the only thing that reads, and it reads as somebody: the
// present is always an observer's present, folded fresh from the facts they
// witnessed.
package world
