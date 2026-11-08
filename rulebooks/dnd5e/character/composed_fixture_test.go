// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"encoding/json"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func composedCharacterRage(s *suite.Suite, member string) dndevents.ConditionBehavior {
	out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: member, Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: member}, Config: json.RawMessage(`{"damage_bonus":{"barbarian_level":true}}`)})
	s.Require().NoError(err)
	return out.Condition
}

func composedCharacterRageData(s *suite.Suite, c dndevents.ConditionBehavior) conditions.ComposedData {
	raw, err := c.ToJSON()
	s.Require().NoError(err)
	var data conditions.ComposedData
	s.Require().NoError(json.Unmarshal(raw, &data))
	return data
}
