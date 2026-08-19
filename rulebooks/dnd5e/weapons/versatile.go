// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package weapons

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
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
// is not on the table. Weapon declarations are validated separately as pure
// NdM pools; passthrough keeps this notation helper total for other callers.
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

// DamageForGrip returns a copy of the weapon's declared damage pools. For a
// versatile weapon wielded in two hands, it steps only the marked primary
// pool's dice. The declaration remains unchanged for later attacks.
func (w Weapon) DamageForGrip(twoHanded bool) ([]damage.Damage, error) {
	pools := make([]damage.Damage, len(w.Damage))
	copy(pools, w.Damage)

	if !w.HasProperty(PropertyVersatile) {
		return pools, nil
	}

	primaryIndex, ok := w.primaryDamageIndex()
	if !ok {
		return nil, fmt.Errorf("versatile weapon %q requires exactly one primary damage pool", w.Name)
	}
	if twoHanded {
		pools[primaryIndex].Dice = VersatileTwoHandedDamage(pools[primaryIndex].Dice)
	}
	return pools, nil
}
