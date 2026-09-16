// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"

// syntheticLevels builds a level record of n levels taken in one class.
//
// The level record is the sheet's only statement of its own level, so a test
// that wants a level-4 character states four levels rather than a number. The
// hit point gain is left at zero: these are fixtures for tests that care about
// the level, not about how the character's hit points got there, and every one
// of them states its own hit points.
func syntheticLevels(class classes.Class, n int) []LevelEntry {
	if n <= 0 {
		return nil
	}

	entries := make([]LevelEntry, 0, n)
	for i := 1; i <= n; i++ {
		entries = append(entries, LevelEntry{
			Level:          i,
			ClassID:        class,
			HitPointMethod: HitPointMethodAverage,
		})
	}
	return entries
}
