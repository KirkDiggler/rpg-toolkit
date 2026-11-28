// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// rageRingItem is an item with two charges that lets its wearer rage.
func rageRingItem(t *testing.T) magicitems.Data {
	t.Helper()
	out, err := magicitems.Compile(&magicitems.CompileInput{ID: "rage-cloak", Authored: json.RawMessage(
		`{"name":"Rage cloak","slot":"cloak","charges":{"max":2,"refills":{"on":["long rest"]}},` +
			`"lets":[{"use":"rage","with":{"damage_bonus":2},"cost":{"charges":1}}]}`)})
	require.NoError(t, err)
	return newItem(t, "rage-cloak", out.Description)
}

// useRageFromTheItem stands in for the resolver's effect machine for a worn
// item's Rage, as useRageFromItsDescription does for the barbarian's own: it
// asks the sheet for the grant's description (a refusal is returned, nothing
// built or paid), builds the status with the item's ability and its wearer as
// the origin, pays the cost at the gate, and publishes the status.
func useRageFromTheItem(ctx context.Context, t *testing.T, c *Character) error {
	t.Helper()
	out, err := c.GrantedAbility(&GrantedAbilityInput{ItemID: "rage-cloak", Grant: "rage"})
	if err != nil {
		return err
	}
	status := out.Definition.Effect.Condition
	built, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref: status.Ref.String(), Config: status.Parameters, MemberID: c.GetID(),
		Origin: &dnd5eEvents.ConditionOrigin{Ref: out.Definition.Ref, EntityID: c.GetID()},
	})
	require.NoError(t, err)
	if err := combat.Pay(c, out.Definition.Cost); err != nil {
		return err
	}
	return dnd5eEvents.ConditionAppliedTopic.On(c.bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: c, Source: dnd5eEvents.ConditionSourceFeature, Condition: built.Condition,
	})
}

func stopRaging(ctx context.Context, t *testing.T, c *Character) {
	t.Helper()
	require.NoError(t, dnd5eEvents.ConditionRemovedTopic.On(c.bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID: c.GetID(), ConditionRef: refs.Conditions.Raging().String(),
	}))
}

func rageRowOf(c *Character, own bool) AvailableAbility {
	for _, a := range c.AvailableAbilities() {
		if a.Ref.ID == refs.Features.Rage().ID && (a.Giver == nil) == own {
			return a
		}
	}
	panic("no rage row")
}

func isRaging(c *Character) bool {
	for _, condition := range c.GetConditions() {
		if condition.Ref().String() == refs.Conditions.Raging().String() {
			return true
		}
	}
	return false
}

// Test 4: a wearer who is not a barbarian is not offered Rage again.
func TestAWearerRagingFromAnItemIsNotOfferedRageAgain(t *testing.T) {
	ctx := context.Background()
	c := wearer(t, classes.Wizard, map[InventorySlot]string{SlotCloak: "rage-cloak"}, rageRingItem(t))
	c.actionEconomy.BonusActionsRemaining = 2 // so only the status can refuse the second use

	require.NoError(t, useRageFromTheItem(ctx, t, c))
	require.True(t, isRaging(c))
	bonus := c.actionEconomy.BonusActionsRemaining
	charges := cloakOf(c, "rage-cloak").Left["charges"]
	require.Equal(t, 1, charges)

	row := rageRowOf(c, false)
	assert.False(t, row.CanUse)
	assert.Equal(t, "already raging", row.Reason)

	err := useRageFromTheItem(ctx, t, c)
	require.Error(t, err)
	assert.Equal(t, "already raging", err.Error())
	assert.Equal(t, rpgerr.CodeConflictingState, rpgerr.GetCode(err))
	assert.Equal(t, bonus, c.actionEconomy.BonusActionsRemaining, "no bonus action spent")
	assert.Equal(t, charges, cloakOf(c, "rage-cloak").Left["charges"], "no charge spent")

	stopRaging(ctx, t, c)
	require.False(t, isRaging(c))
	assert.True(t, rageRowOf(c, false).CanUse, "usable again once the status is gone")
}

// Test 5: whoever gave the status, a held Raging refuses the other giver.
type NobodyRagesTwiceSuite struct {
	suite.Suite
	ctx context.Context
}

func TestNobodyRagesTwiceSuite(t *testing.T) { suite.Run(t, new(NobodyRagesTwiceSuite)) }

func (s *NobodyRagesTwiceSuite) SetupTest() { s.ctx = context.Background() }

func (s *NobodyRagesTwiceSuite) barbarianWearing() (*Character, events.EventBus) {
	data := fullSheet(&s.Suite)
	data.Conditions = nil
	data.ActionEconomy.BonusActionsRemaining = 2
	s.Require().NoError(AddItem(data, rageRingItem(s.T())))
	bus := events.NewEventBus()
	char, err := LoadFromData(s.ctx, data, bus)
	s.Require().NoError(err)
	s.Require().NoError(char.EquipItem(SlotCloak, "rage-cloak"))
	return char, bus
}

func (s *NobodyRagesTwiceSuite) TestABarbarianRagingFromHerOwnRageIsRefusedTheItems() {
	char, bus := s.barbarianWearing()
	s.Require().NoError(useRageFromItsDescription(s.ctx, s.T(), char, bus))
	s.Require().True(isRaging(char))

	row := rageRowOf(char, false)
	s.False(row.CanUse)
	s.Equal("already raging", row.Reason)
	_, err := char.GrantedAbility(&GrantedAbilityInput{ItemID: "rage-cloak", Grant: "rage"})
	s.Require().Error(err)
	s.Equal("already raging", err.Error())
}

func (s *NobodyRagesTwiceSuite) TestABarbarianRagingFromTheItemReadsAlreadyRagingOnHerOwnRow() {
	char, _ := s.barbarianWearing()
	s.Require().NoError(useRageFromTheItem(s.ctx, s.T(), char))
	s.Require().True(isRaging(char))

	own := rageRowOf(char, true)
	s.False(own.CanUse)
	s.Contains(own.Reason, "already raging")
}

// The two rows when two reasons are true at once: raging, and no bonus action
// left.
func (s *NobodyRagesTwiceSuite) TestTheTwoRowsWhenRagingAndNoBonusActionLeft() {
	char, bus := s.barbarianWearing()
	s.Require().NoError(useRageFromItsDescription(s.ctx, s.T(), char, bus))
	s.Require().NoError(combat.Pay(char, &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}}))
	s.Require().Equal(0, char.actionEconomy.BonusActionsRemaining)

	own, item := rageRowOf(char, true), rageRowOf(char, false)
	s.T().Logf("own row reason: %q; item row reason: %q", own.Reason, item.Reason)
	s.False(own.CanUse)
	s.False(item.CanUse)
	s.Equal("already raging", item.Reason, "the held status wins over the slot's reason")
	s.Equal("no bonus action remaining", own.Reason,
		"the barbarian's own row still names the slot first: the two rows differ, reported on the PR")
}
