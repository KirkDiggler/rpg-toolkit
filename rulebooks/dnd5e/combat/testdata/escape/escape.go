// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package escape holds widenings the narrowing pin must refuse. It is never
// built — see README.md.
package escape

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// sheetOf is the escape in its plainest form: a rule holding a cast member
// asserts it back to the live sheet it always was, and every write returns.
func sheetOf(m combat.Member) *character.Character {
	sheet, _ := m.(*character.Character)

	return sheet
}
