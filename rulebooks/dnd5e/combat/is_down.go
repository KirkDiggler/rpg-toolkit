// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

// IsDown reports whether a combatant is down: at zero hit points, or below.
//
// This is a pull read of the sheet and nothing else. There is no stored down
// flag, so the answer moves with the hit points — heal a combatant and it
// stands again, and any route to zero (a weapon, a spell, a condition, a sheet
// loaded from storage already at zero) is noticed at the next ask rather than
// only at the moment it happens. ApplyDamageResult.DroppedToZero reports what
// one blow did; IsDown reports where the combatant stands now.
//
// Below zero counts, and not only defensively: the two sheets in this rulebook
// floor at zero today, but a combatant is defined by what it reports, and one
// reporting -3 is not standing.
//
// Exceptions to the rule live here too. A monster with Undead Fortitude
// (rpg-toolkit#977) reaches zero and is not down; when that answer is built it
// changes this function, not its callers. That is the reason a composition
// holding no hit points asks this question instead of comparing numbers itself.
//
// It takes a [Member] rather than a [Combatant] because the whole of it is one
// read. Every caller passes a combatant today and keeps compiling — Combatant
// embeds Member — but a rule holding a cast member can ask this question too,
// which is the point: "is this creature down" is a rule's question, not a
// keeper's.
//
// c must not be nil.
func IsDown(c Member) bool {
	return c.GetHitPoints() <= 0
}
