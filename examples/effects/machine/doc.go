// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package machine is a state machine over a bag that fires configured events
// on transition.
//
// A Recipe names states, the transitions between them, and the trackers that
// feed the bag. Each transition listens to a beat, asks a guard written in a
// closed vocabulary, performs its acts on the bag, and may move the machine
// to another state; every move fires a Transitioned event carrying from, to
// and reason. The machine knows no chain and no output: the outputs a state
// carries are opaque JSON that the composer (package effects) reads and
// attaches.
//
// The rulebook plugs its words in through a Vocabulary: the facts a guard
// may read, and the beats a transition or tracker may listen to with the
// selectors each supports. Validate refuses, by name, a recipe that names
// anything the vocabulary or the bag does not hold, and a guard over an
// unknown fact answers Depends rather than false.
//
// The bag is the blob (E4): Blob writes the ref, holder, source and state
// with the bag's keys beside them in one flat JSON object.
//
// Contract (design E9 of docs/ideas/composable-conditions): this package
// imports the standard library, core, core/chain and events only, and never
// a rulebook. TestImportsStayInsideTheSeam fails on any other import.
package machine
