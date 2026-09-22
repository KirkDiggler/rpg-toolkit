// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// TestAMultiattackIsProjectedFirstAndCarriesItsOwnReach is what makes the
// whole wave reachable: a definition with no attack arm used to be skipped
// here, so a sequence was invisible to the composition and no driver could
// ever pick one.
func TestAMultiattackIsProjectedFirstAndCarriesItsOwnReach(t *testing.T) {
	views := memberActionsFromMonster(monsters.NewGoblinBoss("boss").ToData().Actions)
	require.Len(t, views, 3, "the script and both components a driver can also take on their own")

	require.Equal(t, refs.MonsterActions.GoblinBossMultiattack(), &views[0].Ref,
		"author order is what a driver reads, and the boss's own line is first")
	require.Equal(t, "Multiattack", views[0].Name)
	require.Equal(t, 5, views[0].RangeFeet, "both scimitar swings reach five feet")
	require.Equal(t, "melee", views[0].Kind)

	require.Equal(t, refs.Weapons.Scimitar(), &views[1].Ref)
	require.Equal(t, refs.Weapons.Javelin(), &views[2].Ref)
}

// TestASequenceIsProjectedAtItsShortestStepsReach is the rule a driver's
// reach check depends on. RangeFeet answers "can I use this from here", and a
// script whose far step could reach while its near step could not is a script
// that would abort the striker's whole verb halfway through.
func TestASequenceIsProjectedAtItsShortestStepsReach(t *testing.T) {
	bow := combatActions.Definition{
		Ref: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "view-bow"}, Name: "Bow",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Ranged: &combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Piercing, FlatBonus: 2}},
		},
	}
	claw := combatActions.Definition{
		Ref: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "view-claw"}, Name: "Claw",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "1d4", Type: damage.Slashing, FlatBonus: 2}},
		},
	}
	script := combatActions.Definition{
		Ref: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "view-mixed"}, Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{{Action: bow.Ref}, {Action: claw.Ref}},
		},
	}

	views := memberActionsFromMonster([]combatActions.Definition{script, bow, claw})
	require.Len(t, views, 3)
	require.Equal(t, 5, views[0].RangeFeet,
		"the binding distance is the step that reaches least far, not the one that reaches furthest")
	require.Equal(t, "ranged", views[0].Kind,
		"a script that is not melee all the way through must not claim to be melee")
}

// TestASequenceNamingSomethingTheMonsterDoesNotCarryIsNotProjected keeps the
// projection saying what resolution would say. Unreachable for content in
// this repository — the rulebook resolves every authored sequence in a
// registry-wide test — so this is the door refusing data from elsewhere.
func TestASequenceNamingSomethingTheMonsterDoesNotCarryIsNotProjected(t *testing.T) {
	script := combatActions.Definition{
		Ref: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "view-dangling"}, Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{
				{Action: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "view-absent"}},
				{Action: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "view-absent"}},
			},
		},
	}

	require.Empty(t, memberActionsFromMonster([]combatActions.Definition{script}))
}
