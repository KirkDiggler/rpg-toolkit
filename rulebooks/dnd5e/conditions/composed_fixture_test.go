// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// composedRageForTest creates a real class-bound instance through the registry.
// Like refContractTable's other constructors, invalid fixture setup panics; it
// does not implement any condition behavior or historical data conversion.
func composedRageForTest(member string) *ComposedCondition {
	out, err := CreateFromRef(&CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: member, Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: member}, Config: json.RawMessage(`{"damage_bonus":{"barbarian_level":true}}`)})
	if err != nil {
		panic(err)
	}
	return out.Condition.(*ComposedCondition)
}
