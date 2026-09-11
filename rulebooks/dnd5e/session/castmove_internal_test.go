// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// castmove_internal_test.go pins the crossing itself, which the verb cannot
// show from outside.
//
// A cast that routes proves the word it uses is one the composition walks. What
// it cannot prove is the REFUSAL — that a policy nobody wrote an executor for
// is stopped here rather than handed across as a string the board will
// misread — because a profile carrying such a policy does not exist to cast.

func TestRoutePolicyCrossesTheTwoWordsBothVocabulariesHave(t *testing.T) {
	line, ok := routePolicy(combatActions.MoveLine)
	require.True(t, ok, "a shove along a line is the policy this seam shipped with")
	require.Equal(t, encounter.MoveLine, line)

	away, ok := routePolicy(combatActions.MoveAway)
	require.True(t, ok, "a flee is the second, and Dissonant Whispers is what brought it")
	require.Equal(t, encounter.MoveAway, away)
}

func TestRoutePolicyRefusesAWordTheCompositionHasNoExecutorFor(t *testing.T) {
	// "toward" is content's reserved third policy, waiting on Thorn Whip. It
	// is the honest example: a word one vocabulary may learn first.
	crossed, ok := routePolicy(combatActions.MovePolicy("toward"))
	require.False(t, ok, "a policy with no executor is refused, not passed through")
	require.Empty(t, crossed, "and nothing is invented to carry across")
}
