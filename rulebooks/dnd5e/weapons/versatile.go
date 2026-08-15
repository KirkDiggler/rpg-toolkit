// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package weapons

import (
	"fmt"
	"strconv"
	"strings"
)

// versatileStepUp is the standard 5e versatile-weapon die progression: a
// versatile weapon's two-handed die is always exactly one step up from its
// one-handed die on this table (longsword 1d8 -> 1d10, spear 1d6 -> 1d8, etc
// — no versatile weapon in the ruleset skips a step).
//
// This table is the one copy. It lived unexported in the character package's
// display code until rpg-toolkit#1003 needed the same step at notation level
// for an attack profile; a second copy there would have been free to drift
// from this one the first time a weapon was added.
var versatileStepUp = map[int]int{4: 6, 6: 8, 8: 10, 10: 12}

// VersatileTwoHandedDamage steps a "NdM" damage notation's die size up one
// notch per versatileStepUp, e.g. "1d8" -> "1d10".
//
// Returns notation unchanged when it does not parse as "NdM" or the die size
// is not on the table. That passthrough is not defensive padding: the catalog
// holds weapons whose damage is not dice at all (a blowgun's "1", a net's
// "0"), and stepping them would invent a die the weapon does not have.
func VersatileTwoHandedDamage(notation string) string {
	parts := strings.SplitN(notation, "d", 2)
	if len(parts) != 2 {
		return notation
	}
	count, err := strconv.Atoi(parts[0])
	if err != nil {
		return notation
	}
	size, err := strconv.Atoi(parts[1])
	if err != nil {
		return notation
	}
	next, ok := versatileStepUp[size]
	if !ok {
		return notation
	}

	return fmt.Sprintf("%dd%d", count, next)
}

// VersatileDamage returns the damage notation this weapon deals gripped in
// two hands: the stepped-up die for a versatile weapon, and its one-handed
// Damage for everything else.
//
// The property check lives here so callers that already know the grip ask for
// the notation they want rather than re-deriving "is this versatile" at every
// site — the attack-profile compiler (rpg-toolkit#1003) and the equipment
// display each needed it, and each would have written the same two lines.
func (w Weapon) VersatileDamage() string {
	if !w.HasProperty(PropertyVersatile) {
		return w.Damage
	}

	return VersatileTwoHandedDamage(w.Damage)
}
