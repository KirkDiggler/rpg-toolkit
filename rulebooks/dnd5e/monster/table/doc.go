// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package table ships the rulebook's answer-table content: the default table
// a creature rolls on when its author wrote none, and the temperament
// profiles that load that roll's die.
//
// It is CONTENT, and only content. A table here is text in the dungeonspec
// `on:` grammar and a profile is five numbers; this package neither parses
// the one nor applies the other. Compiling the grammar belongs to
// `rulebooks/dnd5e/encounter/dungeonspec`, and rolling on the compiled table
// belongs to `rulebooks/dnd5e/encounter`, which owns the dice, the stance
// graph and what a creature has seen. A table that knew here what `enemy:
// seen` means would be that layer written a second time, in the one place
// that cannot see a creature.
//
// It sits beside `monster/monsters`, the package that holds the shipped
// monsters themselves, for the same reason: both answer a ref with what the
// rulebook ships for that kind, and neither interprets it.
//
// Design: rpg-project `ideas/creature-table/design.md` (RULED 2026-09-18),
// §1 layers, §2 the vocabulary, §3 temperament. Tracking rpg-project#466.
package table
