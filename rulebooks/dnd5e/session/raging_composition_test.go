// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ragingForTest builds current fixture data through the shared creator. It owns
// no bus or lifecycle logic; invalid fixture setup is a test construction error.
func ragingForTest(member string) dndevents.ConditionBehavior {
	out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: member, Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: member}, Config: json.RawMessage(`{"damage_bonus":{"barbarian_level":true}}`)})
	if err != nil {
		panic(err)
	}
	return out.Condition
}
