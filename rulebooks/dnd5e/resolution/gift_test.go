// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombatType "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// An ability that is only its price (rpg-project#1131, slice 5b-2): a
// description with no profile whose cost gives a turn slot. The door charges
// and gives, nothing lands, and the outcome tells what was given.
//
// The monster pays from the captain's kit pool, monster.PoolKey("second-wind"),
// one use; the character pays from the fighter's Second Wind pool,
// resources.SecondWind. The description is built here, a ref of the test's own.

const (
	surgeWords = "1 action"
	noGiftWord = "has no supported profile"
)

func surgeRef() *core.Ref { return &core.Ref{Module: "dnd5e", Type: "abilities", ID: "test-surge"} }

// surge is a description that is only its price: one point of the pool in,
// one action out.
func surge(pool coreResources.ResourceKey) *combatActions.Definition {
	return &combatActions.Definition{
		Ref:  *surgeRef(),
		Name: "Test Surge",
		Cost: &combat.SpendProfile{
			Pools:      map[coreResources.ResourceKey]int{pool: 1},
			GrantSlots: map[coreCombatType.ActionType]int{coreCombatType.ActionStandard: 1},
		},
	}
}

// runSurgeOn runs a description as the captain with the cost the caller chose.
func (s *ActivationTestSuite) runSurgeOn(
	data *monster.Data, definition *combatActions.Definition, actor string, cost *Cost,
) (*Output, error) {
	machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: actor})
	s.Require().NoError(err)

	return s.resolveOnBus(events.NewEventBus(), machine, s.captainWorld(),
		[]Participant{{Monster: data}, s.wolfParticipant()}, cost)
}

func (s *ActivationTestSuite) surgeCost(definition *combatActions.Definition, payer string) *Cost {
	return s.effectCost(definition, payer)
}

// TEST 1. A monster is charged and given, and nothing lands.
func (s *ActivationTestSuite) TestAMonsterIsChargedAndGivenAnActionAndNothingLands() {
	data, _ := s.captain(5)
	definition := surge(monster.PoolKey(secondWindKey))
	s.Require().Equal(1, data.SlotsLeft(coreCombatType.ActionStandard), "precondition")

	out, err := s.runSurgeOn(data, definition, captainID, s.surgeCost(definition, captainID))

	s.Require().NoError(err)
	sheet := s.dirtyCaptain(out)
	s.Equal(2, sheet.SlotsLeft(coreCombatType.ActionStandard), "the action is given")
	s.Equal(0, sheet.Left[secondWindKey], "the use is spent")
	s.Equal(5, sheet.HitPoints, "nothing landed")
	outcome, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok, "it reports an ActivationOutcome")
	s.Equal(surgeRef().String(), outcome.Ability)
	s.Equal(surgeWords, outcome.GrantedCapacity)
	s.Require().Len(outcome.Effects, 1)
	s.Equal(EffectCapacityGranted, outcome.Effects[0].Kind)
	s.Equal(captainID, outcome.Effects[0].TargetID)
	s.Equal(surgeWords, outcome.Effects[0].Description)
}

// TEST 2. A character the same.
func (s *ActivationTestSuite) TestACharacterIsChargedAndGivenAnActionAndNothingLands() {
	data := s.fighterAt(1, 4, 12)
	definition := surge(resources.SecondWind)
	machine, err := NewAction(&ActionInput{Definition: *definition, AttackerID: activationFighterID})
	s.Require().NoError(err)

	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.effectWorld(),
		[]Participant{{Character: data}, s.wolfParticipant()}, s.surgeCost(definition, activationFighterID))

	s.Require().NoError(err)
	sheet := dirty(out, activationFighterID)
	s.Require().NotNil(sheet)
	s.Equal(2, sheet.ActionEconomy.ActionsRemaining, "the action is given")
	s.Equal(0, sheet.Resources[resources.SecondWind].Current, "the pool is spent")
	s.Equal(4, sheet.HitPoints, "nothing landed")
	outcome := out.Outcome.(ActivationOutcome)
	s.Equal(surgeWords, outcome.GrantedCapacity)
	s.Require().Len(outcome.Effects, 1)
	s.Equal(EffectCapacityGranted, outcome.Effects[0].Kind)
	s.Equal(activationFighterID, outcome.Effects[0].TargetID)
}

// TEST 3. All or none.
func (s *ActivationTestSuite) TestAnEmptyPoolGivesNothing() {
	data, _ := s.captain(5)
	data.Left[secondWindKey] = 0
	definition := surge(monster.PoolKey(secondWindKey))

	out, err := s.runSurgeOn(data, definition, captainID, s.surgeCost(definition, captainID))

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Nil(out)
	s.Equal(1, data.SlotsLeft(coreCombatType.ActionStandard), "the stored sheet still has one action")
}

// TEST 4. The payment is not optional, each refused before anything is charged.
func (s *ActivationTestSuite) TestThePaymentOfAnAbilityThatIsOnlyItsPriceIsNotOptional() {
	pool := monster.PoolKey(secondWindKey)
	for _, tc := range []struct {
		name    string
		cost    func(*combatActions.Definition) *Cost
		message string
	}{
		{"no cost", func(*combatActions.Definition) *Cost { return nil },
			"is only its price and carries none"},
		{"a cost with no profile", func(*combatActions.Definition) *Cost { return &Cost{PayerID: captainID} },
			"is only its price and carries none"},
		{"a profile that charges nothing", func(*combatActions.Definition) *Cost {
			return &Cost{PayerID: captainID, Profile: &combat.SpendProfile{
				GrantSlots: map[coreCombatType.ActionType]int{coreCombatType.ActionStandard: 1},
			}}
		}, "is only its price and its price charges nothing"},
		{"another payer", func(d *combatActions.Definition) *Cost { return s.surgeCost(d, wolfID) },
			wolfID},
		{"two actions against one", func(d *combatActions.Definition) *Cost {
			c := s.surgeCost(d, captainID)
			c.Profile = &combat.SpendProfile{
				Pools:      map[coreResources.ResourceKey]int{pool: 1},
				GrantSlots: map[coreCombatType.ActionType]int{coreCombatType.ActionStandard: 2},
			}
			return c
		}, "gives 1 action and its payment gives 2 actions"},
		{"a bonus action against an action", func(d *combatActions.Definition) *Cost {
			c := s.surgeCost(d, captainID)
			c.Profile = &combat.SpendProfile{
				Pools:      map[coreResources.ResourceKey]int{pool: 1},
				GrantSlots: map[coreCombatType.ActionType]int{coreCombatType.ActionBonus: 1},
			}
			return c
		}, "gives 1 action and its payment gives 1 bonus action"},
	} {
		s.Run(tc.name, func() {
			data, _ := s.captain(5)
			definition := surge(pool)

			out, err := s.runSurgeOn(data, definition, captainID, tc.cost(definition))

			s.Require().ErrorIs(err, ErrBadAction)
			s.Contains(err.Error(), tc.message)
			s.Nil(out)
			s.Equal(1, data.Left[secondWindKey], "nothing was charged")
			s.Equal(1, data.SlotsLeft(coreCombatType.ActionStandard), "nothing was given")
		})
	}
}

// TEST 4b. The rules call a description whose cost ONLY gives valid; running it
// for nothing is refused here, even when its own cost is sent as the payment.
func (s *ActivationTestSuite) TestAFreeGiftIsRefusedEvenWhenItsOwnCostIsSent() {
	data, _ := s.captain(5)
	definition := surge(monster.PoolKey(secondWindKey))
	definition.Cost = &combat.SpendProfile{
		GrantSlots: map[coreCombatType.ActionType]int{coreCombatType.ActionStandard: 1},
	}

	out, err := s.runSurgeOn(data, definition, captainID, s.surgeCost(definition, captainID))

	s.Require().ErrorIs(err, ErrBadAction)
	s.Contains(err.Error(), "is only its price and its price charges nothing")
	s.Nil(out)
	s.Equal(1, data.SlotsLeft(coreCombatType.ActionStandard), "nothing was given")
}

// TEST 5. Only a slot gift makes a description with no profile legal. The
// rules refuse the others in their own validation, and NewAction wraps that
// sentence in ErrBadAction.
func (s *ActivationTestSuite) TestOnlyAGiftOfASlotMakesADescriptionWithNoProfileLegal() {
	rules := "action must populate exactly one profile, or give a turn slot as its price"
	pool := monster.PoolKey(secondWindKey)
	for _, tc := range []struct {
		name string
		cost *combat.SpendProfile
	}{
		{"no cost", nil},
		{"a cost that only charges", &combat.SpendProfile{Pools: map[coreResources.ResourceKey]int{pool: 1}}},
		{"a cost that gives only capacity", &combat.SpendProfile{
			Grants: map[combat.CapacityType]int{combat.CapacityAttack: 1},
		}},
	} {
		s.Run(tc.name, func() {
			definition := surge(pool)
			definition.Cost = tc.cost

			_, err := NewAction(&ActionInput{Definition: *definition, AttackerID: captainID})

			s.Require().ErrorIs(err, ErrBadAction)
			s.Contains(err.Error(), rules)
			s.NotContains(err.Error(), noGiftWord, "the backstop is not what refuses it")
		})
	}
}

// TEST 6. A profile still wins: an effect with a cost that gives a slot runs
// as an effect, and the slot is given at the door.
func (s *ActivationTestSuite) TestADescriptionWithAProfileIsHandledByItsProfileWhateverItsCostGives() {
	data := s.fighterAt(1, 4, 12)
	definition := s.secondWindDescription(data)
	definition.Cost.GrantSlots = map[coreCombatType.ActionType]int{coreCombatType.ActionStandard: 1}

	out, err := s.resolveEffect(data, definition, events.NewEventBus(), s.wolfParticipant())

	s.Require().NoError(err)
	sheet := dirty(out, activationFighterID)
	s.Equal(11, sheet.HitPoints, "the heal lands")
	s.Equal(2, sheet.ActionEconomy.ActionsRemaining, "the slot is given at the door")
	outcome := out.Outcome.(ActivationOutcome)
	s.Equal("", outcome.GrantedCapacity, "the effect reports its heal, not the gift")
	s.Require().Len(outcome.Effects, 1)
	s.Equal(EffectHealingApplied, outcome.Effects[0].Kind)
}

// TEST 7. It names nobody else.
func (s *ActivationTestSuite) TestAnAbilityThatIsOnlyItsPriceNamesNobodyElse() {
	definition := surge(monster.PoolKey(secondWindKey))

	_, err := NewAction(&ActionInput{Definition: *definition, AttackerID: captainID, TargetID: wolfID})
	s.Require().ErrorIs(err, ErrBadAction)
	_, err = NewAction(&ActionInput{Definition: *definition, AttackerID: captainID,
		TargetIDs: []string{captainID, wolfID}})
	s.Require().ErrorIs(err, ErrBadAction)
	_, err = NewAction(&ActionInput{Definition: *definition})
	s.Require().ErrorIs(err, ErrBadAction, "nobody acts")

	for _, in := range []*ActionInput{
		{Definition: *definition, AttackerID: captainID, TargetID: captainID},
		{Definition: *definition, AttackerID: captainID, TargetIDs: []string{captainID}},
	} {
		data, _ := s.captain(5)
		machine, err := NewAction(in)
		s.Require().NoError(err)
		out, err := s.resolveOnBus(events.NewEventBus(), machine, s.captainWorld(),
			[]Participant{{Monster: data}, s.wolfParticipant()}, s.surgeCost(definition, captainID))
		s.Require().NoError(err)
		s.Equal(2, s.dirtyCaptain(out).SlotsLeft(coreCombatType.ActionStandard))
	}
}

// TEST 8. Given once per payment.
func (s *ActivationTestSuite) TestTheGiftIsGivenOncePerPayment() {
	definition := surge(monster.PoolKey(secondWindKey))

	s.Run("two uses", func() {
		data, _ := s.captain(5)
		data.Left[secondWindKey] = 2
		first, err := s.runSurgeOn(data, definition, captainID, s.surgeCost(definition, captainID))
		s.Require().NoError(err)
		second, err := s.runSurgeOn(s.dirtyCaptain(first), definition, captainID, s.surgeCost(definition, captainID))
		s.Require().NoError(err)
		s.Equal(3, s.dirtyCaptain(second).SlotsLeft(coreCombatType.ActionStandard))
	})

	s.Run("one use", func() {
		data, _ := s.captain(5)
		first, err := s.runSurgeOn(data, definition, captainID, s.surgeCost(definition, captainID))
		s.Require().NoError(err)
		saved := s.dirtyCaptain(first)
		second, err := s.runSurgeOn(saved, definition, captainID, s.surgeCost(definition, captainID))
		s.Require().ErrorIs(err, ErrCannotPay)
		s.Nil(second)
		s.Equal(2, saved.SlotsLeft(coreCombatType.ActionStandard))
	})
}
