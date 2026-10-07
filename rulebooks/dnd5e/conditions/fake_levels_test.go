// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"

// fakeLevels is a level record that answers the named class-level question
// from a map; a class it does not name holds zero levels.
type fakeLevels map[classes.Class]int

func (f fakeLevels) ClassLevel(class classes.Class) int { return f[class] }

// monkLevels is a level record holding level monk levels and nothing else.
func monkLevels(level int) fakeLevels { return fakeLevels{classes.Monk: level} }
