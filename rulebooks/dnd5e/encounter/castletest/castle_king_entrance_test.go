// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_entrance_test.go is THE ENTRANCE REMEMBERS (rpg-project#922,
// Gate S2 of #769) on the castle's own gate. The file's root `front-gate`
// interaction is one run-wide record every gate speaker reads and steps: a
// first failed claim is the captain's warning and closes that claim at the
// whole gate, for every hero, across a reload; the other claim's failure
// then arrests; either claim's success admits, and its `troupe-admitted`
// fact stays the gate post's pass until Gate S5 gives `admitted` its own
// consequences. What `arrested` does to posts and bodies is Gate S5's too, so
// these routes assert only the record and the menu, and that a failed claim
// no longer teaches `party-recognized`: the garrison stays civil.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// frontGate is the castle file's one interaction, by the id it gives it.
const frontGate = encounter.InteractionID("front-gate")

// The captain's warning lines, as the castle file writes them: a failed
// claim is a warning, not recognition.
const (
	guestWarning  = "Guests? What’s going on here? Nobody told me of guests."
	troupeWarning = "Troupe? There’s no troupe on my list. What’s going on here?"
)

// bard is a second player beside the hero, for the routes that prove the
// entrance is the run's and not the speaker's.
const bard = encounter.MemberID("bard")

// entrance is the front gate's state on the record, read off the persisted
// blob the way a host's save carries it.
func (s *kingScene) entrance() string {
	s.t.Helper()
	for _, in := range s.enc.ToData().Interactions {
		if in.ID == frontGate {
			return in.Blob.State
		}
	}
	require.Failf(s.t, "no front gate", "the castle's %q interaction is on the record", frontGate)

	return ""
}

// claimed says a claim to the captain and returns the line he answered with.
func (s *kingScene) claimed(option, skill string, beaten bool) string {
	s.t.Helper()
	out, err := s.speak(kingCaptain, option, skill, beaten)
	require.NoError(s.t, err, "the captain offers %q", option)
	require.NotNil(s.t, out.Answered, "the captain answers %q", option)

	return out.Answered.Answer.Say
}

// gateOffers is which of the two claims each gate speaker offers a member
// now, keyed by speaker.
func (s *kingScene) gateOffers(member encounter.MemberID, option string) map[encounter.MemberID]bool {
	s.t.Helper()
	out := map[encounter.MemberID]bool{}
	for _, speaker := range []encounter.MemberID{kingCaptain, kingGate1, kingGate2} {
		out[speaker] = s.offersTo(member, speaker, option)
	}

	return out
}

// everyGateSpeaker is the answer [kingScene.gateOffers] gives when all three
// gate speakers agree.
func everyGateSpeaker(offered bool) map[encounter.MemberID]bool {
	return map[encounter.MemberID]bool{kingCaptain: offered, kingGate1: offered, kingGate2: offered}
}

// toTheCaptainWithTheBard is the stranger's-word route with the bard beside
// the hero at the start: the hero hears of the feast and walks to the
// captain, and nobody has claimed anything at the gate yet.
func toTheCaptainWithTheBard(t *testing.T) *kingScene {
	t.Helper()
	start := partyStart(t)
	s := openTheCastleAs(t, castleHost{
		at:      start,
		players: []encounter.MemberInput{{ID: bard, Kind: encounter.KindPlayer, Position: spatial.Position{X: start.X + 2, Y: start.Y}}},
	})
	s.heardTheStranger(true)
	require.True(t, s.learners("feast-tonight")[hero], "the stranger told the hero")
	s.walkTo(s.at(kingCaptain))
	require.Equal(t, "fresh", s.entrance(), "precondition: nobody has claimed anything at the gate")
	require.Equal(t, everyGateSpeaker(true), s.gateOffers(hero, "guest"), "precondition: every gate speaker offers the guest claim")
	require.Equal(t, everyGateSpeaker(true), s.gateOffers(hero, "troupe"), "precondition: and, to a hero who knows the feast, the troupe claim")

	return s
}

// TestMadKingTheGateRemembersTheFailedClaim is checklist rows 1–4 of
// rpg-project#922 on the castle: the guest claim fails, the captain warns,
// and the claim is gone at the whole gate — from the captain and from gate-1,
// for the hero and for the bard — while the troupe claim remains; a reload
// keeps it so; and the troupe claim beaten admits the party, its fact the
// post's pass the troupe crosses the gate on.
func TestMadKingTheGateRemembersTheFailedClaim(t *testing.T) {
	t.Parallel()
	s := toTheCaptainWithTheBard(t)

	require.Equal(t, guestWarning, s.claimed("guest", "persuasion", false), "the captain warns the party")

	warned := func(when string) {
		t.Helper()
		require.Equal(t, "warned-guest", s.entrance(), "%s, the entrance is warned of guests", when)
		require.Equal(t, everyGateSpeaker(false), s.gateOffers(hero, "guest"), "%s, no gate speaker offers the hero the guest claim again", when)
		require.Equal(t, everyGateSpeaker(false), s.gateOffers(bard, "guest"), "%s, nor the bard: the entrance is the run's, not the speaker's", when)
		require.Equal(t, everyGateSpeaker(true), s.gateOffers(hero, "troupe"), "%s, the troupe claim remains", when)
		require.Empty(t, s.learners("party-recognized"), "%s, a warning recognizes nobody", when)
		require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty), "%s, the garrison is civil", when)
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, kingCaptain), "%s, and nobody is fighting", when)
	}
	warned("after the failed claim")

	s.reload()
	warned("after a reload")

	s.said(kingCaptain, "troupe", "performance", true)
	require.Equal(t, "admitted", s.entrance(), "the troupe claim admits the party")
	require.True(t, s.learners("troupe-admitted")[hero], "the hero holds the post's pass")
	require.Equal(t, everyGateSpeaker(false), s.gateOffers(hero, "troupe"), "an admitted entrance offers neither claim")

	// The pass still crosses the post: gate-1 stays posted beneath the gate
	// (rpg-project#857) and lets the troupe by on its fact.
	s.stepTo(besideTheCrossing)
	s.open(kingGateDoor)
	s.walkTo(castleCell(0, -6))
	courtyard, ok := s.enc.RegionAt(castleCell(0, -6))
	require.True(t, ok)
	here, ok := s.enc.RegionAt(s.at(hero))
	require.True(t, ok)
	require.Equal(t, courtyard, here, "the admitted hero walks past the post into the yard")
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "and into no fight")
}

// TestMadKingTwoFailedClaimsArrestTheEntrance is the second failure: the
// guest claim fails and warns, the troupe claim then fails too, and the
// entrance is arrested — neither claim is offered by any gate speaker — and
// still nobody learned `party-recognized`: what an arrest does to the
// garrison is Gate S5's, so the garrison stays civil here.
func TestMadKingTwoFailedClaimsArrestTheEntrance(t *testing.T) {
	t.Parallel()
	s := toTheCaptainWithTheBard(t)

	require.Equal(t, guestWarning, s.claimed("guest", "persuasion", false))
	require.Equal(t, "warned-guest", s.entrance())
	require.Equal(t, troupeWarning, s.claimed("troupe", "performance", false), "the second claim's line is the same warning")

	require.Equal(t, "arrested", s.entrance(), "the second failed claim arrests the entrance")
	for _, claim := range []string{"guest", "troupe"} {
		require.Equal(t, everyGateSpeaker(false), s.gateOffers(hero, claim), "an arrested entrance offers no %s claim", claim)
	}
	require.Empty(t, s.learners("party-recognized"), "neither failure taught party-recognized")
	require.Equal(t, encounter.StanceNeutral, s.stance(garrison, encounter.FactionParty), "the garrison stays civil")
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "and nobody is fighting")

	s.reload()
	require.Equal(t, "arrested", s.entrance(), "a reload keeps the arrest")
}
