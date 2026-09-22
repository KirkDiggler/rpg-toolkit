// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// TestGoblinBossMatchesTheSRDStatBlock copies its numbers from the SRD rather
// than from the factory: if the stat block is wrong, this test says so in the
// monster's own vocabulary.
func TestGoblinBossMatchesTheSRDStatBlock(t *testing.T) {
	boss := monsters.NewGoblinBoss("goblin-boss-1")

	require.Equal(t, "goblin-boss-1", boss.GetID())
	require.Equal(t, "Goblin Boss", boss.Name())
	require.Equal(t, refs.Monsters.GoblinBoss(), boss.Ref())
	require.Equal(t, "humanoid", boss.CreatureType())
	require.Equal(t, 21, boss.HP())
	require.Equal(t, 21, boss.MaxHP())
	require.Equal(t, 17, boss.AC())
	require.Equal(t, 30, boss.Speed().Walk)

	scores := boss.AbilityScores()
	require.Equal(t, 10, scores[abilities.STR])
	require.Equal(t, 14, scores[abilities.DEX])
	require.Equal(t, 10, scores[abilities.CON])
	require.Equal(t, 10, scores[abilities.INT])
	require.Equal(t, 8, scores[abilities.WIS])
	require.Equal(t, 10, scores[abilities.CHA])
}

// TestGoblinBossMultiattackIsTheFirstActionAndScriptsItsScimitar is the shape
// of the whole slice in one test: the stat block's own line comes first, and
// what it declares is a script over actions the boss separately carries.
func TestGoblinBossMultiattackIsTheFirstActionAndScriptsItsScimitar(t *testing.T) {
	actions := monsters.NewGoblinBoss("goblin-boss-1").Actions()
	require.GreaterOrEqual(t, len(actions), 3)

	multiattack := actions[0]
	require.Equal(t, *refs.MonsterActions.GoblinBossMultiattack(), multiattack.Ref,
		"a driver reads this list in order, so the boss's own line has to be first")
	require.Equal(t, "Multiattack", multiattack.Name)
	require.Nil(t, multiattack.Attack, "a sequence is not itself an attack")
	require.Nil(t, multiattack.Cost, "a monster has no ledger to charge")
	require.NotNil(t, multiattack.Sequence)

	require.Equal(t, []combatActions.SequenceStep{
		{Action: *refs.Weapons.Scimitar()},
		{Action: *refs.Weapons.Scimitar(), Disadvantage: "second attack of a multiattack"},
	}, multiattack.Sequence.Steps,
		"two scimitar attacks, and only the second one is penalised")

	require.Equal(t, *refs.Weapons.Scimitar(), actions[1].Ref)
	require.Equal(t, *refs.Weapons.Javelin(), actions[2].Ref)
}

// TestGoblinBossMultiattackResolvesAgainstItsOwnActions runs the check that
// Definition.Validate structurally cannot: both steps name something this
// boss actually carries, and neither names another sequence.
func TestGoblinBossMultiattackResolvesAgainstItsOwnActions(t *testing.T) {
	actions := monsters.NewGoblinBoss("goblin-boss-1").Actions()

	components, err := combatActions.ResolveSequence(actions[0], actions)
	require.NoError(t, err)
	require.Len(t, components, 2)

	for index, component := range components {
		require.NotNil(t, component.Definition.Attack,
			"step %d resolves to a swing the strike machine can run", index)
		require.Equal(t, 4, component.Definition.Attack.AttackBonus,
			"both steps are the same +4 scimitar; disadvantage is not a to-hit penalty")
	}
	require.Empty(t, components[0].Step.Disadvantage)
	require.Equal(t, "second attack of a multiattack", components[1].Step.Disadvantage)
}

// TestEveryAuthoredSequenceResolvesAgainstItsOwnMonster is the roster-wide
// liveness check. A sequence naming a component its monster does not carry
// would build, validate, ship, and refuse the first time somebody swung it;
// this is the test that would rather fail here.
func TestEveryAuthoredSequenceResolvesAgainstItsOwnMonster(t *testing.T) {
	sequencesFound := 0

	for _, ref := range monsters.Refs() {
		ref := ref
		t.Run(ref, func(t *testing.T) {
			build, ok := monsters.ByRef(ref)
			require.True(t, ok)

			actions := build("sequence-check-1").Actions()
			for _, action := range actions {
				if action.Sequence == nil {
					continue
				}
				sequencesFound++

				components, err := combatActions.ResolveSequence(action, actions)
				require.NoError(t, err, "%s's %q names something it does not carry", ref, action.Name)
				require.Len(t, components, len(action.Sequence.Steps))
			}
		})
	}

	require.Positive(t, sequencesFound,
		"a roster with no sequences would make every assertion above vacuous")
}
