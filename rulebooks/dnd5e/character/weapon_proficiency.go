// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"

// IsProficientWith reports whether this character's weapon proficiencies
// cover the given weapon — the sheet's half of the question weapons.CoveredBy
// answers per grant.
//
// A nil weapon is not proficient rather than a panic: callers reach this from
// an equipped slot, and an empty slot resolves to nil weapon on a path where
// "no proficiency bonus" is the answer that keeps the arithmetic honest.
func (c *Character) IsProficientWith(w *weapons.Weapon) bool {
	if w == nil {
		return false
	}

	for _, grant := range c.weaponProficiencies {
		if w.CoveredBy(grant) {
			return true
		}
	}

	return false
}
