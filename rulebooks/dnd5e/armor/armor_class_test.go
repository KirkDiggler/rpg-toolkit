// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package armor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
)

func TestArmorClass_MatchesCharacterMath(t *testing.T) {
	cases := []struct {
		name string
		id   armor.ArmorID
		want int
	}{
		{name: "light armor takes the whole DEX", id: armor.Leather, want: 14},
		{name: "medium armor caps DEX at 2", id: armor.ChainShirt, want: 15},
		{name: "heavy armor takes no DEX", id: armor.Plate, want: 18},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worn, err := armor.GetByID(tc.id)
			require.NoError(t, err)
			assert.Equal(t, tc.want, armor.ArmorClass(&worn, 3))
		})
	}

	t.Run("no armor is 10 plus the whole DEX", func(t *testing.T) {
		assert.Equal(t, 13, armor.ArmorClass(nil, 3))
	})

	t.Run("a negative DEX is never capped up", func(t *testing.T) {
		plate, err := armor.GetByID(armor.Plate)
		require.NoError(t, err)
		assert.Equal(t, 17, armor.ArmorClass(&plate, -1))
	})
}
