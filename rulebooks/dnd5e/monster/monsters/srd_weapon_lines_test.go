// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// TestAssembledWeaponLinesMatchTheSRD is the proof that deriving a monster's
// arms from the weapons catalog and its own scores reproduces the stat block
// (rpg-project#448).
//
// EVERY NUMBER HERE IS COPIED FROM THE SRD, not from the code. If the
// assembly is wrong, this test says so in the monster's own vocabulary —
// "goblin scimitar should be +4 for 1d6+2" — rather than in the shape of
// whatever the assembly happened to build.
//
// The flat +2 on "1d6+2" is asserted as the ABILITY CONTRIBUTION, not as a
// FlatBonus on the pool, because that is where it now lives: the assembled
// pool carries damage.AddsAttackAbilityModifier and resolution adds the
// wielder's modifier to it (resolution/strike.go). The hand-typed
// definitions this replaces wrote `FlatBonus: 2` instead. Both roll 1d6+2;
// only one of them can be told that a goblin's DEX changed.
func TestAssembledWeaponLinesMatchTheSRD(t *testing.T) {
	type line struct {
		monster     string
		build       func(string) *monster.Monster
		index       int
		weaponRef   func() string
		name        string
		attackBonus int
		dice        string
		damageType  damage.Type
		flatBonus   int
		melee       *combatActions.MeleeDelivery
		ranged      *combatActions.RangedDelivery
	}

	melee5 := &combatActions.MeleeDelivery{ReachFeet: 5}
	shortbowRange := &combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320}
	lightCrossbowRange := &combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320}
	heavyCrossbowRange := &combatActions.RangedDelivery{NormalFeet: 100, LongFeet: 400}

	ref := func(id string) func() string {
		return func() string { return refs.Weapons.ByID(id).String() }
	}

	lines := []line{
		{"skeleton", monsters.NewSkeleton, 0, ref("shortsword"), "Shortsword",
			4, "1d6", damage.Piercing, 2, melee5, nil},
		{"skeleton", monsters.NewSkeleton, 1, ref("shortbow"), "Shortbow",
			4, "1d6", damage.Piercing, 2, nil, shortbowRange},
		{"goblin", monsters.NewGoblin, 0, ref("scimitar"), "Scimitar",
			4, "1d6", damage.Slashing, 2, melee5, nil},
		{"goblin", monsters.NewGoblin, 1, ref("shortbow"), "Shortbow",
			4, "1d6", damage.Piercing, 2, nil, shortbowRange},
		// The thug's components sit at 1 and 2: its Multiattack is authored
		// first so a driver reaches for the stat block's own line.
		{"thug", monsters.NewThug, 1, ref("mace"), "Mace",
			4, "1d6", damage.Bludgeoning, 2, melee5, nil},
		{"thug", monsters.NewThug, 2, ref("heavy-crossbow"), "Heavy Crossbow",
			2, "1d10", damage.Piercing, 0, nil, heavyCrossbowRange},
		// The goblin boss, for the same reason, plus the javelin's melee
		// half — its thrown 30/120 band has nowhere to land on an
		// exactly-one delivery union and is not faked here.
		{"goblin boss", monsters.NewGoblinBoss, 1, ref("scimitar"), "Scimitar",
			4, "1d6", damage.Slashing, 2, melee5, nil},
		{"goblin boss", monsters.NewGoblinBoss, 2, ref("javelin"), "Javelin",
			2, "1d6", damage.Piercing, 0, melee5, nil},
		{"bandit", monsters.NewBanditMelee, 0, ref("scimitar"), "Scimitar",
			3, "1d6", damage.Slashing, 1, melee5, nil},
		{"bandit", monsters.NewBanditMelee, 1, ref("light-crossbow"), "Light Crossbow",
			3, "1d8", damage.Piercing, 1, nil, lightCrossbowRange},
		{"bandit archer", monsters.NewBanditRanged, 0, ref("light-crossbow"), "Light Crossbow",
			3, "1d8", damage.Piercing, 1, nil, lightCrossbowRange},
	}

	t.Logf("%-14s %-16s %-28s %-7s %-14s %s", "MONSTER", "WEAPON", "REF", "TO HIT", "DAMAGE", "DELIVERY")
	for _, l := range lines {
		l := l
		t.Run(fmt.Sprintf("%s %s", l.monster, l.name), func(t *testing.T) {
			actions := l.build(l.monster + "-1").Actions()
			require.Greater(t, len(actions), l.index,
				"%s should list %s at position %d", l.monster, l.name, l.index)

			action := actions[l.index]
			require.Equal(t, l.weaponRef(), action.Ref.String(),
				"the action's ref is the WEAPON's ref, as it is for a character")
			require.Equal(t, l.name, action.Name)

			require.NotNil(t, action.Attack)
			require.Equal(t, combatActions.AttackCategoryWeapon, action.Attack.Category)
			require.Equal(t, l.attackBonus, action.Attack.AttackBonus,
				"SRD to-hit for the %s %s", l.monster, l.name)

			require.NotNil(t, action.Attack.Ability, "a weapon attack carries its ability evidence")
			require.Equal(t, l.flatBonus, action.Attack.Ability.Modifier,
				"SRD flat damage for the %s %s comes from the wielder's ability modifier", l.monster, l.name)

			require.Len(t, action.Attack.Damage, 1)
			pool := action.Attack.Damage[0]
			require.Equal(t, l.dice, pool.Dice)
			require.Equal(t, l.damageType, pool.Type)
			require.Equal(t, 0, pool.FlatBonus,
				"the flat damage is the ability contribution, not a number typed onto the pool")
			require.True(t, pool.HasProperty(damage.AddsAttackAbilityModifier),
				"the pool resolution adds the wielder's modifier to")

			require.Equal(t, l.melee, action.Attack.Delivery.Melee)
			require.Equal(t, l.ranged, action.Attack.Delivery.Ranged)

			require.NotNil(t, action.Attack.Weapon, "the assembled attack names the weapon it came from")
			require.Equal(t, l.weaponRef(), action.Attack.Weapon.Ref.String())

			delivery := fmt.Sprintf("melee %dft", 0)
			if l.melee != nil {
				delivery = fmt.Sprintf("melee %dft", l.melee.ReachFeet)
			} else if l.ranged != nil {
				delivery = fmt.Sprintf("ranged %d/%d", l.ranged.NormalFeet, l.ranged.LongFeet)
			}
			flat := ""
			if l.flatBonus != 0 {
				flat = fmt.Sprintf("+%d", l.flatBonus)
			}
			t.Logf("%-14s %-16s %-28s %-7s %-14s %s",
				l.monster, l.name, action.Ref.String(),
				fmt.Sprintf("+%d", l.attackBonus), l.dice+flat+" "+string(l.damageType), delivery)
		})
	}
}

// TestNoMonsterCarriesAWeaponShapedActionRef pins the namespace cut: a weapon
// a monster swings is the catalog's weapon, so no monster_actions member
// stands for one any more (rpg-project#448).
//
// The skeleton captain's longsword is the one exception and is named here so
// the exception is a recorded fact rather than an oversight — it was outside
// this slice's scope and derives exactly the same way whenever it is taken.
//
// THE ATTACK ARM IS WHAT THE CUT WAS ABOUT. A multiattack carries an authored
// monster_actions ref and always should: "two scimitar attacks, the second at
// disadvantage" is the monster's own line and no catalog entry describes it.
// So the weapon-ref requirement is asked of the swings, and the sequences are
// checked for the opposite — an authored ref, never a weapon's.
func TestNoMonsterCarriesAWeaponShapedActionRef(t *testing.T) {
	rearmed := map[string]func(string) *monster.Monster{
		"skeleton":      monsters.NewSkeleton,
		"goblin":        monsters.NewGoblin,
		"goblin boss":   monsters.NewGoblinBoss,
		"thug":          monsters.NewThug,
		"bandit":        monsters.NewBanditMelee,
		"bandit archer": monsters.NewBanditRanged,
	}

	for name, build := range rearmed {
		t.Run(name, func(t *testing.T) {
			for _, action := range build(name + "-1").Actions() {
				if action.Sequence != nil {
					require.Equal(t, refs.TypeMonsterActions, action.Ref.Type,
						"%s's %q is authored content, not a catalog entry",
						name, action.Name)
					continue
				}
				require.Equal(t, refs.TypeWeapons, action.Ref.Type,
					"%s's %q should carry the weapon's ref, not an authored one",
					name, action.Name)
			}
		})
	}
}
