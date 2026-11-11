// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// A current creator-produced payload fixture for generic registry/loader tests.
// It proves local data/behavior, not a complete linked application admission.
func linkedForTest(member string) dndevents.ConditionBehavior {
	config, err := json.Marshal(linkedRecipientConfig())
	if err != nil {
		panic(err)
	}
	created, err := CreateFromRef(&CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: member,
		Origin: &dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "caster-a"}, Config: config})
	if err != nil {
		panic(err)
	}
	return created.Condition
}
