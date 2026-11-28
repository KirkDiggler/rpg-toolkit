// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// A barbarian's own Rage (rpg-project#1131, slice 7d): described by her loaded
// sheet and resolved at the one door, the way a fighter's own Action Surge is
// (fighter_action_surge_test.go). Her feature no longer runs its own code, so
// this is the only way left to rage.

// herRageDescription asks her loaded sheet for Rage's description with her own
// numbers.
func (s *ActivationTestSuite) herRageDescription(data *character.Data) *combatActions.Definition {
	sheet, err := character.Load(s.ctx, data)
	s.Require().NoError(err)

	definition, ok, err := sheet.AbilityDefinition(*refs.Features.Rage())
	s.Require().NoError(err)
	s.Require().True(ok, "the barbarian holds Rage, and it carries a description")

	return definition
}

// ragesFromHerSheet is her own Rage at the door: the description from her sheet,
// built into an action, paid with the description's own cost, she alone in the
// fight, on her first turn.
func (s *ActivationTestSuite) ragesFromHerSheet(data *character.Data) (*Output, error) {
	return s.ragesAtTheDoor(events.NewEventBus(), data, s.herRageDescription(data), 1)
}

// TEST 1. A level-1 barbarian with two charges rages from her own sheet.
func (s *ActivationTestSuite) TestABarbariansOwnRageIsPaidAtTheDoorAndLands() {
	bus := events.NewEventBus()
	seen := s.appliedOn(bus)
	data := s.barbarian(2)

	out, err := s.ragesAtTheDoor(bus, data, s.herRageDescription(data), 1)

	s.Require().NoError(err)
	sheet := dirty(out, heroID)
	s.Require().NotNil(sheet, "she comes back dirty")
	s.Zero(sheet.ActionEconomy.BonusActionsRemaining, "the bonus action is spent")
	s.Equal(1, sheet.ActionEconomy.ActionsRemaining, "and nothing else")
	s.Equal(1, sheet.Resources[resources.RageCharges].Current, "the charges go 2 to 1")
	s.Equal([]string{refs.Conditions.Raging().String()}, conditionRefs(sheet), "raging is on the saved sheet")
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Equal(refs.Features.Rage().String(), outcome.Ability, "the outcome names Rage")
	s.Equal([]ActivationEffect{{
		Kind: EffectConditionApplied, TargetID: heroID,
		Ref: refs.Conditions.Raging().String(), Name: "Raging",
		Address: dnd5eEvents.ConditionAddress{MemberID: heroID, ConditionRef: refs.Conditions.Raging().String()},
	}}, outcome.Effects, "one condition-applied fact for raging, on her")
	s.Len(*seen, 1)
}

// TEST 2. Already raging, she is given no description: nothing to build, so
// nothing to charge.
func (s *ActivationTestSuite) TestABarbarianAlreadyRagingGetsNoDescription() {
	first, err := s.ragesFromHerSheet(s.barbarian(2))
	s.Require().NoError(err)
	raging := dirty(first, heroID)
	s.Require().Equal(1, raging.Resources[resources.RageCharges].Current)
	sheet, err := character.Load(s.ctx, raging)
	s.Require().NoError(err)

	definition, ok, err := sheet.AbilityDefinition(*refs.Features.Rage())

	s.Require().Error(err)
	s.Contains(err.Error(), "already raging")
	s.False(ok)
	s.Nil(definition)
	s.Equal(1, raging.Resources[resources.RageCharges].Current, "nothing was charged")
}

// TEST 3. No charge: the door refuses the payment and nothing changes.
func (s *ActivationTestSuite) TestABarbarianWithNoChargeIsRefusedAtTheDoor() {
	data := s.barbarian(0)
	definition := s.herRageDescription(data)

	out, err := s.ragesAtTheDoor(events.NewEventBus(), data, definition, 1)

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Contains(err.Error(), "rage_charges: 1 needed, 0 left")
	s.Nil(out, "nothing comes back to save")
	s.Equal(0, data.Resources[resources.RageCharges].Current)
	s.Equal(1, data.ActionEconomy.BonusActionsRemaining, "the bonus action is untouched")
	s.Empty(data.Conditions)
}

// TEST 4. At barbarian 20 no charge exists to spend, and she rages.
func (s *ActivationTestSuite) TestALevelTwentyBarbarianRagesWithNoChargeSpent() {
	data := s.barbarian(0)
	data.Level = 20
	data.Levels = syntheticLevels(classes.Barbarian, 20, 12, 7)
	definition := s.herRageDescription(data)
	s.Empty(definition.Cost.Pools, "her rages are unlimited: the description names no pool")

	out, err := s.ragesAtTheDoor(events.NewEventBus(), data, definition, 1)

	s.Require().NoError(err)
	sheet := dirty(out, heroID)
	s.Require().NotNil(sheet)
	s.Equal(0, sheet.Resources[resources.RageCharges].Current, "no charge was there to spend")
	s.Equal([]string{refs.Conditions.Raging().String()}, conditionRefs(sheet))
	s.Zero(sheet.ActionEconomy.BonusActionsRemaining, "the bonus action is still spent")
}

// TEST 5. The feature path refuses Rage, as it refuses Second Wind
// (TestTheFeaturePathRefusesSecondWind): the feature's own sentence, nothing
// spent, no condition.
func (s *ActivationTestSuite) TestTheFeaturePathRefusesRage() {
	data := s.barbarian(2)
	bus := events.NewEventBus()
	seen := s.appliedOn(bus)
	machine, err := NewActivation(&ActivationInput{
		MemberID: heroID, Ability: refs.Features.Rage(), Roller: fixedRoller{face: 6},
	})
	s.Require().NoError(err)

	out, err := s.resolveOnBus(bus, machine, s.world(), []Participant{{Character: data}}, nil)

	s.Require().Error(err)
	s.Contains(err.Error(),
		"this ability is used from its description: build the action with Definition and pay its cost at the gate")
	s.Nil(out, "nothing comes back to save: nothing was spent")
	s.Equal(2, data.Resources[resources.RageCharges].Current, "the charges are untouched")
	s.Equal(1, data.ActionEconomy.BonusActionsRemaining, "the bonus action is untouched")
	s.Empty(data.Conditions)
	s.Empty(*seen, "no condition was published")
}
