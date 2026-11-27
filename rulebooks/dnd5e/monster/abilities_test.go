// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// storedSheet is the stored sheet of the human base with Second Wind at level
// 1 and one use, edited by change before it is read. A nil change is the
// sheet as the creature saved it.
func storedSheet(t *testing.T, change func(*monster.Data)) *monster.Data {
	t.Helper()

	sheet := creatureWithAKit(t).ToData()
	if change != nil {
		change(sheet)
	}

	return sheet
}

// sheetRow is one stored sheet of the creature, named for what it has spent.
type sheetRow struct {
	name    string
	change  func(*monster.Data)
	payable bool
}

func sheetRows() []sheetRow {
	turn := func(action, bonus, reaction int) func(*monster.Data) {
		return func(d *monster.Data) {
			d.Economy = &combat.ActionEconomy{
				ActionsRemaining:      action,
				BonusActionsRemaining: bonus,
				ReactionsRemaining:    reaction,
			}
		}
	}

	return []sheetRow{
		{name: "fresh", change: nil, payable: true},
		{name: "bonus action spent", change: turn(1, 0, 1), payable: false},
		{name: "use spent", change: func(d *monster.Data) { d.Left["second-wind"] = 0 }, payable: false},
		{name: "action spent", change: turn(0, 1, 1), payable: true},
		{name: "reaction spent", change: turn(1, 1, 0), payable: true},
	}
}

func loaded(t *testing.T, sheet *monster.Data) *monster.Monster {
	t.Helper()

	m, err := monster.LoadFromData(context.Background(), sheet, events.NewEventBus())
	require.NoError(t, err)

	return m
}

func abilitiesOf(t *testing.T, sheet *monster.Data) []monster.Ability {
	t.Helper()

	out, err := monster.AbilitiesOf(&monster.AbilitiesOfInput{Sheet: sheet})
	require.NoError(t, err)

	return out.Abilities
}

// Test 1.
func TestAStoredSheetSaysWhatItsCreatureCanUse(t *testing.T) {
	abilities := abilitiesOf(t, storedSheet(t, nil))

	require.Len(t, abilities, 1)
	ability := abilities[0]
	assert.Equal(t, "second wind", ability.Word, "the author's word")
	assert.Equal(t, "second-wind", ability.Grant, "the name the kit files it under")
	assert.Equal(t, "Second Wind", ability.Name, "the definition's own name")
	assert.True(t, ability.Payable)

	require.NotNil(t, ability.Definition)
	assert.Equal(t, ability.Definition.Name, ability.Name)
	require.NotNil(t, ability.Definition.Cost)
	assert.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1}, ability.Definition.Cost.Slots)
	assert.Len(t, ability.Definition.Cost.Pools, 1)
	assert.Equal(t, 1, ability.Definition.Cost.Pools[monster.PoolKey("second-wind")])
}

// Test 2. Two definitions are compared whole with assert.Equal: it is
// reflect.DeepEqual over the structs, and a Definition holds no function.
func TestTheDescriptionIsTheLoadedCreaturesOwn(t *testing.T) {
	sheet := storedSheet(t, nil)

	abilities := abilitiesOf(t, sheet)
	require.Len(t, abilities, 1)

	out, err := loaded(t, sheet).GrantedAbility(&monster.GrantedAbilityInput{Grant: "second-wind"})
	require.NoError(t, err)

	assert.Equal(t, out.Definition, abilities[0].Definition)
}

// Test 3.
func TestPayableFollowsTheSheet(t *testing.T) {
	for _, row := range sheetRows() {
		t.Run(row.name, func(t *testing.T) {
			abilities := abilitiesOf(t, storedSheet(t, row.change))

			require.Len(t, abilities, 1)
			assert.Equal(t, row.payable, abilities[0].Payable)
		})
	}
}

// Test 4.
func TestPayableIsTheGatesOwnAnswer(t *testing.T) {
	for _, row := range sheetRows() {
		t.Run(row.name, func(t *testing.T) {
			sheet := storedSheet(t, row.change)
			abilities := abilitiesOf(t, sheet)
			require.Len(t, abilities, 1)

			creature := loaded(t, sheet)

			assert.Equal(t, combat.CanPay(creature, abilities[0].Definition.Cost), abilities[0].Payable)
		})
	}
}

// Test 5.
func TestReadingASheetDoesNotChangeIt(t *testing.T) {
	for _, row := range []string{"fresh", "bonus action spent"} {
		t.Run(row, func(t *testing.T) {
			var change func(*monster.Data)
			for _, candidate := range sheetRows() {
				if candidate.name == row {
					change = candidate.change
				}
			}
			sheet := storedSheet(t, change)
			before := jsonOf(t, sheet)

			abilitiesOf(t, sheet)

			assert.Equal(t, before, jsonOf(t, sheet))
			if row == "fresh" {
				assert.Nil(t, sheet.Economy, "the read does not create the account")
			}
		})
	}
}

// Test 6.
func TestASheetWithNoKitHasNoAbilities(t *testing.T) {
	plain := humanOver("")
	human, err := monster.FromTemplate(&plain)
	require.NoError(t, err)

	for name, sheet := range map[string]*monster.Data{
		"the plain human base": human.ToData(),
		"a wolf":               monsters.NewWolf("w-1").ToData(),
	} {
		t.Run(name, func(t *testing.T) {
			out, err := monster.AbilitiesOf(&monster.AbilitiesOfInput{Sheet: sheet})

			require.NoError(t, err)
			require.NotNil(t, out)
			assert.Empty(t, out.Abilities)
		})
	}
}

// Test 7. The rules have one named ability today ("second wind"), so no kit
// can hold two grants the rules can describe; the order is asserted over the
// one, and the sort is the sorted keys of the kit's grants, the same as
// [monster.Monster.Grants]. When a second word exists, add a kit of two grants
// here and assert their order.
func TestAbilitiesAreSortedByGrant(t *testing.T) {
	abilities := abilitiesOf(t, storedSheet(t, nil))

	grants := make([]string, 0, len(abilities))
	for _, ability := range abilities {
		grants = append(grants, ability.Grant)
	}

	assert.Equal(t, creatureWithAKit(t).Grants(), grants)
}

// Test 8.
func TestNilIsRefused(t *testing.T) {
	_, err := monster.AbilitiesOf(nil)
	require.Error(t, err)
	assert.Equal(t, rpgerr.CodeNil, rpgerr.GetCode(err))

	_, err = monster.AbilitiesOf(&monster.AbilitiesOfInput{})
	require.Error(t, err)
	assert.Equal(t, rpgerr.CodeNil, rpgerr.GetCode(err))
}

// A grant whose ability the rules have no word for cannot be compiled today,
// so a stored sheet that holds one is refused by name rather than given a word
// made up from its name.
func TestAGrantWithNoWordIsRefusedByName(t *testing.T) {
	sheet := storedSheet(t, func(d *monster.Data) {
		d.Kit.Grants["rage"] = kits.Grant{Ability: *refs.Features.Rage(), Parameters: []byte(`{}`)}
	})

	_, err := monster.AbilitiesOf(&monster.AbilitiesOfInput{Sheet: sheet})

	require.Error(t, err)
	assert.Contains(t, err.Error(), `stat block "Human" grant "rage"`)
}
