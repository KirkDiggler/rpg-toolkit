// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// syntheticLevels builds a level record of n levels taken in one class.
//
// The level record is the sheet's only statement of its own level, so a
// fixture that wants a level-3 character states three levels rather than a
// number. A stored sheet that claims a level with no record behind it is
// refused at load (R2.7) — these fixtures are the reason the rule found
// anything to refuse.
//
// The hit point gain is left at zero: these are fixtures for tests that care
// about the level, not about how the character's hit points got there, and
// every one of them states its own hit points.
func syntheticLevels(class classes.Class, n int) []character.LevelEntry {
	entries := make([]character.LevelEntry, 0, n)
	for i := 1; i <= n; i++ {
		entries = append(entries, character.LevelEntry{
			Level:          i,
			ClassID:        class,
			HitPointMethod: character.HitPointMethodAverage,
		})
	}
	return entries
}

// setLevel restates a fixture's level, record and all.
//
// The record is the sheet's statement of its own level, and Data.Level is a
// projection of it (R2.2), so a fixture that reaches past its constructor to
// change the level must change the record with it or fail to load (R2.3).
// The levels are taken in the character's current class, so call this after
// any change to ClassID, not before.
func setLevel(data *character.Data, level int) {
	data.Level = level
	data.Levels = syntheticLevels(data.ClassID, level)
}

// setClass restates a fixture's class, record and all.
//
// Every entry in the record is taken in the character's own class, and a
// differing one is refused at load (R2.4) — the refusal that holds the
// multiclassing seam shut. A fixture that re-dresses one class's sheet as
// another must re-take its levels in the new class.
func setClass(data *character.Data, class classes.Class) {
	data.ClassID = class
	data.Levels = syntheticLevels(class, data.Level)
}
