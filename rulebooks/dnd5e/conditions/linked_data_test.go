// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	dndfx "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type linkedDataSuite struct{ suite.Suite }

func TestLinkedDataSuite(t *testing.T) { suite.Run(t, new(linkedDataSuite)) }

func linkedOwnerConfig() LinkedConditionConfig {
	return LinkedConditionConfig{ApplicationID: "application-1", Role: LinkedCoordinator, AddressSourceID: "application-1",
		Bindings: &LinkedBindings{Name: "Bless", Parts: []dndfx.PartDeclaration{
			{Key: "hold", Kind: dndfx.KindConcentration, OnTerminal: generic.EndApplication},
			{Key: "duration", Kind: dndfx.KindDuration, Params: json.RawMessage(`{"turn_ends":10,"skip_first":true}`), OnTerminal: generic.EndApplication},
		}, Members: []LinkedMemberBinding{{Key: "recipient", Address: dndevents.ConditionAddress{
			MemberID: "recipient", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "caster-a",
		}, Mode: LinkComposed}}}}
}

func linkedRecipientConfig() LinkedConditionConfig {
	return LinkedConditionConfig{ApplicationID: "application-1", Role: LinkedRecipient, AddressSourceID: "caster-a",
		Bindings: &LinkedBindings{Name: "Blessed", MemberKey: "recipient", Owner: &dndevents.ConditionAddress{
			MemberID: "caster-a", ConditionRef: refs.Conditions.LinkedEffects().String(), SourceID: "application-1",
		}, Parts: []dndfx.PartDeclaration{
			{Key: "bless", Kind: dndfx.KindAddToRoll, Amount: &generic.Amount{Dice: "1d4"}, Params: json.RawMessage(`{"polarity":"positive","rolls":["attack","saving_throw"],"selection":{"mode":"oldest_applicable","group":"bless"}}`), OnTerminal: generic.EndPart},
		}}}
}

func (s *linkedDataSuite) create(member string, config LinkedConditionConfig) *LinkedCondition {
	return s.createOrigin(member, config, "caster-a")
}

func (s *linkedDataSuite) createOrigin(member string, config LinkedConditionConfig, source string) *LinkedCondition {
	raw, err := json.Marshal(config)
	s.Require().NoError(err)
	created, err := CreateFromRef(&CreateFromRefInput{Ref: refs.Conditions.LinkedEffects().String(), MemberID: member,
		Origin: &dndevents.ConditionOrigin{Ref: *refs.Spells.Bless(), EntityID: source}, Config: raw})
	s.Require().NoError(err)
	linked, ok := created.Condition.(*LinkedCondition)
	s.Require().True(ok)
	return linked
}

func (s *linkedDataSuite) data(condition *LinkedCondition) LinkedConditionData {
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	var data LinkedConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	return data
}

func (s *linkedDataSuite) load(data LinkedConditionData) (*LinkedCondition, error) {
	raw, err := json.Marshal(data)
	s.Require().NoError(err)
	condition, err := LoadJSON(raw)
	if err != nil {
		return nil, err
	}
	linked, ok := condition.(*LinkedCondition)
	s.Require().True(ok)
	return linked, nil
}

func (s *linkedDataSuite) TestOwnerAndRecipientRoundTripWithoutDuplicatedState() {
	owner := s.create("caster-a", linkedOwnerConfig())
	s.Empty(owner.lifetime.Snapshot().LiveMembers)
	s.Equal([]string{"recipient"}, owner.lifetime.Snapshot().PendingMembers)
	_, err := owner.lifetime.FinalizeMember(generic.FinalizeMemberInput{Key: "recipient", Delivered: true})
	s.Require().NoError(err)
	recipient := s.create("recipient", linkedRecipientConfig())
	ownerData, recipientData := s.data(owner), s.data(recipient)
	s.Require().NotNil(ownerData.Runtime.Lifetime)
	s.Nil(recipientData.Runtime.Lifetime)
	s.Equal([]string{"recipient"}, ownerData.Runtime.Lifetime.LiveMembers)
	s.Require().Len(ownerData.Runtime.Parts.Parts, 2)
	s.Require().Len(recipientData.Runtime.Parts.Parts, 1)
	snapshot := ownerData.Runtime.Parts.Parts["duration"]
	snapshot.State = "active"
	snapshot.Bag["remaining"] = 7
	ownerData.Runtime.Parts.Parts["duration"] = snapshot
	loaded, err := s.load(ownerData)
	s.Require().NoError(err)
	info, err := loaded.MaintenanceInfo()
	s.Require().NoError(err)
	s.Require().NotNil(info.Concentration)
	s.Equal("dnd5e:spells:bless", info.Concentration.SourceRef.String())
	s.Require().NotNil(info.Duration)
	s.Equal(7, info.Duration.Remaining)
	s.False(info.Duration.SkipNext)
	reloaded, err := s.load(recipientData)
	s.Require().NoError(err)
	s.Equal(recipient.ConditionAddress(), reloaded.ConditionAddress())
	rows, err := reloaded.DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindAttack})
	s.Require().NoError(err)
	s.Require().Len(rows.Contributions, 1)
	s.Equal("1d4", rows.Contributions[0].Dice)
}

func (s *linkedDataSuite) TestMissingForeignDuplicateAndWrongRoleStateIsRefused() {
	valid := s.data(s.create("caster-a", linkedOwnerConfig()))
	for _, change := range []func(*LinkedConditionData){
		func(data *LinkedConditionData) { data.Runtime = nil },
		func(data *LinkedConditionData) { data.Bindings = nil },
		func(data *LinkedConditionData) { data.Origin = nil },
		func(data *LinkedConditionData) { data.ApplicationID = "" },
		func(data *LinkedConditionData) { data.Role = "unknown" },
		func(data *LinkedConditionData) { data.AddressSourceID = "different-application" },
		func(data *LinkedConditionData) { data.Runtime.Lifetime = nil },
		func(data *LinkedConditionData) { data.Runtime.Lifetime.LiveMembers = []string{"foreign"} },
		func(data *LinkedConditionData) {
			data.Bindings.Parts = append(data.Bindings.Parts, data.Bindings.Parts[0])
		},
		func(data *LinkedConditionData) { delete(data.Runtime.Parts.Parts, "duration") },
		func(data *LinkedConditionData) { data.Role = LinkedRecipient },
	} {
		raw, err := json.Marshal(valid)
		s.Require().NoError(err)
		var data LinkedConditionData
		s.Require().NoError(json.Unmarshal(raw, &data))
		change(&data)
		got, err := s.load(data)
		s.Error(err)
		s.Nil(got)
	}
}

func (s *linkedDataSuite) TestOriginAddressAndApplicationStayDistinct() {
	condition := s.create("recipient", linkedRecipientConfig())
	info := condition.LinkedApplicationInfo()
	s.Equal("application-1", info.ID)
	s.Equal("caster-a", condition.ConditionAddress().SourceID)
	s.Equal("recipient", condition.ConditionAddress().MemberID)
	s.Equal("caster-a", condition.ConditionOrigin().EntityID)
	info.Owner.MemberID = "edited"
	s.Equal("caster-a", condition.LinkedApplicationInfo().Owner.MemberID)
	data := s.data(condition)
	data.Bindings.Parts[0].Params[0] = '!'
	_, err := condition.ToJSON()
	s.Require().NoError(err, "changing caller-owned data cannot mutate the condition")
}

func (s *linkedDataSuite) TestRecipientOnlyInformationDoesNotInventDuration() {
	condition := s.create("recipient", linkedRecipientConfig())
	info, err := condition.MaintenanceInfo()
	s.Require().NoError(err)
	s.Nil(info.Concentration)
	s.Nil(info.Duration)
	described, err := DescribeCondition(&DescribeConditionInput{Condition: condition})
	s.Require().NoError(err)
	s.Equal("Blessed", described.Display.Name)
	s.Nil(described.Duration)
	s.Equal("caster-a", described.Origin.EntityID)
}

func (s *linkedDataSuite) TestEachPartKeepsItsGroupInExecutionAndInformation() {
	mixed := linkedRecipientConfig()
	mixed.Bindings.Name = "Mixed ward"
	mixed.Bindings.Parts = append(mixed.Bindings.Parts, dndfx.PartDeclaration{Key: "bane", Kind: dndfx.KindAddToRoll, Amount: &generic.Amount{Dice: "1d4"},
		Params: json.RawMessage(`{"polarity":"negative","rolls":["attack","saving_throw"],"selection":{"mode":"oldest_applicable","group":"bane"}}`), OnTerminal: generic.EndPart})
	first := s.create("recipient", mixed)
	other := linkedRecipientConfig()
	other.ApplicationID = "application-2"
	other.AddressSourceID = "caster-b"
	other.Bindings.Owner.MemberID = "caster-b"
	other.Bindings.Owner.SourceID = "application-2"
	second := s.createOrigin("recipient", other, "caster-b")
	held := []dndevents.ConditionBehavior{first, second}
	dice, err := DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{Conditions: held,
		Request: &dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindAttack}})
	s.Require().NoError(err)
	s.Require().Len(dice.Contributions, 2)
	s.False(dice.Contributions[0].Subtract)
	s.True(dice.Contributions[1].Subtract)
	rows, err := AssessActionEffects(&AssessActionEffectsInput{Conditions: held,
		Frame: contributions.Frame{Actor: "recipient", Action: contributions.ActionFacts{Roll: contributions.Known(contributions.RollKindAttack)}}})
	s.Require().NoError(err)
	s.Require().Len(rows.Effects, 3)
	s.Equal(contributions.StateApplies, rows.Effects[0].State)
	s.Equal(contributions.StateApplies, rows.Effects[1].State)
	s.Equal(contributions.StateDoesNotApply, rows.Effects[2].State)
	s.Empty(rows.Effects[2].Benefit)
	dice, err = DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{Conditions: []dndevents.ConditionBehavior{second},
		Request: &dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindAttack}})
	s.Require().NoError(err)
	s.Require().Len(dice.Contributions, 1)
	s.Equal("caster-b", dice.Contributions[0].Source.SourceID)
}
