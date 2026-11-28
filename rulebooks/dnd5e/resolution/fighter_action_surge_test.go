// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// The fighter's own Action Surge (rpg-project#1131, slice 6b): counted in her
// shared pool, described by her sheet, resolved at the action door the way her
// Second Wind is (resolveEffect, effect_test.go). These are methods of the
// activation suite so they reuse its fighter, world and door helpers.

// surgeFighter is a level-2 fighter holding Action Surge: the feature carries
// no count, the pool she spends is her saved resources'.
func (s *ActivationTestSuite) surgeFighter() *character.Data {
	data := s.fighterAt(2, 12, 12)
	surgeFeature, err := json.Marshal(features.ActionSurgeData{
		Ref: refs.Features.ActionSurge(), ID: activationFighterID + "-action-surge", Name: "Action Surge",
		CharacterID: activationFighterID,
	})
	s.Require().NoError(err)
	data.Features = append(data.Features, surgeFeature)
	data.Resources[resources.ActionSurge] = character.RecoverableResourceData{
		Current: 1, Maximum: 1, ResetType: coreResources.ResetShortRest,
	}

	return data
}

func (s *ActivationTestSuite) actionSurgeDescription(data *character.Data) *combatActions.Definition {
	sheet, err := character.Load(s.ctx, data)
	s.Require().NoError(err)

	definition, ok, err := sheet.AbilityDefinition(*refs.Features.ActionSurge())
	s.Require().NoError(err)
	s.Require().True(ok, "the level-2 fighter holds Action Surge, and it carries a description")

	return definition
}

// TEST 2. Her own Action Surge at the door: the pool is spent, an action is
// given, the outcome says so; a second time in the same turn is refused.
func (s *ActivationTestSuite) TestAFightersOwnActionSurgeIsPaidAtTheDoorAndGivesAnAction() {
	data := s.surgeFighter()
	definition := s.actionSurgeDescription(data)
	s.Require().Equal(1, data.ActionEconomy.ActionsRemaining, "precondition: one action on the turn")

	out, err := s.resolveEffect(data, definition, events.NewEventBus(), s.wolfParticipant())

	s.Require().NoError(err)
	sheet := dirty(out, activationFighterID)
	s.Require().NotNil(sheet, "the fighter comes back dirty")
	s.Equal(0, sheet.Resources[resources.ActionSurge].Current, "the pool reads 0 of 1")
	s.Equal(1, sheet.Resources[resources.ActionSurge].Maximum)
	s.Equal(2, sheet.ActionEconomy.ActionsRemaining, "she has two actions")
	s.Equal(12, sheet.HitPoints, "nothing landed")
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok, "it reports an ActivationOutcome")
	s.Equal(refs.Features.ActionSurge().String(), outcome.Ability)
	s.Equal(surgeWords, outcome.GrantedCapacity)
	s.Require().Len(outcome.Effects, 1)
	s.Equal(EffectCapacityGranted, outcome.Effects[0].Kind)
	s.Equal(activationFighterID, outcome.Effects[0].TargetID)
	s.Equal(surgeWords, outcome.Effects[0].Description)

	// The same turn again, on the sheet that came back: refused, nothing changes.
	again, err := s.resolveEffect(sheet, definition, events.NewEventBus(), s.wolfParticipant())

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Nil(again, "nothing comes back to save")
	s.Equal(0, sheet.Resources[resources.ActionSurge].Current, "the pool is still 0")
	s.Equal(2, sheet.ActionEconomy.ActionsRemaining, "still two actions, no third")
}

// TEST 3. The feature path refuses it, as it does Second Wind.
func (s *ActivationTestSuite) TestTheFeaturePathRefusesActionSurge() {
	data := s.surgeFighter()
	machine, err := NewActivation(&ActivationInput{
		MemberID: activationFighterID, Ability: refs.Features.ActionSurge(), Roller: fixedRoller{face: 6},
	})
	s.Require().NoError(err)

	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.effectWorld(),
		[]Participant{{Character: data}, s.wolfParticipant()}, nil)

	s.Require().Error(err)
	s.Contains(err.Error(),
		"this ability is used from its description: build the action with Definition and pay its cost at the gate")
	s.Nil(out, "nothing comes back to save: nothing was spent")
	s.Equal(1, data.Resources[resources.ActionSurge].Current, "the pool is untouched")
	s.Equal(1, data.ActionEconomy.ActionsRemaining, "the actions are untouched")
}
