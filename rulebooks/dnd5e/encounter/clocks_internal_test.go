// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// clocks_internal_test.go pins #2220's sentinels at the two seams this change
// gave them: bubbleFor's errNotInBubble and disengageBeforeWalk's
// errNoDisengage. Neither signal crosses the package's API, so an external
// test cannot see them.

// TestBubbleForAnswersNotInBubble: a member no bubble holds is the news
// itself — not (nil, nil), which could not be told from a wiring failure
// (#2220: exactly one of a valid object or an error). The world clock is the
// usual reading; clock_of re-checks because a member on NO clock reads the
// same here.
func TestBubbleForAnswersNotInBubble(t *testing.T) {
	var e Encounter // no bubbles: nobody is held

	bubble, err := e.bubbleFor("member")
	require.Nil(t, bubble)
	require.ErrorIs(t, err, errNotInBubble)
}

// TestDisengageBeforeWalkAnswersNoDisengage: a walk that needs no
// bonus-action Disengage — no disengager wired, no bonus action, no path, or
// a sheet with none to spend — is the news itself, not (nil, nil) (#2220).
func TestDisengageBeforeWalkAnswersNoDisengage(t *testing.T) {
	var e Encounter // no disengager wired

	deltas, err := e.disengageBeforeWalk("member", &memberRecord{}, nil, &TurnBudget{})
	require.Nil(t, deltas)
	require.ErrorIs(t, err, errNoDisengage)
}