package refs_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func TestMonsterActionRefsAreUniqueContentIdentities(t *testing.T) {
	tests := []struct {
		name string
		ref  *core.Ref
		id   string
	}{
		{"bandit scimitar", refs.MonsterActions.BanditScimitar(), "bandit-scimitar"},
		{"bandit light crossbow", refs.MonsterActions.BanditLightCrossbow(), "bandit-light-crossbow"},
		{"brown bear bite", refs.MonsterActions.BrownBearBite(), "brown-bear-bite"},
		{"brown bear claw", refs.MonsterActions.BrownBearClaw(), "brown-bear-claw"},
		{"ghoul bite", refs.MonsterActions.GhoulBite(), "ghoul-bite"},
		{"ghoul claw", refs.MonsterActions.GhoulClaw(), "ghoul-claw"},
		{"giant rat bite", refs.MonsterActions.GiantRatBite(), "giant-rat-bite"},
		{"goblin scimitar", refs.MonsterActions.GoblinScimitar(), "goblin-scimitar"},
		{"skeleton captain longsword", refs.MonsterActions.SkeletonCaptainLongsword(), "skeleton-captain-longsword"},
		{"skeleton shortsword", refs.MonsterActions.SkeletonShortsword(), "skeleton-shortsword"},
		{"skeleton shortbow", refs.MonsterActions.SkeletonShortbow(), "skeleton-shortbow"},
		{"thug mace", refs.MonsterActions.ThugMace(), "thug-mace"},
		{"wolf bite", refs.MonsterActions.WolfBite(), "wolf-bite"},
		{"zombie slam", refs.MonsterActions.ZombieSlam(), "zombie-slam"},
	}

	seen := make(map[string]string, len(tests))
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.ref)
			assert.Equal(t, refs.Module, tc.ref.Module)
			assert.Equal(t, refs.TypeMonsterActions, tc.ref.Type)
			assert.Equal(t, tc.id, tc.ref.ID)
			assert.Equal(t, "dnd5e:monster_actions:"+tc.id, tc.ref.String())

			previous, duplicate := seen[tc.ref.String()]
			assert.False(t, duplicate, "%s duplicates %s", tc.name, previous)
			seen[tc.ref.String()] = tc.name
		})
	}
	assert.Len(t, seen, 14)
}
