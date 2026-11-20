// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ONE ENTRY PER (WORD, PLACE) (rpg-project#990): a word may be written in
// more than one place, each with its own label, but never twice in one —
// [WhoWordAt] would answer only the first.
func TestEachWhoWordIsListedOncePerPlace(t *testing.T) {
	seen := map[[2]string]bool{}
	for _, w := range WhoWords() {
		key := [2]string{w.Word, w.Allowed}
		require.False(t, seen[key], "%s at %s is listed twice", w.Word, w.Allowed)
		seen[key] = true
	}
}
