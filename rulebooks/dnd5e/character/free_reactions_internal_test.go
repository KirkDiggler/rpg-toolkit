// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
)

// TestFreeReactionsMatchTheConditionsList pins the reactions a character's
// attach carries to the one list conditions.HeldAddresses reads, so "what a
// member holds" has one answer whether it is read from the loaded sheet or
// from the stored one.
func TestFreeReactionsMatchTheConditionsList(t *testing.T) {
	want := make([]string, 0)
	for _, carried := range conditions.FreeReactions("x") {
		want = append(want, carried.Ref().String())
	}
	got := make([]string, 0, len(freeReactionRefs))
	for _, ref := range freeReactionRefs {
		got = append(got, ref.String())
	}
	require.Equal(t, want, got)
}
