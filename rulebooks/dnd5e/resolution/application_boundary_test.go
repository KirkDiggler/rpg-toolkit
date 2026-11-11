// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

type applicationBoundaryProbe struct {
	prepared *conditions.PreparedApplication
}

func (p applicationBoundaryProbe) Start(_ context.Context, cast *Participants) (Step, error) {
	candidate, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: p.prepared, SpecIndex: 0})
	if err != nil {
		return nil, err
	}
	return Gather{name: "deliberately omit delivery finalization", run: func(ctx context.Context, bus events.EventBus) (Step, error) {
		target, err := cast.entity(heroID)
		if err != nil {
			return nil, err
		}
		if err := dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: target, Condition: candidate.Condition, ApplicationID: p.prepared.ID()}); err != nil {
			return nil, err
		}
		return Done{}, nil
	}}, nil
}

// A broken source must not hand a saveable Output to the host merely because the
// machine reached Done. This deliberately omits finalization after actual Apply.
func (s *LinkedApplicationSuite) TestUnfinalizedActualDeliveryCannotLeaveASuccessfulInteraction() {
	f := s.fixtures()
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bless})
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "unfinished", CoordinatorID: bardID, Origin: dndevents.ConditionOrigin{Ref: definition.Ref, EntityID: bardID},
		Existing: []conditions.ParticipantConditions{{MemberID: bardID}, {MemberID: heroID}, {MemberID: wolfID}}, Spec: conditions.ApplicationSpec{Name: definition.Name, CoordinatorParts: definition.Cast.Maintenance,
			Recipients: []conditions.RecipientSpec{{MemberID: heroID, Ref: definition.Cast.Effects[0].Ref.String(), AddressSourceID: bardID, Lifetime: conditions.LifetimeShared}}}})
	s.Require().NoError(err)
	owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	s.Require().NoError(err)
	raw, err := owner.Condition.ToJSON()
	s.Require().NoError(err)
	caster := baneCaster(1)
	caster.Conditions = append(caster.Conditions, raw)
	out, err := f.resolve(f.saver(14), applicationBoundaryProbe{prepared: prepared}, nil, caster)
	s.Require().ErrorContains(err, "unfinalized actual")
	s.Nil(out, "a failed successful-result boundary returns no persistable output")
}
