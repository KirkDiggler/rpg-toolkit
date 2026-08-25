// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources

import (
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/stretchr/testify/require"
)

func TestDisplayNameIsClosedToTheStatusCatalog(t *testing.T) {
	tests := []struct {
		key  coreResources.ResourceKey
		name string
	}{
		{RageCharges, "Rage"},
		{Ki, "Ki"},
		{HitDice, "Hit Dice"},
		{SecondWind, "Second Wind"},
		{ActionSurge, "Action Surge"},
	}
	for _, tc := range tests {
		t.Run(string(tc.key), func(t *testing.T) {
			name, ok := DisplayName(tc.key)
			require.True(t, ok)
			require.Equal(t, tc.name, name)
		})
	}

	name, ok := DisplayName(coreResources.ResourceKey("spell_slots"))
	require.False(t, ok)
	require.Empty(t, name, "unknown keys must not become valid-looking display names")
}
