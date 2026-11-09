// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type wardFixture struct {
	bus     events.EventBus
	binding *conditions.ApplicationBinding
	sheets  []*character.Character
	owner   *conditions.LinkedCondition
}

// ward creates and delivers through the real protocol, then reloads every holder.
// This is toolkit composition proof, not native item acquisition or casting proof.
func (s *applicationBindingSuite) ward(concentrating bool) wardFixture {
	ctx, bus := context.Background(), events.NewEventBus()
	sheets := s.sheets()
	for i := range sheets {
		sheets[i] = s.reloadConditions(sheets[i])
	}
	binding := s.bind(sheets, bus)
	parts := []dndfx.PartDeclaration{{Key: "empty", Kind: dndfx.KindUntilEmpty, OnTerminal: generic.EndApplication}}
	if concentrating {
		parts = append(parts, dndfx.PartDeclaration{Key: "hold", Kind: dndfx.KindConcentration, OnTerminal: generic.EndApplication})
	}
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: "ward", CoordinatorID: "caster",
		Origin: dndevents.ConditionOrigin{Ref: core.Ref{Module: "example", Type: "items", ID: "ward-ring"}, EntityID: "caster"},
		Spec: conditions.ApplicationSpec{Name: "Shared physical ward", CoordinatorParts: parts, Recipients: []conditions.RecipientSpec{{
			Ref: refs.Conditions.LinkedEffects().String(), Name: "Physical ward", AddressSourceID: "ward-ring", Lifetime: conditions.LifetimeShared,
			Parts: []dndfx.PartDeclaration{{Key: "resistance", Kind: dndfx.KindResistance, Params: json.RawMessage(`{"types":["bludgeoning","piercing","slashing"]}`), OnTerminal: generic.EndPart}},
		}}}, Existing: bindingParticipants(sheets)})
	s.Require().NoError(err)
	created, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0, MemberID: "one"})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: sheets[1], Condition: created.Condition}))
	sealed, err := conditions.SealApplication(&conditions.SealApplicationInput{Prepared: prepared, Delivered: []conditions.DeliveryReceipt{created.Receipt}, Binding: binding})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: sheets[0], Condition: sealed.Condition}))
	s.Require().NoError(binding.CheckComplete())
	s.Require().NoError(binding.Close(ctx))
	for i, sheet := range sheets {
		data, err := sheet.ToData()
		s.Require().NoError(err)
		sheets[i], err = character.Load(ctx, data)
		s.Require().NoError(err)
	}
	bus = events.NewEventBus()
	binding = s.bind(sheets, bus)
	return wardFixture{bus: bus, binding: binding, sheets: sheets, owner: sheets[0].GetConditions()[0].(*conditions.LinkedCondition)}
}

func (s *applicationBindingSuite) wardDamage(bus events.EventBus, kind damage.Type) (int, []dndevents.DamageMultiplier) {
	amount := 9
	ref := core.Ref{Module: "example", Type: "actions", ID: "hit"}
	frame := contributions.Frame{Actor: "enemy", Target: contributions.Known("one"), Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)}}
	sent, err := dndevents.NewIncomingDamageEvent(dndevents.IncomingDamageInput{TargetID: "one", SourceID: "enemy", Frame: frame, Dealt: []dndevents.DamageComponent{{Source: dndevents.DamageSourceWeapon, DamageType: kind, Roll: dndevents.RollComponent{Source: dndevents.RollSource{Ref: &ref, Name: "Hit"}, Modifier: &amount}}}})
	s.Require().NoError(err)
	ctx := context.Background()
	chain, err := dndevents.IncomingDamageChain.On(bus).PublishWithChain(ctx, sent.Clone(), events.NewStagedChain[*dndevents.IncomingDamageEvent](combat.ModifierStages))
	s.Require().NoError(err)
	folded, err := chain.Execute(ctx, sent.Clone())
	s.Require().NoError(err)
	s.Require().NoError(folded.CheckUnaltered(sent))
	settled, err := combat.SettleDamage(&combat.SettleDamageInput{Dealt: sent.Dealt(), Reductions: folded.Reductions, Multipliers: folded.Multipliers})
	s.Require().NoError(err)
	return settled.Types[0].Taken, folded.Multipliers
}

func (s *applicationBindingSuite) TestConcentratedResistanceWithAndWithoutMaintenance() {
	for _, concentrating := range []bool{false, true} {
		w := s.ward(concentrating)
		ctx := context.Background()
		for _, kind := range []damage.Type{damage.Bludgeoning, damage.Piercing, damage.Slashing} {
			taken, _ := s.wardDamage(w.bus, kind)
			s.Equal(4, taken)
		}
		taken, _ := s.wardDamage(w.bus, damage.Fire)
		s.Equal(9, taken)
		recipient := &dndevents.DamageTakenEvent{MemberID: "one", Amount: 4}
		s.Require().NoError(dndevents.DamageTakenTopic.On(w.bus).Publish(ctx, recipient))
		s.Empty(recipient.FollowUps, "the recipient is not the maintainer")
		maintainer := &dndevents.DamageTakenEvent{MemberID: "caster", Amount: 6}
		s.Require().NoError(dndevents.DamageTakenTopic.On(w.bus).Publish(ctx, maintainer))
		if concentrating {
			s.Len(maintainer.FollowUps, 1)
		} else {
			s.Empty(maintainer.FollowUps)
		}
		s.Require().NoError(dndevents.DamageTakenTopic.On(w.bus).Publish(ctx, &dndevents.DamageTakenEvent{MemberID: "caster", Amount: 10, DroppedToZero: true}))
		taken, _ = s.wardDamage(w.bus, damage.Slashing)
		if concentrating {
			s.Equal(9, taken)
			s.Empty(w.sheets[1].GetConditions())
		} else {
			s.Equal(4, taken)
			s.Len(w.sheets[1].GetConditions(), 1)
		}
		s.Require().NoError(w.binding.CheckComplete())
		s.Require().NoError(w.binding.Close(ctx))
	}
}

func (s *applicationBindingSuite) TestIndependentRagingSurvivesWardEnd() {
	w := s.ward(true)
	ctx := context.Background()
	raging, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: "one", Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: "one"}, Config: json.RawMessage(`{"damage_bonus":{"fixed":2}}`)})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(w.bus).Publish(ctx, dndevents.ConditionAppliedEvent{Target: w.sheets[1], Condition: raging.Condition}))
	taken, sources := s.wardDamage(w.bus, damage.Slashing)
	s.Equal(4, taken)
	s.Len(sources, 2, "resistance sources do not quarter damage")
	s.Require().NoError(dndevents.DamageTakenTopic.On(w.bus).Publish(ctx, &dndevents.DamageTakenEvent{MemberID: "caster", Amount: 10, DroppedToZero: true}))
	taken, sources = s.wardDamage(w.bus, damage.Slashing)
	s.Equal(4, taken)
	s.Require().Len(sources, 1)
	s.Equal("one", sources[0].Source.SourceID)
	s.Require().Len(w.sheets[1].GetConditions(), 1)
	s.Equal(refs.Conditions.Raging().String(), w.sheets[1].GetConditions()[0].Ref().String())
	s.Require().NoError(w.binding.CheckComplete())
	s.Require().NoError(w.binding.Close(ctx))
}

func (s *applicationBindingSuite) TestGenericSourceDoesNotPretendToBeASpell() {
	w := s.ward(true)
	ctx := context.Background()
	info, err := w.owner.MaintenanceInfo()
	s.Require().NoError(err)
	s.Require().NotNil(info.Concentration)
	s.Equal("example:items:ward-ring", info.Concentration.SourceRef.String())
	s.Nil(info.Duration, "no invented clock")
	var ended []dndevents.ConcentrationEndedEvent
	_, err = dndevents.ConcentrationEndedTopic.On(w.bus).Subscribe(ctx, func(_ context.Context, event dndevents.ConcentrationEndedEvent) error {
		ended = append(ended, event)
		return nil
	})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.DamageTakenTopic.On(w.bus).Publish(ctx, &dndevents.DamageTakenEvent{MemberID: "caster", Amount: 10, DroppedToZero: true}))
	s.Require().Len(ended, 1)
	s.Equal("example:items:ward-ring", ended[0].SourceRef.String())
	s.Equal("Shared physical ward", ended[0].SourceName)
	s.Equal("caster", ended[0].MaintainerID)
	s.Require().NoError(w.binding.Close(ctx))
}
