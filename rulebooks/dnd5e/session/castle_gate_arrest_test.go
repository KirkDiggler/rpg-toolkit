// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_arrest_test.go is arrest slice one at the session door
// (rpg-project#925): both of the party's claims at the front gate fail and
// the guards arrest. Until arrest has its own session slice, its `arrested`
// beat is the record's: no player — taken, wanted, or a friend who was
// neither — is sent any act for it, at each their own or at shared view,
// live or on catch-up.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// onTheRoadUnseen is where bob joins: the road below the gate, out of
// every gate guard's sight, so the arrest leaves him neither taken nor
// wanted.
var onTheRoadUnseen = spatial.Position{X: -2, Y: 16}

// TestCastleTheArrestSendsNoPlayerAnActAtOwn: the warned gate (alice's troupe
// claim missed, rpg-project#942's walk), the cleric — seen on the road —
// gone off out of every guard's sight, bob joins on the road where no guard
// sees him, and alice's guest claim fails: the gate arrests. Alice is taken,
// the cleric wanted, bob neither. Alice's story holds the `arrested` beat;
// no player is sent an act for it.
//
// THE WITNESS IS BOB. In the castle no player sees the gate's line without a
// guard seeing them back, and a player a guard sees is taken; so the friend
// who watched is the one at shared view, shown alice's arrest through her.
func TestCastleTheArrestSendsNoPlayerAnActAtOwn(t *testing.T) {
	theArrestSendsNoPlayerAnAct(t, encounter.SharingOwn)
}

// TestCastleTheArrestSendsNoPlayerAnActAtView is the same at shared view,
// where each friend's stream merges the others' stories.
func TestCastleTheArrestSendsNoPlayerAnActAtView(t *testing.T) {
	theArrestSendsNoPlayerAnAct(t, encounter.SharingView)
}

// theArrestSendsNoPlayerAnAct plays the arrest at a sharing stop.
func theArrestSendsNoPlayerAnAct(t *testing.T, sharing encounter.Sharing) {
	t.Helper()
	warnedGate.once.Do(func() { warnedGate.stores, warnedGate.err = buildTheWarnedGate() })
	require.NoError(t, warnedGate.err)
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{1}},
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(), seats: newFakeSeats(),
	}
	require.NoError(t, restoreInto(c, warnedGate.stores))
	c.encounters.byID["sess"].Sharing = sharing
	c.characters.byID["bob"] = withFreeReactions(armedFighter("bob"))
	c.mgr = c.manager(t)
	ctx := context.Background()
	_, err := c.mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "bob", Position: onTheRoadUnseen})
	require.NoError(t, err)
	// Gone off down the road, out of every guard's sight; they
	// remember her from where she watched.
	c.standInTheRecord(t, "cleric", downTheRoad)

	played := c.play(t, "alice's guest claim fails", func() error {
		_, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "guest"})
		return err
	})
	arrest, ok := c.world(t).Arrests()["front-gate"]
	require.True(t, ok, "precondition: the gate arrested")
	require.Equal(t, []encounter.MemberID{"alice"}, takenOf(arrest), "precondition: alice is taken")
	require.Equal(t, []encounter.MemberID{"cleric"}, wantedOf(arrest), "precondition: the cleric is wanted")
	require.NotEmpty(t, beatsNamed(storyBeats(t, c.mgr, "sess", "alice"), encounter.BeatArrested),
		"precondition: alice's story holds the arrest")

	for _, member := range []string{"alice", "cleric", "bob"} {
		for _, a := range played.to(member) {
			require.NotContains(t, a.Undecoded, encounter.BeatArrested, "%s, live", member)
		}
		caught, err := c.mgr.Acts(ctx, &session.ActsInput{Session: "sess", Member: member})
		require.NoError(t, err)
		for _, a := range caught.Acts {
			require.NotContains(t, a.Undecoded, encounter.BeatArrested, "%s, on catch-up", member)
		}
	}
}

// takenOf is an arrest's taken members.
func takenOf(r encounter.ArrestRecord) []encounter.MemberID {
	var out []encounter.MemberID
	for _, m := range r.Taken {
		out = append(out, m.ID)
	}
	return out
}

// wantedOf is an arrest's wanted members.
func wantedOf(r encounter.ArrestRecord) []encounter.MemberID {
	var out []encounter.MemberID
	for _, m := range r.Wanted {
		out = append(out, m.ID)
	}
	return out
}
