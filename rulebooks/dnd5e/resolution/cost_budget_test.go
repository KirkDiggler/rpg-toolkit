// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

const ringCharges coreResources.ResourceKey = "charges"

// ringBudget is a budget that owns "charges" and records what it is asked.
type ringBudget struct {
	charges int
	spends  int
}

func (b *ringBudget) HasPool(key coreResources.ResourceKey) bool { return key == ringCharges }
func (b *ringBudget) PoolLeft(key coreResources.ResourceKey) int {
	if key == ringCharges {
		return b.charges
	}
	return 0
}

func (b *ringBudget) SpendPool(key coreResources.ResourceKey, n int) {
	if key == ringCharges {
		b.charges -= n
		b.spends++
	}
}

func ringDescription(t *testing.T) *combatActions.Definition {
	t.Helper()
	definition, err := features.DescribeFromRef(&features.DescribeFromRefInput{
		Ref:        *refs.Features.SecondWind(),
		Parameters: json.RawMessage(`{"level": 1}`),
		Giver: dnd5eEvents.RollSource{
			Ref:      &core.Ref{Module: "example", Type: "items", ID: "second_wind_ring"},
			Name:     "Second Wind ring",
			Label:    "Ring level",
			SourceID: "ring-17",
		},
		Pools: map[coreResources.ResourceKey]int{ringCharges: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	return definition
}

// budgetFixtures borrows the activation suite's fighter, world and door, which
// is how every other effect in this module is resolved.
func budgetFixtures(t *testing.T) *ActivationTestSuite {
	t.Helper()
	a := new(ActivationTestSuite)
	a.SetT(t)
	a.SetupTest()
	return a
}

// wizardData is a hurt character in a fight with a bonus action and no fighter
// levels, adapted from the activation suite's fighter fixture: same sheet, a
// wizard's class, no Second Wind feature and no pools of its own.
func wizardData(a *ActivationTestSuite) *character.Data {
	data := a.fighter(activationFighterID, 4, 12)
	data.Name = "Wizard"
	data.ClassID = classes.Wizard
	data.Features = nil
	data.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{}
	return data
}

func resolveRing(a *ActivationTestSuite, data *character.Data, definition *combatActions.Definition,
	cost *Cost, extra ...Participant,
) (*Output, error) {
	machine, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: activationFighterID, Roller: fixedRoller{face: 6},
	})
	a.Require().NoError(err)
	return a.resolveOnBus(events.NewEventBus(), machine, a.effectWorld(),
		append([]Participant{{Character: data}}, extra...), cost)
}

func TestCostBudget(t *testing.T) {
	t.Run("a wizard uses the ring", func(t *testing.T) {
		a := budgetFixtures(t)
		data := wizardData(a)
		budget := &ringBudget{charges: 1}
		definition := ringDescription(t)
		cost := a.effectCost(definition, activationFighterID)
		cost.Budget = budget

		out, err := resolveRing(a, data, definition, cost, a.wolfParticipant())

		a.Require().NoError(err)
		sheet := dirty(out, activationFighterID)
		a.Require().NotNil(sheet)
		a.Equal(11, sheet.HitPoints, "4 plus a rolled 6 plus the ring's 1")
		a.Zero(sheet.ActionEconomy.BonusActionsRemaining, "the bonus action is spent")
		a.Zero(budget.charges, "the budget paid its charge")
		a.Equal(1, budget.spends)
		a.Empty(sheet.Resources, "the character's own pools are untouched")
	})

	t.Run("the bonus action is already spent", func(t *testing.T) {
		a := budgetFixtures(t)
		data := wizardData(a)
		data.ActionEconomy.BonusActionsRemaining = 0
		budget := &ringBudget{charges: 1}
		definition := ringDescription(t)
		cost := a.effectCost(definition, activationFighterID)
		cost.Budget = budget

		out, err := resolveRing(a, data, definition, cost, a.wolfParticipant())

		a.Require().ErrorIs(err, ErrCannotPay)
		a.Nil(out, "nobody was healed")
		a.Equal(1, budget.charges)
		a.Zero(budget.spends, "the budget was never asked to spend")
		a.Equal(4, data.HitPoints)
	})

	t.Run("the budget is at 0 and the bonus action is kept", func(t *testing.T) {
		a := budgetFixtures(t)
		data := wizardData(a)
		definition := ringDescription(t)
		empty := &ringBudget{}
		cost := a.effectCost(definition, activationFighterID)
		cost.Budget = empty

		out, err := resolveRing(a, data, definition, cost, a.wolfParticipant())

		a.Require().ErrorIs(err, ErrCannotPay)
		a.Contains(err.Error(), "charges")
		a.Nil(out)
		a.Zero(empty.spends)
		a.Equal(1, data.ActionEconomy.BonusActionsRemaining, "the bonus action was not taken")

		// The same saved data, a budget of 1: it succeeds.
		full := &ringBudget{charges: 1}
		cost = a.effectCost(definition, activationFighterID)
		cost.Budget = full
		out, err = resolveRing(a, data, definition, cost, a.wolfParticipant())
		a.Require().NoError(err)
		a.Equal(11, dirty(out, activationFighterID).HitPoints)
		a.Zero(full.charges)
	})

	t.Run("no budget named is refused as today", func(t *testing.T) {
		a := budgetFixtures(t)
		data := wizardData(a)
		definition := ringDescription(t)
		cost := a.effectCost(definition, activationFighterID)

		out, err := resolveRing(a, data, definition, cost, a.wolfParticipant())

		a.Require().ErrorIs(err, ErrCannotPay)
		a.Contains(err.Error(), "charges: 1 needed, 0 left")
		a.Nil(out)
	})

	t.Run("a fighter's own pool is paid from the fighter", func(t *testing.T) {
		a := budgetFixtures(t)
		data := a.fighterAt(1, 4, 12)
		definition := a.secondWindDescription(data)
		budget := &ringBudget{charges: 1}
		cost := a.effectCost(definition, activationFighterID)
		cost.Budget = budget

		out, err := resolveRing(a, data, definition, cost, a.wolfParticipant())

		a.Require().NoError(err)
		sheet := dirty(out, activationFighterID)
		a.Equal(0, sheet.Resources[resources.SecondWind].Current, "the fighter's own pool paid")
		a.Equal(1, budget.charges)
		a.Zero(budget.spends, "the budget was untouched")
	})

	t.Run("a monster payer with a budget is refused", func(t *testing.T) {
		a := budgetFixtures(t)
		data := wizardData(a)
		definition := ringDescription(t)
		budget := &ringBudget{charges: 1}
		cost := a.effectCost(definition, wolfID)
		cost.Budget = budget

		out, err := resolveRing(a, data, definition, cost, a.wolfParticipant())

		a.Require().ErrorIs(err, ErrNoPayer)
		a.Contains(err.Error(), "one reaction and nothing else")
		a.Nil(out)
		a.Equal(1, budget.charges)
		a.Zero(budget.spends)
	})
}

func TestCostBudgetOnASpellIsRefusedBeforePayment(t *testing.T) {
	s := new(CastActionTestSuite)
	s.SetT(t)
	s.SetupTest()
	f := s.fixtures()
	caster := baneCaster(1)
	ch, err := character.Load(s.ctx, caster)
	s.Require().NoError(err)
	definition := ch.CastDefinition(spells.CureWounds)
	machine, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: bardID, TargetIDs: []string{wolfID},
		Roller: &countingCastRoller{facedRoller: facedRoller{other: 5}},
	})
	s.Require().NoError(err)
	budget := &ringBudget{charges: 1}

	out, err := Resolve(s.ctx, &Input{
		World:        f.world(),
		Participants: []Participant{{Character: caster}, {Character: f.saver(14)}, {Monster: f.wolfData()}},
		Machine:      machine,
		Cost: &Cost{
			SpellTurn: "scene/round-1/bard", PayerID: bardID, Profile: definition.Cost,
			Turn: &Turn{Number: mockeryTurn, Speed: mockerySpeed}, Budget: budget,
		},
		Capabilities: budgetCapabilities(),
	})

	s.Require().ErrorIs(err, ErrBadCost)
	s.Contains(err.Error(), "a spell's cost cannot name a second budget")
	s.Nil(out, "nothing was paid")
	s.Equal(1, budget.charges)
	s.Zero(budget.spends)
	s.Equal(1, caster.ActionEconomy.ActionsRemaining, "the caster's action is untouched")
}

// budgetCapabilities is the capability set every door test in this module uses.
func budgetCapabilities() encounter.Capabilities {
	return encounter.Capabilities{
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Participation: everyoneParticipates{},
		Sight:         everyoneSeesTheWholeMap{},
		Equipment:     noHandsAreObserved{},
		Conditions:    noHandsAreObserved{},
		Sheets:        noSheetsAsked{},
		Roller:        dice.NewRoller(),
		Actors:        Actors,
	}
}
