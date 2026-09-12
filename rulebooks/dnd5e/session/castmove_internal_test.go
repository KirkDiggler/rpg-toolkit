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

func TestRoutePolicyCrossesEveryWordBothVocabulariesHave(t *testing.T) {
	line, ok := routePolicy(combatActions.MoveLine)
	require.True(t, ok, "a shove along a line is the policy this seam shipped with")
	require.Equal(t, encounter.MoveLine, line)

	away, ok := routePolicy(combatActions.MoveAway)
	require.True(t, ok, "a flee is the second, and Dissonant Whispers is what brought it")
	require.Equal(t, encounter.MoveAway, away)

	toward, ok := routePolicy(combatActions.MoveToward)
	require.True(t, ok, "and closing on somebody is the third, which Command brought")
	require.Equal(t, encounter.MoveToward, toward)
}

func TestRoutePolicyRefusesAWordTheCompositionHasNoExecutorFor(t *testing.T) {
	// The example used to be "toward", which was content's reserved word until
	// Command shipped its executor. That is exactly the gap this function
	// exists to hold open, so the test keeps a word neither vocabulary has —
	// a policy one side may learn first is a refusal here until the other
	// side does too.
	crossed, ok := routePolicy(combatActions.MovePolicy("sideways"))
	require.False(t, ok, "a policy with no executor is refused, not passed through")
	require.Empty(t, crossed, "and nothing is invented to carry across")
}
