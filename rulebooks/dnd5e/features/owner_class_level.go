// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// ownerClassLevel asks a feature's activating owner how many levels it holds
// in class, through the named class-level question its level record answers.
// An owner that cannot answer, or that holds no levels in the class, is
// refused: the feature's number scales with that class and cannot be
// computed, and zero is never read as level one.
func ownerClassLevel(owner core.Entity, class classes.Class, feature string) (int, error) {
	holder, ok := owner.(classes.LevelHolder)
	if !ok {
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"%s: owner cannot answer its %s level", feature, class)
	}
	level := holder.ClassLevel(class)
	if level < 1 {
		return 0, rpgerr.Newf(rpgerr.CodePrerequisiteNotMet,
			"%s: owner holds no %s levels", feature, class)
	}
	return level, nil
}
