// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"testing"

	"github.com/stretchr/testify/suite"

	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type BlessedSuite struct{ suite.Suite }

func TestBlessedSuite(t *testing.T) { suite.Run(t, new(BlessedSuite)) }

func (s *BlessedSuite) condition(source string) *BlessedCondition {
	s.T().Helper()
	condition, err := NewBlessedCondition(NewBlessedConditionInput{
		MemberID: "target", SourceID: source, SourceRef: refs.Spells.Bless(),
	})
	s.Require().NoError(err)
	return condition
}

func (s *BlessedSuite) TestRoundTripAndDisplay() {
	condition := s.condition("cleric-a")
	s.Same(refs.Conditions.Blessed(), condition.Ref())
	s.Equal("dnd5e:conditions:blessed", condition.Ref().String())
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	s.NotContains(string(raw), "face")
	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)
	s.Equal(condition.ConditionAddress(), ConditionAddressOf("target", loaded))
	back, ok := loaded.(*BlessedCondition)
	s.Require().True(ok)
	s.Equal(refs.Spells.Bless(), back.SourceRef)
	display, ok := DisplayFor(*condition.Ref())
	s.Require().True(ok)
	s.Equal("Blessed", display.Name)
}

func (s *BlessedSuite) TestRequiresRecipientAndCanonicalSource() {
	for _, input := range []NewBlessedConditionInput{
		{SourceID: "cleric", SourceRef: refs.Spells.Bless()},
		{MemberID: "target", SourceRef: refs.Spells.Bless()},
		{MemberID: "target", SourceID: "cleric"},
		{MemberID: "target", SourceID: "cleric", SourceRef: refs.Spells.Bane()},
	} {
		_, err := NewBlessedCondition(input)
		s.Error(err)
	}
}

func (s *BlessedSuite) TestAddsAnUnresolvedDieToAttacksAndSavesOnly() {
	condition := s.condition("cleric-a")
	for _, kind := range []dnd5eEvents.RollKind{dnd5eEvents.RollKindAttack, dnd5eEvents.RollKindSavingThrow} {
		out, err := condition.DescribeRollContributions(&dnd5eEvents.DescribeRollContributionsInput{Kind: kind})
		s.Require().NoError(err)
		s.Equal([]dnd5eEvents.DiceContribution{{
			Source: dnd5eEvents.RollSource{Ref: refs.Spells.Bless(), Name: "Bless", SourceID: "cleric-a"}, Dice: "1d4",
		}}, out.Contributions)
	}
	for _, input := range []*dnd5eEvents.DescribeRollContributionsInput{nil, {Kind: "ability_check"}, {Kind: "damage"}} {
		out, err := condition.DescribeRollContributions(input)
		s.Require().NoError(err)
		s.Empty(out.Contributions)
	}
}

func (s *BlessedSuite) TestTwoBlessesDoNotStackAndBaneRemainsIndependent() {
	a, b := s.condition("cleric-a"), s.condition("cleric-b")
	bane, err := NewBanedCondition(NewBanedConditionInput{MemberID: "target", SourceID: "bard", SourceRef: refs.Spells.Bane()})
	s.Require().NoError(err)
	request := &dnd5eEvents.DescribeRollContributionsInput{Kind: dnd5eEvents.RollKindSavingThrow}
	out, err := DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{
		Conditions: []dnd5eEvents.ConditionBehavior{a, b, bane}, Request: request,
	})
	s.Require().NoError(err)
	s.Require().Len(out.Contributions, 2)
	s.Equal("cleric-a", out.Contributions[0].Source.SourceID)
	s.False(out.Contributions[0].Subtract)
	s.True(out.Contributions[1].Subtract)
	out, err = DescribeSelectedRollContributions(&DescribeSelectedRollContributionsInput{
		Conditions: []dnd5eEvents.ConditionBehavior{b, bane}, Request: request,
	})
	s.Require().NoError(err)
	s.Require().Len(out.Contributions, 2)
	s.Equal("cleric-b", out.Contributions[0].Source.SourceID, "removing one caster exposes the remaining Bless")
}
