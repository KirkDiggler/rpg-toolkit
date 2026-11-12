// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"

	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func (s *linkedDataSuite) TestNamedBlessAndBaneCreateOnlyCurrentLinkedData() {
	for _, ref := range []string{refs.Conditions.Blessed().String(), refs.Conditions.Baned().String()} {
		config := linkedRecipientConfig()
		config.Bindings.Parts = nil // the named descriptor owns its parts
		raw, err := json.Marshal(config)
		s.Require().NoError(err)
		created, err := CreateFromRef(&CreateFromRefInput{Ref: ref, MemberID: "recipient", Origin: &dndevents.ConditionOrigin{
			Ref: *refs.Spells.Bless(), EntityID: "caster-a",
		}, Config: raw})
		s.Require().NoError(err)
		linked, ok := created.Condition.(*LinkedCondition)
		s.Require().True(ok)
		s.Equal(ref, linked.Ref().String())
		out, err := linked.DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindAttack})
		s.Require().NoError(err)
		s.Require().Len(out.Contributions, 1)
		s.Equal(ref == refs.Conditions.Baned().String(), out.Contributions[0].Subtract)
		stored, err := linked.ToJSON()
		s.Require().NoError(err)
		loaded, err := LoadJSON(stored)
		s.Require().NoError(err)
		s.Equal(linked.ConditionAddress(), ConditionAddressOf("recipient", loaded))
		var changed LinkedConditionData
		s.Require().NoError(json.Unmarshal(stored, &changed))
		changed.Bindings.Parts[0].Params = json.RawMessage(`{"dice":"2d4","group":"bless","rolls":["attack","saving_throw"],"subtract":false}`)
		bad, err := json.Marshal(changed)
		s.Require().NoError(err)
		_, err = LoadJSON(bad)
		s.Error(err, "saved named behavior cannot silently change its definition")
	}
}

func (s *linkedDataSuite) TestNamedBlessAndBaneRefuseHistoricalDataAndCreation() {
	for _, tc := range []struct{ condition, spell string }{
		{refs.Conditions.Blessed().String(), refs.Spells.Bless().String()},
		{refs.Conditions.Baned().String(), refs.Spells.Bane().String()},
	} {
		raw, err := json.Marshal(map[string]any{"ref": tc.condition, "member_id": "recipient", "source_id": "caster-a", "source_ref": tc.spell})
		s.Require().NoError(err)
		_, err = LoadJSON(raw)
		s.Error(err, "obsolete data is refused, not normalized into a fresh application")
		_, err = CreateFromRef(&CreateFromRefInput{Ref: tc.condition, MemberID: "recipient", SourceRef: tc.spell, Config: json.RawMessage(`{"source_id":"caster-a"}`)})
		s.Error(err)
	}
}
