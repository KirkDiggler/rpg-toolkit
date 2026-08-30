// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package quest holds goals: templates, the instances offered from them, who
// claimed them, and whether their objectives are met.
//
// # Objectives are predicates, and predicates do not know about methods
//
// An objective is a [Predicate] over derived state, and that is the whole of
// it. "The camp is no longer hostile to the guild" is [NoEdge]. It is satisfied
// by storming the camp, by talking it round, by replacing its leader with
// somebody wearing his face, and by whatever the fourth thing turns out to be —
// because none of those appear in the predicate. Nothing in this package can
// tell which one happened, and nothing in it needs to.
//
// # An objective is read in somebody's view
//
// Every [Objective] names an Observer, and its predicate is evaluated against
// [graph.World.StateFor] for that observer. This is not a detail. A camp
// following an impostor is not hostile *in its own view*, and its own view is
// what it acts on, so the camp's view is where the objective is honestly read.
// Reading it in the truth view would fail a quest the world has already
// resolved.
//
// # Watching, never acting
//
// [Instance.Observe] derives, evaluates, and reports. It writes no facts, moves
// no edges, and calls no resolver. Emissions come back as values — a host turns
// [QuestCompleted] into three hundred experience points if that is what its
// rulebook says, and this package never learns that it did.
package quest
