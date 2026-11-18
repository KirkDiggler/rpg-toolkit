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
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// onTheRoadUnseen is where bob joins: the road below the gate, out of
// every gate guard's sight, so the arrest leaves him neither taken nor
// wanted.
var onTheRoadUnseen = spatial.Position{X: -2, Y: 16}

// arrestPlay is the arrest played at one stop of the dial: what alice's
// failed claim published, each player's catch-up read after, the record
// it made, and alice's attempts to walk away after.
type arrestPlay struct {
	played            beat
	caught            map[string][]session.Act
	taken, wanted     []encounter.MemberID
	walkErr, sneakErr error
}

// arrestPlays is the arrest at each their own and at shared view, played
// ONCE in [TestMain] (the castle's verbs are heavy, and the CI budget is
// judged per test).
var arrestPlays struct {
	once  sync.Once
	err   error
	plays map[encounter.Sharing]arrestPlay
}

// arrestPlaysTests are the tests that read [arrestPlays].
var arrestPlaysTests = []string{"TestCastleTheArrestAtEachTheirOwn", "TestCastleTheArrestAtSharedView", "TestCastleTheTakenCannotWalkAway"}

// buildTheArrestPlays plays the arrest at both stops. It takes no
// *testing.T because [TestMain] runs it.
func buildTheArrestPlays() {
	warnedGate.once.Do(func() { warnedGate.stores, warnedGate.err = buildTheWarnedGate() })
	if warnedGate.err != nil {
		arrestPlays.err = warnedGate.err
		return
	}
	arrestPlays.plays = map[encounter.Sharing]arrestPlay{}
	view, own, err := playTheArrest()
	if err != nil {
		arrestPlays.err = err
		return
	}
	arrestPlays.plays[encounter.SharingView], arrestPlays.plays[encounter.SharingOwn] = view, own
}

// playTheArrest is the warned gate (alice's troupe claim missed,
// rpg-project#942's walk), the cleric — seen on the road — gone off out of
// every guard's sight, bob joined on the road where no guard sees him, and
// alice's guest claim failed: the gate arrests. PLAYED ONCE, at shared view;
// each their own is the same record folded at that stop, read as a host
// catches up (its "played" is that read).
func playTheArrest() (view, own arrestPlay, err error) {
	ctx := context.Background()
	stores := warnedGate.stores
	stores.characters = map[string]*character.Data{"bob": withFreeReactions(armedFighter("bob"))}
	for id, d := range warnedGate.stores.characters {
		stores.characters[id] = d
	}
	c, err := newJailHost(stores, encounter.SharingView, 1)
	if err != nil {
		return view, own, err
	}
	if _, err := c.mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "bob", Position: onTheRoadUnseen}); err != nil {
		return view, own, fmt.Errorf("bob joins: %w", err)
	}
	// Gone off down the road, out of every guard's sight; they remember her
	// from where she watched.
	standIn(c, "cleric", downTheRoad)
	if view.played, err = capture(c, "alice's guest claim fails", func() error {
		_, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "guest"})
		return err
	}); err != nil {
		return view, own, fmt.Errorf("alice's guest claim: %w", err)
	}
	enc, err := c.mgr.LoadWorldForTest(ctx, "sess")
	if err != nil {
		return view, own, err
	}
	made := enc.Arrests()["front-gate"]
	view.taken, view.wanted = takenOf(made), wantedOf(made)
	own.taken, own.wanted = view.taken, view.wanted
	o, err := newJailHost(storesOf(c), encounter.SharingOwn, 1)
	if err != nil {
		return view, own, err
	}
	view.caught, own.caught = map[string][]session.Act{}, map[string][]session.Act{}
	own.played = beat{label: "alice's guest claim fails, caught up"}
	for _, member := range []string{"alice", "cleric", "bob"} {
		read, err := c.mgr.Acts(ctx, &session.ActsInput{Session: "sess", Member: member})
		if err != nil {
			return view, own, err
		}
		view.caught[member] = read.Acts
		if read, err = o.mgr.Acts(ctx, &session.ActsInput{Session: "sess", Member: member}); err != nil {
			return view, own, err
		}
		own.caught[member] = read.Acts
		own.played.acts = append(own.played.acts, read.Acts...)
	}
	_, own.walkErr = c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{backDownTheRoad[0]}})
	_, own.sneakErr = c.mgr.SetSneaking(ctx, &session.SetSneakingInput{Session: "sess", Member: "alice", Sneaking: true})
	view.walkErr, view.sneakErr = own.walkErr, own.sneakErr
	return view, own, nil
}

// theArrest is the arrest played at one stop, its preconditions checked.
func theArrest(t *testing.T, sharing encounter.Sharing) arrestPlay {
	t.Helper()
	arrestPlays.once.Do(buildTheArrestPlays)
	require.NoError(t, arrestPlays.err, "the arrest plays")
	p := arrestPlays.plays[sharing]
	require.Equal(t, []encounter.MemberID{"alice"}, p.taken, "precondition: alice is taken")
	require.Equal(t, []encounter.MemberID{"cleric"}, p.wanted, "precondition: the cleric is wanted")
	return p
}

// arrestActs is every ArrestAct member is sent, live and on catch-up, and
// the member's whole act stream as JSON, for the leak check.
func arrestActs(t *testing.T, played beat, caught []session.Act, member string) (live, read []found[session.ArrestAct], stream string) {
	t.Helper()
	all := append(append([]session.Act(nil), played.to(member)...), caught...)
	for _, a := range all {
		require.Empty(t, a.Undecoded, "%s: no undecoded act", member)
	}
	raw, err := json.Marshal(all)
	require.NoError(t, err)
	return where[session.ArrestAct](played.to(member), func(session.ArrestAct) bool { return true }),
		where[session.ArrestAct](caught, func(session.ArrestAct) bool { return true }), string(raw)
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
	p := theArrest(t, encounter.SharingOwn)
	live, caught, stream := arrestActs(t, p.played, p.caught["alice"], "alice")
	require.Len(t, live, 1, "alice is shown her arrest")
	require.Equal(t, session.ArrestAct{Interaction: "front-gate", Taken: []session.Party{aliceParty}, Cell: "region-cells"}, live[0].body)
	require.Equal(t, live[0].body, only[session.ArrestAct](t, caught2acts(caught)).body, "and reads it back on catch-up")
	require.Nil(t, live[0].act.Perceived, "her own")
	requireNoWanted(t, "alice", live, stream)
	for _, member := range []string{"cleric", "bob"} {
		live, caught, stream := arrestActs(t, p.played, p.caught[member], member)
		require.Empty(t, live, "%s is shown no arrest at each their own", member)
		require.Empty(t, caught, "%s, on catch-up", member)
		requireNoWanted(t, member, live, stream)
	}
}

// TestCastleTheArrestAtSharedView: the cleric and bob are shown alice's
// arrest through her, as she was shown it; still nobody's stream carries
// who is wanted.
func TestCastleTheArrestAtSharedView(t *testing.T) {
	p := theArrest(t, encounter.SharingView)
	for _, member := range []string{"cleric", "bob"} {
		live, caught, stream := arrestActs(t, p.played, p.caught[member], member)
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
	p := theArrest(t, encounter.SharingOwn)
	requireHeldYou(t, p.walkErr, "a walk")
	requireHeldYou(t, p.sneakErr, "a sneak")
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
