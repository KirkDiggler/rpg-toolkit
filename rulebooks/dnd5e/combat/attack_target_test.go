// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

func TestCanBeAttackTargetPreservesDerivedLifeStateTruthTable(t *testing.T) {
	tests := []struct {
		name string
		kind combat.CombatantKind
		down bool
		want bool
	}{
		{name: "standing character", kind: combat.CombatantKindCharacter, want: true},
		{name: "dying character", kind: combat.CombatantKindCharacter, down: true, want: true},
		{name: "standing monster", kind: combat.CombatantKindMonster, want: true},
		{name: "defeated monster", kind: combat.CombatantKindMonster, down: true, want: false},
		{name: "unknown kind", kind: combat.CombatantKind("unknown"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, combat.CanBeAttackTarget(tt.kind, tt.down))
		})
	}
}
