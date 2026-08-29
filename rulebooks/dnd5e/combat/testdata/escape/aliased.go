// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package escape

import c "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"

// keeperOf is the escape a pin that matched source text would miss twice over:
// the combat package is imported under another name, and the member is passed
// through an any first, so neither "combat.Combatant" nor a Member-typed
// operand appears anywhere in the expression.
func keeperOf(m c.Member) c.Combatant {
	var laundered any = m
	keeper, _ := laundered.(c.Combatant)

	return keeper
}
