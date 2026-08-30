// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// A monster sheet must be able to be told it changed by something that is not
// the code changing it. A condition keeping its own turn-scoped memory — the
// opportunity attack's once-per-turn flag is the first — stores that memory in
// its own fields, which are serialized as part of this sheet. Nothing else
// notices, so a sheet that cannot be marked dirty from outside is a sheet whose
// condition state is silently discarded on save.
func TestAMonsterCanBeToldItsPersistedStateChanged(t *testing.T) {
	m, err := monster.Load(context.Background(), &monster.Data{
		ID: "wolf-1", Name: "Wolf", HitPoints: 11, MaxHitPoints: 11, ArmorClass: 13,
	})
	require.NoError(t, err)
	require.False(t, m.IsDirty(), "a freshly loaded sheet has nothing to save")

	m.MarkDirty()
	require.True(t, m.IsDirty(), "MarkDirty is what stops a silent update being dropped")
}
