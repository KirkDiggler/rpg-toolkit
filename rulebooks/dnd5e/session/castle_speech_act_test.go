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
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_speech_act_test.go is the one Speech act at the session door
// (rpg-project#934, R6): every perceiver of a spoken thing in the castle is
// sent one [session.SpeechAct] carrying exactly what they caught.

// Where the castle's rooms are, for these routes: the grounds outside the
// gate and the courtyard west of the gatehouse are one room; the gatehouse
// is one room in, through its open south doorway; the great hall lies
// behind closed doors, farther still.
var (
	inTheCourtyard      = spatial.Position{X: -12, Y: 7}
	inTheGatehouseFloor = spatial.Position{X: -7, Y: 7}
	inTheGreatHall      = spatial.Position{X: -6, Y: -16}
)

// besideTheStranger opens the castle with the cleric beside the stranger
// outside the gate and alice where given.
func besideTheStranger(t *testing.T, alice spatial.Position, raw ...[]byte) *freeRoamCastle {
	t.Helper()
	var c *freeRoamCastle
	if len(raw) > 0 {
		c = openTheCastleFrom(t, raw[0], []*character.Data{armedFighter("alice"), healingCleric()}, 18, 18, 18, 18)
	} else {
		c = openTheCastleForTwo(t, 18, 18, 18, 18)
	}
	stranger := c.where(t, "pedlar")
	c.placeInTheRecord(t, "cleric", spatial.Position{X: stranger.X + 1, Y: stranger.Y})
	c.placeInTheRecord(t, "alice", alice)
	return c
}

// clericSays plays the cleric's line to the stranger and returns the acts.
func (c *freeRoamCastle) clericSays(t *testing.T, option string) beat {
	t.Helper()
	return c.play(t, "the cleric says "+option, func() error {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
			Session: "sess", Member: "cleric", Target: "pedlar", OptionID: option,
		})
		return err
	})
}

func strangerParty() session.Party { return session.Party{Kind: session.PartyMember, ID: "pedlar"} }
func clericParty() session.Party   { return session.Party{Kind: session.PartyMember, ID: "cleric"} }

// TestCastleAShoutFromOutsideIsWordsFromSomeoneInTheGatehouse: the
// operator's test for rpg-project#934. The cleric shouts to the stranger
// outside the gate; alice, one room in, in the gatehouse, out of sight, is
// sent the shout's words from someone at a place one room off — no speaker,
// no one spoken to. The cleric is sent her own line whole.
func TestCastleAShoutFromOutsideIsWordsFromSomeoneInTheGatehouse(t *testing.T) {
	c := besideTheStranger(t, inTheGatehouseFloor)
	played := c.clericSays(t, "hail-the-pedlar")

	heard := where[session.SpeechAct](played.to("alice"), func(s session.SpeechAct) bool { return s.Loudness == "shout" })
	require.Len(t, heard, 1, "one Speech act for the shout")
	s := heard[0].body
	require.Equal(t, "Oi! Pedlar! What have you got to sell?", s.Words, "the words reached her")
	require.Equal(t, "hail-the-pedlar", s.Option, "and which line it was")
	require.Equal(t, session.Party{}, s.Speaker, "from someone she did not see")
	require.Equal(t, session.Party{}, s.SpokenTo, "said to nobody she saw")
	require.NotNil(t, s.Place, "from a place")
	require.Equal(t, 1, s.Hops, "one room off")
	require.Nil(t, s.Roll, "a plain line rolls nothing")
	require.Equal(t, &session.Perceived{Senses: []session.Sense{session.SenseHearing},
		Via: []session.Party{{Kind: session.PartyMember, ID: "alice"}}}, heard[0].act.Perceived)
	require.Equal(t, session.FamilySpeech, heard[0].act.Family)

	own := where[session.SpeechAct](played.to("cleric"), func(s session.SpeechAct) bool { return s.Speaker == clericParty() })
	require.Len(t, own, 1, "the speaker is sent her own line")
	require.Equal(t, strangerParty(), own[0].body.SpokenTo)
	require.Equal(t, "Oi! Pedlar! What have you got to sell?", own[0].body.Words)
	require.Nil(t, own[0].body.Place, "she saw herself say it: no place")
	require.Equal(t, []session.Sense{session.SenseSight, session.SenseHearing}, own[0].act.Perceived.Senses)
}

// TestCastleALineSeenAndHeardCarriesItsRoll: alice beside the cleric in the
// grounds sees and hears her checked line — the speaker, whom it was said
// to, the words and the roll.
func TestCastleALineSeenAndHeardCarriesItsRoll(t *testing.T) {
	c := besideTheStranger(t, spatial.Position{})
	stranger := c.where(t, "pedlar")
	c.placeInTheRecord(t, "alice", spatial.Position{X: stranger.X - 1, Y: stranger.Y})
	played := c.clericSays(t, "tonight")

	lines := where[session.SpeechAct](played.to("alice"), func(s session.SpeechAct) bool { return s.Option == "tonight" })
	require.Len(t, lines, 1)
	s := lines[0].body
	require.Equal(t, clericParty(), s.Speaker)
	require.Equal(t, strangerParty(), s.SpokenTo)
	require.Equal(t, "What’s all the bustle?", s.Words)
	require.Equal(t, "persuasion", s.Skill)
	require.NotNil(t, s.Roll, "the roll travels with the words, the cleric's")
	require.Nil(t, s.Place)
	require.Equal(t, []session.Sense{session.SenseSight, session.SenseHearing}, lines[0].act.Perceived.Senses)
}

// TestCastleALineHeardUnseenIsSomeonesRoll: alice in the courtyard, the same
// room as the stranger but out of sight of the cleric, is sent the words and
// someone's roll, from a place in her own room.
func TestCastleALineHeardUnseenIsSomeonesRoll(t *testing.T) {
	c := besideTheStranger(t, inTheCourtyard)
	played := c.clericSays(t, "tonight")

	lines := where[session.SpeechAct](played.to("alice"), func(s session.SpeechAct) bool { return s.Option == "tonight" })
	require.Len(t, lines, 1)
	s := lines[0].body
	require.Equal(t, session.Party{}, s.Speaker, "someone")
	require.Equal(t, session.Party{}, s.SpokenTo)
	require.Equal(t, "What’s all the bustle?", s.Words)
	require.NotNil(t, s.Roll, "someone rolled")
	require.NotNil(t, s.Place)
	require.Equal(t, 0, s.Hops, "in her own room")
	require.Equal(t, []session.Sense{session.SenseHearing}, lines[0].act.Perceived.Senses)
}

// TestCastleBeyondReachNothingIsSent: in the great hall, behind closed
// doors, alice is sent no Speech act of the shout at all.
func TestCastleBeyondReachNothingIsSent(t *testing.T) {
	c := besideTheStranger(t, inTheGreatHall)
	played := c.clericSays(t, "hail-the-pedlar")
	require.Empty(t, where[session.SpeechAct](played.to("alice"), func(session.SpeechAct) bool { return true }))
}

// TestCastleAWhisperWatchedIsNoWordsAndNoRoll: the stranger's line authored
// whispered for this route, alice watching from his other side sees the
// cleric whisper to him and is sent no words and no roll.
func TestCastleAWhisperWatchedIsNoWordsAndNoRoll(t *testing.T) {
	source := castleSource(t)
	line := []byte("          - id: tonight\n            say: What’s all the bustle?\n")
	require.Equal(t, 1, bytes.Count(source, line), "precondition: the stranger's line, once")
	whispered := bytes.Replace(source, line, append(append([]byte(nil), line...), []byte("            loudness: whisper\n")...), 1)
	c := besideTheStranger(t, spatial.Position{}, whispered)
	stranger := c.where(t, "pedlar")
	c.placeInTheRecord(t, "alice", spatial.Position{X: stranger.X - 1, Y: stranger.Y})
	played := c.clericSays(t, "tonight")

	lines := where[session.SpeechAct](played.to("alice"), func(s session.SpeechAct) bool { return s.Speaker == clericParty() })
	require.Len(t, lines, 1, "she watched it said")
	s := lines[0].body
	require.Equal(t, "whisper", s.Loudness)
	require.Empty(t, s.Words, "no words")
	require.Empty(t, s.Option, "nor which line")
	require.Nil(t, s.Roll, "nor its roll")
	require.Equal(t, []session.Sense{session.SenseSight}, lines[0].act.Perceived.Senses)
}
