// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package escape

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// launderedSheet is sheetOf with one line of misdirection in front of it. The
// operand is an any by the time it is asserted, which is why the pin asks about
// the destination as well as the source.
func launderedSheet(m combat.Member) *character.Character {
	var anonymous any = m
	sheet, _ := anonymous.(*character.Character)

	return sheet
}
