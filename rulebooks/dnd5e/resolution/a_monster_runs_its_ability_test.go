// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombatType "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// A monster built from a stat block whose `lets` gives it Second Wind with one
// use runs that ability through the action door as its own actor and payer
// (rpg-project#1131, step 3, slice 3a). The dice are the fixture the effect
// tests use, fixedRoller{face: 6}: 1d10 reads 6 and the level adds 1.

const (
	captainID     = "captain-1"
	captainMaxHP  = 18 // four d8 hit dice at the floored average of the whole notation, CON modifier 0
	secondWindKey = "second-wind"
)

const captainLets = `[{"use":"second wind","with":{"level":1},` +
	`"uses":{"max":1,"refills":{"on":["short rest","long rest"]}}}]`

// captain is the human base with four hit dice and Second Wind with one use,
// built by the rules' own FromTemplate, hurt to hitPoints.
func (s *ActivationTestSuite) captain(hitPoints int) (*monster.Data, *combatActions.Definition) {
	built, err := monster.FromTemplate(&monster.FromTemplateInput{
		ID:  captainID,
		Ref: &core.Ref{Module: "dnd5e", Type: "monsters", ID: "captain"},
		Template: monster.Template{
			Base:    refs.Monsters.Human(),
			Name:    "Captain",
			HitDice: "4d8",
			Lets:    json.RawMessage(captainLets),
		},
		Base: monsters.Human,
	})
	s.Require().NoError(err)
	granted, err := built.GrantedAbility(&monster.GrantedAbilityInput{Grant: secondWindKey})
	s.Require().NoError(err)
	data := built.ToData()
	s.Require().Equal(captainMaxHP, data.MaxHitPoints)
	data.HitPoints = hitPoints

	return data, granted.Definition
}

func (s *ActivationTestSuite) captainWorld() encounter.EncounterData {
	return s.world(
		encounter.MemberInput{ID: captainID, Kind: encounter.KindMonster, Position: spatial.Position{X: 1, Y: 1}},
		encounter.MemberInput{ID: wolfID, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 3}},
	)
}

// runCaptain runs the definition as the captain through the action door, the
// cost paid by payer, on the given bus.
func (s *ActivationTestSuite) runCaptain(
	data *monster.Data, definition *combatActions.Definition, actor, payer string, bus events.EventBus,
) (*Output, error) {
	machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: actor, Roller: fixedRoller{face: 6}})
	s.Require().NoError(err)

	return s.resolveOnBus(bus, machine, s.captainWorld(),
		[]Participant{{Monster: data}, s.wolfParticipant()}, s.effectCost(definition, payer))
}

// dirtyCaptain finds the captain's sheet among the sheets an output handed back.
func (s *ActivationTestSuite) dirtyCaptain(out *Output) *monster.Data {
	for _, sheet := range out.DirtyMonsters {
		if sheet.ID == captainID {
			return sheet
		}
	}
	s.FailNow("the captain came back clean", "%q is not among the dirty monsters", captainID)

	return nil
}

func heal(s *ActivationTestSuite, out *Output) ActivationEffect {
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok, "an effect reports an ActivationOutcome")
	s.Require().Len(outcome.Effects, 1)

	return outcome.Effects[0]
}

// TEST 1. Wounded, it heals: paid at the door, landed on it, reported as a
// player's heal is, and the changed sheet comes back.
func (s *ActivationTestSuite) TestAWoundedMonsterHealsItselfFromItsOwnKit() {
	data, definition := s.captain(5)

	out, err := s.runCaptain(data, definition, captainID, captainID, events.NewEventBus())

	s.Require().NoError(err)
	s.Require().Len(out.DirtyMonsters, 1)
	sheet := s.dirtyCaptain(out)
	s.Equal(12, sheet.HitPoints, "5 plus a rolled 6 plus level 1")
	s.Equal(0, sheet.SlotsLeft(coreCombatType.ActionBonus), "the bonus action is spent")
	s.Equal(1, sheet.SlotsLeft(coreCombatType.ActionStandard), "the action is not")
	s.Equal(0, sheet.Left[secondWindKey], "none of the one use is left")

	outcome := out.Outcome.(ActivationOutcome)
	s.Equal(refs.Features.SecondWind().String(), outcome.Ability)
	effect := heal(s, out)
	s.Equal(EffectHealingApplied, effect.Kind)
	s.Equal(captainID, effect.TargetID)
	s.Equal(7, effect.Requested)
	s.Equal(5, effect.Before)
	s.Equal(12, effect.After)

	// Capped at the maximum.
	nearly, definition := s.captain(captainMaxHP - 3)
	out, err = s.runCaptain(nearly, definition, captainID, captainID, events.NewEventBus())
	s.Require().NoError(err)
	s.Equal(captainMaxHP, s.dirtyCaptain(out).HitPoints)
}

// TEST 2. Paid before it lands, all or none.
func (s *ActivationTestSuite) TestAMonsterThatCannotPayHealsNothingAndKeepsTheRest() {
	s.Run("its bonus action is already spent", func() {
		data, definition := s.captain(5)
		data.Economy = combat.NewActionEconomy()
		data.Economy.SpendSlots(coreCombatType.ActionBonus, 1)
		bus := events.NewEventBus()
		healed := s.healingsOn(bus)

		out, err := s.runCaptain(data, definition, captainID, captainID, bus)

		s.Require().ErrorIs(err, ErrCannotPay)
		s.Nil(out)
		s.Zero(*healed)
		s.Equal(5, data.HitPoints)
		s.Equal(1, data.Left[secondWindKey], "the use is still there")
	})

	s.Run("its use is already spent", func() {
		data, definition := s.captain(5)
		data.Left[secondWindKey] = 0
		bus := events.NewEventBus()
		healed := s.healingsOn(bus)

		out, err := s.runCaptain(data, definition, captainID, captainID, bus)

		s.Require().ErrorIs(err, ErrCannotPay)
		s.Nil(out)
		s.Zero(*healed)
		s.Equal(5, data.HitPoints)
		s.Equal(1, data.SlotsLeft(coreCombatType.ActionBonus), "the bonus action is still there")
	})
}

// TEST 3. At full hit points it still runs: whatever the outcome reports is
// asserted here as it is.
func (s *ActivationTestSuite) TestAMonsterAtFullHitPointsStillRunsItsAbility() {
	data, definition := s.captain(captainMaxHP)

	out, err := s.runCaptain(data, definition, captainID, captainID, events.NewEventBus())

	s.Require().NoError(err)
	outcome := out.Outcome.(ActivationOutcome)
	s.Equal("", outcome.GrantedCapacity)
	s.Require().Len(outcome.Effects, 1, "the heal is still reported, as one healing-applied effect")
	effect := outcome.Effects[0]
	s.Equal(EffectHealingApplied, effect.Kind)
	s.Equal(captainID, effect.TargetID)
	s.Equal(0, effect.Amount, "nothing was gained")
	s.Equal(7, effect.Requested, "what the roll asked for")
	s.Equal(captainMaxHP, effect.Before)
	s.Equal(captainMaxHP, effect.After)
	sheet := s.dirtyCaptain(out)
	s.Equal(captainMaxHP, sheet.HitPoints)
	s.Equal(0, sheet.SlotsLeft(coreCombatType.ActionBonus))
	s.Equal(0, sheet.Left[secondWindKey])
}

// TEST 4. Who gave it is named.
func (s *ActivationTestSuite) TestTheHealNamesTheCreatureThatGaveIt() {
	data, definition := s.captain(5)

	out, err := s.runCaptain(data, definition, captainID, captainID, events.NewEventBus())

	s.Require().NoError(err)
	calculation := heal(s, out).Calculation
	s.Require().NotNil(calculation)
	var named bool
	for _, component := range calculation.Components {
		if component.Modifier != nil {
			named = true
			s.Equal("Captain", component.Source.Label)
			s.Equal(captainID, component.Source.SourceID)
		}
	}
	s.True(named, "the level is a modifier component, and it is named for the creature")
}

// TEST 5. A second time the same turn is refused, and the monster's own turn
// start gives back the bonus action and not the use.
func (s *ActivationTestSuite) TestTheUseDoesNotComeBackWithTheMonstersTurn() {
	data, definition := s.captain(2)
	first, err := s.runCaptain(data, definition, captainID, captainID, events.NewEventBus())
	s.Require().NoError(err)
	saved := s.dirtyCaptain(first)

	again, err := s.runCaptain(saved, definition, captainID, captainID, events.NewEventBus())
	s.Require().ErrorIs(err, ErrCannotPay, "the same turn: no bonus action and no use")
	s.Nil(again)

	bus := events.NewEventBus()
	loaded, err := monster.LoadFromData(s.ctx, saved, bus)
	s.Require().NoError(err)
	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.TurnStartEvent{SubjectID: captainID, Round: 2}))
	next := loaded.ToData()
	s.Require().Equal(1, next.SlotsLeft(coreCombatType.ActionBonus), "precondition: the turn came back")
	s.Require().Equal(0, next.Left[secondWindKey], "precondition: the use did not")

	again, err = s.runCaptain(next, definition, captainID, captainID, events.NewEventBus())
	s.Require().ErrorIs(err, ErrCannotPay)
	s.Nil(again)
}

// TEST 6. A monster with no kit asked for the same definition is refused at
// the door, as a wolf is.
func (s *ActivationTestSuite) TestAMonsterWithNoKitCannotPayForTheDefinition() {
	data, definition := s.captain(5)
	wolf := s.wolfParticipant().Monster
	wolf.HitPoints = 3

	machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: wolfID, Roller: fixedRoller{face: 6}})
	s.Require().NoError(err)
	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.captainWorld(),
		[]Participant{{Monster: data}, {Monster: wolf}}, s.effectCost(definition, wolfID))

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Contains(err.Error(), wolfID)
	s.Nil(out)
	s.Equal(3, wolf.HitPoints)
}

// TEST 7. A declared activation still refuses a monster.
func (s *ActivationTestSuite) TestADeclaredActivationStillRefusesAMonster() {
	data, _ := s.captain(5)
	machine, err := NewActivation(&ActivationInput{
		MemberID: captainID, Ability: refs.Features.SecondWind(), Roller: fixedRoller{face: 6},
	})
	s.Require().NoError(err)

	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.captainWorld(),
		[]Participant{{Monster: data}, s.wolfParticipant()}, nil)

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "its abilities are driven, not declared")
	s.Nil(out)
}
