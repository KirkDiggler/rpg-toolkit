// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

// The long sentence for one creature and for a routine several creatures run
// (rpg-project#1131, amendment 1): the second mention is the word "its".
func TestAHitPointsConditionSaysItsMaximum(t *testing.T) {
	b := &builder{names: map[string]string{}}
	w := behavior.When{HitPoints: &behavior.HitPointsWhen{AtMost: 30, Percent: true}}

	s, conns := b.whenWords(w, "X (x)")
	require.Equal(t, "X (x)'s hit points are at most 30% of its maximum", s)
	require.Empty(t, conns)

	require.Equal(t, "the creature running it's", whose("the creature running it"))
	s, _ = b.whenWords(w, "the creature running it")
	require.Equal(t, "the creature running it's hit points are at most 30% of its maximum", s)
}

// A number of hit points is said as a number, with no maximum in it, for one
// creature and for a routine several creatures run (rpg-project#1131, 4b-2).
func TestAHitPointsNumberIsSaidWithoutAMaximum(t *testing.T) {
	b := &builder{names: map[string]string{}}
	w := behavior.When{HitPoints: &behavior.HitPointsWhen{AtMost: 8}}

	s, conns := b.whenWords(w, "X (x)")
	require.Equal(t, "X (x)'s hit points are at most 8", s)
	require.Empty(t, conns)

	s, _ = b.whenWords(w, "the creature running it")
	require.Equal(t, "the creature running it's hit points are at most 8", s)
}
