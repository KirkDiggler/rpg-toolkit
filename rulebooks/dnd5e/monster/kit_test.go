// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// humanWithAMaceBeforeTheSlice is the JSON of the ToData() of the human base
// with a mace, built through FromTemplate with no Lets, captured on origin/main
// at d9a8cc1b, before the sheet held a kit.
const humanWithAMaceBeforeTheSlice = `{"creature_type":"humanoid","id":"g-1","name":"Human","ref":"dnd5e:monsters:test-template","hit_points":4,"max_hit_points":4,"armor_class":10,"ability_scores":{"cha":10,"con":10,"dex":10,"int":10,"str":10,"wis":10},"proficiency_bonus":2,"speed":{"walk":30},"senses":{"passive_perception":10},"actions":[{"description":"Make a melee attack against a creature in reach.","ref":"dnd5e:weapons:mace","name":"Mace","attack":{"category":"weapon","delivery":{"melee":{"reach_feet":5}},"attack_bonus":2,"ability":{"ability":"str","modifier":0},"weapon":{"ref":"dnd5e:weapons:mace","two_handed":false},"damage":[{"dice":"1d6","type":"bludgeoning","properties":["adds-attack-ability-modifier"]}]}}]}`

// secondWindLets is a stat block's one entry: Second Wind with its own one use.
const secondWindLets = `[{"use":"second wind","with":{"level":1},"uses":{"max":1,"refills":{"on":["short rest","long rest"]}}}]`

func humanOver(lets string) monster.FromTemplateInput {
	template := monster.Template{Base: refs.Monsters.Human(), Actions: []weapons.WeaponID{weapons.Mace}}
	if lets != "" {
		template.Lets = json.RawMessage(lets)
	}

	return monster.FromTemplateInput{ID: "g-1", Ref: testRef, Template: template, Base: monsters.Human}
}

func creatureWithAKit(t *testing.T) *monster.Monster {
	t.Helper()

	in := humanOver(secondWindLets)
	m, err := monster.FromTemplate(&in)
	require.NoError(t, err)

	return m
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()

	raw, err := json.Marshal(v)
	require.NoError(t, err)

	return string(raw)
}

// Test 1.
func TestACreatureWithNoKitSavesWhatItSaved(t *testing.T) {
	in := humanOver("")
	m, err := monster.FromTemplate(&in)
	require.NoError(t, err)

	saved := jsonOf(t, m.ToData())

	require.NotContains(t, saved, "\"kit\"")
	require.NotContains(t, saved, "\"left\"")
	require.Equal(t, humanWithAMaceBeforeTheSlice, saved, "byte for byte what it saved before the slice")
}

// Test 2.
func TestAStatBlocksLetsBuildsAKitWithFullCounts(t *testing.T) {
	data := creatureWithAKit(t).ToData()

	require.NotNil(t, data.Kit)
	assert.Equal(t, map[string]kits.Pool{
		"second-wind": {Maximum: 1, RestoreOn: []coreResources.ResetType{coreResources.ResetShortRest, coreResources.ResetLongRest}},
	}, data.Kit.Pools)
	require.Len(t, data.Kit.Grants, 1)
	grant := data.Kit.Grants["second-wind"]
	assert.Equal(t, *refs.Features.SecondWind(), grant.Ability)
	assert.Equal(t, "second-wind", grant.Pool)
	assert.Equal(t, 1, grant.Uses)
	assert.JSONEq(t, `{"level":1}`, string(grant.Parameters))
	assert.Equal(t, map[string]int{"second-wind": 1}, data.Left)

	saved := jsonOf(t, data)
	assert.Contains(t, saved, `"left":{"second-wind":1}`)
	assert.Contains(t, saved, `"restore_on":["short_rest","long_rest"]`)
}

func payBonusAction(t *testing.T, m *monster.Monster) error {
	t.Helper()

	return combat.Pay(m, &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}})
}

// Test 4.
func TestOnePaymentTakesTheSlotAndTheUseAllOrNone(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	m := creatureWithAKit(t)
	require.NoError(t, m.SheetKeeper().Apply(ctx, bus))
	out, err := m.GrantedAbility(&monster.GrantedAbilityInput{Grant: "second-wind"})
	require.NoError(t, err)
	cost := out.Definition.Cost
	key := monster.PoolKey("second-wind")

	require.True(t, combat.CanPay(m, cost))
	require.NoError(t, combat.Pay(m, cost))
	assert.Equal(t, 0, m.SlotsLeft(coreCombat.ActionBonus), "the bonus action is gone")
	assert.Equal(t, 0, m.PoolLeft(key), "and the use is gone")

	require.NoError(t, dnd5eEvents.TurnStartTopic.On(bus).Publish(ctx,
		dnd5eEvents.TurnStartEvent{SubjectID: m.GetID(), Round: 2}))
	require.Equal(t, 1, m.SlotsLeft(coreCombat.ActionBonus))

	require.Error(t, combat.Pay(m, cost), "a second payment is refused: the use is gone")
	assert.Equal(t, 1, m.SlotsLeft(coreCombat.ActionBonus), "and the bonus action is still there")

	other := creatureWithAKit(t)
	require.NoError(t, payBonusAction(t, other))
	require.Error(t, combat.Pay(other, cost), "with the bonus action spent first, the payment is refused")
	assert.Equal(t, 1, other.PoolLeft(key), "and the use is still there")
}

// Test 5.
func TestATurnStartGivesTheTurnBackAndLeavesTheCountAlone(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	m := creatureWithAKit(t)
	require.NoError(t, m.SheetKeeper().Apply(ctx, bus))
	out, err := m.GrantedAbility(&monster.GrantedAbilityInput{Grant: "second-wind"})
	require.NoError(t, err)
	require.NoError(t, combat.Pay(m, out.Definition.Cost))

	require.NoError(t, dnd5eEvents.TurnStartTopic.On(bus).Publish(ctx,
		dnd5eEvents.TurnStartEvent{SubjectID: m.GetID(), Round: 2}))

	assert.Equal(t, 1, m.SlotsLeft(coreCombat.ActionBonus), "the turn is back")
	assert.Equal(t, 0, m.PoolLeft(monster.PoolKey("second-wind")), "the count is not")
	assert.Equal(t, map[string]int{"second-wind": 0}, m.ToData().Left)
}

// Test 6.
func TestSaveAndLoadCarryTheKitAndShareNothing(t *testing.T) {
	m := creatureWithAKit(t)
	out, err := m.GrantedAbility(&monster.GrantedAbilityInput{Grant: "second-wind"})
	require.NoError(t, err)
	require.NoError(t, combat.Pay(m, out.Definition.Cost))

	var back monster.Data
	require.NoError(t, json.Unmarshal([]byte(jsonOf(t, m.ToData())), &back))
	loaded, err := monster.Load(context.Background(), &back)
	require.NoError(t, err)
	assert.Equal(t, 0, loaded.PoolLeft(monster.PoolKey("second-wind")), "none left, after a save and a load")
	assert.Equal(t, jsonOf(t, m.ToData()), jsonOf(t, loaded.ToData()))

	// The returned Data shares nothing with the live creature.
	data := m.ToData()
	data.Left["second-wind"] = 1
	data.Kit.Pools["second-wind"] = kits.Pool{Maximum: 9}
	grant := data.Kit.Grants["second-wind"]
	grant.Parameters[len(grant.Parameters)-2] = '9'
	data.Kit.Grants["other"] = kits.Grant{}
	assert.Equal(t, 0, m.PoolLeft(monster.PoolKey("second-wind")))
	assert.Equal(t, []string{"second-wind"}, m.Grants())
	after := m.ToData()
	assert.Equal(t, 1, after.Kit.Pools["second-wind"].Maximum)
	assert.JSONEq(t, `{"level":1}`, string(after.Kit.Grants["second-wind"].Parameters))

	// ...and the live creature shares nothing with the Data it was loaded from.
	saved := m.ToData()
	fresh, err := monster.Load(context.Background(), saved)
	require.NoError(t, err)
	fresh.SpendPool(monster.PoolKey("second-wind"), 1)
	saved.Left["second-wind"] = 1
	saved.Kit.Pools["second-wind"] = kits.Pool{Maximum: 9}
	assert.Equal(t, 0, fresh.PoolLeft(monster.PoolKey("second-wind")))
	assert.Equal(t, 1, fresh.ToData().Kit.Pools["second-wind"].Maximum)
}

// Test 7.
func TestABadSavedSheetIsRefusedAtLoad(t *testing.T) {
	good := creatureWithAKit(t).ToData()
	cases := map[string]struct {
		change func(d *monster.Data)
		want   string
	}{
		"a count for a pool the kit does not declare": {
			func(d *monster.Data) { d.Left["rage"] = 1 },
			`stat block "Human" keeps a count for pool "rage", which the stat block does not declare`,
		},
		"no count for a pool it does": {
			func(d *monster.Data) { d.Left = nil },
			`stat block "Human" keeps no count for pool "second-wind"`,
		},
		"a count above the maximum": {
			func(d *monster.Data) { d.Left["second-wind"] = 2 },
			`stat block "Human" pool "second-wind" holds 2, outside 0 to 1`,
		},
		"a count below zero": {
			func(d *monster.Data) { d.Left["second-wind"] = -1 },
			`stat block "Human" pool "second-wind" holds -1, outside 0 to 1`,
		},
		"counts with no kit": {
			func(d *monster.Data) { d.Kit = nil },
			`stat block "Human" keeps a count for pool "second-wind", which the stat block does not declare`,
		},
		"a kit the rulebook cannot run": {
			func(d *monster.Data) {
				d.Kit.Pools["second-wind"] = kits.Pool{Maximum: 0}
			},
			`stat block "Human" pool "second-wind" maximum 0 must be at least 1`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var d monster.Data
			require.NoError(t, json.Unmarshal([]byte(jsonOf(t, good)), &d))
			tc.change(&d)

			_, err := monster.Load(context.Background(), &d)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.NotContains(t, err.Error(), "item")
		})
	}
}

// Test 10.
func TestACreatureWithNoKitHasNothingToUse(t *testing.T) {
	in := humanOver("")
	m, err := monster.FromTemplate(&in)
	require.NoError(t, err)

	_, err = m.GrantedAbility(&monster.GrantedAbilityInput{Grant: "second-wind"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"second-wind"`)
	assert.Empty(t, m.Grants())
	assert.Equal(t, 0, m.PoolLeft(monster.PoolKey("second-wind")))
	assert.Equal(t, 0, m.PoolLeft(coreResources.ResourceKey("anything")))

	m.SpendPool(monster.PoolKey("second-wind"), 1)
	assert.False(t, m.IsDirty(), "spending a pool nobody has changes nothing")
}

func TestAKitAnswersOnlyForItsOwnPools(t *testing.T) {
	m := creatureWithAKit(t)

	assert.Equal(t, 1, m.PoolLeft(monster.PoolKey("second-wind")))
	assert.Equal(t, 0, m.PoolLeft(monster.PoolKey("rage")), "a pool the kit does not declare")
	assert.Equal(t, 0, m.PoolLeft("second-wind"), "the key is compared whole")
	assert.Equal(t, 0, m.PoolLeft("item:g-1/second-wind"))

	m.SpendPool(monster.PoolKey("rage"), 1)
	assert.False(t, m.IsDirty())
	m.SpendPool(monster.PoolKey("second-wind"), 5)
	assert.True(t, m.IsDirty())
	assert.Equal(t, 0, m.PoolLeft(monster.PoolKey("second-wind")), "floored at zero")
}
