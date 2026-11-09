// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// physicalResistances remains the existing helper for handwritten conditions
// such as Blade Ward. Moving Rage does not migrate those condition types.
func physicalResistances(dealt []damage.Type, source dndevents.RollSource) []dndevents.DamageMultiplier {
	var resistances []dndevents.DamageMultiplier
	for _, kind := range dealt {
		if !kind.IsPhysical() {
			continue
		}
		resistances = append(resistances, dndevents.DamageMultiplier{Category: dndevents.DamageSourceCondition, Source: dndevents.CloneRollSource(source), DamageType: kind, Factor: dndevents.DamageFactorResistance})
	}
	return resistances
}
