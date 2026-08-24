// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func TestMonsterActionViewsCarryMaximumRange(t *testing.T) {
	views := memberActionsFromMonster(monsters.NewSkeleton("skeleton").ToData().Actions)
	require.Len(t, views, 2)

	require.Equal(t, refs.MonsterActions.SkeletonShortsword(), &views[0].Ref)
	require.Equal(t, "shortsword", views[0].Name)
	require.Equal(t, 5, views[0].RangeFeet)
	require.Equal(t, "melee", views[0].Kind)

	require.Equal(t, refs.MonsterActions.SkeletonShortbow(), &views[1].Ref)
	require.Equal(t, "shortbow", views[1].Name)
	require.Equal(t, 320, views[1].RangeFeet)
	require.Equal(t, "ranged", views[1].Kind)
}
