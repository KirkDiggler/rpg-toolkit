// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

func TestGrantedWordsSaysWhatAPriceGives(t *testing.T) {
	gives := func(slots map[coreCombat.ActionType]int) *combat.SpendProfile {
		return &combat.SpendProfile{GrantSlots: slots}
	}

	require.Equal(t, "1 action", combat.GrantedWords(gives(map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1})))
	require.Equal(t, "2 actions", combat.GrantedWords(gives(map[coreCombat.ActionType]int{coreCombat.ActionStandard: 2})))
	require.Equal(t, "1 bonus action", combat.GrantedWords(gives(map[coreCombat.ActionType]int{coreCombat.ActionBonus: 1})))
	require.Equal(t, "1 reaction", combat.GrantedWords(gives(map[coreCombat.ActionType]int{coreCombat.ActionReaction: 1})))
	require.Equal(t, "1 action and 1 bonus action", combat.GrantedWords(gives(map[coreCombat.ActionType]int{
		coreCombat.ActionBonus: 1, coreCombat.ActionStandard: 1,
	})))
	require.Equal(t, "1 action and 1 bonus action and 2 reactions", combat.GrantedWords(gives(map[coreCombat.ActionType]int{
		coreCombat.ActionReaction: 2, coreCombat.ActionBonus: 1, coreCombat.ActionStandard: 1,
	})))

	require.Empty(t, combat.GrantedWords(nil))
	require.Empty(t, combat.GrantedWords(&combat.SpendProfile{}))
	require.Empty(t, combat.GrantedWords(&combat.SpendProfile{
		Grants: map[combat.CapacityType]int{combat.CapacityAttack: 2},
	}))
}
