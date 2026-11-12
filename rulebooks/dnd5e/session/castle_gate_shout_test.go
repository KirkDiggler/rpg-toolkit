// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// TestCastleGateOneAnswersAShoutNotTalk is the operator's ruling on
// rpg-project#930 at the session door (rpg-project#933): the cleric beside
// the stranger outside the gate asks him about the bustle, an ordinary
// spoken line gate-1 hears from the gatehouse, and he stays on his post;
// she hails the stranger with the castle's shouted line, and on the next
// round he walks out toward it.
func TestCastleGateOneAnswersAShoutNotTalk(t *testing.T) {
	c := openTheCastleForTwo(t, 18, 18, 18, 18)
	ctx := context.Background()
	require.NotEmpty(t, c.walkMemberTo(t, "cleric", "pedlar").Steps, "the cleric walks to the stranger")
	say := func(option, why string) {
		t.Helper()
		_, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "cleric", Target: "pedlar", OptionID: option})
		require.NoError(t, err, why)
	}
	posted := c.where(t, "gate-1")
	stranger := c.where(t, "pedlar")

	say("tonight", "a spoken line, which pays the world a round")
	require.Equal(t, posted, c.where(t, "gate-1"), "talk outside leaves him on his post")
	require.Equal(t, "posted", stateOrPosted(stateOf(t, c.world(t), "gate-1")))

	say("hail-the-pedlar", "the shout")
	say("tonight", "and a round passes")
	enc := c.world(t)
	require.Less(t, enc.Distance(c.where(t, "gate-1"), stranger), enc.Distance(posted, stranger),
		"the shout drew him off his post toward it")
}
