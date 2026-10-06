// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
)

// TestFreeReactionsMatchTheConditionsList pins the reactions a monster's
// attach carries to the one list conditions.HeldAddresses reads, so "what a
// member holds" has one answer whether it is read from the loaded sheet or
// from the stored one.
func TestFreeReactionsMatchTheConditionsList(t *testing.T) {
	want := make([]string, 0)
	for _, carried := range conditions.FreeReactions("x") {
		want = append(want, carried.Ref().String())
	}
	got := make([]string, 0, len(freeReactions))
	for _, reaction := range freeReactions {
		got = append(got, reaction.ref.String())
		require.Equal(t, reaction.ref.String(), reaction.build("x").Ref().String(), "a carried reaction builds what it names")
	}
	require.Equal(t, want, got)
}
