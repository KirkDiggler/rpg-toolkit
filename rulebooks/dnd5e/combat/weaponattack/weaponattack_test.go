// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package weaponattack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/weaponattack"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// wielder is a stand-in for whoever holds the weapon. It exists so this
// package's tests can state a wielder's numbers outright instead of building
// a character sheet or a stat block to imply them.
type wielder struct {
	modifiers  map[abilities.Ability]int
	proficient bool
	bonus      int
}

func (w wielder) GetAbilityModifier(ability abilities.Ability) int { return w.modifiers[ability] }
func (w wielder) ProficiencyBonus() int                            { return w.bonus }
func (w wielder) IsProficientWith(*weapons.Weapon) bool            { return w.proficient }

func weapon(t *testing.T, id weapons.WeaponID) *weapons.Weapon {
	t.Helper()
	found, err := weapons.GetByID(id)
	require.NoError(t, err)
	return &found
}

func TestTheAttackBonusIsTheWieldersOwnNumbers(t *testing.T) {
	trained := wielder{modifiers: map[abilities.Ability]int{abilities.DEX: 2}, proficient: true, bonus: 2}

	definition, err := weaponattack.Assemble(&weaponattack.Input{
		Wielder: trained,
		Weapon:  weapon(t, weapons.Shortbow),
	})
	require.NoError(t, err)
	require.Equal(t, 4, definition.Attack.AttackBonus, "DEX +2 and a +2 proficiency bonus")
	require.Equal(t, abilities.DEX, definition.Attack.Ability.Ability)
	require.Equal(t, 2, definition.Attack.Ability.Modifier)
}

// TestProficiencyIsAskedOfTheWielder is the reserved seat from
// rpg-project#448 decision 7 under test: the assembly does not decide whether
// the proficiency bonus applies, it ASKS. A monster answers yes today; the
// placement's `proficient: false` switch, when it is built, changes only this
// answer.
func TestProficiencyIsAskedOfTheWielder(t *testing.T) {
	untrained := wielder{modifiers: map[abilities.Ability]int{abilities.DEX: 2}, proficient: false, bonus: 2}

	definition, err := weaponattack.Assemble(&weaponattack.Input{
		Wielder: untrained,
		Weapon:  weapon(t, weapons.Shortbow),
	})
	require.NoError(t, err)
	require.Equal(t, 2, definition.Attack.AttackBonus,
		"an untrained wielder attacks at its ability modifier alone")
	require.Equal(t, 2, definition.Attack.Ability.Modifier,
		"damage keeps its ability modifier, which never came from proficiency")
}

func TestUnarmedIsProficientWhateverTheWielderSays(t *testing.T) {
	untrained := wielder{modifiers: map[abilities.Ability]int{abilities.STR: 3}, proficient: false, bonus: 2}
	unarmed := weapons.SpecialWeapons[weapons.UnarmedStrike]

	definition, err := weaponattack.Assemble(&weaponattack.Input{
		Wielder:          untrained,
		Weapon:           &unarmed,
		AlwaysProficient: true,
	})
	require.NoError(t, err)
	require.Equal(t, 5, definition.Attack.AttackBonus)
}

func TestFinesseTakesTheBetterOfStrengthAndDexterity(t *testing.T) {
	cases := []struct {
		name string
		str  int
		dex  int
		want abilities.Ability
	}{
		{"dexterous rogue", 1, 4, abilities.DEX},
		{"strong duellist", 4, 1, abilities.STR},
		{"a tie goes to strength", 3, 3, abilities.STR},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			held := wielder{
				modifiers:  map[abilities.Ability]int{abilities.STR: tc.str, abilities.DEX: tc.dex},
				proficient: true,
				bonus:      2,
			}
			definition, err := weaponattack.Assemble(&weaponattack.Input{
				Wielder: held,
				Weapon:  weapon(t, weapons.Scimitar),
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, definition.Attack.Ability.Ability)
		})
	}
}

func TestDeliveryComesOffTheWeapon(t *testing.T) {
	held := wielder{modifiers: map[abilities.Ability]int{abilities.STR: 0, abilities.DEX: 0}, bonus: 2}

	scimitar, err := weaponattack.Assemble(&weaponattack.Input{Wielder: held, Weapon: weapon(t, weapons.Scimitar)})
	require.NoError(t, err)
	require.Equal(t, &combatActions.MeleeDelivery{ReachFeet: 5}, scimitar.Attack.Delivery.Melee)
	require.Nil(t, scimitar.Attack.Delivery.Ranged)

	glaive, err := weaponattack.Assemble(&weaponattack.Input{Wielder: held, Weapon: weapon(t, weapons.Glaive)})
	require.NoError(t, err)
	require.Equal(t, &combatActions.MeleeDelivery{ReachFeet: 10}, glaive.Attack.Delivery.Melee,
		"a reach weapon adds five feet")

	crossbow, err := weaponattack.Assemble(&weaponattack.Input{Wielder: held, Weapon: weapon(t, weapons.HeavyCrossbow)})
	require.NoError(t, err)
	require.Equal(t, &combatActions.RangedDelivery{NormalFeet: 100, LongFeet: 400}, crossbow.Attack.Delivery.Ranged)
	require.Nil(t, crossbow.Attack.Delivery.Melee)
}

func TestTheVersatileGripChangesTheDice(t *testing.T) {
	held := wielder{modifiers: map[abilities.Ability]int{abilities.STR: 3}, proficient: true, bonus: 2}

	oneHanded, err := weaponattack.Assemble(&weaponattack.Input{Wielder: held, Weapon: weapon(t, weapons.Longsword)})
	require.NoError(t, err)
	require.Equal(t, "1d8", oneHanded.Attack.Damage[0].Dice)
	require.False(t, oneHanded.Attack.Weapon.TwoHanded)

	twoHanded, err := weaponattack.Assemble(&weaponattack.Input{
		Wielder:   held,
		Weapon:    weapon(t, weapons.Longsword),
		TwoHanded: true,
	})
	require.NoError(t, err)
	require.Equal(t, "1d10", twoHanded.Attack.Damage[0].Dice)
	require.True(t, twoHanded.Attack.Weapon.TwoHanded)
}

func TestTheFlatDamageIsTheAbilityContributionNotATypedNumber(t *testing.T) {
	held := wielder{modifiers: map[abilities.Ability]int{abilities.DEX: 2}, proficient: true, bonus: 2}

	definition, err := weaponattack.Assemble(&weaponattack.Input{Wielder: held, Weapon: weapon(t, weapons.Shortbow)})
	require.NoError(t, err)
	require.Len(t, definition.Attack.Damage, 1)
	require.Equal(t, 0, definition.Attack.Damage[0].FlatBonus)
	require.True(t, definition.Attack.Damage[0].HasProperty(damage.AddsAttackAbilityModifier),
		"resolution adds the wielder's modifier to this pool")
}

func TestTheDefinitionCannotReachBackIntoTheCatalog(t *testing.T) {
	held := wielder{modifiers: map[abilities.Ability]int{abilities.DEX: 2}, proficient: true, bonus: 2}

	definition, err := weaponattack.Assemble(&weaponattack.Input{Wielder: held, Weapon: weapon(t, weapons.Shortbow)})
	require.NoError(t, err)
	definition.Attack.Damage[0].Dice = "9d9"
	definition.Attack.Weapon.Ref.ID = "trebuchet"

	again, err := weaponattack.Assemble(&weaponattack.Input{Wielder: held, Weapon: weapon(t, weapons.Shortbow)})
	require.NoError(t, err)
	require.Equal(t, "1d6", again.Attack.Damage[0].Dice)
	require.Equal(t, "shortbow", again.Attack.Weapon.Ref.ID)
	require.Equal(t, "shortbow", again.Ref.ID)
}

func TestAssembleRefusesWhatItCannotCompile(t *testing.T) {
	held := wielder{modifiers: map[abilities.Ability]int{abilities.DEX: 2}, bonus: 2}

	_, err := weaponattack.Assemble(nil)
	require.Error(t, err)

	_, err = weaponattack.Assemble(&weaponattack.Input{Weapon: weapon(t, weapons.Shortbow)})
	require.ErrorContains(t, err, "no wielder")

	_, err = weaponattack.Assemble(&weaponattack.Input{Wielder: held})
	require.ErrorContains(t, err, "no weapon")

	_, err = weaponattack.Assemble(&weaponattack.Input{
		Wielder: held,
		Weapon:  &weapons.Weapon{ID: "trebuchet", Name: "Trebuchet", Category: weapons.CategorySimpleMelee},
	})
	require.ErrorContains(t, err, "no ref for weapon")
}
