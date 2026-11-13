// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_gate_hint_test.go is rpg-project#936 at the session door: at the
// admitted gate, the cleric — a stranger to the gate guards — asks after her
// friends, and gate-1 SHOUTS the hint that one of them can speak for her.
// Alice, admitted, is shown it as her own ears or her party's carry it.

// theShoutedHint is gate-1's answer to `friends-inside`.
const theShoutedHint = "The troupe? Aye, they’re inside. If one of them comes back and speaks for you, in you go."

var (
	// inTheYardOutOfSight is courtyard floor out of gate-1's sight, east of
	// the gate's line, one room in from the gatehouse through the gate's
	// open doorway. (Much of the yard in line with the open gate sees him.)
	inTheYardOutOfSight = spatial.Position{X: 4, Y: 0}
	// besideGateOne is gatehouse floor in gate-1's sight and reach.
	besideGateOne = spatial.Position{X: -4, Y: 8}
)

// theHintAsked restores the admitted gate at the sharing stop given, stands
// alice where given (the hall's door shut when she is in the great hall)
// and the cleric beside gate-1, has the cleric ask after her friends, and
// returns what alice was sent of the hint.
func theHintAsked(t *testing.T, sharing encounter.Sharing, alice spatial.Position) []found[session.SpeechAct] {
	t.Helper()
	c := atTheAdmittedGate(t)
	world := c.encounters.byID["sess"]
	world.Sharing = sharing
	if alice == inTheGreatHall {
		shut := false
		for i := range world.Doors {
			if world.Doors[i].ID == "the-mad-king/hall-wall-door" {
				world.Doors[i].State, shut = string(encounter.DoorClosed), true
			}
		}
		require.True(t, shut, "precondition: the hall's door")
	}
	c.placeInTheRecord(t, "cleric", besideGateOne)
	// Alice is stood a cell short and takes the last step herself, so she
	// looks from where she stands; the walk there is not what this is about.
	c.placeInTheRecord(t, "alice", spatial.Position{X: alice.X - 1, Y: alice.Y})
	c.move(t, []spatial.Position{alice})
	played := c.play(t, "the cleric asks after her friends", func() error {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
			Session: "sess", Member: "cleric", Target: "gate-1", OptionID: friendsLine,
		})
		return err
	})
	require.NotEmpty(t, where[session.SpeechAct](played.to("cleric"), func(s session.SpeechAct) bool {
		return s.Words != nil && *s.Words == theShoutedHint
	}), "precondition: the cleric, spoken to, is told the hint")
	return where[session.SpeechAct](played.to("alice"), func(s session.SpeechAct) bool {
		return s.Words != nil && *s.Words == theShoutedHint
	})
}

// TestCastleTheHintIsShoutedToTheYard: at each their own, alice in the yard
// out of the gate guards' sight is shown the hint's words, from someone she
// did not see, said to nobody she saw, one room off, shouted.
func TestCastleTheHintIsShoutedToTheYard(t *testing.T) {
	got := theHintAsked(t, encounter.SharingOwn, inTheYardOutOfSight)
	require.Len(t, got, 1)
	s := got[0].body
	require.Equal(t, session.Party{}, s.Speaker, "no speaker")
	require.Equal(t, session.Party{}, s.SpokenTo, "no one spoken to")
	require.NotNil(t, s.Hops)
	require.Equal(t, 1, *s.Hops, "one room off")
	require.Equal(t, "shout", s.Loudness)
	require.Equal(t, &session.Perceived{Senses: []session.Sense{session.SenseHearing}}, got[0].act.Perceived, "her own ears")
}

// TestCastleTheHintDoesNotReachTheShutHall: at each their own, alice in the
// great hall behind its closed door is shown nothing of it.
func TestCastleTheHintDoesNotReachTheShutHall(t *testing.T) {
	require.Empty(t, theHintAsked(t, encounter.SharingOwn, inTheGreatHall))
}

// TestCastleTheHintReachesTheShutHallThroughAFriend: at the default, shared
// view, the same alice behind the same closed door IS shown the hint —
// through the cleric, who was spoken to and saw who said it — whole.
func TestCastleTheHintReachesTheShutHallThroughAFriend(t *testing.T) {
	got := theHintAsked(t, encounter.SharingView, inTheGreatHall)
	require.Len(t, got, 1)
	s := got[0].body
	require.Equal(t, session.Party{Kind: session.PartyMember, ID: "gate-1"}, s.Speaker, "the cleric saw who said it")
	require.Equal(t, clericParty(), s.SpokenTo, "and to whom")
	require.Nil(t, s.Hops, "a speaker named, and no distance of a friend's")
	require.Equal(t, []session.Party{clericParty()}, got[0].act.Perceived.Via, "through the cleric")
}
