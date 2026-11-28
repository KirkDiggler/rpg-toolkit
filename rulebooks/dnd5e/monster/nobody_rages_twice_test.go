// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const rageAndSecondWindLets = `[` +
	`{"use":"rage","with":{"damage_bonus":2},"uses":{"max":2,"refills":{"on":["long rest"]}}},` +
	`{"use":"second wind","with":{"level":1},"uses":{"max":1,"refills":{"on":["short rest","long rest"]}}}]`

// ragingCreature loads a creature with Rage and Second Wind and puts the
// raging status on it the way the door does: built with the ability and its
// user as the origin, then published to the creature as a feature's.
func ragingCreature(t *testing.T) *monster.Monster {
	t.Helper()
	ctx := context.Background()
	in := humanOver(rageAndSecondWindLets)
	made, err := monster.FromTemplate(&in)
	require.NoError(t, err)
	bus := events.NewEventBus()
	creature, err := monster.LoadFromData(ctx, made.ToData(), bus)
	require.NoError(t, err)

	granted, err := creature.GrantedAbility(&monster.GrantedAbilityInput{Grant: "rage"})
	require.NoError(t, err, "not raging: the description")
	status := granted.Definition.Effect.Condition
	built, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref: status.Ref.String(), Config: status.Parameters, MemberID: creature.GetID(),
		Origin: &dnd5eEvents.ConditionOrigin{Ref: granted.Definition.Ref, EntityID: creature.GetID()},
	})
	require.NoError(t, err)
	require.NoError(t, dnd5eEvents.ConditionAppliedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: creature, Source: dnd5eEvents.ConditionSourceFeature, Condition: built.Condition,
	}))
	require.Len(t, creature.GetConditions(), 1)
	require.Equal(t, refs.Conditions.Raging().String(), creature.GetConditions()[0].Ref().String())
	return creature
}

// Test 6: a creature, live.
func TestARagingCreatureIsRefusedItsRageAgain(t *testing.T) {
	creature := ragingCreature(t)

	_, err := creature.GrantedAbility(&monster.GrantedAbilityInput{Grant: "rage"})
	require.Error(t, err)
	assert.Equal(t, "already raging", err.Error())
	assert.Equal(t, rpgerr.CodeConflictingState, rpgerr.GetCode(err))

	wind, err := creature.GrantedAbility(&monster.GrantedAbilityInput{Grant: "second-wind"})
	require.NoError(t, err)
	assert.Equal(t, "Second Wind", wind.Definition.Name)
}

// Test 7: a creature's stored sheet.
func TestAStoredRagingSheetListsRageRefusedAndStillPayable(t *testing.T) {
	creature := ragingCreature(t)
	byGrant := func(sheet *monster.Data) map[string]monster.Ability {
		out := map[string]monster.Ability{}
		for _, a := range abilitiesOf(t, sheet) {
			out[a.Grant] = a
		}
		return out
	}

	raging := byGrant(creature.ToData())
	require.Len(t, raging, 2)
	assert.True(t, raging["rage"].Payable, "Payable is the gate's answer about the price, unchanged")
	assert.Equal(t, "already raging", raging["rage"].Refused)
	assert.True(t, raging["second-wind"].Payable)
	assert.Empty(t, raging["second-wind"].Refused)

	in := humanOver(rageAndSecondWindLets)
	made, err := monster.FromTemplate(&in)
	require.NoError(t, err)
	calm := byGrant(made.ToData())
	assert.Empty(t, calm["rage"].Refused)
	assert.Empty(t, calm["second-wind"].Refused)
	assert.True(t, calm["rage"].Payable)
}
