// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The two stat blocks the Intimidate design names by number
// (ideas/shenanigans/intimidate.md): a goblin is DC 9, a thug is DC 10 when
// the placement authors no `intimidate:` list of its own.
func TestPassiveInsightOfTheNamedStatBlocks(t *testing.T) {
	require.Equal(t, 9, NewGoblin("goblin-1").PassiveInsight(),
		"WIS 8 is -1, so a goblin is talked down on a 9")
	require.Equal(t, 10, NewThug("thug-1").PassiveInsight(),
		"WIS 10 is +0, so a thug is talked down on a 10")
}
