// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func TestHuman_IsAssembledByFromTemplate(t *testing.T) {
	m, err := monster.FromTemplate(&monster.FromTemplateInput{
		ID: "h", Ref: refs.Monsters.Human(), Base: monsters.Human,
	})
	require.NoError(t, err)

	assert.Equal(t, 4, m.MaxHP())
	assert.Equal(t, 10, m.AC())
	assert.Equal(t, "Human", m.Name())
	assert.Equal(t, refs.Monsters.Human(), m.Ref())
	assert.Equal(t, "humanoid", m.CreatureType())

	require.Len(t, m.Actions(), 1)
	unarmed := m.Actions()[0]
	assert.Equal(t, "dnd5e:weapons:unarmed-strike", unarmed.Ref.String())
	require.NotNil(t, unarmed.Attack)
	assert.Equal(t, 2, unarmed.Attack.AttackBonus)
}

func TestHuman_IsABaseNotAConstructor(t *testing.T) {
	base, ok := monsters.BaseByRef("dnd5e:monsters:human")
	require.True(t, ok)
	assert.Nil(t, base.Base, "a base names no base of its own")

	_, ok = monsters.ByRef("dnd5e:monsters:human")
	assert.False(t, ok, "a placement cannot spawn a bare base without a template")

	_, ok = monsters.BaseByRef("dnd5e:monsters:goblin")
	assert.False(t, ok, "a rulebook monster is a constructor, not a base")
}
