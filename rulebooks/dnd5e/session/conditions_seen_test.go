// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ConditionSeamSuite holds the conditions answer the session gives the
// composition: what each member holds, read from the sheets this verb holds.
type ConditionSeamSuite struct {
	suite.Suite
	data  *SessionData
	chars *equipmentCharacters
	seam  equipmentSeam
}

func TestConditionSeamSuite(t *testing.T) { suite.Run(t, new(ConditionSeamSuite)) }

func (s *ConditionSeamSuite) SetupTest() {
	s.data = &SessionData{}
	s.chars = &equipmentCharacters{byID: map[string]*character.Data{}}
	s.seam = equipmentBeside(standingSeam{
		ctx: context.Background(), chars: s.chars, data: s.data,
		kinds: map[string]encounter.MemberKind{
			"player": encounter.KindPlayer, "ghost": encounter.KindPlayer,
			"goblin": encounter.KindMonster, "unsheeted": encounter.KindMonster,
			"door": encounter.KindWorld,
		},
	})
}

func (s *ConditionSeamSuite) blob(condition interface {
	ToJSON() (json.RawMessage, error)
}) json.RawMessage {
	data, err := condition.ToJSON()
	s.Require().NoError(err)
	return data
}

func (s *ConditionSeamSuite) TestConditionSeamReportsSheets() {
	ff, err := conditions.NewFaerieFireCondition(conditions.NewFaerieFireConditionInput{
		MemberID: "player", SourceID: "cleric", SourceRef: refs.Spells.FaerieFire(),
	})
	s.Require().NoError(err)
	s.chars.byID["player"] = &character.Data{ID: "player", Conditions: []json.RawMessage{
		s.blob(conditions.NewProneCondition("player")), s.blob(ff),
	}}
	trait := json.RawMessage(`{"ref":{"module":"dnd5e","type":"monster_traits","id":"vulnerability"}}`)
	s.data.NPCs = []monster.Data{{ID: "goblin", Conditions: []json.RawMessage{
		trait, s.blob(conditions.NewDodgingCondition("goblin")),
	}}}

	got, err := s.seam.Conditions([]encounter.MemberID{"player", "goblin", "door", "ghost", "unsheeted"})
	s.Require().NoError(err)

	s.Equal(map[encounter.MemberID]*encounter.ConditionSet{
		"player": {Conditions: []encounter.ConditionKey{
			{ConditionRef: refs.Conditions.Prone().String()},
			{ConditionRef: refs.Conditions.FaerieFire().String(), SourceID: "cleric"},
			{ConditionRef: refs.Conditions.OpportunityAttack().String()},
		}},
		"goblin": {Conditions: []encounter.ConditionKey{
			{ConditionRef: refs.Conditions.Dodging().String()},
			{ConditionRef: refs.Conditions.OpportunityAttack().String()},
		}},
		"door":      nil,
		"ghost":     nil,
		"unsheeted": nil,
	}, got, "in sheet order, then what a combatant carries by existing; a trait is not a condition; no sheet is nothing to observe")
}

// TestASheetHoldingNothingIsSeenHoldingOnlyItsOpportunityAttack: a sheet with
// nothing stored is an observation, and what it observes is what the member
// holds as a participant — the opportunity attack every combatant carries.
func (s *ConditionSeamSuite) TestASheetHoldingNothingIsSeenHoldingOnlyItsOpportunityAttack() {
	s.chars.byID["player"] = &character.Data{ID: "player"}

	got, err := s.seam.Conditions([]encounter.MemberID{"player"})
	s.Require().NoError(err)

	s.Require().NotNil(got["player"], "a sheet was read: an observation, not an absence")
	s.Equal([]encounter.ConditionKey{{ConditionRef: refs.Conditions.OpportunityAttack().String()}}, got["player"].Conditions)
}

func (s *ConditionSeamSuite) TestAnswersOnlyForWhoWasAsked() {
	s.chars.byID["player"] = &character.Data{ID: "player"}
	s.chars.byID["stranger"] = &character.Data{ID: "stranger"}

	got, err := s.seam.Conditions([]encounter.MemberID{"player"})
	s.Require().NoError(err)

	s.Len(got, 1, "the store holds strangers; the answer names only the asked")
	s.Contains(got, encounter.MemberID("player"))
}

func (s *ConditionSeamSuite) TestAMemberWithNoRosterKindIsRefused() {
	_, err := s.seam.Conditions([]encounter.MemberID{"nobody"})
	s.ErrorIs(err, ErrInvalidSession)
}

// TestAnUnreadableConditionMakesTheMemberUnknown: a sheet holding a condition
// that cannot be read is nothing observed, never a list claiming the
// unreadable one is absent.
func (s *ConditionSeamSuite) TestAnUnreadableConditionMakesTheMemberUnknown() {
	s.chars.byID["player"] = &character.Data{ID: "player", Conditions: []json.RawMessage{
		s.blob(conditions.NewProneCondition("player")), json.RawMessage(`{"ref":"nonsense","x":`),
	}}

	got, err := s.seam.Conditions([]encounter.MemberID{"player"})
	s.Require().NoError(err, "the verb plays on, as the sheet's lenient projection does")

	s.Contains(got, encounter.MemberID("player"))
	s.Nil(got["player"])
}
