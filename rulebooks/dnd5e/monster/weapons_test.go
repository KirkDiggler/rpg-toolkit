// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// A monster is a weapon wielder, checked at compile time rather than by a
// test that could be deleted.
var _ weaponattack.Wielder = (*monster.Monster)(nil)

func goblin() *monster.Monster {
	return monster.New(monster.Config{
		ID:   "gob-1",
		Name: "Goblin",
		Ref:  refs.Monsters.Goblin(),
		HP:   7,
		AC:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 8,
			abilities.CHA: 8,
		},
	})
}

func TestAMonsterIsProficientWithEveryWeapon(t *testing.T) {
	m := goblin()

	require.True(t, m.IsProficientWith(nil),
		"a monster's answer does not depend on which weapon it is handed (decision 7)")

	require.NoError(t, m.AddWeapon(weapons.Greataxe))
	actions := m.Actions()
	require.Len(t, actions, 1)
	require.Equal(t, 1, actions[0].Attack.AttackBonus,
		"a goblin handed a greataxe swings at STR -1 plus its +2 proficiency bonus, not at -1")
}

func TestAddWeaponAppendsInTheOrderGiven(t *testing.T) {
	m := goblin()
	require.NoError(t, m.AddWeapon(weapons.Scimitar))
	require.NoError(t, m.AddWeapon(weapons.Shortbow))

	actions := m.Actions()
	require.Len(t, actions, 2)
	require.Equal(t, refs.Weapons.Scimitar().String(), actions[0].Ref.String())
	require.Equal(t, refs.Weapons.Shortbow().String(), actions[1].Ref.String())
}

func TestAddWeaponBuildsTheWieldersOwnLine(t *testing.T) {
	m := goblin()
	require.NoError(t, m.AddWeapon(weapons.Scimitar))

	scimitar := m.Actions()[0]
	require.Equal(t, refs.Weapons.Scimitar().String(), scimitar.Ref.String(),
		"the action's ref is the weapon's ref, as it is for a character")
	require.Equal(t, 4, scimitar.Attack.AttackBonus)
	require.Equal(t, abilities.DEX, scimitar.Attack.Ability.Ability, "DEX 14 beats STR 8 on a finesse blade")
	require.Equal(t, 2, scimitar.Attack.Ability.Modifier)
	require.Equal(t, "1d6", scimitar.Attack.Damage[0].Dice)
	require.Equal(t, damage.Slashing, scimitar.Attack.Damage[0].Type)
	require.Equal(t, &combatActions.MeleeDelivery{ReachFeet: 5}, scimitar.Attack.Delivery.Melee)
}

func TestAddWeaponRefusesAWeaponTheCatalogDoesNotHave(t *testing.T) {
	m := goblin()
	require.NoError(t, m.AddWeapon(weapons.Scimitar))

	err := m.AddWeapon("trebuchet")
	require.Error(t, err)
	require.ErrorContains(t, err, "trebuchet")
	require.Len(t, m.Actions(), 1, "the refusal leaves the monster holding what it held")
}

func TestSetWeaponsReplacesEverythingTheMonsterCarried(t *testing.T) {
	m := goblin()
	require.NoError(t, m.AddWeapon(weapons.Scimitar))
	require.NoError(t, m.AddWeapon(weapons.Shortbow))

	require.NoError(t, m.SetWeapons([]weapons.WeaponID{weapons.Shortbow}))

	actions := m.Actions()
	require.Len(t, actions, 1, "the author said bow and nothing else")
	require.Equal(t, refs.Weapons.Shortbow().String(), actions[0].Ref.String())
}

func TestSetWeaponsKeepsTheAuthorsOrder(t *testing.T) {
	m := goblin()
	require.NoError(t, m.SetWeapons([]weapons.WeaponID{weapons.Scimitar, weapons.Shortbow}))
	first := m.Actions()

	other := goblin()
	require.NoError(t, other.SetWeapons([]weapons.WeaponID{weapons.Shortbow, weapons.Scimitar}))
	second := other.Actions()

	require.Equal(t, refs.Weapons.Scimitar().String(), first[0].Ref.String())
	require.Equal(t, refs.Weapons.Shortbow().String(), second[0].Ref.String(),
		"the author's order is the driver's preference order, so it is not sorted or normalised")
}

func TestSetWeaponsIsAllOrNothing(t *testing.T) {
	m := goblin()
	require.NoError(t, m.AddWeapon(weapons.Scimitar))

	err := m.SetWeapons([]weapons.WeaponID{weapons.Shortbow, "trebuchet"})
	require.Error(t, err)

	actions := m.Actions()
	require.Len(t, actions, 1, "a list with one bad entry leaves the monster holding what it held")
	require.Equal(t, refs.Weapons.Scimitar().String(), actions[0].Ref.String())
}

func TestSetWeaponsRefusesAnEmptyList(t *testing.T) {
	m := goblin()
	require.NoError(t, m.AddWeapon(weapons.Scimitar))

	require.Error(t, m.SetWeapons(nil), "silently disarming a monster looks exactly like the default case")
	require.Len(t, m.Actions(), 1)
}
