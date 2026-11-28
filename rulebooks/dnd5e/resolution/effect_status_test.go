// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombatType "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// An effect that puts a named status on its user (rpg-project#1131, slice 7b).
// Rage is the first and the measure: the same barbarian rages here from a
// hand-built description and, in Test 2, through her feature, and the two must
// tell the same story.

// ragingBindings are the bindings Rage's own code writes (features/rage.go).
func ragingBindings() json.RawMessage {
	raw, err := json.Marshal(conditions.RagingBindings{DamageBonus: conditions.RagingDamageBonus{BarbarianLevel: true}})
	if err != nil {
		panic(err)
	}

	return raw
}

// rageDescription is Rage as a description: a bonus action and a charge, and a
// status on its user.
func rageDescription(ref core.Ref, parameters json.RawMessage) *combatActions.Definition {
	return &combatActions.Definition{
		Ref:  *refs.Features.Rage(),
		Name: "Rage",
		Cost: &combat.SpendProfile{
			Slots: map[coreCombatType.ActionType]int{coreCombatType.ActionBonus: 1},
			Pools: map[coreResources.ResourceKey]int{resources.RageCharges: 1},
		},
		Effect: &combatActions.EffectProfile{
			To:        combatActions.EffectSelf,
			Condition: &combatActions.ConditionApplication{Ref: ref, Parameters: parameters},
		},
	}
}

func (s *ActivationTestSuite) rageAtTheDoorDescription() *combatActions.Definition {
	return rageDescription(*refs.Conditions.Raging(), ragingBindings())
}

// rageWorld is the barbarian alone in the fight.
func (s *ActivationTestSuite) rageWorld() encounter.EncounterData { return s.world() }

// ragesAtTheDoor resolves a description for the barbarian at the one door,
// with its cost, on a bus the caller holds. No roller: a status rolls nothing.
func (s *ActivationTestSuite) ragesAtTheDoor(
	bus events.EventBus, data *character.Data, definition *combatActions.Definition, turn int,
) (*Output, error) {
	machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: heroID})
	if err != nil {
		return nil, err
	}
	cost := &Cost{PayerID: heroID, Profile: definition.Cost, Turn: &Turn{Number: turn, Speed: 30}}

	return s.resolveOnBus(bus, machine, s.rageWorld(), []Participant{{Character: data}}, cost)
}

// appliedOn records every condition-applied fact a bus carries: who it landed
// on, what said it did, and which condition.
type appliedFact struct {
	Target string
	Source dnd5eEvents.ConditionSource
	Ref    string
}

func (s *ActivationTestSuite) appliedOn(bus events.EventBus) *[]appliedFact {
	var seen []appliedFact
	_, err := dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			seen = append(seen, appliedFact{
				Target: event.Target.GetID(), Source: event.Source, Ref: event.Condition.Ref().String(),
			})
			return nil
		})
	s.Require().NoError(err)

	return &seen
}

// TEST 1. She rages at the door.
func (s *ActivationTestSuite) TestAnEffectThatPutsAStatusOnItsUserIsPaidAtTheDoorAndLands() {
	out, err := s.ragesAtTheDoor(events.NewEventBus(), s.barbarian(2), s.rageAtTheDoorDescription(), 1)

	s.Require().NoError(err)
	sheet := dirty(out, heroID)
	s.Require().NotNil(sheet)
	s.Zero(sheet.ActionEconomy.BonusActionsRemaining, "the bonus action is spent")
	s.Equal(1, sheet.Resources[resources.RageCharges].Current, "the charges go 2 to 1")
	s.Equal([]string{refs.Conditions.Raging().String()}, conditionRefs(sheet))
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Equal(refs.Features.Rage().String(), outcome.Ability)
	s.Require().Len(outcome.Effects, 1)
	s.Equal(EffectConditionApplied, outcome.Effects[0].Kind)
	s.Equal(heroID, outcome.Effects[0].TargetID)
	s.Equal(refs.Conditions.Raging().String(), outcome.Effects[0].Ref)
}

// TEST 2, THE SLICE'S MEASURE. One barbarian rages at the door; an identical
// one rages the old way, through NewActivation and her Rage feature. What each
// caller is told (the outcome, and the condition-applied fact on the bus, with
// its source kind) and what each sheet saves must be the same, field by field.
// Both use the same member id, so nothing is "equal but for ids": it is equal.
func (s *ActivationTestSuite) TestTheDoorTellsTheSameStoryAsTheOldRage() {
	doorBus := events.NewEventBus()
	doorSeen := s.appliedOn(doorBus)
	door, err := s.ragesAtTheDoor(doorBus, s.barbarian(2), s.rageAtTheDoorDescription(), 1)
	s.Require().NoError(err)

	oldBus := events.NewEventBus()
	oldSeen := s.appliedOn(oldBus)
	machine, err := NewActivation(&ActivationInput{
		MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller(),
	})
	s.Require().NoError(err)
	old, err := s.resolveOnBus(oldBus, machine, s.rageWorld(), []Participant{{Character: s.barbarian(2)}}, nil)
	s.Require().NoError(err)

	doorOutcome, oldOutcome := door.Outcome.(ActivationOutcome), old.Outcome.(ActivationOutcome)
	s.Equal(oldOutcome.Ability, doorOutcome.Ability, "the ability named")
	s.Require().Len(doorOutcome.Effects, len(oldOutcome.Effects))
	for i := range oldOutcome.Effects {
		o, d := oldOutcome.Effects[i], doorOutcome.Effects[i]
		s.Equal(o.Kind, d.Kind, "effect %d kind", i)
		s.Equal(o.TargetID, d.TargetID, "effect %d recipient", i)
		s.Equal(o.Ref, d.Ref, "effect %d ref", i)
		s.Equal(o.Name, d.Name, "effect %d description", i)
		s.Equal(o.Detail, d.Detail, "effect %d detail", i)
		s.Equal(o.Address, d.Address, "effect %d address", i)
	}
	s.Equal(oldOutcome, doorOutcome, "the whole outcome")
	s.Equal(*oldSeen, *doorSeen, "the condition-applied facts on the bus, source kind included")
	s.Require().Len(*doorSeen, 1)

	doorSheet, oldSheet := dirty(door, heroID), dirty(old, heroID)
	s.Require().Len(doorSheet.Conditions, 1)
	s.Require().Len(oldSheet.Conditions, 1)
	s.JSONEq(string(oldSheet.Conditions[0]), string(doorSheet.Conditions[0]), "the saved raging condition")
	s.Equal(oldSheet.Resources, doorSheet.Resources, "the saved charges")
	s.Equal(oldSheet.ActionEconomy, doorSheet.ActionEconomy, "the saved economy")
}

// TEST 3. The status is live: raging gives advantage on a Strength check,
// the assertion check_test.go's raging tests make.
func (s *ActivationTestSuite) TestTheStatusPutOnAtTheDoorIsLive() {
	out, err := s.ragesAtTheDoor(events.NewEventBus(), s.barbarian(2), s.rageAtTheDoorDescription(), 1)
	s.Require().NoError(err)

	checked, err := MakeCheck(s.ctx, &CheckInput{
		Character:  dirty(out, heroID),
		Approaches: []encounter.CheckApproach{route(skills.Athletics, 12)},
		Roller:     &scriptedRoller{single: straightRoll, pair: []int{straightRoll, advantageRoll}},
	})
	s.Require().NoError(err)

	s.Equal(advantageRoll, checked.Result.Roll, "only a rolled-twice-take-higher gives this die")
	keep := keepOf(s.T(), checked.Calculation)
	s.Require().NotNil(keep)
	s.Equal(dnd5eEvents.KeepAdvantage, keep.Rule)
	s.Require().Len(keep.Granted, 1)
	s.Equal(refs.Conditions.Raging().String(), keep.Granted[0].Ref.String())
}

// TEST 4. Refused before anything is charged: construction refuses, so no
// door was reached and the sheet handed over is untouched.
func (s *ActivationTestSuite) TestAStatusThatCannotBeBuiltIsRefusedBeforeAnythingIsCharged() {
	for _, tc := range []struct {
		name       string
		definition *combatActions.Definition
		sentence   string
	}{
		{"values the definition refuses",
			rageDescription(*refs.Conditions.Raging(), json.RawMessage(`{"damage_bonus":{}}`)),
			"raging damage_bonus requires exactly one fixed or barbarian_level strategy"},
		{"a ref that names no condition",
			rageDescription(core.Ref{Module: "dnd5e", Type: "conditions", ID: "no-such-status"}, nil),
			"condition ref is not registered"},
	} {
		s.Run(tc.name, func() {
			data := s.barbarian(2)

			_, err := NewAction(&ActionInput{Definition: *tc.definition, AttackerID: heroID})

			s.Require().ErrorIs(err, ErrBadAction)
			s.Contains(err.Error(), tc.sentence, "the catalog's own sentence")
			s.Equal(1, data.ActionEconomy.BonusActionsRemaining, "the bonus action is untouched")
			s.Equal(2, data.Resources[resources.RageCharges].Current, "the charges are untouched")
			s.Empty(data.Conditions)
		})
	}
}

// TEST 5. A definition that takes no origin is refused with the catalog's
// sentence. Brutal Critical is an older condition whose entry does not accept
// an origin (conditions/factory.go: "condition does not support Origin").
func (s *ActivationTestSuite) TestAStatusWhoseDefinitionTakesNoOriginIsRefused() {
	definition := rageDescription(*refs.Conditions.BrutalCritical(), nil)

	_, err := NewAction(&ActionInput{Definition: *definition, AttackerID: heroID})

	s.Require().ErrorIs(err, ErrBadAction)
	s.Contains(err.Error(), "condition does not support Origin: "+refs.Conditions.BrutalCritical().String())
}

// TEST 6. A status rolls nothing, so it needs no roller; a heal still does.
func (s *ActivationTestSuite) TestAStatusNeedsNoRollerAndAHealStillDoes() {
	_, err := NewAction(&ActionInput{Definition: *s.rageAtTheDoorDescription(), AttackerID: heroID})
	s.Require().NoError(err, "no roller was given")

	heal := s.secondWindDescription(s.fighterAt(1, 4, 12))
	_, err = NewAction(&ActionInput{Definition: *heal, AttackerID: activationFighterID})
	s.Require().ErrorIs(err, ErrBadAction)
	s.Contains(err.Error(), "healing requires a roller")
}

// TEST 7. It lands on its user and names nobody else.
func (s *ActivationTestSuite) TestAStatusOnItsUserNamesNobodyElse() {
	definition := s.rageAtTheDoorDescription()

	_, err := NewAction(&ActionInput{Definition: *definition, AttackerID: heroID, TargetID: wolfID})
	s.Require().ErrorIs(err, ErrBadAction)
	_, err = NewAction(&ActionInput{Definition: *definition, AttackerID: heroID, TargetIDs: []string{heroID, wolfID}})
	s.Require().ErrorIs(err, ErrBadAction)
	_, err = NewAction(&ActionInput{Definition: *definition})
	s.Require().ErrorIs(err, ErrBadAction, "nobody uses it")

	for _, in := range []*ActionInput{
		{Definition: *definition, AttackerID: heroID, TargetID: heroID},
		{Definition: *definition, AttackerID: heroID, TargetIDs: []string{heroID}},
	} {
		_, err := NewAction(in)
		s.Require().NoError(err, "the actor named as the one target is accepted")
	}
}

// TEST 8. A user who already holds the status. This records what the door
// does; the rule that Rage is refused while raging belongs to the description's
// giver (the next slice), not to this door.
//
// OBSERVED: the door does not refuse. It charges a second time, replaces the
// instance she holds (the one-instance-per-address rule every delivery obeys)
// and says so: a condition-removed fact with reason "replaced", then the
// condition-applied fact. She holds ONE raging afterwards, not two.
func (s *ActivationTestSuite) TestAUserWhoAlreadyHoldsTheStatusIsGivenItAgainAtTheDoor() {
	definition := s.rageAtTheDoorDescription()
	first, err := s.ragesAtTheDoor(events.NewEventBus(), s.barbarian(2), definition, 1)
	s.Require().NoError(err)
	saved := dirty(first, heroID)
	saved.ActionEconomy.TurnNumber = 2
	saved.ActionEconomy.BonusActionsRemaining = 1
	s.Require().Equal(1, saved.Resources[resources.RageCharges].Current, "a charge is left")

	second, err := s.ragesAtTheDoor(events.NewEventBus(), saved, definition, 2)

	s.Require().NoError(err, "the door does not refuse a user who already holds it")
	again := dirty(second, heroID)
	s.Equal([]string{refs.Conditions.Raging().String()}, conditionRefs(again), "one instance, not two")
	s.Zero(again.Resources[resources.RageCharges].Current, "and she was charged again")
	effects := second.Outcome.(ActivationOutcome).Effects
	s.Require().Len(effects, 2)
	s.Equal(EffectConditionRemoved, effects[0].Kind)
	s.Equal("replaced", effects[0].Reason)
	s.Equal(EffectConditionApplied, effects[1].Kind)
}
