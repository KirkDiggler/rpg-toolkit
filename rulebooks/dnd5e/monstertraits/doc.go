// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package monstertraits composes a monster's attach: [LoadMonster] is the pure
// load of a sheet, and [AttachMonster] puts it on a bus with its sheet keeper,
// the free reaction every combatant carries, and every condition its sheet
// holds.
//
// It holds no trait type. A monster's stat-block traits (damage immunity,
// damage vulnerability, pack tactics, undead fortitude) are conditions like
// any other: they live in the conditions package beside every condition's
// descriptor, and every blob of a monster's sheet loads through
// [conditions.LoadJSON]. A ref nothing registers is refused, never skipped.
//
// The one thing attach adds is the roller. A loaded condition carries none, so
// attach hands its roller to every loaded condition that implements
// [conditions.RollerBinder] — Undead Fortitude is one, and refuses its save
// when none was bound.
package monstertraits
