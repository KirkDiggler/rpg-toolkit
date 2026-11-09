// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

type ConcentrationKeeperSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestConcentrationKeeperSuite(t *testing.T) { suite.Run(t, new(ConcentrationKeeperSuite)) }
func (s *ConcentrationKeeperSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func (s *ConcentrationKeeperSuite) bardData(carry ...dndevents.ConditionBehavior) *Data {
	var blobs []json.RawMessage
	for _, condition := range carry {
		raw, err := condition.ToJSON()
		s.Require().NoError(err)
		blobs = append(blobs, raw)
	}
	return &Data{ID: "bard-1", PlayerID: "player-1", Name: "Keeper", Level: 1, ProficiencyBonus: 2, RaceID: races.Human, ClassID: classes.Bard,
		AbilityScores: shared.AbilityScores{abilities.STR: 10, abilities.DEX: 14, abilities.CON: 12, abilities.INT: 10, abilities.WIS: 10, abilities.CHA: 16},
		HitPoints:     9, MaxHitPoints: 9, EquipmentSlots: EquipmentSlots{}, Conditions: blobs}
}

func (s *ConcentrationKeeperSuite) bind(members ...*Character) *conditions.ApplicationBinding {
	var participants []conditions.ParticipantConditions
	for _, member := range members {
		participants = append(participants, conditions.ParticipantConditions{MemberID: member.GetID(), Conditions: member.GetConditions()})
	}
	binding, err := conditions.BindApplications(&conditions.BindApplicationsInput{Participants: participants, Bus: s.bus})
	s.Require().NoError(err)
	for _, member := range members {
		s.Require().NoError(Attach(s.ctx, member, s.bus))
	}
	s.T().Cleanup(func() { s.NoError(binding.Close(s.ctx)) })
	return binding
}

func (s *ConcentrationKeeperSuite) damage(member string, amount int) *dndevents.DamageTakenEvent {
	event := &dndevents.DamageTakenEvent{MemberID: member, Amount: amount}
	s.Require().NoError(dndevents.DamageTakenTopic.On(s.bus).Publish(s.ctx, event))
	return event
}

func (s *ConcentrationKeeperSuite) attack(actor, target string) dndevents.AttackChainEvent {
	event := dndevents.AttackChainEvent{AttackerID: actor, TargetID: target, Frame: contributions.Frame{Actor: actor, Target: contributions.Known(target), Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)}}}
	chain, err := dndevents.AttackChain.On(s.bus).PublishWithChain(s.ctx, event, events.NewStagedChain[dndevents.AttackChainEvent](combat.ModifierStages))
	s.Require().NoError(err)
	out, err := chain.Execute(s.ctx, event)
	s.Require().NoError(err)
	return out
}

func (s *ConcentrationKeeperSuite) TestAPrunedConditionIsUnsubscribed() {
	hold, err := currentHoldFixture(holdFixtureInput{Member: "bard-1", Source: refs.Spells.TrueStrike(), Name: conditions.TrueStrikeName, Turns: 2})
	s.Require().NoError(err)
	loaded, err := Load(s.ctx, s.bardData(hold))
	s.Require().NoError(err)
	s.bind(loaded)
	s.Require().NotEmpty(s.damage("bard-1", 12).FollowUps)
	address := hold.ConditionAddress()
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dndevents.ConditionRemovedEvent{MemberID: address.MemberID, ConditionRef: address.ConditionRef, SourceID: address.SourceID, Reason: conditions.ConcentrationEndedRecast}))
	s.Empty(authored(loaded))
	s.Empty(s.damage("bard-1", 12).FollowUps, "removal detaches the actual save-request subscriber")
}

func (s *ConcentrationKeeperSuite) twoBaneOwners() (*Character, *Character, *Character) {
	a, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Baned(), MemberID: "target-1", SourceID: "bard-a", SourceRef: refs.Spells.Bane()})
	s.Require().NoError(err)
	b, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Baned(), MemberID: "target-1", SourceID: "bard-b", SourceRef: refs.Spells.Bane()})
	s.Require().NoError(err)
	var owners []*Character
	for _, item := range []struct {
		id    string
		child *conditions.LinkedCondition
	}{{"bard-a", a}, {"bard-b", b}} {
		hold, err := currentHoldFixture(holdFixtureInput{Member: item.id, Source: refs.Spells.Bane(), Name: "Bane", Turns: 10, Skip: true,
			Members: []conditions.LinkedMemberBinding{{Key: "target-1", Address: item.child.ConditionAddress(), Mode: conditions.LinkComposed}}})
		s.Require().NoError(err)
		data := s.bardData(hold)
		data.ID = item.id
		loaded, err := Load(s.ctx, data)
		s.Require().NoError(err)
		owners = append(owners, loaded)
	}
	data := s.bardData(a, b)
	data.ID = "target-1"
	target, err := Load(s.ctx, data)
	s.Require().NoError(err)
	target, err = Load(s.ctx, mustToData(s.T(), target))
	s.Require().NoError(err)
	s.bind(owners[0], owners[1], target)
	return owners[0], owners[1], target
}

func (s *ConcentrationKeeperSuite) assertOtherOwnerPreserved(a, b, target *Character) {
	s.Empty(authored(a))
	s.Require().Len(authored(b), 1)
	out, err := target.DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindSavingThrow})
	s.Require().NoError(err)
	s.Require().Len(out.Contributions, 1)
	s.Equal("bard-b", out.Contributions[0].Source.SourceID)
	owner := authored(b)[0].(*conditions.LinkedCondition)
	info, err := owner.MaintenanceInfo()
	s.Require().NoError(err)
	s.Equal(10, info.Duration.Remaining, "the other owner's clock is neither reset nor consumed")
	s.Equal([]string{"target-1"}, owner.LinkedApplicationInfo().LiveMembers)
}

func (s *ConcentrationKeeperSuite) TestTwoRealBaneOwnersRemoveOnlyTheirQualifiedChildren() {
	a, b, target := s.twoBaneOwners()
	before, err := target.DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindAttack})
	s.Require().NoError(err)
	s.Require().Len(before.Contributions, 1)
	s.Equal("bard-a", before.Contributions[0].Source.SourceID)
	owner := authored(a)[0].(*conditions.LinkedCondition).ConditionAddress()
	s.Require().NoError(dndevents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dndevents.ConditionRemovedEvent{MemberID: owner.MemberID, ConditionRef: owner.ConditionRef, SourceID: owner.SourceID, Reason: conditions.ConcentrationEndedRecast}))
	s.assertOtherOwnerPreserved(a, b, target)
}

func (s *ConcentrationKeeperSuite) TestLongRestRemovesOnlyTheQualifiedBaneOwnerAndChildren() {
	a, b, target := s.twoBaneOwners()
	s.Require().NoError(restErr(a.LongRest(s.ctx)))
	s.assertOtherOwnerPreserved(a, b, target)
}

func (s *ConcentrationKeeperSuite) TestWrongBaneSourceDoesNotRemoveOrDirtyCharacter() {
	bane, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Baned(), MemberID: "bard-1", SourceID: "bard-a", SourceRef: refs.Spells.Bane()})
	s.Require().NoError(err)
	loaded, err := Load(s.ctx, s.bardData(bane))
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, s.bus))
	s.False(loaded.IsDirty())
	for _, source := range []string{"bard-b", ""} {
		s.Require().NoError(dndevents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dndevents.ConditionRemovedEvent{MemberID: "bard-1", ConditionRef: refs.Conditions.Baned().String(), SourceID: source}))
		s.Require().NotEmpty(authored(loaded))
		s.False(loaded.IsDirty(), "exact identity is never a wildcard")
	}
}

func (s *ConcentrationKeeperSuite) TestASelfEndingConditionIsStillDroppedCleanly() {
	child := conditions.NewTrueStrikeCondition("bard-1", "goblin-1", "")
	loaded, err := Load(s.ctx, s.bardData(child))
	s.Require().NoError(err)
	s.Require().NoError(Attach(s.ctx, loaded, s.bus))
	s.Require().NotEmpty(s.attack("bard-1", "goblin-1").AdvantageSources)
	s.Empty(authored(loaded))
	s.Empty(s.attack("bard-1", "goblin-1").AdvantageSources)
}

func (s *ConcentrationKeeperSuite) TestACharacterHoldingNothingSaysSo() {
	loaded, err := Load(s.ctx, s.bardData())
	s.Require().NoError(err)
	view, err := loaded.Concentration()
	s.Require().NoError(err)
	s.False(view.Held)
}

func (s *ConcentrationKeeperSuite) TestACasterAnswersWhichSpellItIsHolding() {
	child := dndevents.ConditionAddress{MemberID: "bard-1", ConditionRef: refs.Conditions.TrueStrike().String()}
	hold, err := currentHoldFixture(holdFixtureInput{Member: "bard-1", Source: refs.Spells.TrueStrike(), Name: conditions.TrueStrikeName, Turns: 2,
		Members: []conditions.LinkedMemberBinding{{Key: "strike", Address: child, Mode: conditions.LinkAdopted}}})
	s.Require().NoError(err)
	loaded, err := Load(s.ctx, s.bardData(hold))
	s.Require().NoError(err)
	view, err := loaded.Concentration()
	s.Require().NoError(err)
	s.True(view.Held)
	s.Equal(refs.Spells.TrueStrike().String(), view.SourceRef)
	s.Equal(conditions.TrueStrikeName, view.SourceName)
	s.Equal(1, view.ChildCount)
}

func (s *ConcentrationKeeperSuite) TestAHoldEndedByAFactStripsItsChildren() {
	for _, reason := range []string{conditions.ConcentrationEndedRecast, conditions.ConcentrationEndedDamage} {
		s.Run(reason, func() {
			s.SetupTest()
			child := conditions.NewTrueStrikeCondition("bard-1", "goblin-1", "")
			address := conditions.ConditionAddressOf("bard-1", child)
			hold, err := currentHoldFixture(holdFixtureInput{Member: "bard-1", Source: refs.Spells.TrueStrike(), Name: conditions.TrueStrikeName, Turns: 2,
				Members: []conditions.LinkedMemberBinding{{Key: "strike", Address: address, Mode: conditions.LinkAdopted}}})
			s.Require().NoError(err)
			loaded, err := Load(s.ctx, s.bardData(hold, child))
			s.Require().NoError(err)
			var ended []dndevents.ConcentrationEndedEvent
			_, err = dndevents.ConcentrationEndedTopic.On(s.bus).Subscribe(s.ctx, func(_ context.Context, e dndevents.ConcentrationEndedEvent) error {
				ended = append(ended, e)
				return nil
			})
			s.Require().NoError(err)
			s.bind(loaded)
			owner := hold.ConditionAddress()
			s.Require().NoError(dndevents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dndevents.ConditionRemovedEvent{MemberID: owner.MemberID, ConditionRef: owner.ConditionRef, SourceID: owner.SourceID, Reason: reason}))
			s.Require().Len(ended, 1)
			s.Equal(reason, ended[0].Reason)
			s.Equal([]dndevents.ConditionAddress{address}, ended[0].Removed)
			s.Empty(authored(loaded), "nested removal must not restore the child")
			s.Empty(s.attack("bard-1", "goblin-1").AdvantageSources)
			s.Empty(s.damage("bard-1", 12).FollowUps)
		})
	}
}
