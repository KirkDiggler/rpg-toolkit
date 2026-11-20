// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package conditions holds the effects a creature can have in D&D 5e: the
// named conditions, the data-defined effects authors write, and the readers a
// sheet asks for what they add.
//
// # What is stored, and what is worked out
//
// An effect is stored only when something in play put it there: a spell, a
// status an author gave, a feature a level granted. It lives on the sheet,
// is saved with it, can end, and shows as something the creature has.
//
// What follows from what a creature IS — Dexterity adding its modifier to
// initiative, for every creature, always — is not stored. It is a rule,
// written as data in the same part shape an author's effect compiles to, held
// once in this package's rules library, and worked out each time it is read.
// So no saved sheet carries it, none needs a migration to gain it, it cannot
// be removed or counted twice, and it is never a status a player "has".
// Ruled on https://github.com/KirkDiggler/rpg-project/issues/990#issuecomment-7034098169.
//
// [Initiative] reads both: the rules first, then the held effects, each with
// its source.
package conditions
