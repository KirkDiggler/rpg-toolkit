// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"

	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func (s *linkedDataSuite) TestConcentrationQueryUsesMaintenanceAndRefusesTwoClaims() {
	owner := s.create("caster-a", linkedOwnerConfig())
	_, err := owner.lifetime.FinalizeMember(generic.FinalizeMemberInput{Key: "recipient", Delivered: true})
	s.Require().NoError(err)
	out, err := ConcentrationHeld(&ConcentrationQueryInput{Conditions: []dndevents.ConditionBehavior{owner}})
	s.Require().NoError(err)
	s.True(out.Held)
	s.Equal("caster-a", out.Info.Owner.MemberID)
	s.Equal("application-1", out.Info.Owner.SourceID)
	s.Equal("dnd5e:spells:bless", out.Info.SourceRef.String())
	s.Equal(1, out.Info.ChildCount)
	out, err = ConcentrationHeld(&ConcentrationQueryInput{})
	s.Require().NoError(err)
	s.False(out.Held)
	other := linkedOwnerConfig()
	other.ApplicationID = "application-2"
	other.AddressSourceID = "application-2"
	second := s.create("caster-a", other)
	_, err = ConcentrationHeld(&ConcentrationQueryInput{Conditions: []dndevents.ConditionBehavior{owner, second}})
	s.Error(err)
	_, err = ConcentrationHeld(nil)
	s.Error(err)
}

func (s *linkedDataSuite) TestHistoricalConcentrationIsNotASecondLoadPath() {
	_, err := LoadJSON(json.RawMessage(`{"ref":"dnd5e:conditions:concentrating","member_id":"caster-a","source_id":"caster-a","spell_ref":"dnd5e:spells:bless","spell_name":"Bless","turn_ends_left":10}`))
	s.Error(err)
}
