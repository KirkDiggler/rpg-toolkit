// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/stretchr/testify/suite"
)

type BlessedCharacterSuite struct{ suite.Suite }

func TestBlessedCharacterSuite(t *testing.T) { suite.Run(t, new(BlessedCharacterSuite)) }

func (s *BlessedCharacterSuite) TestBlessChangesADeathSaveAfterJSONReload() {
	char := dyingCharacterForTurn(s.T(), &saves.DeathSaveState{})
	blessed, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
		MemberID: char.GetID(), SourceID: "cleric", SourceRef: refs.Spells.Bless(),
	})
	s.Require().NoError(err)
	char.conditions = append(char.conditions, blessed)
	raw, err := json.Marshal(char.ToData())
	s.Require().NoError(err)
	var data Data
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := Load(context.Background(), &data)
	s.Require().NoError(err)
	roller := &scriptedRoller{results: []int{9, 2}}
	out, err := loaded.MakeDeathSave(context.Background(), &MakeDeathSaveInput{Roller: roller})
	s.Require().NoError(err)
	s.Equal(DeathSaveOutcomeSuccess, out.Outcome)
	s.Equal(1, out.SuccessesAdded)
	s.Require().NotNil(out.Calculation)
	s.Equal(11, out.Calculation.Total)
	s.Equal([]int{20, 4}, roller.calls)
	s.Equal("cleric", out.Calculation.Components[1].Source.SourceID)
}

func (s *ConcentrationKeeperSuite) TestBlessOwnersCleanUpOnlyTheirOwnRecipientsAfterReload() {
	var owners []*Character
	var children []dnd5eEvents.ConditionBehavior
	for _, caster := range []string{"cleric-a", "cleric-b"} {
		child, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
			MemberID: "target", SourceID: caster, SourceRef: refs.Spells.Bless(),
		})
		s.Require().NoError(err)
		children = append(children, child)
		hold := conditions.NewConcentratingConditionWithInput(conditions.NewConcentratingConditionInput{
			MemberID: caster, SourceID: caster, SpellRef: refs.Spells.Bless().String(), SpellName: "Bless",
			TurnEnds: 10, SkipFirstTurnEnd: true,
		})
		s.Require().NoError(hold.AddChild(s.ctx, child.ConditionAddress()))
		data := s.bardData(hold)
		data.ID = caster
		owner, err := Load(s.ctx, data)
		s.Require().NoError(err)
		owners = append(owners, owner)
	}
	data := s.bardData(children...)
	data.ID = "target"
	target, err := Load(s.ctx, data)
	s.Require().NoError(err)
	target, err = Load(s.ctx, target.ToData())
	s.Require().NoError(err)
	for _, member := range append(owners, target) {
		s.Require().NoError(Attach(s.ctx, member, s.bus))
	}
	s.Require().NoError(dnd5eEvents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID: "cleric-a", ConditionRef: refs.Conditions.Concentrating().String(), SourceID: "cleric-a",
		Reason: conditions.ConcentrationEndedRecast,
	}))
	s.Empty(owners[0].GetConditions())
	s.Require().Len(owners[1].GetConditions(), 1)
	s.Require().Len(target.GetConditions(), 1)
	out, err := target.DescribeRollContributions(&dnd5eEvents.DescribeRollContributionsInput{Kind: dnd5eEvents.RollKindAttack})
	s.Require().NoError(err)
	s.Require().Len(out.Contributions, 1)
	s.Equal("cleric-b", out.Contributions[0].Source.SourceID)
	s.False(out.Contributions[0].Subtract)
}
