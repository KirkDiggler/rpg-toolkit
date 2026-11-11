// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

// Current rulebook end-reason vocabulary for application facts and records.
// These constants do not supply a separate concentrating condition or codec.
const (
	// ConcentrationEndedDamage names failure of the maintainer's damage save.
	ConcentrationEndedDamage = "damage"
	// ConcentrationEndedRecast names displacement by a paid replacement.
	ConcentrationEndedRecast = "recast"
	// ConcentrationEndedDuration names the authored duration expiring.
	ConcentrationEndedDuration = "duration"
	// ConcentrationEndedCombatEnd names the configured combat-end policy.
	ConcentrationEndedCombatEnd = "combat_end"
	// ConcentrationEndedSpellEnded names actual last-member loss.
	ConcentrationEndedSpellEnded = "spell_ended"
	// ConcentrationEndedCasterDown names the maintainer dropping to zero HP.
	ConcentrationEndedCasterDown = "caster_down"
	// ConcentrationDCFloor is the minimum DC described by concentration upkeep.
	ConcentrationDCFloor = 10
)
