// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_arrest_test.go is the arrest at the session door
// (rpg-project#925): both of the party's claims at the front gate fail and
// the guards arrest. Each player is shown an [session.ArrestAct] naming the
// members taken as they perceived them; who the guards want is never in any
// player's view, at any stop of the dial. The taken are held: their own
// attempts to walk away are refused with the session's custody sentence.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

// arrestAtTheGate is the warned gate (alice's troupe claim missed,
// rpg-project#942's walk) at the sharing stop given, the cleric — seen on
// the road — gone off out of every guard's sight, bob joined on the road
// where no guard sees him, and alice's guest claim failed: the gate arrests.
// It returns the run and what the claim sent each player.
func arrestAtTheGate(t *testing.T, sharing encounter.Sharing) (*freeRoamCastle, beat) {
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
	// Gone off down the road, out of every guard's sight; they remember her
	// from where she watched.
	c.standInTheRecord(t, "cleric", downTheRoad)

	played := c.play(t, "alice's guest claim fails", func() error {
		_, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "guest"})
		return err
	})
	arrest, ok := c.world(t).Arrests()["front-gate"]
	require.True(t, ok, "precondition: the gate arrested")
	require.Equal(t, []encounter.MemberID{"alice"}, takenOf(arrest), "precondition: alice is taken")
	require.Equal(t, []encounter.MemberID{"cleric"}, wantedOf(arrest), "precondition: the cleric is wanted")
	return c, played
}

// arrestActs is every ArrestAct member is sent, live and on catch-up, and
// the member's whole act stream as JSON, for the leak check.
func arrestActs(t *testing.T, c *freeRoamCastle, played beat, member string) (live, caught []found[session.ArrestAct], stream string) {
	t.Helper()
	live = where[session.ArrestAct](played.to(member), func(session.ArrestAct) bool { return true })
	read, err := c.mgr.Acts(context.Background(), &session.ActsInput{Session: "sess", Member: member})
	require.NoError(t, err)
	caught = where[session.ArrestAct](read.Acts, func(session.ArrestAct) bool { return true })
	for _, a := range append(append([]session.Act(nil), played.to(member)...), read.Acts...) {
		require.Empty(t, a.Undecoded, "%s: no undecoded act", member)
	}
	raw, err := json.Marshal(append(append([]session.Act(nil), played.to(member)...), read.Acts...))
	require.NoError(t, err)
	return live, caught, string(raw)
}

// requireNoWanted fails when a player's stream carries the guards' wanted
// knowledge in any form: a field so named, or the wanted cleric named as
// taken.
func requireNoWanted(t *testing.T, member string, acts []found[session.ArrestAct], stream string) {
	t.Helper()
	require.NotContains(t, strings.ToLower(stream), "wanted", "%s: the guards' wanted knowledge is theirs alone", member)
	for _, a := range acts {
		require.NotContains(t, a.body.Taken, session.Party{Kind: session.PartyMember, ID: "cleric"},
			"%s: the wanted cleric is never shown taken", member)
	}
}

// TestCastleTheArrestAtEachTheirOwn: alice, taken, is shown the arrest
// naming herself; the cleric, wanted, and bob, who saw none of it, are shown
// nothing of it. Nobody's stream carries who is wanted.
func TestCastleTheArrestAtEachTheirOwn(t *testing.T) {
	c, played := arrestAtTheGate(t, encounter.SharingOwn)
	live, caught, stream := arrestActs(t, c, played, "alice")
	require.Len(t, live, 1, "alice is shown her arrest")
	require.Equal(t, session.ArrestAct{Interaction: "front-gate", Taken: []session.Party{aliceParty}, Cell: "region-cells"}, live[0].body)
	require.Equal(t, live[0].body, only[session.ArrestAct](t, caught2acts(caught)).body, "and reads it back on catch-up")
	require.Nil(t, live[0].act.Perceived, "her own")
	requireNoWanted(t, "alice", live, stream)
	for _, member := range []string{"cleric", "bob"} {
		live, caught, stream := arrestActs(t, c, played, member)
		require.Empty(t, live, "%s is shown no arrest at each their own", member)
		require.Empty(t, caught, "%s, on catch-up", member)
		requireNoWanted(t, member, live, stream)
	}
}

// TestCastleTheArrestAtSharedView: the cleric and bob are shown alice's
// arrest through her, as she was shown it; still nobody's stream carries
// who is wanted.
func TestCastleTheArrestAtSharedView(t *testing.T) {
	c, played := arrestAtTheGate(t, encounter.SharingView)
	for _, member := range []string{"cleric", "bob"} {
		live, caught, stream := arrestActs(t, c, played, member)
		require.Len(t, live, 1, "%s is shown alice's arrest", member)
		require.Equal(t, []session.Party{aliceParty}, live[0].body.Taken, member)
		require.NotNil(t, live[0].act.Perceived, member)
		require.Equal(t, []session.Party{aliceParty}, live[0].act.Perceived.Via, "%s: through alice", member)
		require.Len(t, caught, 1, "%s, on catch-up", member)
		requireNoWanted(t, member, live, stream)
	}
}

// caught2acts is a found list's acts.
func caught2acts(in []found[session.ArrestAct]) []session.Act {
	out := make([]session.Act, 0, len(in))
	for _, f := range in {
		out = append(out, f.act)
	}
	return out
}

// TestCastleTheTakenCannotWalkAway: alice, held since the arrest, asks to
// walk and to start sneaking. Each is refused with the session's own
// custody sentence; the composition's error does not cross the seam.
func TestCastleTheTakenCannotWalkAway(t *testing.T) {
	c, _ := arrestAtTheGate(t, encounter.SharingOwn)
	ctx := context.Background()
	_, err := c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{backDownTheRoad[0]}})
	requireHeldYou(t, err, "a walk")
	_, err = c.mgr.SetSneaking(ctx, &session.SetSneakingInput{Session: "sess", Member: "alice", Sneaking: true})
	requireHeldYou(t, err, "a sneak")
}

// requireHeldYou is the walking-away refusal: the session's ErrInCustody,
// whose text is the player's sentence, and no encounter error in the chain.
func requireHeldYou(t *testing.T, err error, what string) {
	t.Helper()
	require.ErrorIs(t, err, session.ErrInCustody, what)
	require.False(t, errors.Is(err, encounter.ErrInCustody), "%s: the composition's error stays behind the seam", what)
	require.Equal(t, "The guards hold you; you cannot walk away.", session.ErrInCustody.Error())
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
