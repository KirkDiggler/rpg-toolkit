// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// drivenDash is the input a run sends: the goblin, driven, naming the Dash.
func drivenDash() *ActivationInput {
	return &ActivationInput{
		MemberID: goblinID, Ability: refs.CombatAbilities.Dash(),
		Driven: true, Roller: dice.NewRoller(),
	}
}

// dashCost is the rules' price of a monster's Dash, from the rulebook.
func (s *ActivationTestSuite) dashCost(payer string, sheet *monster.Data) *Cost {
	out, err := monster.CostOfDash(&monster.CostOfDashInput{Sheet: sheet})
	s.Require().NoError(err)
	return &Cost{PayerID: payer, Profile: out.Cost}
}

// disengageCost is the rules' price of a monster's Disengage.
func (s *ActivationTestSuite) disengageCost(sheet *monster.Data) *Cost {
	out, err := monster.CostOfDisengage(&monster.CostOfDisengageInput{Sheet: sheet})
	s.Require().NoError(err)
	return &Cost{PayerID: goblinID, Profile: out.Cost}
}

// resolveDash resolves a driven Dash on a fresh bus with the given cost.
func (s *ActivationTestSuite) resolveDash(in *ActivationInput, sheet *monster.Data, cost *Cost) (*Output, error) {
	machine, err := NewActivation(in)
	s.Require().NoError(err)
	return s.resolveOnBus(events.NewEventBus(), machine, s.drivenWorld(), s.drivenCast(sheet), cost)
}

// A driven Dash charges one action at the door and lands nothing else: no
// effect, no capacity, no condition, no movement on the sheet.
func (s *ActivationTestSuite) TestADrivenDashChargesOneActionAndLandsNothingElse() {
	sheet := monsters.NewGoblin(goblinID).ToData()
	before := monsterConditionRefs(sheet)

	out, err := s.resolveDash(drivenDash(), sheet, s.dashCost(goblinID, sheet))
	s.Require().NoError(err)

	got, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Equal(refs.CombatAbilities.Dash().String(), got.Ability)
	s.Empty(got.Effects)
	s.Empty(got.GrantedCapacity)

	goblin := s.dirtyMonster(out)
	s.Equal(0, goblin.SlotsLeft(coreCombat.ActionStandard), "the action was paid at the door")
	s.Equal(1, goblin.SlotsLeft(coreCombat.ActionBonus))
	s.Equal(1, goblin.SlotsLeft(coreCombat.ActionReaction))
	s.Equal(before, monsterConditionRefs(goblin), "no condition it did not have")
	s.Require().NotNil(goblin.Economy)
	s.Equal(0, goblin.Economy.MovementRemaining, "no movement is banked on a monster's sheet")
}

// A goblin whose action is already spent cannot run.
func (s *ActivationTestSuite) TestAGoblinWithNoActionLeftCannotRun() {
	spent := monsters.NewGoblin(goblinID).ToData()
	spent.Economy = combat.NewActionEconomy()
	spent.Economy.SpendSlots(coreCombat.ActionStandard, 1)

	out, err := s.resolveDash(drivenDash(), spent, s.dashCost(goblinID, spent))

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Contains(err.Error(), goblinID)
	s.Nil(out)
}

// A run and a Disengage in one turn are both payable: an action, then a bonus
// action, on the sheet the first returned.
func (s *ActivationTestSuite) TestARunAndADisengageInOneTurnAreBothPayable() {
	sheet := monsters.NewGoblin(goblinID).ToData()
	first, err := s.resolveDash(drivenDash(), sheet, s.dashCost(goblinID, sheet))
	s.Require().NoError(err)
	afterRun := s.dirtyMonster(first)

	second, err := s.resolveDash(drivenDisengage(), afterRun, s.disengageCost(afterRun))
	s.Require().NoError(err)

	goblin := s.dirtyMonster(second)
	s.Equal(0, goblin.SlotsLeft(coreCombat.ActionStandard))
	s.Equal(0, goblin.SlotsLeft(coreCombat.ActionBonus))
	s.Equal(1, goblin.SlotsLeft(coreCombat.ActionReaction))
}

// No payment, no Dash: refused before the world is loaded. The test calls
// Input.Validate on an input that carries no world and no cast at all, so a
// refusal that came from loading or paying could not be the one asserted, and
// the sheet is shown uncharged.
func (s *ActivationTestSuite) TestADrivenDashWithNoPaymentIsRefusedBeforeAnythingLoads() {
	sheet := monsters.NewGoblin(goblinID).ToData()
	cases := map[string]*Cost{
		"no cost":       nil,
		"nil profile":   {PayerID: goblinID},
		"empty profile": {PayerID: goblinID, Profile: &combat.SpendProfile{}},
		"grant only": {PayerID: goblinID, Profile: &combat.SpendProfile{
			Grants: map[combat.CapacityType]int{combat.CapacityMovement: 30},
		}},
	}
	for name, cost := range cases {
		s.Run(name, func() {
			machine, err := NewActivation(drivenDash())
			s.Require().NoError(err)
			in := &Input{Machine: machine, Cost: cost, Roller: dice.NewRoller()}

			err = in.Validate()

			s.Require().ErrorIs(err, ErrBadActivation)
			s.Contains(err.Error(), "only its payment and")
		})
	}
	s.Equal(1, sheet.SlotsLeft(coreCombat.ActionStandard), "nothing was charged")
}

// Paid by the one who runs: a cost naming anyone else is refused.
func (s *ActivationTestSuite) TestADrivenDashIsPaidByTheOneWhoRuns() {
	sheet := monsters.NewGoblin(goblinID).ToData()
	machine, err := NewActivation(drivenDash())
	s.Require().NoError(err)
	in := &Input{Machine: machine, Cost: s.dashCost(heroID, sheet), Roller: dice.NewRoller()}

	err = in.Validate()

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "paid by the one who runs")
}

// A driven Dash takes no target.
func (s *ActivationTestSuite) TestADrivenDashWithATargetIsRefused() {
	in := drivenDash()
	in.TargetID = heroID
	sheet := monsters.NewGoblin(goblinID).ToData()

	_, err := s.resolveDash(in, sheet, s.dashCost(goblinID, sheet))

	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "takes no target")
}

// A character is still refused at the driven door for Dash, and a driven Hide
// is still not yet driven and now names both driven abilities.
func (s *ActivationTestSuite) TestTheDrivenDoorStillRefusesACharacterAndHide() {
	sheet := monsters.NewGoblin(goblinID).ToData()

	asHero := drivenDash()
	asHero.MemberID = heroID
	_, err := s.resolveDash(asHero, sheet, s.dashCost(heroID, sheet))
	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "characters declare")

	hide := drivenDash()
	hide.Ability = refs.CombatAbilities.Hide()
	_, err = s.resolveDash(hide, sheet, nil)
	s.Require().ErrorIs(err, ErrBadActivation)
	s.Contains(err.Error(), "not yet driven")
	s.Contains(err.Error(), "Disengage and Dash are the driven abilities")
}
