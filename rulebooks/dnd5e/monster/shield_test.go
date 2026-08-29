// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// A monster is a combat.Combatant, stated here rather than left to be inferred
// from the call sites that happen to pass one. The interface gained a question
// in this change; an implementor that stopped satisfying it would otherwise
// surface as a compile error in whichever unrelated package passed a monster
// next, which is a long way from the sheet that failed to answer.
var _ combat.Combatant = (*monster.Monster)(nil)

// TestAMonsterCarriesNoShield pins the constant as a RULE rather than a stub.
//
// A monster has no equipment slots. Whatever defence a shield gives one is
// already inside the stat block AC its author wrote, so false is that question
// ANSWERED and not deferred — and the features that ask (Unarmored Movement's
// speed bonus, Fighting Style (Protection)'s reaction) are character features
// that were never written for a monster in the first place.
//
// Pinned by value because the alternative is a constant nobody checks: flip
// the body to true and nothing else in this repository notices today.
func TestAMonsterCarriesNoShield(t *testing.T) {
	m, err := monster.Load(context.Background(), &monster.Data{
		ID: "wolf-1", Name: "Wolf", HitPoints: 11, MaxHitPoints: 11, ArmorClass: 13,
	})
	require.NoError(t, err)

	require.False(t, m.HasShieldEquipped(),
		"a monster's shield is baked into its stat block AC; there is nothing left here to report")
	require.Equal(t, 13, m.AC(),
		"and that stat block AC is the whole of its defence — the number the author wrote")
}
