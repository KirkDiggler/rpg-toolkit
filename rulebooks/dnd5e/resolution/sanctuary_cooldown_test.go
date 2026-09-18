// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"fmt"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

func (s *CastActionTestSuite) TestSanctuaryRecipientCooldownSurvivesWardAndExpiresAfterReload() {
	f := s.fixtures()
	caster := baneCaster(1, 2)
	caster.ActionEconomy.BonusActionsRemaining = 1
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Sanctuary})
	roller := &countingCastRoller{}
	cast := func(actor string, sheets []Participant, turn string) (*Output, error) {
		machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: actor, TargetIDs: []string{bardID}, Roller: roller})
		if err != nil {
			return nil, err
		}
		return Resolve(s.ctx, &Input{World: f.world(), Participants: sheets, Machine: machine,
			Cost:       &Cost{PayerID: actor, Profile: definition.Cost, SpellTurn: turn},
			Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller()})
	}
	out, err := cast(bardID, []Participant{{Character: caster}}, "first")
	s.Require().NoError(err)
	caster = f.sheet(out, bardID)
	s.True(hasConditionRef(s.T(), caster.Conditions, refs.Conditions.Sanctuary().String()))
	s.True(hasConditionRef(s.T(), caster.Conditions, refs.Conditions.SanctuaryImmune().String()))
	s.Equal(1, caster.Resources[resources.SpellSlotLevel1].Current)
	for _, blob := range caster.Conditions {
		c, err := conditions.LoadJSON(blob)
		s.Require().NoError(err)
		if hold, ok := c.(*conditions.ConcentratingCondition); ok {
			s.Require().Len(hold.Children, 1)
			s.Equal(refs.Conditions.Sanctuary().String(), hold.Children[0].ConditionRef)
		}
	}
	caster.ActionEconomy.BonusActionsRemaining = 1
	other := f.saver(10)
	for _, actor := range []string{bardID, heroID} {
		sheets := []Participant{{Character: caster}, {Character: other}}
		before, err := json.Marshal(sheets)
		s.Require().NoError(err)
		refused, err := cast(actor, sheets, "recast")
		s.ErrorIs(err, ErrBadAction)
		s.Nil(refused)
		after, err := json.Marshal(sheets)
		s.Require().NoError(err)
		s.Equal(before, after, "refusal must not spend or replace concentration")
	}
	for turn := 1; turn <= conditions.SanctuaryImmuneTurnEnds; turn++ {
		boundary, err := NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: bardID, Round: turn}}})
		s.Require().NoError(err)
		out = s.blessRun(f.world(), []Participant{{Character: caster}}, boundary, nil)
		caster = f.sheet(out, bardID)
		if turn == 11 {
			s.False(hasConditionRef(s.T(), caster.Conditions, refs.Conditions.Sanctuary().String()))
			s.True(hasConditionRef(s.T(), caster.Conditions, refs.Conditions.SanctuaryImmune().String()))
		}
		allowed, err := definition.Cast.AllowsRecipient(caster.Conditions)
		s.Require().NoError(err)
		s.Equal(turn == conditions.SanctuaryImmuneTurnEnds, allowed, fmt.Sprintf("turn %d", turn))
	}
	caster.ActionEconomy.BonusActionsRemaining = 1
	out, err = cast(bardID, []Participant{{Character: caster}}, "after-cooldown")
	s.Require().NoError(err)
	final := f.sheet(out, bardID)
	s.Zero(final.Resources[resources.SpellSlotLevel1].Current)
	s.True(hasConditionRef(s.T(), final.Conditions, refs.Conditions.SanctuaryImmune().String()))
}
