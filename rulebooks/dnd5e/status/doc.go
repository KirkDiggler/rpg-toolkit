// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package status turns a status effect written in an author's words into the
// definition library the conditions package runs.
//
// A status effect is a named piece an author writes and shares — "Well fed",
// say — that changes what a player or an NPC can do (advantage on Charisma
// checks, resistance to fire, a bonus to damage), with rules for how long it
// stays (until the run ends, until the next rest). An author writes it with
// four words: name, changes, maintenance and again.
//
// The engine owns the list of changes and maintenance rules. An author can
// only pick from them, never invent one: every word an author may write, where
// it may appear, its allowed values and its plain-English label are one table
// in this package (see [Words]). [Compile] reads only that table, so renaming
// a word is a one-line change there and nowhere else.
//
// The four top-level words are settled; the words inside them are still
// placeholders. Both are recorded at
// https://github.com/KirkDiggler/rpg-project/issues/1024.
package status
