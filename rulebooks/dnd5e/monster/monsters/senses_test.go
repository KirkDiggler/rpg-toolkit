// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEveryShippedMonsterHears walks every registered constructor: a hand-built
// block that authors no Senses has a passive Perception of 0 and fails every
// hearing DC (rpg-project#569 R5), so a new constructor that forgets its senses
// fails here rather than shipping deaf. The stored value also has to survive
// the stat block's round trip, because the session reads it off the stored
// block, not off the constructor.
func TestEveryShippedMonsterHears(t *testing.T) {
	for _, ref := range monsters.Refs() {
		t.Run(ref, func(t *testing.T) {
			construct, ok := monsters.ByRef(ref)
			require.True(t, ok)
			m := construct("senses-" + ref)

			assert.Positive(t, m.PassivePerception(), "a shipped monster must author its passive Perception")

			loaded, err := monster.Load(context.Background(), m.ToData())
			require.NoError(t, err)
			assert.Equal(t, m.PassivePerception(), loaded.PassivePerception(),
				"passive Perception must survive the stored stat block")
		})
	}
}

// TestShippedMonsterPassivePerception pins each block's SRD 5.1 passive
// Perception and checks it against the block's own Wisdom: 10 + WIS modifier,
// plus the proficiency bonus where the SRD lists a Perception skill. A
// hand-typed number that disagrees with the authored WIS fails here.
func TestShippedMonsterPassivePerception(t *testing.T) {
	cases := []struct {
		ref        string
		passive    int
		perception bool // SRD lists a Perception skill
	}{
		{refs.Monsters.Skeleton().String(), 9, false},
		{refs.Monsters.Zombie().String(), 8, false},
		{refs.Monsters.SkeletonCaptain().String(), 9, false}, // not an SRD block; derived
		{refs.Monsters.Ghoul().String(), 10, false},
		{refs.Monsters.GiantRat().String(), 10, false},
		{refs.Monsters.Wolf().String(), 13, true},
		{refs.Monsters.BrownBear().String(), 13, true},
		{refs.Monsters.Bandit().String(), 10, false},
		{refs.Monsters.BanditArcher().String(), 10, false},
		{refs.Monsters.Thug().String(), 10, false},
		{refs.Monsters.Goblin().String(), 9, false},
		{refs.Monsters.GoblinBoss().String(), 9, false},
		{refs.Monsters.AnimatedArmor().String(), 6, false},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			construct, ok := monsters.ByRef(tc.ref)
			require.True(t, ok)
			m := construct("senses-" + tc.ref)

			assert.Equal(t, tc.passive, m.PassivePerception(), "SRD passive Perception")

			derived := 10 + m.AbilityScores().Modifier(abilities.WIS)
			if tc.perception {
				derived += m.ProficiencyBonus()
			}
			assert.Equal(t, derived, m.PassivePerception(), "passive Perception agrees with the block's WIS")
		})
	}
}
