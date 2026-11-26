// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// One slot, as a monster's price: the door charges a monster through the same
// gate it charges a character through (rpg-project#1131, 2026-11-26).
func oneSlot(slot coreCombat.ActionType) *combat.SpendProfile {
	return &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{slot: 1}}
}

// TestTheDoorChargesAMonsterEachSlotOnceFromItsSheet: a bonus action, an
// action and a reaction are each paid once from the monster's own sheet, the
// other two slots stay where they were, and a second bill is the gate's
// refusal with the sheet exactly as the first left it.
func TestTheDoorChargesAMonsterEachSlotOnceFromItsSheet(t *testing.T) {
	slots := map[string]coreCombat.ActionType{
		"a bonus action": coreCombat.ActionBonus,
		"an action":      coreCombat.ActionStandard,
		"a reaction":     coreCombat.ActionReaction,
	}
	for name, paid := range slots {
		t.Run(name, func(t *testing.T) {
			cast, wolf := monsterCast(t)
			cost := &Cost{PayerID: wolfID, Profile: oneSlot(paid)}

			require.NoError(t, payAtTheDoor(context.Background(), cost, cast))
			require.True(t, wolf.IsDirty(), "a spend dirties the sheet, or the write-back drops it")
			for _, slot := range slots {
				want := 1
				if slot == paid {
					want = 0
				}
				require.Equal(t, want, wolf.SlotsLeft(slot), "only the paid slot is gone")
			}

			err := payAtTheDoor(context.Background(), cost, cast)
			require.ErrorIs(t, err, ErrCannotPay)
			require.Contains(t, err.Error(), wolfID)
			for _, slot := range slots {
				want := 1
				if slot == paid {
					want = 0
				}
				require.Equal(t, want, wolf.SlotsLeft(slot), "the refused bill changed nothing")
			}
		})
	}
}

// TestTheDoorChargesAMonsterAllOrNone: a price of a bonus action and a point
// of a pool is refused whole, because a monster has no pool, and the bonus
// action is still there.
func TestTheDoorChargesAMonsterAllOrNone(t *testing.T) {
	cast, wolf := monsterCast(t)
	cost := &Cost{PayerID: wolfID, Profile: &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools: map[coreResources.ResourceKey]int{ringCharges: 1},
	}}

	err := payAtTheDoor(context.Background(), cost, cast)

	require.ErrorIs(t, err, ErrCannotPay)
	require.Equal(t, 1, wolf.SlotsLeft(coreCombat.ActionBonus), "the bonus action was not taken")
	require.False(t, wolf.IsDirty(), "a refusal writes nothing")
}

// TestTheDoorChargesAMonsterNothingForANilProfile: a free price is free for a
// monster as for a character, and the sheet is neither changed nor dirty.
func TestTheDoorChargesAMonsterNothingForANilProfile(t *testing.T) {
	cast, wolf := monsterCast(t)

	require.NoError(t, payAtTheDoor(context.Background(), &Cost{PayerID: wolfID}, cast))

	require.False(t, wolf.IsDirty())
	require.Nil(t, wolf.ToData().Economy, "no account was made for a free price")
}

// TestATurnOnAMonstersCostRefreshesNothing: Turn is a character's marker. A
// monster's sheet gives its turn back at its own turn start, so a new turn
// number on its cost does not refill what it spent.
func TestATurnOnAMonstersCostRefreshesNothing(t *testing.T) {
	cast, wolf := monsterCast(t)
	cost := &Cost{PayerID: wolfID, Profile: oneSlot(coreCombat.ActionBonus), Turn: &Turn{Number: 1, Speed: 30}}
	require.NoError(t, payAtTheDoor(context.Background(), cost, cast))

	cost.Turn = &Turn{Number: 2, Speed: 30}
	err := payAtTheDoor(context.Background(), cost, cast)

	require.ErrorIs(t, err, ErrCannotPay)
	require.Equal(t, 0, wolf.SlotsLeft(coreCombat.ActionBonus), "a new turn number refilled nothing")
}

// TestASecondBudgetPaysItsPoolsForAMonster: the budget pays the pool it owns,
// the monster pays its bonus action, in one payment.
func TestASecondBudgetPaysItsPoolsForAMonster(t *testing.T) {
	cast, wolf := monsterCast(t)
	budget := &ringBudget{charges: 1}
	cost := &Cost{PayerID: wolfID, Profile: &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1},
		Pools: map[coreResources.ResourceKey]int{ringCharges: 1},
	}}

	require.NoError(t, payAtTheDoorFrom(context.Background(), cost, budget, cast))

	require.Equal(t, 0, wolf.SlotsLeft(coreCombat.ActionBonus))
	require.Equal(t, 0, budget.charges)
	require.Equal(t, 1, budget.spends, "the budget was debited once")
}

// A driven Disengage with a cost is charged at the door, then lands.
func (s *ActivationTestSuite) TestADrivenDisengageWithACostIsChargedThenLands() {
	machine, err := NewActivation(drivenDisengage())
	s.Require().NoError(err)

	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.drivenWorld(),
		s.drivenCast(monsters.NewGoblin(goblinID).ToData()),
		&Cost{PayerID: goblinID, Profile: oneSlot(coreCombat.ActionBonus)})
	s.Require().NoError(err)

	got, ok := out.Outcome.(ActivationOutcome)
	s.Require().True(ok)
	s.Require().Len(got.Effects, 1)
	s.Equal(refs.Conditions.Disengaging().String(), got.Effects[0].Ref)
	goblin := s.dirtyMonster(out)
	s.Equal(0, goblin.SlotsLeft(coreCombat.ActionBonus), "the bonus action was paid at the door")
	s.Equal(1, goblin.SlotsLeft(coreCombat.ActionStandard))
	s.Equal(1, goblin.SlotsLeft(coreCombat.ActionReaction))
}

// A driven Disengage the monster cannot pay for does not land.
func (s *ActivationTestSuite) TestADrivenDisengageTheMonsterCannotPayForDoesNotLand() {
	spent := monsters.NewGoblin(goblinID).ToData()
	spent.Economy = combat.NewActionEconomy()
	spent.Economy.SpendSlots(coreCombat.ActionBonus, 1)
	machine, err := NewActivation(drivenDisengage())
	s.Require().NoError(err)

	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.drivenWorld(), s.drivenCast(spent),
		&Cost{PayerID: goblinID, Profile: oneSlot(coreCombat.ActionBonus)})

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Contains(err.Error(), goblinID)
	s.Nil(out, "no outcome, so no condition landed")
	s.NotContains(monsterConditionRefs(spent), refs.Conditions.Disengaging().String())
}

// A driven Disengage with no cost charges nothing, as before: its driver
// decided it, and the sheet keeps its bonus action.
func (s *ActivationTestSuite) TestADrivenDisengageWithNoCostChargesNothing() {
	out, err := s.run(drivenDisengage(), s.drivenWorld(), s.drivenCast(monsters.NewGoblin(goblinID).ToData()))
	s.Require().NoError(err)

	s.Equal(1, s.dirtyMonster(out).SlotsLeft(coreCombat.ActionBonus))
}

// THE TURN COMES BACK: the sheet a paid driven Disengage left has no bonus
// action; a turn start for the goblin, published on the bus its sheet is
// attached to (as the composition's boundary publishes it), leaves a full turn.
func (s *ActivationTestSuite) TestAMonstersTurnStartGivesBackWhatTheDoorCharged() {
	machine, err := NewActivation(drivenDisengage())
	s.Require().NoError(err)
	out, err := s.resolveOnBus(events.NewEventBus(), machine, s.drivenWorld(),
		s.drivenCast(monsters.NewGoblin(goblinID).ToData()),
		&Cost{PayerID: goblinID, Profile: oneSlot(coreCombat.ActionBonus)})
	s.Require().NoError(err)
	saved := s.dirtyMonster(out)
	s.Require().Equal(0, saved.SlotsLeft(coreCombat.ActionBonus), "precondition: the door charged it")

	bus := events.NewEventBus()
	goblin, err := monster.LoadFromData(s.ctx, saved, bus)
	s.Require().NoError(err)
	s.Require().NoError(dnd5eEvents.TurnStartTopic.On(bus).Publish(s.ctx,
		dnd5eEvents.TurnStartEvent{SubjectID: goblinID, Round: 2}))

	for _, slot := range []coreCombat.ActionType{
		coreCombat.ActionStandard, coreCombat.ActionBonus, coreCombat.ActionReaction,
	} {
		s.Equal(1, goblin.SlotsLeft(slot), "a full turn")
	}
}
