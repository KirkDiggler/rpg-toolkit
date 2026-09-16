// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// syntheticLevels builds a level record of n levels taken in one class, the
// first at the full hit die and the rest at the average.
//
// The level record is the sheet's only statement of its own level, so a
// fixture that wants a level-4 character states four levels rather than a
// number; a stored sheet claiming a level with no record behind it is refused
// at load (R2.7). firstGain and laterGain let a fixture keep its stated
// maximum hit points true of the record that produced them.
func syntheticLevels(class classes.Class, n, firstGain, laterGain int) []character.LevelEntry {
	entries := make([]character.LevelEntry, 0, n)
	for i := 1; i <= n; i++ {
		entry := character.LevelEntry{
			Level:          i,
			ClassID:        class,
			HitPointGain:   laterGain,
			HitPointMethod: character.HitPointMethodAverage,
		}
		if i == 1 {
			entry.HitPointGain = firstGain
			entry.HitPointMethod = character.HitPointMethodMax
		}
		entries = append(entries, entry)
	}
	return entries
}
