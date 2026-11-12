// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"encoding/json"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	effects "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func (s *ConcentrationKeeperSuite) TestStatusViewLinkedSourceIsOriginNotApplicationOrAddress() {
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "opaque-invocation", CoordinatorID: "bard-1", Origin: dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: "bard-1"},
		Existing: []conditions.ParticipantConditions{{MemberID: "bard-1"}},
		Spec: conditions.ApplicationSpec{Name: "Bless", CoordinatorParts: []effects.PartDeclaration{{Key: "hold", Kind: effects.KindConcentration, OnTerminal: generic.EndApplication}},
			Recipients: []conditions.RecipientSpec{{MemberID: "bard-1", Ref: refs.Conditions.Blessed().String(), AddressSourceID: "opaque-grant-address", Lifetime: conditions.LifetimeShared}}}})
	s.Require().NoError(err)
	owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	s.Require().NoError(err)
	recipient, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0})
	s.Require().NoError(err)
	for _, condition := range []dndevents.ConditionBehavior{owner.Condition, recipient.Condition} {
		raw, err := condition.ToJSON()
		s.Require().NoError(err)
		data := s.bardData()
		data.Conditions = []json.RawMessage{raw}
		loaded, err := Load(s.ctx, data)
		s.Require().NoError(err)
		status, err := loaded.StatusView(&StatusViewInput{})
		s.Require().NoError(err)
		s.Require().Len(status.View.Conditions, 1)
		source := status.View.Conditions[0].SourceMember
		s.Require().NotNil(source)
		s.Equal("bard-1", *source, "provenance is independent of both invocation and replacement address")
		s.NotEqual("opaque-invocation", *source)
		s.NotEqual("opaque-grant-address", *source)
	}
}
