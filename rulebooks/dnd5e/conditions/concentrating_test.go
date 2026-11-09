// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// These are the retired concentrating runtime's gameplay assertions through the
// current parts, complete binding and real keepers. Old codec/constructor-shape
// assertions are replaced by explicit obsolete-format refusal in LinkedDataSuite.
func (s *applicationBindingSuite) TestConcentrationDamageRequestsAndDCFloor() {
	sheets := s.sheetsClock(10, true)
	ctx, bus := context.Background(), events.NewEventBus()
	binding := s.bind(sheets, bus)
	for _, fact := range []*dndevents.DamageTakenEvent{nil, {MemberID: "other", Amount: 12}, {MemberID: "caster"}, {MemberID: "caster", Amount: -1}} {
		s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(ctx, fact))
		if fact != nil {
			s.Empty(fact.FollowUps)
		}
	}
	for _, tc := range []struct{ damage, dc int }{{1, 10}, {6, 10}, {19, 10}, {20, 10}, {21, 10}, {22, 11}, {25, 12}, {100, 50}} {
		fact := &dndevents.DamageTakenEvent{MemberID: "caster", Amount: tc.damage}
		s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(ctx, fact))
		s.Require().Len(fact.FollowUps, 1)
		check := fact.FollowUps[0]
		s.Equal(abilities.CON, check.Ability)
		s.Equal(tc.dc, check.DC)
		s.Equal(refs.Spells.Bless().String(), check.Cause.EffectRef.String())
		s.Equal("app", check.OnFailure.Owner.SourceID)
		s.Empty(check.OnFailure.Remove, "the current consequence ends the owner, which owns child cleanup")
	}
	zero := &dndevents.DamageTakenEvent{MemberID: "caster", Amount: 5, DroppedToZero: true}
	s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(ctx, zero))
	s.Empty(zero.FollowUps)
	for _, sheet := range sheets {
		s.Empty(sheet.GetConditions())
	}
	s.Require().NoError(binding.Close(ctx))
}

func (s *applicationBindingSuite) TestEveryOwnedEndPathPublishesOneFact() {
	for _, reason := range []string{"damage", "recast", "duration", "combat_end", "rest", "caster_down", "spell_ended"} {
		s.Run(reason, func() {
			sheets := s.sheets()
			ctx, bus := context.Background(), events.NewEventBus()
			var ended []dndevents.ConcentrationEndedEvent
			_, err := dndevents.ConcentrationEndedTopic.On(bus).Subscribe(ctx, func(_ context.Context, event dndevents.ConcentrationEndedEvent) error {
				ended = append(ended, event)
				return nil
			})
			s.Require().NoError(err)
			binding := s.bind(sheets, bus)
			switch reason {
			case "damage", "recast":
				err = dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{MemberID: "caster", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "app", ApplicationID: "app", Reason: reason})
			case "duration":
				err = dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "caster"})
			case "combat_end":
				err = dndevents.CombatEndTopic.On(bus).Publish(ctx, dndevents.CombatEndEvent{SubjectID: "caster"})
			case "rest":
				err = dndevents.RestTopic.On(bus).Publish(ctx, dndevents.RestEvent{CharacterID: "caster", RestType: coreResources.ResetLongRest})
			case "caster_down":
				err = dndevents.DamageTakenTopic.On(bus).Publish(ctx, &dndevents.DamageTakenEvent{MemberID: "caster", Amount: 5, DroppedToZero: true})
			case "spell_ended":
				for _, id := range []string{"one", "two"} {
					s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{MemberID: id, ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "caster", ApplicationID: "app", Reason: "consumed"}))
				}
			}
			s.Require().NoError(err)
			s.Require().Len(ended, 1)
			s.Equal(reason, ended[0].Reason)
			s.Equal("caster", ended[0].CasterID)
			s.Equal(refs.Spells.Bless().String(), ended[0].SpellRef)
			for _, sheet := range sheets {
				s.Empty(sheet.GetConditions())
			}
			fact := &dndevents.DamageTakenEvent{MemberID: "caster", Amount: 12}
			s.Require().NoError(dndevents.DamageTakenTopic.On(bus).Publish(ctx, fact))
			s.Empty(fact.FollowUps)
			s.Require().NoError(binding.CheckComplete())
			s.Require().NoError(binding.Close(ctx))
		})
	}
}

func (s *applicationBindingSuite) TestClockCountsCasterEndsAndPreservesGraceAcrossReload() {
	sheets := s.sheetsClock(10, true)
	rounds := []int{4, 4, 100, 0, 1, 3, 3, 99, 2, 0, 8}
	for index, round := range rounds {
		ctx, bus := context.Background(), events.NewEventBus()
		binding := s.bind(sheets, bus)
		s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "unrelated", Round: round}))
		s.Require().Len(sheets[0].GetConditions(), 1)
		s.Require().NoError(dndevents.TurnEndTopic.On(bus).Publish(ctx, dndevents.TurnEndEvent{SubjectID: "caster", Round: round}))
		if index < 10 {
			s.Require().Len(sheets[0].GetConditions(), 1, "first boundary is grace, then ten actual ends")
			info, err := sheets[0].GetConditions()[0].(*conditions.LinkedCondition).MaintenanceInfo()
			s.Require().NoError(err)
			s.Equal(10-index, info.Duration.Remaining)
			s.False(info.Duration.SkipNext)
		} else {
			for _, sheet := range sheets {
				s.Empty(sheet.GetConditions())
			}
		}
		s.Require().NoError(binding.Close(ctx))
		var reloaded []*character.Character
		for _, sheet := range sheets {
			data, err := sheet.ToData()
			s.Require().NoError(err)
			loaded, err := character.Load(ctx, data)
			s.Require().NoError(err)
			reloaded = append(reloaded, loaded)
		}
		sheets = reloaded
	}
}

func (s *applicationBindingSuite) TestOtherRemovalCannotChangeHoldMembership() {
	sheets := s.sheetsClock(10, true)
	ctx, bus := context.Background(), events.NewEventBus()
	binding := s.bind(sheets, bus)
	owner := sheets[0].GetConditions()[0].(*conditions.LinkedCondition)
	before := owner.LinkedApplicationInfo().LiveMembers
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(bus).Publish(ctx, dndevents.ConditionRemovedEvent{MemberID: "one", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "other", Reason: "consumed"}))
	s.Equal(before, owner.LinkedApplicationInfo().LiveMembers)
	s.Require().NoError(binding.Close(ctx))
}
