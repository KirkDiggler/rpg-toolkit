// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The effect tests ride the activation suite: its fighter is the fighter this
// slice is about, and its world and resolve helpers are the ones the feature
// path is already proven through, so the two ways can be set side by side.

// secondWindDescription is Second Wind's own description for this sheet, built
// by the rulebook rather than written out here.
func (s *ActivationTestSuite) secondWindDescription(data *character.Data) *combatActions.Definition {
	sheet, err := character.Load(s.ctx, data)
	s.Require().NoError(err)

	definition, ok, err := sheet.AbilityDefinition(*refs.Features.SecondWind())
	s.Require().NoError(err)
	s.Require().True(ok, "the fighter holds Second Wind, and it carries a description")

	return definition
}

// fighterAt is the activation fighter at a class level, hurt.
func (s *ActivationTestSuite) fighterAt(level, hitPoints, maxHitPoints int) *character.Data {
	data := s.fighter(activationFighterID, hitPoints, maxHitPoints)
	data.Level = level
	data.Levels = syntheticLevels(classes.Fighter, level, 10, 6)

	return data
}

func (s *ActivationTestSuite) effectWorld() encounter.EncounterData {
	return s.world(
		encounter.MemberInput{ID: activationFighterID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 3}},
	)
}

func (s *ActivationTestSuite) effectCost(definition *combatActions.Definition, payer string) *Cost {
	return &Cost{PayerID: payer, Profile: definition.Cost, Turn: &Turn{Number: 1, Speed: 30}}
}

// healingsOn counts the healing events a bus carries, so "nobody was healed"
// is read off the bus and not off a return value.
func (s *ActivationTestSuite) healingsOn(bus events.EventBus) *int {
	count := 0
	_, err := dnd5eEvents.HealingReceivedTopic.On(bus).Subscribe(s.ctx,
		func(context.Context, dnd5eEvents.HealingReceivedEvent) error {
			count++
			return nil
		})
	s.Require().NoError(err)

	return &count
}

// resolveEffect resolves a description by the one door every verb pays at.
func (s *ActivationTestSuite) resolveEffect(
	data *character.Data, definition *combatActions.Definition, bus events.EventBus, extra ...Participant,
) (*Output, error) {
	machine, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: activationFighterID, Roller: fixedRoller{face: 6},
	})
	s.Require().NoError(err)

	return s.resolveOnBus(bus, machine, s.effectWorld(),
		append([]Participant{{Character: data}}, extra...), s.effectCost(definition, activationFighterID))
}

// resolveSecondWindFromItsDescription resolves this fighter's Second Wind the
// one way it is used: its description built into an action, its cost paid at
// the door, a fixed d10 of 6, the fighter alone in the fight.
func (s *ActivationTestSuite) resolveSecondWindFromItsDescription(data *character.Data) (*Output, error) {
	definition := s.secondWindDescription(data)
	machine, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: activationFighterID, Roller: fixedRoller{face: 6},
	})
	s.Require().NoError(err)

	return s.resolveOnBus(events.NewEventBus(), machine,
		s.world(encounter.MemberInput{ID: activationFighterID, Kind: encounter.KindPlayer,
			Position: spatial.Position{X: 1, Y: 1}}),
		[]Participant{{Character: data}}, s.effectCost(definition, activationFighterID))
}

func (s *ActivationTestSuite) wolfParticipant() Participant {
	return Participant{Monster: monsters.NewWolf(wolfID).ToData()}
}

// IT RUNS. The fighter is hurt, the description is Second Wind's own, and the
// door takes a bonus action and a point of the pool before the heal lands.
func (s *ActivationTestSuite) TestAnActionThatIsAnEffectHealsItsUserAndPaysAtTheDoor() {
	data := s.fighterAt(1, 4, 12)
	definition := s.secondWindDescription(data)

	out, err := s.resolveEffect(data, definition, events.NewEventBus(), s.wolfParticipant())

	s.Require().NoError(err)
	sheet := dirty(out, activationFighterID)
	s.Require().NotNil(sheet, "the fighter comes back dirty")
	s.Equal(11, sheet.HitPoints, "4 plus a rolled 6 plus fighter level 1")
	s.Zero(sheet.ActionEconomy.BonusActionsRemaining, "the bonus action is spent")
	s.Equal(0, sheet.Resources[resources.SecondWind].Current, "the pool reads 0 of 1")
	s.Equal(1, sheet.Resources[resources.SecondWind].Maximum)

	// What the caller gets: the same kind of fact a heal carries everywhere
	// else, in the outcome an activation reports.
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok, "an effect reports an ActivationOutcome")
	s.Equal(refs.Features.SecondWind().String(), outcome.Ability)
	s.Require().Len(outcome.Effects, 1)
	effect := outcome.Effects[0]
	s.Equal(EffectHealingApplied, effect.Kind)
	s.Equal(activationFighterID, effect.TargetID)
	s.Equal(7, effect.Requested)
	s.Equal(4, effect.Before)
	s.Equal(11, effect.After)
}

func (s *ActivationTestSuite) TestAnEffectIsCappedAtTheMaximum() {
	data := s.fighterAt(1, 8, 10)
	definition := s.secondWindDescription(data)

	out, err := s.resolveEffect(data, definition, events.NewEventBus(), s.wolfParticipant())

	s.Require().NoError(err)
	s.Equal(10, dirty(out, activationFighterID).HitPoints)
}

// IT HEALS THE FIGURES THE FEATURE ONCE ANNOUNCED. A fixed d10 of 6 and a
// fighter level, at a level where a fixed +1 would be seen: 7 at level 1, 9 at
// level 3.
func (s *ActivationTestSuite) TestAnEffectAnnouncesTheHealForTheFighterLevel() {
	for _, tc := range []struct{ level, heal int }{{1, 7}, {3, 9}} {
		s.Run(fmt.Sprintf("fighter level %d", tc.level), func() {
			data := s.fighterAt(tc.level, 4, 40)
			definition := s.secondWindDescription(data)

			out, err := s.resolveEffect(data, definition, events.NewEventBus(), s.wolfParticipant())
			s.Require().NoError(err)
			got, ok := out.Outcome.(ActivationOutcome)
			s.Require().True(ok)
			s.Require().Len(got.Effects, 1)

			s.Equal(tc.heal, got.Effects[0].Requested, "1d10 at a fixed 6, plus the fighter level")
			s.Equal(tc.heal, got.Effects[0].Calculation.Total)
			s.Equal(4+tc.heal, got.Effects[0].After)
		})
	}
}

// THE FEATURE PATH REFUSES IT. Second Wind is used from its description; asking
// the feature to activate itself is refused with the rulebook's sentence,
// nothing is spent and nobody is healed.
func (s *ActivationTestSuite) TestTheFeaturePathRefusesSecondWind() {
	data := s.fighterAt(1, 4, 12)
	bus := events.NewEventBus()
	healed := s.healingsOn(bus)
	machine, err := NewActivation(&ActivationInput{
		MemberID: activationFighterID, Ability: refs.Features.SecondWind(), Roller: fixedRoller{face: 6},
	})
	s.Require().NoError(err)

	out, err := s.resolveOnBus(bus, machine, s.effectWorld(),
		[]Participant{{Character: data}, s.wolfParticipant()}, nil)

	s.Require().Error(err)
	s.Contains(err.Error(),
		"this ability is used from its description: build the action with Definition and pay its cost at the gate")
	s.Nil(out, "nothing comes back to save: nothing was spent")
	s.Zero(*healed, "nobody was healed")
	s.Equal(4, data.HitPoints, "the sheet handed over is untouched")
	s.Equal(1, data.Resources[resources.SecondWind].Current, "the pool is untouched")
	s.Equal(1, data.ActionEconomy.BonusActionsRemaining, "the bonus action is untouched")
}

// REFUSED AT THE DOOR: nothing spent, nobody healed.
func (s *ActivationTestSuite) TestAnEffectIsRefusedAtTheDoorWhenItCannotBePaid() {
	for _, tc := range []struct {
		name   string
		spoil  func(*character.Data)
		reason string
	}{
		{"the bonus action is already spent", func(d *character.Data) { d.ActionEconomy.BonusActionsRemaining = 0 }, "bonus"},
		{"the pool is at 0", func(d *character.Data) {
			d.Resources[resources.SecondWind] = character.RecoverableResourceData{
				Current: 0, Maximum: 1, ResetType: coreResources.ResetShortRest,
			}
		}, "second_wind"},
		{"the fighter is not in a fight", func(d *character.Data) { d.ActionEconomy = nil }, "combat"},
	} {
		s.Run(tc.name, func() {
			data := s.fighterAt(1, 4, 12)
			definition := s.secondWindDescription(data)
			tc.spoil(data)
			bus := events.NewEventBus()
			healed := s.healingsOn(bus)

			out, err := s.resolveEffect(data, definition, bus, s.wolfParticipant())

			s.Require().ErrorIs(err, ErrCannotPay)
			s.Nil(out, "nothing comes back to save: nothing was spent")
			s.Zero(*healed, "nobody was healed")
			s.Equal(4, data.HitPoints, "the sheet handed over is untouched")
		})
	}
}

// REFUSED IN PREFLIGHT: before the door, so nothing is charged.
func (s *ActivationTestSuite) TestAnEffectIsRefusedBeforePaymentWhenItCannotLand() {
	s.Run("an empty actor", func() {
		definition := s.secondWindDescription(s.fighterAt(1, 4, 12))

		_, err := NewAction(&ActionInput{Definition: *definition, Roller: fixedRoller{face: 6}})

		s.Require().ErrorIs(err, ErrBadAction)
	})

	s.Run("a named target who is not the actor", func() {
		definition := s.secondWindDescription(s.fighterAt(1, 4, 12))

		_, err := NewAction(&ActionInput{
			Definition: *definition, AttackerID: activationFighterID, TargetID: wolfID, Roller: fixedRoller{face: 6},
		})
		s.Require().ErrorIs(err, ErrBadAction)

		_, err = NewAction(&ActionInput{
			Definition: *definition, AttackerID: activationFighterID,
			TargetIDs: []string{activationFighterID, wolfID}, Roller: fixedRoller{face: 6},
		})
		s.Require().ErrorIs(err, ErrBadAction)
	})

	s.Run("the actor named as the one target is accepted", func() {
		data := s.fighterAt(1, 4, 12)
		definition := s.secondWindDescription(data)
		for _, in := range []*ActionInput{
			{Definition: *definition, AttackerID: activationFighterID, TargetID: activationFighterID, Roller: fixedRoller{face: 6}},
			{Definition: *definition, AttackerID: activationFighterID, TargetIDs: []string{activationFighterID}, Roller: fixedRoller{face: 6}},
		} {
			machine, err := NewAction(in)
			s.Require().NoError(err)

			out, err := s.resolveOnBus(events.NewEventBus(), machine, s.effectWorld(),
				[]Participant{{Character: data}, s.wolfParticipant()}, s.effectCost(definition, activationFighterID))
			s.Require().NoError(err)
			s.Equal(11, dirty(out, activationFighterID).HitPoints)
		}
	})

	s.Run("an actor who cannot receive healing", func() {
		data := s.fighterAt(1, 0, 12)
		data.DeathSaveState = &saves.DeathSaveState{Dead: true}
		definition := s.secondWindDescription(s.fighterAt(1, 4, 12))
		bus := events.NewEventBus()
		healed := s.healingsOn(bus)

		out, err := s.resolveEffect(data, definition, bus, s.wolfParticipant())

		s.Require().ErrorIs(err, ErrBadAction, "refused by the rule healing spells use")
		s.NotErrorIs(err, ErrCannotPay, "refused in preflight, before the door")
		s.Nil(out)
		s.Zero(*healed)
	})
}

// A MONSTER CANNOT PAY IT: a monster has no count to pay Second Wind's use
// from, so the gate refuses the whole price as ErrCannotPay. This is what step
// 2 of rpg-project#1131 changes: a monster's counts are not on its sheet yet.
func (s *ActivationTestSuite) TestAMonsterAsPayerOfAnEffectsCostIsRefused() {
	data := s.fighterAt(1, 4, 12)
	definition := s.secondWindDescription(data)
	machine, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: activationFighterID, Roller: fixedRoller{face: 6},
	})
	s.Require().NoError(err)

	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.effectWorld(),
		[]Participant{{Character: data}, s.wolfParticipant()}, s.effectCost(definition, wolfID))

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Contains(err.Error(), wolfID)
	s.Nil(out)
}

// NOT A SPELL: the runner pays an effect at the door alone. A cost carrying a
// spell turn, which a cast must classify, does not make an effect a cast.
func (s *ActivationTestSuite) TestAnEffectIsNotPaidByTheSpellRules() {
	data := s.fighterAt(1, 4, 12)
	definition := s.secondWindDescription(data)
	machine, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: activationFighterID, Roller: fixedRoller{face: 6},
	})
	s.Require().NoError(err)
	s.NotContains(fmt.Sprintf("%T", machine), "castMachine")

	cost := s.effectCost(definition, activationFighterID)
	cost.SpellTurn = "scene/round-1/fighter"
	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.effectWorld(),
		[]Participant{{Character: data}, s.wolfParticipant()}, cost)

	s.Require().NoError(err, "no spell classification was asked of an effect")
	s.Equal(11, dirty(out, activationFighterID).HitPoints)
}

// THE NEIGHBOURS ARE NOT DISTURBED, and a definition with no body is still
// refused.
func TestNewActionStillDispatchesItsNeighboursAndRefusesNoBody(t *testing.T) {
	t.Run("an attack", func(t *testing.T) {
		machine, err := NewAction(&ActionInput{Definition: validMeleeDefinition(), AttackerID: "a", TargetID: "b"})
		require.NoError(t, err)
		require.IsType(t, &strikeMachine{}, machine)
	})

	t.Run("a cast", func(t *testing.T) {
		sheet, err := character.Load(context.Background(), baneCaster(1))
		require.NoError(t, err)
		machine, err := NewAction(&ActionInput{
			Definition: *sheet.CastDefinition(spells.CureWounds), AttackerID: bardID,
			TargetIDs: []string{wolfID}, Roller: facedRoller{other: 5},
		})
		require.NoError(t, err)
		require.IsType(t, &castMachine{}, machine)
	})

	t.Run("a sequence", func(t *testing.T) {
		claw := validMeleeDefinition()
		machine, err := NewAction(&ActionInput{
			Definition: twoClaws(), AttackerID: wolfID, TargetID: heroID,
			Components: []combatActions.Definition{claw, twoClaws()},
		})
		require.NoError(t, err)
		require.IsType(t, &sequenceMachine{}, machine)
	})

	t.Run("no body", func(t *testing.T) {
		definition := validMeleeDefinition()
		definition.Attack = nil

		_, err := NewAction(&ActionInput{Definition: definition, AttackerID: "a", TargetID: "b"})
		require.ErrorIs(t, err, ErrBadAction)
	})
}
