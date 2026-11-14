// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type monsterContributionOrderSuite struct {
	suite.Suite
	ctx     context.Context
	bus     events.EventBus
	binding *conditions.ApplicationBinding
	holders []*monster.Monster
}

func TestMonsterContributionOrderSuite(t *testing.T) {
	suite.Run(t, new(monsterContributionOrderSuite))
}
func (s *monsterContributionOrderSuite) SetupTest() {
	s.ctx = context.Background()
	for _, id := range []string{"z-old", "a-new", "bane", "recipient"} {
		m, err := LoadMonster(s.ctx, goblinData(id))
		s.Require().NoError(err)
		s.holders = append(s.holders, m)
	}
	s.bind()
}
func (s *monsterContributionOrderSuite) TearDownTest() { s.close() }
func (s *monsterContributionOrderSuite) participants() []conditions.ParticipantConditions {
	result := make([]conditions.ParticipantConditions, 0, len(s.holders))
	for _, holder := range s.holders {
		result = append(result, conditions.ParticipantConditions{MemberID: holder.GetID(), Conditions: holder.GetConditions()})
	}
	return result
}
func (s *monsterContributionOrderSuite) holder(id string) *monster.Monster {
	for _, holder := range s.holders {
		if holder.GetID() == id {
			return holder
		}
	}
	s.Require().FailNow("fixture holder missing", id)
	return nil
}
func (s *monsterContributionOrderSuite) bind() {
	s.bus = events.NewEventBus()
	binding, err := conditions.BindApplications(&conditions.BindApplicationsInput{Participants: s.participants(), Bus: s.bus})
	s.Require().NoError(err)
	s.binding = binding
	for _, holder := range s.holders {
		s.Require().NoError(AttachMonster(s.ctx, holder, s.bus, nil))
	}
}
func (s *monsterContributionOrderSuite) close() {
	if s.binding == nil {
		return
	}
	s.NoError(s.binding.Close(s.ctx))
	for _, holder := range s.holders {
		for _, effect := range holder.GetConditions() {
			s.NoError(effect.Remove(s.ctx, s.bus))
		}
		s.NoError(holder.SheetKeeper().Remove(s.ctx, s.bus))
	}
	s.binding = nil
}
func (s *monsterContributionOrderSuite) reload() {
	s.Require().NoError(s.binding.CheckComplete())
	saved := make([]*monster.Data, 0, len(s.holders))
	for _, holder := range s.holders {
		saved = append(saved, holder.ToData())
	}
	s.close()
	s.holders = nil
	for _, data := range saved {
		loaded, err := LoadMonster(s.ctx, data)
		s.Require().NoError(err)
		s.holders = append(s.holders, loaded)
	}
	s.bind()
}

type monsterRollGrantInput struct {
	Source           string
	Condition, Spell *core.Ref
}

func (s *monsterContributionOrderSuite) grant(in monsterRollGrantInput) {
	maintenance, err := effects.CompileParts(&effects.CompilePartsInput{Parts: []effects.PartSpec{
		{Key: "concentration", Effect: effects.Concentration{}, OnTerminal: generic.EndApplication},
		{Key: "duration", Effect: effects.Duration{TurnEnds: 5}, OnTerminal: generic.EndApplication},
	}})
	s.Require().NoError(err)
	app := "application-" + in.Source
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: app, CoordinatorID: in.Source, Origin: dndevents.ConditionOrigin{Ref: *in.Spell, EntityID: in.Source}, Existing: s.participants(), Spec: conditions.ApplicationSpec{
		Name: in.Source, CoordinatorParts: maintenance.Parts, Recipients: []conditions.RecipientSpec{{MemberID: "recipient", Ref: in.Condition.String(), AddressSourceID: in.Source, Lifetime: conditions.LifetimeShared}},
	}})
	s.Require().NoError(err)
	owner, err := conditions.CreateApplication(&conditions.CreateApplicationInput{Prepared: prepared})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dndevents.ConditionAppliedEvent{Target: s.holder(in.Source), Condition: owner.Condition, ApplicationID: app}))
	recipient, err := conditions.CreateRecipient(&conditions.CreateRecipientInput{Prepared: prepared, SpecIndex: 0})
	s.Require().NoError(err)
	s.Require().NoError(dndevents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dndevents.ConditionAppliedEvent{Target: s.holder("recipient"), Condition: recipient.Condition, ApplicationID: app}))
	_, err = s.binding.FinalizeRecipient(s.ctx, &conditions.FinalizeRecipientInput{ApplicationID: app, Key: recipient.Candidate.Key, Delivered: true})
	s.Require().NoError(err)
	s.Require().NoError(s.binding.CheckComplete())
}
func (s *monsterContributionOrderSuite) selected() []string {
	out, err := s.holder("recipient").DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindAttack})
	s.Require().NoError(err)
	var result []string
	for _, term := range out.Contributions {
		result = append(result, term.Source.SourceID)
	}
	return result
}
func (s *monsterContributionOrderSuite) remaining(source string) int {
	held := s.holder(source).GetConditions()
	s.Require().Len(held, 1)
	linked, ok := held[0].(*conditions.LinkedCondition)
	s.Require().True(ok)
	info, err := linked.MaintenanceInfo()
	s.Require().NoError(err)
	s.Require().NotNil(info.Duration)
	return info.Duration.Remaining
}

// This is full linked custody around a monster storage/attachment test, not a
// claim that this goblin fixture acquired a native spellcasting feature.
func (s *monsterContributionOrderSuite) TestReloadAndDifferentSubscriptionOrderPreserveAge() {
	s.grant(monsterRollGrantInput{Source: "z-old", Condition: refs.Conditions.Blessed(), Spell: refs.Spells.Bless()})
	s.grant(monsterRollGrantInput{Source: "a-new", Condition: refs.Conditions.Blessed(), Spell: refs.Spells.Bless()})
	s.grant(monsterRollGrantInput{Source: "bane", Condition: refs.Conditions.Baned(), Spell: refs.Spells.Bane()})
	s.Equal([]string{"z-old", "bane"}, s.selected())
	s.Require().NoError(dndevents.TurnEndTopic.On(s.bus).Publish(s.ctx, dndevents.TurnEndEvent{SubjectID: "a-new"}))
	s.Equal(4, s.remaining("a-new"))
	s.reload()
	current := s.holder("recipient").GetConditions()
	for _, condition := range current {
		s.Require().NoError(condition.Remove(s.ctx, s.bus))
	}
	for i := len(current) - 1; i >= 0; i-- {
		s.Require().NoError(current[i].Apply(s.ctx, dndevents.BusForEffect(s.bus, refOf(current[i]))))
	}
	s.Equal([]string{"z-old", "bane"}, s.selected(), "runtime registration order is not durable age")
	owner := conditions.ConditionAddressOf("z-old", s.holder("z-old").GetConditions()[0])
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dndevents.ConditionRemovedEvent{MemberID: owner.MemberID, ConditionRef: owner.ConditionRef, SourceID: owner.SourceID, ApplicationID: owner.SourceID, Reason: "concentration_failed"}))
	s.Equal([]string{"a-new", "bane"}, s.selected())
	s.Equal(4, s.remaining("a-new"), "selection handoff does not refresh remaining duration")
	s.Require().NoError(s.binding.CheckComplete())
	s.reload()
	s.Equal([]string{"a-new", "bane"}, s.selected())
	s.Equal(4, s.remaining("a-new"))
}
