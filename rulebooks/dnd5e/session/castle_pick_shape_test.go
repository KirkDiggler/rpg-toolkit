// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// castle_pick_shape_test.go is rpg-project#948 at the session door: the
// captain's stand-aside has a success table and a failure table of different
// shapes, and a member who sees the captain but caught none of the line is
// sent his pick as who answered and nothing of the die.

// standAsideWatched plays alice's stand-aside, WHISPERED in this test's copy
// of the castle, beside the captain, with the cleric watching from a cell
// that sees him and is not beside either, and returns the captain's table
// answers each of them is sent.
func standAsideWatched(t *testing.T, d20 int) (alice, cleric []session.TableAnswer) {
	t.Helper()
	source := castleSource(t)
	line := []byte("          - id: stand-aside\n            say: Stand aside or bleed.\n")
	require.Equal(t, 1, bytes.Count(source, line), "precondition: the captain's stand-aside, once")
	whispered := bytes.Replace(source, line, append(append([]byte(nil), line...), []byte("            loudness: whisper\n")...), 1)
	c := openTheCastleFrom(t, eachTheirOwn(t, whispered), []*character.Data{armedFighter("alice"), healingCleric()}, d20, d20, d20, d20)
	require.NotEmpty(t, c.walkMemberTo(t, "alice", "captain").Steps, "alice walks up to the captain")
	c.placeInTheRecord(t, "cleric", besideTheHailCell)
	played := c.play(t, "alice tells the captain to stand aside", func() error {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
			Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside",
		})
		return err
	})
	picks := func(who string) []session.TableAnswer {
		var out []session.TableAnswer
		for _, f := range where[session.TableAnswer](played.to(who), func(a session.TableAnswer) bool { return a.Who == captainParty }) {
			if f.act.Act != "" && f.act.Verb == "speak" {
				out = append(out, f.body)
			}
		}
		return out
	}
	watched := where[session.SpeechAct](played.to("cleric"), func(s session.SpeechAct) bool { return s.Speaker == aliceParty })
	require.Len(t, watched, 1, "precondition: the cleric saw alice say it")
	require.Nil(t, watched[0].body.Words, "precondition: and caught none of it")
	return picks("alice"), picks("cleric")
}

// TestCastleAWatcherIsSentOnlyWhoAnswered: the cleric, watching, is sent the
// captain's pick as exactly who answered, landed or missed; alice, who said
// it, is sent its die and its candidates.
func TestCastleAWatcherIsSentOnlyWhoAnswered(t *testing.T) {
	var watched [2]session.TableAnswer
	for i, d20 := range []int{18, 2} {
		alice, cleric := standAsideWatched(t, d20)
		require.Len(t, cleric, 1, "the cleric sees the captain answer")
		require.Equal(t, session.TableAnswer{Who: captainParty}, cleric[0], "who answered, and nothing of the die")
		watched[i] = cleric[0]

		require.Len(t, alice, 1)
		require.NotNil(t, alice[0].Roll, "alice, who said it, is sent the die")
		require.NotEmpty(t, alice[0].Candidates, "and every candidate")
	}
	require.Equal(t, watched[0], watched[1], "the same pick, landed or missed")
}
