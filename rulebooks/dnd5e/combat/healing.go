// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

// CanReceiveHealing admits living creatures, including dying or stabilized
// characters. Ordinary healing does not revive the dead or defeated monsters.
// Spell-specific no-effect rules are separate from this eligibility check.
func CanReceiveHealing(state LifeState) bool {
	return state == LifeStateConscious || state == LifeStateDying || state == LifeStateStabilized
}
