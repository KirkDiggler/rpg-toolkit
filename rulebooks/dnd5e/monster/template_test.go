// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// testRef is the derived creature's own ref for tests that do not care which.
var testRef = &core.Ref{Module: "dnd5e", Type: "monsters", ID: "test-template"}

func armorRef(id armor.ArmorID) *armor.ArmorID { return &id }

// actionByRef finds one assembled action by its weapon ref string, failing the
// test when the monster does not carry it.
func actionByRef(t *testing.T, m *monster.Monster, ref string) combatActions.Definition {
	t.Helper()
	for _, a := range m.Actions() {
		if a.Ref.String() == ref {
			return a
		}
	}
	require.Failf(t, "action not carried", "%q is not among the monster's actions", ref)
	return combatActions.Definition{}
}

// assertFlatDamage checks a weapon line's flat damage ("1d8+2") where the
// assembly puts it: on the ability contribution resolution adds to the pool,
// never as a FlatBonus typed onto the pool (see srd_weapon_lines_test.go).
func assertFlatDamage(t *testing.T, action combatActions.Definition, want int) {
	t.Helper()
	require.NotNil(t, action.Attack.Ability)
	assert.Equal(t, want, action.Attack.Ability.Modifier, "flat damage is the wielder's ability modifier")
	assert.Equal(t, 0, action.Attack.Damage[0].FlatBonus, "nothing is typed onto the pool")
	assert.True(t, action.Attack.Damage[0].HasProperty(damage.AddsAttackAbilityModifier))
}

func TestFromTemplate_GuardDerivesTheSRDNumbers(t *testing.T) {
	guard := monster.Template{
		Base:      refs.Monsters.Human(),
		Abilities: map[abilities.Ability]int{abilities.CON: 12},
		HitDice:   "2d8",
		Armor:     armorRef(armor.ChainShirt),
		Skills:    []skills.Skill{skills.Perception},
		Actions:   []weapons.WeaponID{weapons.Spear},
	}

	m, err := monster.FromTemplate("guard-1", testRef, guard, monsters.Human)
	require.NoError(t, err)

	assert.Equal(t, 11, m.MaxHP(), "2d8 averages 9, plus CON +1 per die")
	assert.Equal(t, 11, m.HP(), "current HP starts at max")
	assert.Equal(t, 13, m.AC(), "chain shirt 13 + min(DEX +0, 2)")

	require.Len(t, m.Actions(), 1, "a stated action list replaces the base's wholesale")
	spear := actionByRef(t, m, "dnd5e:weapons:spear")
	require.NotNil(t, spear.Attack)
	assert.Equal(t, 2, spear.Attack.AttackBonus)
	require.Len(t, spear.Attack.Damage, 1)
	assert.Equal(t, "1d6", spear.Attack.Damage[0].Dice)
	assertFlatDamage(t, spear, 0)

	assert.Equal(t, 12, m.PassivePerception(), "10 + WIS +0 + proficiency 2")
	assert.Equal(t, []monster.ProficiencyData{{Skill: "perception", Bonus: 2}}, m.ToData().Proficiencies)
}

func TestFromTemplate_CaptainWithProficiency3(t *testing.T) {
	captain := monster.Template{
		Base: refs.Monsters.Human(),
		Abilities: map[abilities.Ability]int{
			abilities.STR: 15, abilities.DEX: 14, abilities.CON: 14, abilities.CHA: 14,
		},
		HitDice:     "10d8",
		Armor:       armorRef(armor.Breastplate),
		Proficiency: 3,
		Actions:     []weapons.WeaponID{weapons.Longsword, weapons.Javelin},
	}

	m, err := monster.FromTemplate("captain-1", testRef, captain, monsters.Human)
	require.NoError(t, err)

	assert.Equal(t, 65, m.MaxHP(), "10d8 averages 45, plus CON +2 per die")
	assert.Equal(t, 16, m.AC(), "breastplate 14 + min(DEX +2, 2)")
	assert.Equal(t, 3, m.ProficiencyBonus())

	require.Len(t, m.Actions(), 2)
	longsword := actionByRef(t, m, "dnd5e:weapons:longsword")
	require.NotNil(t, longsword.Attack)
	assert.Equal(t, 5, longsword.Attack.AttackBonus, "STR +2 and proficiency 3")
	require.Len(t, longsword.Attack.Damage, 1)
	assert.Equal(t, "1d8", longsword.Attack.Damage[0].Dice)
	assertFlatDamage(t, longsword, 2)
	actionByRef(t, m, "dnd5e:weapons:javelin")
}

func TestFromTemplate_CookInheritsEverythingButTheKnife(t *testing.T) {
	cook := monster.Template{
		Base:    refs.Monsters.Human(),
		Actions: []weapons.WeaponID{weapons.Dagger},
	}

	m, err := monster.FromTemplate("cook-1", testRef, cook, monsters.Human)
	require.NoError(t, err)

	assert.Equal(t, 4, m.MaxHP(), "the base's 1d8 averages 4, CON +0")
	assert.Equal(t, 10, m.AC(), "the base wears nothing: 10 + DEX +0")
	assert.Equal(t, 0, m.Experience(), "unstated experience is worth nothing")
	assert.Equal(t, "Human", m.Name(), "unstated name is the base's")

	require.Len(t, m.Actions(), 1)
	dagger := actionByRef(t, m, "dnd5e:weapons:dagger")
	require.NotNil(t, dagger.Attack)
	assert.Equal(t, 2, dagger.Attack.AttackBonus)
	require.Len(t, dagger.Attack.Damage, 1)
	assert.Equal(t, "1d4", dagger.Attack.Damage[0].Dice)
}

func TestFromTemplate_ConOverrideMovesHP(t *testing.T) {
	at := func(con int) int {
		t.Helper()
		m, err := monster.FromTemplate("x", testRef, monster.Template{
			Base:      refs.Monsters.Human(),
			Abilities: map[abilities.Ability]int{abilities.CON: con},
			HitDice:   "2d8",
		}, monsters.Human)
		require.NoError(t, err)
		return m.MaxHP()
	}

	assert.Equal(t, 9, at(10))
	assert.Equal(t, 13, at(14), "HP follows the score; no stored total survives a CON change")
}

func TestFromTemplate_RefusesByName(t *testing.T) {
	cases := []struct {
		name     string
		template monster.Template
		base     monster.Template
		names    string
	}{
		{
			name:     "bad hit dice notation",
			template: monster.Template{Base: refs.Monsters.Human(), HitDice: "2d"},
			base:     monsters.Human,
			names:    "hit dice",
		},
		{
			name:     "unknown armor",
			template: monster.Template{Base: refs.Monsters.Human(), Armor: armorRef("plate-of-nothing")},
			base:     monsters.Human,
			names:    "armor",
		},
		{
			name: "unknown weapon",
			template: monster.Template{
				Base:    refs.Monsters.Human(),
				Actions: []weapons.WeaponID{"dnd5e:weapons:lightsaber"},
			},
			base:  monsters.Human,
			names: "lightsaber",
		},
		{
			name: "ability score above 30",
			template: monster.Template{
				Base:      refs.Monsters.Human(),
				Abilities: map[abilities.Ability]int{abilities.STR: 31},
			},
			base:  monsters.Human,
			names: "str",
		},
		{
			name:     "missing base",
			template: monster.Template{HitDice: "2d8"},
			base:     monster.Template{},
			names:    "base",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := monster.FromTemplate("x", testRef, tc.template, tc.base)
			require.Error(t, err)
			assert.Nil(t, m, "a refusal carries no half-built monster")
			assert.Contains(t, err.Error(), tc.names)
		})
	}
}

func TestTemplate_MergeIsPerField(t *testing.T) {
	merged := monster.Template{
		Abilities:  map[abilities.Ability]int{abilities.CON: 12},
		Experience: 0,
	}.Merge(monsters.Human)

	assert.Equal(t, 12, merged.Abilities[abilities.CON], "a stated key wins")
	assert.Equal(t, 10, merged.Abilities[abilities.STR], "an unstated key is the base's")
	assert.Equal(t, "1d8", merged.HitDice)
	assert.Equal(t, "Human", merged.Name)
	assert.Equal(t, refs.Monsters.Human(), merged.Base)
	assert.Equal(t, []weapons.WeaponID{weapons.UnarmedStrike}, merged.Actions)

	worth := monster.Template{Experience: 0}.Merge(monster.Template{Experience: 50})
	assert.Equal(t, 0, worth.Experience, "experience never inherits")
}

func TestTemplate_MergeDoesNotAliasTheBase(t *testing.T) {
	merged := monster.Template{}.Merge(monsters.Human)
	merged.Abilities[abilities.STR] = 18
	merged.Actions[0] = weapons.Dagger

	assert.Equal(t, 10, monsters.Human.Abilities[abilities.STR], "the rulebook base is not mutated through a merge")
	assert.Equal(t, weapons.UnarmedStrike, monsters.Human.Actions[0])
}

func TestFromTemplate_SheetCarriesTheTemplateRef(t *testing.T) {
	guardRef := &core.Ref{Module: "dnd5e", Type: "monsters", ID: "guard"}
	m, err := monster.FromTemplate("guard-1", guardRef, monster.Template{
		Base:      refs.Monsters.Human(),
		Abilities: map[abilities.Ability]int{abilities.CON: 12},
		HitDice:   "2d8",
		Actions:   []weapons.WeaponID{weapons.Spear},
	}, monsters.Human)
	require.NoError(t, err)

	data := m.ToData()
	require.NotNil(t, data.Ref)
	assert.Equal(t, "dnd5e:monsters:guard", data.Ref.String(), "the sheet names the template, not the base")
	assert.Equal(t, "humanoid", m.CreatureType(), "the creature type is the base's")
	assert.Equal(t, "humanoid", data.CreatureType, "and it survives the sheet")
}

func TestFromTemplate_RefusesANilRef(t *testing.T) {
	m, err := monster.FromTemplate("guard-1", nil, monster.Template{}, monsters.Human)
	require.Error(t, err)
	assert.Nil(t, m)
	assert.Contains(t, err.Error(), "ref")
}
