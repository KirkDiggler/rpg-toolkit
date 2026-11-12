// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"testing"

	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/suite"
)

type BlessedSuite struct{ suite.Suite }

func TestBlessedSuite(t *testing.T) { suite.Run(t, new(BlessedSuite)) }

func (s *BlessedSuite) condition(source string) *LinkedCondition {
	condition, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Blessed(), MemberID: "target", SourceID: source, SourceRef: refs.Spells.Bless()})
	s.Require().NoError(err)
	return condition
}

func (s *BlessedSuite) TestRoundTripAndDisplay() {
	condition := s.condition("cleric-a")
	s.Equal(*refs.Conditions.Blessed(), *condition.Ref())
	s.Equal("dnd5e:conditions:blessed", condition.Ref().String())
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	s.NotContains(string(raw), "face")
	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)
	s.Equal(condition.ConditionAddress(), ConditionAddressOf("target", loaded))
	back, ok := loaded.(*LinkedCondition)
	s.Require().True(ok)
	s.Equal(*refs.Spells.Bless(), back.ConditionOrigin().Ref)
	display, ok := DisplayFor(*condition.Ref())
	s.Require().True(ok)
	s.Equal("Blessed", display.Name)
}

func (s *BlessedSuite) TestRequiresRecipientAndValidOrigin() {
	for _, input := range []namedFixtureInput{
		{Ref: refs.Conditions.Blessed(), SourceID: "cleric", SourceRef: refs.Spells.Bless()},
		{Ref: refs.Conditions.Blessed(), MemberID: "target", SourceRef: refs.Spells.Bless()},
		{Ref: refs.Conditions.Blessed(), MemberID: "target", SourceID: "cleric"},
	} {
		_, err := namedFixture(input)
		s.Error(err)
	}
}

func (s *BlessedSuite) TestAddsAnUnresolvedDieToAttacksAndSavesOnly() {
	condition := s.condition("cleric-a")
	provider := condition.RollContributors()[0]
	for _, kind := range []dndevents.RollKind{dndevents.RollKindAttack, dndevents.RollKindSavingThrow} {
		out, err := provider.DescribeRollContributions(&dndevents.DescribeRollContributionsInput{Kind: kind})
		s.Require().NoError(err)
		s.Equal([]dndevents.DiceContribution{{Source: dndevents.RollSource{Ref: refs.Spells.Bless(), Name: "Bless", SourceID: "cleric-a"}, Dice: "1d4"}}, out.Contributions)
	}
	for _, in := range []*dndevents.DescribeRollContributionsInput{nil, {Kind: "ability_check"}, {Kind: "damage"}} {
		out, err := provider.DescribeRollContributions(in)
		s.Require().NoError(err)
		s.Empty(out.Contributions)
	}
}

func (s *BlessedSuite) TestTwoBlessesDoNotStackAndBaneRemainsIndependent() {
	a, b := s.condition("cleric-a"), s.condition("cleric-b")
	bane, err := namedFixture(namedFixtureInput{Ref: refs.Conditions.Baned(), MemberID: "target", SourceID: "bard", SourceRef: refs.Spells.Bane()})
	s.Require().NoError(err)
	request := &dndevents.DescribeRollContributionsInput{Kind: dndevents.RollKindSavingThrow}
	out, err := DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{Conditions: []dndevents.ConditionBehavior{a, b, bane}, Request: request})
	s.Require().NoError(err)
	s.Require().Len(out.Contributions, 2)
	s.Equal("cleric-a", out.Contributions[0].Source.SourceID)
	s.False(out.Contributions[0].Subtract)
	s.True(out.Contributions[1].Subtract)
	out, err = DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{Conditions: []dndevents.ConditionBehavior{b, bane}, Request: request})
	s.Require().NoError(err)
	s.Require().Len(out.Contributions, 2)
	s.Equal("cleric-b", out.Contributions[0].Source.SourceID, "removing one caster exposes the remaining Bless")
}
